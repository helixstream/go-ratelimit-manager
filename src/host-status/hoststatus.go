package host_status

import (
	"../host-config"
)

type HostStatus struct {
	Host                  string
	SustainedRequests     int
	BurstRequests         int
	PendingRequests       int
	FirstSustainedRequest int
	FirstBurstRequest     int
}

func (h *HostStatus) IsInSustainedPeriod(hostConfig host_config.HostConfig, currentTime int) bool {
	//current time <= start of period + length of period
	if currentTime <= (*h).FirstSustainedRequest+hostConfig.SustainedTimePeriod {
		return true
	}
	//updates host status values
	h.recordOutOfPeriodSustainedRequests(currentTime)
	return false
}

func (h *HostStatus) IsInBurstPeriod(hostConfig host_config.HostConfig, currentTime int) bool {
	//current time <= start of period + length of period
	if currentTime <= (*h).FirstBurstRequest+hostConfig.BurstTimePeriod {
		return true
	}
	//updates host status values
	h.recordOutOfPeriodBurstRequests(currentTime)
	return false
}

func (h *HostStatus) recordOutOfPeriodSustainedRequests(currentTime int) {
	h.SetFirstSustainedRequest(currentTime)
	h.SetSustainedRequests(0)
	h.IncrementPendingRequests(1)
}

func (h *HostStatus) recordOutOfPeriodBurstRequests(currentTime int) {
	h.SetFirstBurstRequest(currentTime)
	h.SetBurstRequests(0)
	h.IncrementPendingRequests(1)

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
