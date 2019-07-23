package limiter

import (
	"time"

	"github.com/mediocregopher/radix/v3"
)

//Limiter controls how often requests can be made. It uses a radix pool to connect
//to your redis database and the web api's RateLimitConfig to keep the number of
//allowed requests under the ratelimit.
//
//The request weight of any request is how much it counts against the ratelimit.
//If an api's ratelimit allows 10 requests per second and a specific type of request
//counts for two of the 10 allowed requests per second, the request weight is two.
//However, in most cases the request weight is one.
type Limiter struct {
	status RequestsStatus
	config RateLimitConfig
	pool   *radix.Pool
}

//NewLimiter returns a new Limiter and requires a radix pool to allow the limiter
//to connect to the redis database. It also requires a RateLimitConfig so it can
//throttle requests to stay under the ratelimit while allowing as many requests as possible.
func NewLimiter(config RateLimitConfig, pool *radix.Pool) (Limiter, error) {
	limiter := Limiter{
		newRequestsStatus(0, 0, 0, 0),
		config,
		pool,
	}

	statusKey := limiter.getStatusKey()
	configKey := limiter.getConfigKey()

	err := pool.Do(radix.WithConn(statusKey, func(c radix.Conn) error {
		//must be done before the multi call
		doesStatusExist, err := limiter.doesHashKeyExist(c, statusKey)
		if err != nil {
			return err
		}

		doesConfigExist, err := limiter.doesHashKeyExist(c, statusKey)
		if err != nil {
			return err
		}

		if err := c.Do(radix.Cmd(nil, "MULTI")); err != nil {
			return err
		}

		defer func() {
			if err != nil {
				//err doesn't matter. any error is a network err, so client will close conn.
				c.Do(radix.Cmd(nil, "DISCARD"))
			}
		}()

		if !doesStatusExist {
			//if the status key does not exist save the current status to the database
			err = c.Do(radix.FlatCmd(nil, "HSET",
				statusKey,
				requests, 0,
				pendingRequests, 0,
				firstRequest, 0,
				lastErrorTime, 1, //set to one so a new limiter will update config on first CanMakeRequest
			))

			if err != nil {
				return err
			}
		}

		if !doesConfigExist {
			//if the config key does not exist, save the current config to the database
			err = c.Do(radix.FlatCmd(nil, "HSET",
				configKey,
				limit, config.requestLimit,
				timePeriod, config.timePeriod,
				timeBetweenRequests, config.timeBetweenRequests,
			))

			if err != nil {
				return err
			}
		}

		if err = c.Do(radix.Cmd(nil, "EXEC")); err != nil {
			return err
		}

		return nil
	}))

	if err != nil {
		return Limiter{}, err
	}

	return limiter, nil
}

//RequestSuccessful must be called only after CanMakeRequest returned true and
//when a request has been completed and returned without a 429 or 419 status code
func (l *Limiter) RequestSuccessful(requestWeight int) error {
	key := l.getStatusKey()

	err := l.pool.Do(radix.WithConn(key, func(c radix.Conn) error {
		if err := c.Do(radix.Cmd(nil, "MULTI")); err != nil {
			return err
		}
		// If any of the calls after the MULTI call error it's important that
		// the transaction is discarded. This isn't strictly necessary if the
		// error was a network error, as the connection would be closed by the
		// client anyway, but it's important otherwise.
		var err error
		defer func() {
			if err != nil {
				c.Do(radix.Cmd(nil, "DISCARD"))
			}
		}()

		if err = l.requestFinished(requestWeight, c, key); err != nil {
			return err
		}

		if err := c.Do(radix.Cmd(nil, "EXEC")); err != nil {
			return err
		}

		return nil
	}))

	if err != nil {
		return err
	}

	return nil
}

//HitRateLimit must be called only after CanMakeRequest returned true and a request
//has been completed with a status code of 429 or 419. This will automatically adjust
//the RateLimitConfig in the Limiter struct to prevent more 429s in the future.

func (l *Limiter) HitRateLimit(requestWeight int, wait ...int64) error {

	statusKey := l.getStatusKey()

	err := l.pool.Do(radix.WithConn(statusKey, func(c radix.Conn) error {
		if err := c.Do(radix.Cmd(nil, "MULTI")); err != nil {
			return err
		}
		// If any of the calls after the MULTI call error it's important that
		// the transaction is discarded. This isn't strictly necessary if the
		// error was a network error, as the connection would be closed by the
		// client anyway, but it's important otherwise.
		var err error
		defer func() {
			if err != nil {
				c.Do(radix.Cmd(nil, "DISCARD"))
			}
		}()

		if err = l.requestFinished(requestWeight, c, statusKey); err != nil {
			return err
		}

		if len(wait) == 0 {
			wait = append(wait, 0)
		}

		if err = l.adjustConfig(requestWeight, wait[0], c); err != nil {
			return err
		}

		if err := c.Do(radix.Cmd(nil, "EXEC")); err != nil {
			return err
		}

		return nil
	}))

	if err != nil {
		return err
	}

	return nil
}

//requestFinished updates the RequestsStatus struct by removing a pending request into the
//sustained and burst categories should be called directly after the request has finished
func (l *Limiter) requestFinished(requestWeight int, c radix.Conn, key string) error {
	if err := c.Do(radix.FlatCmd(nil, "HINCRBY", key, requests, requestWeight)); err != nil {
		return err
	}

	if err := c.Do(radix.FlatCmd(nil, "HINCRBY", key, pendingRequests, -requestWeight)); err != nil {
		return err
	}

	return nil
}

//RequestCancelled must be called if CanMakeRequest returned true, but the request
//to the api was never actually made.
func (l *Limiter) RequestCancelled(requestWeight int) error {
	key := l.getStatusKey()
	//this is radix's way of doing a transaction
	err := l.pool.Do(radix.WithConn(key, func(c radix.Conn) error {

		if err := c.Do(radix.Cmd(nil, "MULTI")); err != nil {
			return err
		}
		// If any of the calls after the MULTI call error it's important that
		// the transaction is discarded. This isn't strictly necessary if the
		// error was a network error, as the connection would be closed by the
		// client anyway, but it's important otherwise.
		var err error
		defer func() {
			if err != nil {
				c.Do(radix.Cmd(nil, "DISCARD"))
			}
		}()

		if err = c.Do(radix.FlatCmd(nil, "HINCRBY", key, pendingRequests, -requestWeight)); err != nil {
			return err
		}

		if err = c.Do(radix.Cmd(nil, "EXEC")); err != nil {
			return err
		}

		return nil
	}))

	if err != nil {
		return err
	}

	return nil
}

//CanMakeRequest communicates with the database to figure out when it is possible to
//make a request. If a request can be made it returns true, 0. If a request cannot be made
//it returns false and the amount of time to sleep before your program should call CanMakeRequest again
func (l *Limiter) CanMakeRequest(requestWeight int) (bool, int64) {
	statusKey := l.getStatusKey()
	configKey := l.getConfigKey()
	var canMake bool
	var wait int64
	var resp []string

	err := l.pool.Do(radix.WithConn(statusKey, func(c radix.Conn) error {
		if err := c.Do(radix.Cmd(nil, "WATCH", statusKey, configKey)); err != nil {
			return err
		}

		lastError := l.status.lastErrorTime

		if err := l.status.updateStatusFromDatabase(c, statusKey); err != nil {
			return err
		}
		//only updates config from database when the lastErrorTime changes
		if lastError != l.status.lastErrorTime {
			if err := l.config.updateConfigFromDatabase(c, configKey); err != nil {
				return err
			}
		}

		canMake, wait = l.status.canMakeRequestLogic(requestWeight, l.config)

		if !canMake {
			err := c.Do(radix.Cmd(nil, "UNWATCH"))
			if err != nil {
				return err
			}
			return nil
		}

		if err := c.Do(radix.Cmd(nil, "MULTI")); err != nil {
			return err
		}

		// If any of the calls after the MULTI call error it's important that
		// the transaction is discarded. This isn't strictly necessary if the
		// error was a network error, as the connection would be closed by the
		// client anyway, but it's important otherwise.
		var err error
		defer func() {
			if err != nil {
				c.Do(radix.Cmd(nil, "DISCARD"))
			}
		}()

		err = c.Do(radix.FlatCmd(nil, "HSET",
			statusKey,
			requests, l.status.requests,
			pendingRequests, l.status.pendingRequests,
			firstRequest, l.status.firstRequest,
			lastErrorTime, l.status.lastErrorTime,
		))
		if err != nil {
			return err
		}

		if err := c.Do(radix.Cmd(&resp, "EXEC")); err != nil {
			return err
		}

		return nil
	}))
	if err != nil {
		return false, wait
	}
	//resp is the response to the EXEC command
	//if resp is nil the transaction was aborted
	if resp == nil {
		if wait == 0 {
			return l.CanMakeRequest(requestWeight)
		}
		return false, wait
	}

	return canMake, wait
}

//WaitForRatelimit recursively calls CanMakeRequest until a request can be made.
//It handles the sleeping when a request cannot be made and it blocks until
//a request can be made.
func (l *Limiter) WaitForRatelimit(requestWeight int) {
	canMake, sleepTime := l.CanMakeRequest(requestWeight)
	if canMake {
		return
	}
	time.Sleep(time.Duration(sleepTime) * time.Millisecond)
	l.WaitForRatelimit(requestWeight)
}

//adjustConfig reduces the number of allowed requests per time period by one and saves
//the new config to the database updates the lastErrorTime to the current time

func (l *Limiter) adjustConfig(requestWeight int, wait int64, c radix.Conn) error {
	if l.config.requestLimit-requestWeight > 0 {
		l.config.requestLimit -= requestWeight
		l.config.setTimeBetweenRequests()
	}

	configKey := l.getConfigKey()
	statusKey := l.getStatusKey()

	err := c.Do(radix.FlatCmd(nil, "HSET", configKey,
		limit, l.config.requestLimit,
		timePeriod, l.config.timePeriod,
		timeBetweenRequests, l.config.timeBetweenRequests,
	))

	if err != nil {
		return err
	}

	err = c.Do(radix.FlatCmd(nil, "HSET", statusKey,
		lastErrorTime, getUnixTimeMilliseconds()+wait+l.config.timePeriod*1000,
	))

	if err != nil {
		return err
	}

	return nil
}

//GetStatus returns the status of the requests, which includes the number on requests
//made in the period, the number of pending requests, the timestamp of the beginning
//of the period, and the timestamp for when the last error occurred.
func (l *Limiter) GetStatus() RequestsStatus {
	return l.status
}

func (l *Limiter) getStatusKey() string {
	return "status:" + l.config.host
}

func (l *Limiter) getConfigKey() string {
	return "config:" + l.config.host
}

func (l *Limiter) doesHashKeyExist(c radix.Conn, key string) (bool, error) {
	var length int
	if err := c.Do(radix.Cmd(&length, "HLEN", key)); err != nil {
		return false, err
	}

	if length == 0 {
		return false, nil
	}

	return true, nil
}
