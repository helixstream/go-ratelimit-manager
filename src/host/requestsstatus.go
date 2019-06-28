package host

import (
	"fmt"
	"github.com/mediocregopher/radix"
	"math/rand"
	"strconv"
	"time"
)

//requestsStatus struct contains all info pertaining to the cumulative requests made to a specific host
type RequestsStatus struct {
	requests        int //total number of completed requests made during the current sustained period
	pendingRequests int //number of requests that have started but have not completed
	firstRequest    int64
	lastErrorTime   int64
}

const (
	requests        = "requests"
	pendingRequests = "pendingRequests"
	firstRequest    = "firstRequest"
	lastErrorTime   = "lasterror"
)

//key convention redis: struct:host
//example: status:com.binance.api
//example: config:com.binance.api

//updateStatusFromDatabase gets the current request status information from the database and updates the struct
func (r *RequestsStatus) updateStatusFromDatabase(c radix.Conn, key string) error {
	var values []string
	err := c.Do(radix.Cmd(&values, "HVALS", key))
	if err != nil {
		fmt.Print(err)
		return err
	}

	if len(values) != 4 {
		return nil
	}

	requests, _ := strconv.Atoi(values[0])
	pending, _ := strconv.Atoi(values[1])
	first, _ := strconv.ParseInt(values[2], 10, 64)
	last, _ := strconv.ParseInt(values[3], 10, 64)

	*r = newRequestsStatus(requests, pending, first, last)
	return nil
}

//canMakeRequestLogic checks to see if a request can be made
//returns true, 0 if request can be made
//returns false and the number of milliseconds to wait if a request cannot be made
func (r *RequestsStatus) canMakeRequestLogic(requestWeight int, config RateLimitConfig) (bool, int64) {
	now := getUnixTimeMilliseconds()

	if r.isInPeriod(now, config) {

		if r.willHitLimit(requestWeight, config) {
			return false, r.timeUntilEndOfPeriod(now, config)
		}

		if r.hasEnoughTimePassed(now, config) {
			r.pendingRequests += requestWeight
			return true, 0
		}

		return false, r.timeUntilPeriodBetweenRequestsEnds(now, config)

	}
	//out of period, able to make request
	r.requests = 0
	r.firstRequest = now

	if r.hasEnoughTimePassed(now, config) {
		r.pendingRequests += requestWeight
		return true, 0
	}

	return false, r.timeUntilPeriodBetweenRequestsEnds(now, config)
}

//isInPeriod checks if the current request falls in the time frame of the period
func (r *RequestsStatus) isInPeriod(currentTime int64, hostConfig RateLimitConfig) bool {
	timeSincePeriodStart := currentTime - r.firstRequest
	//								converts seconds to milliseconds
	return timeSincePeriodStart < hostConfig.timePeriod*1000 && timeSincePeriodStart >= 0
}

//willHitLimit checks if the current request will hit the rate limit
//if the total number of requests plus the weight of the requested request is greater than the limit
//than the requested request should not occur because it would cause us to go over the limit
func (r *RequestsStatus) willHitLimit(requestWeight int, host RateLimitConfig) bool {
	totalRequests := r.requests + r.pendingRequests

	return totalRequests+requestWeight > host.requestLimit
}

//timeUntilEndOfPeriod calculates the time in milliseconds until the end of the period
func (r *RequestsStatus) timeUntilEndOfPeriod(currentTime int64, host RateLimitConfig) (millisecondsToWait int64) {
	// 											converts from seconds to milliseconds
	endOfPeriod := r.firstRequest + (host.timePeriod * 1000)

	return endOfPeriod - currentTime - rand.Int63n(11) - 15
}

//hasEnoughTimePassed determines if the time between the last request and the present is greater
//than the minimum time between requests
func (r *RequestsStatus) hasEnoughTimePassed(currentTime int64, config RateLimitConfig) bool {
	totalRequests := r.requests + r.pendingRequests
	nextRequestTime := (int64(totalRequests) * config.timeBetweenRequests) + r.firstRequest

	return currentTime-nextRequestTime >= 0
}

//timeUntilPeriodBetweenRequestsEnds calculates the time in milliseconds until enough time has passed between requests
//so that it will be greater than the minimum time between requests of the host config
func (r *RequestsStatus) timeUntilPeriodBetweenRequestsEnds(currentTime int64, config RateLimitConfig) int64 {
	totalRequests := r.requests + r.pendingRequests
	nextTime := (int64(totalRequests) * config.timeBetweenRequests) + r.firstRequest
	return nextTime - currentTime
}

//GetUnixTimeMilliseconds returns the current UTC time in milliseconds
func getUnixTimeMilliseconds() int64 {
	return time.Now().UTC().UnixNano() / int64(time.Millisecond)
}

func newRequestsStatus(requests int, pending int, firstRequest int64, lastErrorTime int64) RequestsStatus {
	return RequestsStatus{requests, pending, firstRequest, lastErrorTime}
}
