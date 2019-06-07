package host_status

import (
	"../host-config"

	t "time"
)

type HostStatus struct {
	Host                  string
	SustainedRequests     int
	BurstRequests         int
	PendingRequests       int
	FirstSustainedRequest int
	FirstBurstRequest     int
}
//recursively calls CanMakeRequest until a request can be  made
func (h *HostStatus) CheckRequest(requestWeight int, time int, host host_config.HostConfig) bool {
	canMake, wait := h.CanMakeRequest(requestWeight, time, host)

	if wait != 0 {
		//sleeps out of burst or sustained period where limit has been reached
		t.Sleep(t.Duration(wait) * t.Millisecond)

		canMake, _ = h.CanMakeRequest(requestWeight, int(t.Now().UTC().Unix()), host)
		return canMake
	}

	return true
}
//checks to see if a request can be made
//returns true, 0 if request can be made
//returns false and the number of milliseconds to wait if a request cannot be made
func (h *HostStatus) CanMakeRequest(requestWeight int, now int, host host_config.HostConfig) (bool, int) {
	nowUnit := int(now)
	//if request is in the current burst period
	if h.IsInBurstPeriod(host, nowUnit) {
		//will the request push us over the burst limit
		if h.WillHitBurstLimit(requestWeight, host) {
			//is so do not make the request and wait
			return false, h.WaitUntilEndOfBurst(host)
		}

		//determines if the request will go over the sustained limit
		if h.WillHitSustainedLimit(requestWeight, host) {
			//is so do not make the request and wait
			return false, h.WaitUntilEndOfSustained(host)
		}


		//did not hit either burst or sustained limit
		//so can make request and increments pending requests
		h.IncrementPendingRequests(requestWeight)
		return true, 0

		//not in burst period, but in sustained period
	} else if h.IsInSustainedPeriod(host, nowUnit) {


		//reset burst to 0 and sets start of new burst period to now
		h.SetBurstRequests(0)
		h.SetFirstBurstRequest(nowUnit)

		if h.WillHitSustainedLimit(requestWeight, host) {
			 return false, h.WaitUntilEndOfSustained(host)
		}

		//can make request because did not hit sustained limit
		h.IncrementPendingRequests(requestWeight)
		return true, 0


	} else {
		//out of burst and sustained, able to make request

		//reset number of sustained and burst in new time period
		h.SetSustainedRequests(0)
		h.SetBurstRequests(0)
		//set start of both sustained and burst to now
		h.SetFirstSustainedRequest(nowUnit)
		h.SetFirstBurstRequest(nowUnit)
		//increment the number of pending requests to the weight of the request
		h.IncrementPendingRequests(requestWeight)
		return true, 0
	}
}

func (h *HostStatus) IsInSustainedPeriod(hostConfig host_config.HostConfig, currentTime int) bool {
			//current time - start of period <= length of the period
	return currentTime - h.GetFirstSustainedRequest() < hostConfig.SustainedTimePeriod && currentTime - h.GetFirstSustainedRequest() >= 0
}

func (h *HostStatus) IsInBurstPeriod(hostConfig host_config.HostConfig, currentTime int) bool {
		//current time - start of period == length of period
	return currentTime - h.GetFirstBurstRequest() < hostConfig.BurstTimePeriod && currentTime - h.GetFirstBurstRequest() >= 0
}

func (h *HostStatus) WillHitSustainedLimit(requestWeight int, host host_config.HostConfig) bool {
	totalRequests := h.GetSustainedRequests() + h.GetPendingRequests()
	//if the total number of requests plus the weight of the requested request is greater than the limit
	//than the requested request should not occur because it would cause us to go over the limit
	return totalRequests + requestWeight > host.SustainedRequestLimit
}

func (h *HostStatus) WillHitBurstLimit(requestWeight int, host host_config.HostConfig) bool {
	totalRequests := h.GetBurstRequests() + h.GetPendingRequests()
	//if the total number of requests plus the weight of the requested request is greater than the limit
	//than the requested request should not occur because it would cause us to go over the limit
	return totalRequests + requestWeight > host.BurstRequestLimit
}

func (h *HostStatus) WaitUntilEndOfSustained(host host_config.HostConfig) int {
	//end = start of period plus length of period
	endOfPeriod := h.GetFirstSustainedRequest() + host.SustainedTimePeriod
	//converts seconds to milliseconds
	endMS := endOfPeriod * 1000

	now := t.Now().UTC().UnixNano()
	//converts nanoseconds to milliseconds
	nowMS := now/1000000

	return endMS - int(nowMS)
}

func (h *HostStatus) WaitUntilEndOfBurst(host host_config.HostConfig) int {
	//end = start of period plus length of period
	endOfPeriod := h.GetFirstBurstRequest() + host.BurstTimePeriod
	//converts seconds to milliseconds
	endMS := endOfPeriod * 1000

	now := t.Now().UTC().UnixNano()
	//converts nanoseconds to milliseconds
	nowMS := now/1000000

	return endMS - int(nowMS)
}

func NewHostStatus(host string, sustainedRequests int, burstRequests int, pending int, firstSustainedRequests int, firstBurstRequest int) HostStatus {
	return HostStatus{host, sustainedRequests, burstRequests, pending, firstSustainedRequests, firstBurstRequest}
}

func (h *HostStatus) IncrementSustainedRequests(increment int) {
	(*h).SustainedRequests += increment
}

func (h *HostStatus) SetSustainedRequests(value int) {
	(*h).SustainedRequests = value
}

func (h *HostStatus) IncrementBurstRequests(increment int) {
	(*h).BurstRequests += increment
}

func (h *HostStatus) SetBurstRequests(value int) {
	(*h).BurstRequests = value
}

func (h *HostStatus) IncrementPendingRequests(increment int) {
	(*h).PendingRequests += increment
}

func (h *HostStatus) DecrementPendingRequests(increment int) {
	(*h).PendingRequests -= increment
}

func (h *HostStatus) SetFirstSustainedRequest(value int) {
	(*h).FirstSustainedRequest = value
}

func (h *HostStatus) SetFirstBurstRequest(value int) {
	(*h).FirstBurstRequest = value
}

func (h *HostStatus) GetSustainedRequests() int {
	return (*h).SustainedRequests
}

func (h *HostStatus) GetBurstRequests() int {
	return (*h).BurstRequests
}

func (h *HostStatus) GetPendingRequests() int {
	return (*h).PendingRequests
}

func (h *HostStatus) GetFirstSustainedRequest() int {
	return (*h).FirstSustainedRequest
}

func (h *HostStatus) GetFirstBurstRequest() int {
	return (*h).FirstBurstRequest
}
