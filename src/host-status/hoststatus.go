package host_status

type HostStatus struct {
	Host                  string
	SustainedRequests     int
	BurstRequests         int
	Pending               int
	FirstSustainedRequest int
	FirstBurstRequest     int
}

func NewHostStatus(host string, sustainedRequests int, burstRequests int, pending int, firstSustainedRequests int, firstBurstRequest int) HostStatus {
	return HostStatus{host, sustainedRequests, burstRequests, pending, firstSustainedRequests, firstBurstRequest}
}
