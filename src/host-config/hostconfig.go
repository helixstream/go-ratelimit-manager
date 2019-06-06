package host_config


type HostConfig struct {
	Host                  string //may change to different data type later
	SustainedRequestLimit int
	SustainedTimePeriod   int //seconds
	BurstRequestLimit     int
	BurstTimePeriod       int //seconds
}

func NewHostConfig(host string, sustainedRequestLimit int, sustainedTimePeriod int, burstRequestLimit int, burstTimePeriod int) HostConfig {
	return HostConfig{host, sustainedRequestLimit, sustainedTimePeriod, burstRequestLimit, burstTimePeriod}
}
