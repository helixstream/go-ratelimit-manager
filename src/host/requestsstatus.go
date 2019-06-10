package host

import (
	"time"
)

/*
RequestsStatus struct contains all info pertaining to the cumulative requests made to a specific host
SustainedRequests -> total number of completed requests made during the current sustained period
BurstRequests -> total number of completed requests made during the current burst period
PendingRequests -> number of requests that have started but have not completed
FirstSustainedRequest -> timestamp that represents when the sustained period began
FirstBurstRequest -> timestamp that represents when the burst period began
*/
type RequestsStatus struct {
	Host                  string
	SustainedRequests     int
	BurstRequests         int
	PendingRequests       int
	FirstSustainedRequest int64
	FirstBurstRequest     int64
}

//recursively calls CanMakeRequest until a request can be  made
//returns true when a request can be made
//if a request cannot be made it waits the correct amount of time and check again to see if a request can be made
func (h *RequestsStatus) CheckRequest(requestWeight int, host RateLimitConfig) bool {
	canMake, wait := h.CanMakeRequest(requestWeight, time.Now().UTC().Unix(), host)

	if wait != 0 {
		//sleeps out of burst or sustained period where limit has been reached
		time.Sleep(time.Duration(wait) * time.Millisecond)

		canMake, _ = h.CanMakeRequest(requestWeight, time.Now().UTC().Unix(), host)
		return canMake
	}

	return true
}

//CanMakeRequest checks to see if a request can be made
//returns true, 0 if request can be made
//returns false and the number of milliseconds to wait if a request cannot be made
func (h *RequestsStatus) CanMakeRequest(requestWeight int, now int64, host RateLimitConfig) (bool, int64) {

	//if request is in the current burst period
	if h.isInBurstPeriod(host, now) {
		//will the request push us over the burst limit
		if h.willHitBurstLimit(requestWeight, host) {
			//is so do not make the request and wait
			return false, h.timeUntilEndOfBurst(host)
		}

		//determines if the request will go over the sustained limit
		if h.willHitSustainedLimit(requestWeight, host) {
			//is so do not make the request and wait
			return false, h.timeUntilEndOfSustained(host)
		}

		//did not hit either burst or sustained limit
		//so can make request and increments pending requests
		h.IncrementPendingRequests(requestWeight)
		return true, 0

		//not in burst period, but in sustained period
	}

	if h.isInSustainedPeriod(host, now) {

		//reset burst to 0 and sets start of new burst period to now
		h.SetBurstRequests(0)
		h.SetFirstBurstRequest(now)

		if h.willHitSustainedLimit(requestWeight, host) {
			return false, h.timeUntilEndOfSustained(host)
		}

		//can make request because did not hit sustained limit
		h.IncrementPendingRequests(requestWeight)
		return true, 0

	}

	//out of burst and sustained, able to make request

	//reset number of sustained and burst in new time period
	h.SetSustainedRequests(0)
	h.SetBurstRequests(0)
	//set start of both sustained and burst to now
	h.SetFirstSustainedRequest(now)
	h.SetFirstBurstRequest(now)
	//increment the number of pending requests by the weight of the request
	h.IncrementPendingRequests(requestWeight)

	return true, 0

}

//isInSustainedPeriod checks if the current request is in the sustained period
func (h *RequestsStatus) isInSustainedPeriod(hostConfig RateLimitConfig, currentTime int64) bool {
	timeSincePeriodStart := currentTime - h.GetFirstSustainedRequest()

	return timeSincePeriodStart < hostConfig.SustainedTimePeriod && timeSincePeriodStart >= 0
}

//isInBurstPeriod checks if the current request is in the burst period
func (h *RequestsStatus) isInBurstPeriod(hostConfig RateLimitConfig, currentTime int64) bool {
	timeSincePeriodStart := currentTime - h.GetFirstBurstRequest()

	return timeSincePeriodStart < hostConfig.BurstTimePeriod && timeSincePeriodStart >= 0
}

//willHitSustainedLimit checks if the current request will hit the sustained rate limit
//if the total number of requests plus the weight of the requested request is greater than the limit
//than the requested request should not occur because it would cause us to go over the limit
func (h *RequestsStatus) willHitSustainedLimit(requestWeight int, host RateLimitConfig) bool {
	totalRequests := h.GetSustainedRequests() + h.GetPendingRequests()

	return totalRequests+requestWeight > host.SustainedRequestLimit
}

//willHitBurstLimit checks if the current request will hit the burst rate limit
//if the total number of requests plus the weight of the requested request is greater than the limit
//than the requested request should not occur because it would cause us to go over the limit
func (h *RequestsStatus) willHitBurstLimit(requestWeight int, host RateLimitConfig) bool {
	totalRequests := h.GetBurstRequests() + h.GetPendingRequests()

	return totalRequests+requestWeight > host.BurstRequestLimit
}

//timeUntilEndOfSustained calculates the time in milliseconds until the end of the sustained period
func (h *RequestsStatus) timeUntilEndOfSustained(host RateLimitConfig) (millisecondsToWait int64) {
	endOfPeriod := h.GetFirstSustainedRequest() + host.SustainedTimePeriod
	//converts seconds to milliseconds
	endMS := endOfPeriod * 1000

	now := time.Now().UTC().UnixNano()
	//converts nanoseconds to milliseconds
	nowMS := now / 1000000

	return endMS - nowMS
}

//timeUntilEndOfBurst calculates the time in milliseconds until the end of the burst period
func (h *RequestsStatus) timeUntilEndOfBurst(host RateLimitConfig) (millisecondsToWait int64) {
	endOfPeriod := h.GetFirstBurstRequest() + host.BurstTimePeriod
	//converts seconds to milliseconds
	endMS := endOfPeriod * 1000

	now := time.Now().UTC().UnixNano()
	//converts nanoseconds to milliseconds
	nowMS := now / 1000000

	return endMS - nowMS
}

func NewRequestsStatus(host string, sustainedRequests int, burstRequests int, pending int, firstSustainedRequests int64, firstBurstRequest int64) RequestsStatus {
	return RequestsStatus{host, sustainedRequests, burstRequests, pending, firstSustainedRequests, firstBurstRequest}
}

func (h *RequestsStatus) IncrementSustainedRequests(increment int) {
	h.SustainedRequests += increment
}

func (h *RequestsStatus) SetSustainedRequests(value int) {
	h.SustainedRequests = value
}

func (h *RequestsStatus) IncrementBurstRequests(increment int) {
	h.BurstRequests += increment
}

func (h *RequestsStatus) SetBurstRequests(value int) {
	h.BurstRequests = value
}

func (h *RequestsStatus) IncrementPendingRequests(increment int) {
	h.PendingRequests += increment
}

func (h *RequestsStatus) DecrementPendingRequests(increment int) {
	h.PendingRequests -= increment
}

func (h *RequestsStatus) SetFirstSustainedRequest(value int64) {
	h.FirstSustainedRequest = value
}

func (h *RequestsStatus) SetFirstBurstRequest(value int64) {
	h.FirstBurstRequest = value
}

func (h *RequestsStatus) GetSustainedRequests() int {
	return (*h).SustainedRequests
}

func (h *RequestsStatus) GetBurstRequests() int {
	return h.BurstRequests
}

func (h *RequestsStatus) GetPendingRequests() int {
	return h.PendingRequests
}

func (h *RequestsStatus) GetFirstSustainedRequest() int64 {
	return h.FirstSustainedRequest
}

func (h *RequestsStatus) GetFirstBurstRequest() int64 {
	return h.FirstBurstRequest
}
