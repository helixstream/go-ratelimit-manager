package host

import (
	"fmt"
	"github.com/mediocregopher/radix"
)

//Limiter counts the number of requests made and keeps the number of
//requests under the sustained and burst limits specified by the user
type Limiter struct {
	status RequestsStatus
	config RateLimitConfig
	pool   *radix.Pool
}

func NewLimiter(config RateLimitConfig, pool *radix.Pool) (Limiter, error) {
	limiter := Limiter{
		newRequestsStatus(0, 0, 0),
		config,
		pool,
	}

	key := limiter.getStatusKey()

	err := pool.Do(radix.WithConn(key, func(c radix.Conn) error {
		if err := c.Do(radix.Cmd(nil, "MULTI")); err != nil {
			return err
		}

		var err error
		defer func() {
			if err != nil {
				// The return from DISCARD doesn't matter. If it's an error then
				// it's a network error and the Conn will be closed by the
				// client.
				c.Do(radix.Cmd(nil, "DISCARD"))
			}
		}()

		err = c.Do(radix.FlatCmd(nil, "HSET",
			limiter.getStatusKey(),
			requests, 0,
			pendingRequests, 0,
			firstRequest, 0,
		))

		if err != nil {
			return err
		}

		err = c.Do(radix.FlatCmd(nil, "HSET",
			limiter.getConfigKey(),
			limit, config.requestLimit,
			timePeriod, config.timePeriod,
			timeBetweenRequests, config.timeBetweenRequests,
		))

		if err != nil {
			return err
		}

		if err = c.Do(radix.Cmd(nil, "EXEC")); err != nil {
			return err
		}

		return nil
	}))

	if err != nil {
		return Limiter{}, nil
	}

	return limiter, nil
}

//RequestFinished updates the RequestsStatus struct by removing a pending request into the sustained and burst categories
//should be called directly after the request has finished
func (l *Limiter) RequestFinished(requestWeight int) error {
	key := l.getStatusKey()
	//this is radix's way of doing a transaction
	err := l.pool.Do(radix.WithConn(key, func(c radix.Conn) error {
		//start of transaction
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
				// The return from DISCARD doesn't matter. If it's an error then
				// it's a network error and the Conn will be closed by the
				// client.
				c.Do(radix.Cmd(nil, "DISCARD"))
			}
		}()

		if err = c.Do(radix.FlatCmd(nil, "HINCRBY", key, requests, requestWeight)); err != nil {
			return err
		}

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

//RequestCancelled updates the RequestStatus struct by removing a pending request as the request did not complete
//and so does not could against the rate limit. Should be called directly after the request was cancelled/failed
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
				// The return from DISCARD doesn't matter. If it's an error then
				// it's a network error and the Conn will be closed by the
				// client.
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

//CanMakeRequest communicates with the database to figure out when it is possible to make a request
//returns true, 0 if a request can be made, and false and the amount of time to sleep when a request cannot be made
func (l *Limiter) CanMakeRequest(requestWeight int) (bool, int64) {
	key := l.getStatusKey()
	var canMake bool
	var wait int64
	var resp []string

	err := l.pool.Do(radix.WithConn(key, func(c radix.Conn) error {
		if err := c.Do(radix.Cmd(nil, "WATCH", key, l.getConfigKey())); err != nil {
			return err
		}

		if err := l.status.updateStatusFromDatabase(c, key); err != nil {
			return err
		}

		if err := l.config.updateConfigFromDatabase(c, l.getConfigKey()); err != nil {
			return err
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
				// The return from DISCARD doesn't matter. If it's an error then
				// it's a network error and the Conn will be closed by the
				// client.
				c.Do(radix.Cmd(nil, "DISCARD"))
			}
		}()

		err = c.Do(radix.FlatCmd(nil, "HSET",
			key,
			requests, l.status.requests,
			pendingRequests, l.status.pendingRequests,
			firstRequest, l.status.firstRequest,
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
		fmt.Printf("Error: %v. ", err)
		return false, wait
	}
	//resp is the response to the EXEC command
	//if resp is nil the transaction was aborted
	if resp == nil {
		return false, 0
	}

	return canMake, wait
}

func (l *Limiter) AdjustConfig(requestWeight int) error{
	return nil
}

func (l *Limiter) GetStatus() RequestsStatus {
	return l.status
}

func (l *Limiter) getStatusKey() string {
	return "status:" + l.config.host
}

func (l *Limiter) getConfigKey() string {
	return "config:" + l.config.host
}
