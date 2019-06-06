package host_status

type HostStatus struct {
	Host                  string
	SustainedRequests     int
	BurstRequests         int
	PendingRequests       int
	FirstSustainedRequest int
	FirstBurstRequest     int
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
