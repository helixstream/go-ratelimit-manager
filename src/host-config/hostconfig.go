package host_config

type HostConfig struct {
	Host                  string //may change to different data type later
	SustainedRequestLimit int
	SustainedTimePeriod   int64 //seconds
	BurstRequestLimit     int
	BurstTimePeriod       int64 //seconds
}

func NewHostConfig(host string, sustainedRequestLimit int, sustainedTimePeriod int64, burstRequestLimit int, burstTimePeriod int64) HostConfig {
	return HostConfig{host, sustainedRequestLimit, sustainedTimePeriod, burstRequestLimit, burstTimePeriod}
}
