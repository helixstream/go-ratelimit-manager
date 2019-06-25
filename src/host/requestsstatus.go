package host

import (
	"fmt"
	"github.com/mediocregopher/radix"
	"strconv"
	"time"
)

//requestsStatus struct contains all info pertaining to the cumulative requests made to a specific host
type requestsStatus struct {
	sustainedRequests     int   //total number of completed requests made during the current sustained period
	burstRequests         int   //total number of completed requests made during the current burst period
	pendingRequests       int   //number of requests that have started but have not completed
	firstSustainedRequest int64 //timestamp in milliseconds that represents when the sustained period began
	firstBurstRequest     int64 //timestamp in milliseconds that represents when the burst period began
}

const (
	sustainedRequests     = "sustainedRequests"
	burstRequests         = "burstRequests"
	pendingRequests       = "pendingRequests"
	firstSustainedRequest = "firstSustainedRequest"
	firstBurstRequest     = "firstBurstRequest"
)

//key convention redis: struct:host
//example: status:com.binance.api
//example: config:com.binance.api

//updateStatusFromDatabase gets the current request status information from the database and updates the struct
func (r *requestsStatus) updateStatusFromDatabase(c radix.Conn, key string) error {
	var values []string
	err := c.Do(radix.Cmd(&values, "HVALS", key))
	if err != nil {
		fmt.Print(err)
		return err
	}

	if len(values) != 5 {
		*r = newRequestsStatus(0, 0, 0, 0, 0)
		return nil
	}

	sus, _ := strconv.Atoi(values[0])
	burst, _ := strconv.Atoi(values[1])
	pending, _ := strconv.Atoi(values[2])
	firstSus, _ := strconv.ParseInt(values[3], 10, 64)
	firstBurst, _ := strconv.ParseInt(values[4], 10, 64)

	*r = newRequestsStatus(sus, burst, pending, firstSus, firstBurst)
	return nil
}

//canMakeRequestLogic checks to see if a request can be made
//returns true, 0 if request can be made
//returns false and the number of milliseconds to wait if a request cannot be made
func (r *requestsStatus) canMakeRequestLogic(requestWeight int, config RateLimitConfig) (bool, int64) {
	now := getUnixTimeMilliseconds()
	//if request is in the current burst period
	if r.isInBurstPeriod(now, config) {
		//will the request push us over the burst limit
		if r.willHitBurstLimit(requestWeight, config) {
			//is so do not make the request and wait
			return false, r.timeUntilEndOfBurst(now, config)
		}

		//determines if the request will go over the sustained limit
		if r.willHitSustainedLimit(requestWeight, config) {
			//is so do not make the request and wait
			return false, r.timeUntilEndOfSustained(now, config)
		}

		//did not hit either burst or sustained limit
		//so can make request and increments pending requests
		r.incrementPendingRequests(requestWeight)
		return true, 0

		//not in burst period, but in sustained period
	}

	if r.isInSustainedPeriod(now, config) {
		//reset burst to 0 and sets start of new burst period to now
		r.setBurstRequests(0)
		r.setFirstBurstRequest(now)

		if r.willHitSustainedLimit(requestWeight, config) {
			return false, r.timeUntilEndOfSustained(now, config)
		}

		//can make request because did not hit sustained limit
		r.incrementPendingRequests(requestWeight)
		return true, 0

	}
	//out of burst and sustained, able to make request

	//reset number of sustained and burst in new time period
	r.setSustainedRequests(0)
	r.setBurstRequests(0)
	//set start of both sustained and burst to now
	r.setFirstSustainedRequest(now)
	r.setFirstBurstRequest(now)
	//increment the number of pending requests by the weight of the request
	r.incrementPendingRequests(requestWeight)

	return true, 0

}

//isInSustainedPeriod checks if the current request is in the sustained period
func (r *requestsStatus) isInSustainedPeriod(currentTime int64, hostConfig RateLimitConfig) bool {
	timeSincePeriodStart := currentTime - r.firstSustainedRequest
	//								converts seconds to milliseconds
	return timeSincePeriodStart < hostConfig.sustainedTimePeriod*1000 && timeSincePeriodStart >= 0
}

//isInBurstPeriod checks if the current request is in the burst period
func (r *requestsStatus) isInBurstPeriod(currentTime int64, hostConfig RateLimitConfig) bool {
	timeSincePeriodStart := currentTime - r.firstBurstRequest
	//								converts seconds to milliseconds
	return timeSincePeriodStart < hostConfig.burstTimePeriod*1000 && timeSincePeriodStart >= 0
}

//willHitSustainedLimit checks if the current request will hit the sustained rate limit
//if the total number of requests plus the weight of the requested request is greater than the limit
//than the requested request should not occur because it would cause us to go over the limit
func (r *requestsStatus) willHitSustainedLimit(requestWeight int, host RateLimitConfig) bool {
	totalRequests := r.sustainedRequests + r.pendingRequests

	return totalRequests+requestWeight > host.sustainedRequestLimit
}

//willHitBurstLimit checks if the current request will hit the burst rate limit
//if the total number of requests plus the weight of the requested request is greater than the limit
//than the requested request should not occur because it would cause us to go over the limit
func (r *requestsStatus) willHitBurstLimit(requestWeight int, host RateLimitConfig) bool {
	totalRequests := r.burstRequests + r.pendingRequests

	return totalRequests+requestWeight > host.burstRequestLimit
}

//timeUntilEndOfSustained calculates the time in milliseconds until the end of the sustained period
func (r *requestsStatus) timeUntilEndOfSustained(currentTime int64, host RateLimitConfig) (millisecondsToWait int64) {
	// 											converts from seconds to milliseconds
	endOfPeriod := r.firstSustainedRequest + (host.sustainedTimePeriod * 1000)

	return endOfPeriod - currentTime
}

//timeUntilEndOfBurst calculates the time in milliseconds until the end of the burst period
func (r *requestsStatus) timeUntilEndOfBurst(currentTime int64, host RateLimitConfig) (millisecondsToWait int64) {
	//  								converts from seconds to milliseconds
	endOfPeriod := r.firstBurstRequest + (host.burstTimePeriod * 1000)

	return endOfPeriod - currentTime
}

//GetUnixTimeMilliseconds returns the current UTC time in milliseconds
func getUnixTimeMilliseconds() int64 {
	return time.Now().UTC().UnixNano() / int64(time.Millisecond)
}

func newRequestsStatus(sustainedRequests int, burstRequests int, pending int, firstSustainedRequests int64, firstBurstRequest int64) requestsStatus {
	return requestsStatus{sustainedRequests, burstRequests, pending, firstSustainedRequests, firstBurstRequest}
}

func (r *requestsStatus) incrementSustainedRequests(increment int) {
	r.sustainedRequests += increment
}

func (r *requestsStatus) setSustainedRequests(value int) {
	r.sustainedRequests = value
}

func (r *requestsStatus) incrementBurstRequests(increment int) {
	r.burstRequests += increment
}

func (r *requestsStatus) setBurstRequests(value int) {
	r.burstRequests = value
}

func (r *requestsStatus) incrementPendingRequests(increment int) {
	r.pendingRequests += increment
}

func (r *requestsStatus) decrementPendingRequests(increment int) {
	r.pendingRequests -= increment
}

func (r *requestsStatus) setFirstSustainedRequest(value int64) {
	r.firstSustainedRequest = value
}

func (r *requestsStatus) setFirstBurstRequest(value int64) {
	r.firstBurstRequest = value
}
