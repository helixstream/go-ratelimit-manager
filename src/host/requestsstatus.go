package host

import (
	"fmt"
	"github.com/mediocregopher/radix"
	"strconv"
	"time"
)

//RequestsStatus struct contains all info pertaining to the cumulative requests made to a specific host
type RequestsStatus struct {
	Host                  string
	SustainedRequests     int   //total number of completed requests made during the current sustained period
	BurstRequests         int   //total number of completed requests made during the current burst period
	PendingRequests       int   //number of requests that have started but have not completed
	FirstSustainedRequest int64 //timestamp in milliseconds that represents when the sustained period began
	FirstBurstRequest     int64 //timestamp in milliseconds that represents when the burst period began
}

const (
	host                  = "host"
	sustainedRequests     = "sustainedRequests"
	burstRequests         = "burstRequests"
	pendingRequests       = "pendingRequests"
	firstSustainedRequest = "firstSustainedRequest"
	firstBurstRequest     = "firstBurstRequest"
)

//key convention redis: struct:host
//example: status:com.binance.api
//example: config:com.binance.api

//isConnectedToRedis pings the redis database and returns
//whether the ping was successful
func isConnectedToRedis(p *radix.Pool) bool {
	var resp string
	err := p.Do(radix.Cmd(&resp, "PING"))
	if err != nil {
		return false
	}
	return true
}

//RequestFinished updates the RequestsStatus struct by removing a pending request into the sustained and burst categories
//should be called directly after the request has finished
func (h *RequestsStatus) RequestFinished(requestWeight int, p *radix.Pool) error {
	key := "status:" + h.Host
	//this is radix's way of doing a transaction
	err := p.Do(radix.WithConn(key, func(c radix.Conn) error {
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

		if err = c.Do(radix.FlatCmd(nil, "HINCRBY", key, sustainedRequests, requestWeight)); err != nil {
			return err
		}

		if err = c.Do(radix.FlatCmd(nil, "HINCRBY", key, burstRequests, requestWeight)); err != nil {
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
func (h *RequestsStatus) RequestCancelled(requestWeight int, p *radix.Pool) error {
	key := "status:" + h.Host
	//this is radix's way of doing a transaction
	err := p.Do(radix.WithConn(key, func(c radix.Conn) error {

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
func (h *RequestsStatus) CanMakeRequest(p *radix.Pool, requestWeight int, config RateLimitConfig) (bool, int64) {
	key := "status:" + h.Host
	var canMake bool
	var wait int64
	var resp []string
	//var respWatch []string

	err := p.Do(radix.WithConn(key, func(c radix.Conn) error {
		if err := c.Do(radix.Cmd(nil, "WATCH", key)); err != nil {
			return err
		}

		if err := h.updateStatusFromDatabase(c, key); err != nil {
			return err
		}

		canMake, wait = h.canMakeRequestLogic(requestWeight, config)

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
			host, h.Host,
			sustainedRequests, h.SustainedRequests,
			burstRequests, h.BurstRequests,
			pendingRequests, h.PendingRequests,
			firstSustainedRequest, h.FirstSustainedRequest,
			firstBurstRequest, h.FirstBurstRequest,
		))
		if err != nil {
			return err
		}

		if err = c.Do(radix.Cmd(&resp, "EXEC")); err != nil {
			return err
		}

		return nil
	}))
	if err != nil {
		fmt.Printf("Error: %v. ", err)
		return false, 500
	}
	//resp is the response to the EXEC command
	//if resp is nil the transaction was aborted
	if resp == nil {
		return false, 0
	}
	return canMake, wait
}

//updateStatusFromDatabase gets the current request status information from the database and updates the struct
func (h *RequestsStatus) updateStatusFromDatabase(c radix.Conn, key string) error {
	var values []string
	err := c.Do(radix.Cmd(&values, "HVALS", key))
	if err != nil {
		fmt.Print(err)
		return err
	}

	if len(values) != 6 {
		*h = NewRequestsStatus(h.Host, 0, 0, 0, 0, 0)
		return nil
	}

	host := values[0]
	sus, _ := strconv.Atoi(values[1])
	burst, _ := strconv.Atoi(values[2])
	pending, _ := strconv.Atoi(values[3])
	firstSus, _ := strconv.ParseInt(values[4], 10, 64)
	firstBurst, _ := strconv.ParseInt(values[5], 10, 64)

	*h = NewRequestsStatus(host, sus, burst, pending, firstSus, firstBurst)
	return nil
}

//canMakeRequestLogic checks to see if a request can be made
//returns true, 0 if request can be made
//returns false and the number of milliseconds to wait if a request cannot be made
func (h *RequestsStatus) canMakeRequestLogic(requestWeight int, config RateLimitConfig) (bool, int64) {
	now := GetUnixTimeMilliseconds()
	//if request is in the current burst period
	if h.isInBurstPeriod(now, config) {
		//will the request push us over the burst limit
		if h.willHitBurstLimit(requestWeight, config) {
			//is so do not make the request and wait
			return false, h.timeUntilEndOfBurst(now, config)
		}

		//determines if the request will go over the sustained limit
		if h.willHitSustainedLimit(requestWeight, config) {
			//is so do not make the request and wait
			return false, h.timeUntilEndOfSustained(now, config)
		}

		//did not hit either burst or sustained limit
		//so can make request and increments pending requests
		h.incrementPendingRequests(requestWeight)
		return true, 0

		//not in burst period, but in sustained period
	}

	if h.isInSustainedPeriod(now, config) {
		//reset burst to 0 and sets start of new burst period to now
		h.setBurstRequests(0)
		h.setFirstBurstRequest(now)

		if h.willHitSustainedLimit(requestWeight, config) {
			return false, h.timeUntilEndOfSustained(now, config)
		}

		//can make request because did not hit sustained limit
		h.incrementPendingRequests(requestWeight)
		return true, 0

	}
	//out of burst and sustained, able to make request

	//reset number of sustained and burst in new time period
	h.setSustainedRequests(0)
	h.setBurstRequests(0)
	//set start of both sustained and burst to now
	h.setFirstSustainedRequest(now)
	h.setFirstBurstRequest(now)
	//increment the number of pending requests by the weight of the request
	h.incrementPendingRequests(requestWeight)

	return true, 0

}

//isInSustainedPeriod checks if the current request is in the sustained period
func (h *RequestsStatus) isInSustainedPeriod(currentTime int64, hostConfig RateLimitConfig) bool {
	timeSincePeriodStart := currentTime - h.getFirstSustainedRequest()
	//								converts seconds to milliseconds
	return timeSincePeriodStart < hostConfig.SustainedTimePeriod*1000 && timeSincePeriodStart >= 0
}

//isInBurstPeriod checks if the current request is in the burst period
func (h *RequestsStatus) isInBurstPeriod(currentTime int64, hostConfig RateLimitConfig) bool {
	timeSincePeriodStart := currentTime - h.getFirstBurstRequest()
	//								converts seconds to milliseconds
	return timeSincePeriodStart < hostConfig.BurstTimePeriod*1000 && timeSincePeriodStart >= 0
}

//willHitSustainedLimit checks if the current request will hit the sustained rate limit
//if the total number of requests plus the weight of the requested request is greater than the limit
//than the requested request should not occur because it would cause us to go over the limit
func (h *RequestsStatus) willHitSustainedLimit(requestWeight int, host RateLimitConfig) bool {
	totalRequests := h.getSustainedRequests() + h.getPendingRequests()

	return totalRequests+requestWeight > host.SustainedRequestLimit
}

//willHitBurstLimit checks if the current request will hit the burst rate limit
//if the total number of requests plus the weight of the requested request is greater than the limit
//than the requested request should not occur because it would cause us to go over the limit
func (h *RequestsStatus) willHitBurstLimit(requestWeight int, host RateLimitConfig) bool {
	totalRequests := h.getBurstRequests() + h.getPendingRequests()

	return totalRequests+requestWeight > host.BurstRequestLimit
}

//timeUntilEndOfSustained calculates the time in milliseconds until the end of the sustained period
func (h *RequestsStatus) timeUntilEndOfSustained(currentTime int64, host RateLimitConfig) (millisecondsToWait int64) {
	// 											converts from seconds to milliseconds
	endOfPeriod := h.getFirstSustainedRequest() + (host.SustainedTimePeriod * 1000)

	return endOfPeriod - currentTime
}

//timeUntilEndOfBurst calculates the time in milliseconds until the end of the burst period
func (h *RequestsStatus) timeUntilEndOfBurst(currentTime int64, host RateLimitConfig) (millisecondsToWait int64) {
	//  								converts from seconds to milliseconds
	endOfPeriod := h.getFirstBurstRequest() + (host.BurstTimePeriod * 1000)

	return endOfPeriod - currentTime
}

//GetUnixTimeMilliseconds returns the current UTC time in milliseconds
func GetUnixTimeMilliseconds() int64 {
	return time.Now().UTC().UnixNano() / int64(time.Millisecond)
}

func NewRequestsStatus(host string, sustainedRequests int, burstRequests int, pending int, firstSustainedRequests int64, firstBurstRequest int64) RequestsStatus {
	return RequestsStatus{host, sustainedRequests, burstRequests, pending, firstSustainedRequests, firstBurstRequest}
}

func (h *RequestsStatus) incrementSustainedRequests(increment int) {
	h.SustainedRequests += increment
}

func (h *RequestsStatus) setSustainedRequests(value int) {
	h.SustainedRequests = value
}

func (h *RequestsStatus) incrementBurstRequests(increment int) {
	h.BurstRequests += increment
}

func (h *RequestsStatus) setBurstRequests(value int) {
	h.BurstRequests = value
}

func (h *RequestsStatus) incrementPendingRequests(increment int) {
	h.PendingRequests += increment
}

func (h *RequestsStatus) decrementPendingRequests(increment int) {
	h.PendingRequests -= increment
}

func (h *RequestsStatus) setFirstSustainedRequest(value int64) {
	h.FirstSustainedRequest = value
}

func (h *RequestsStatus) setFirstBurstRequest(value int64) {
	h.FirstBurstRequest = value
}

func (h *RequestsStatus) getSustainedRequests() int {
	return (*h).SustainedRequests
}

func (h *RequestsStatus) getBurstRequests() int {
	return h.BurstRequests
}

func (h *RequestsStatus) getPendingRequests() int {
	return h.PendingRequests
}

func (h *RequestsStatus) getFirstSustainedRequest() int64 {
	return h.FirstSustainedRequest
}

func (h *RequestsStatus) getFirstBurstRequest() int64 {
	return h.FirstBurstRequest
}
