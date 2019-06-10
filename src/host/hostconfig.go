package host

type RateLimitConfig struct {
	Host                  string //may change to different data type later
	SustainedRequestLimit int
	SustainedTimePeriod   int64 //seconds
	BurstRequestLimit     int
	BurstTimePeriod       int64 //seconds
}

func NewHostConfig(host string, sustainedRequestLimit int, sustainedTimePeriod int64, burstRequestLimit int, burstTimePeriod int64) RateLimitConfig {
	return RateLimitConfig{host, sustainedRequestLimit, sustainedTimePeriod, burstRequestLimit, burstTimePeriod}
}
