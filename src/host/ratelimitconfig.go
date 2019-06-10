package host

/*
RateLimitConfig struct contains the rate limit information for a specific host
SustainedRequestLimit -> the number of requests that can be made during the sustained period
SustainedTimePeriod -> length of sustained period in seconds
BurstRequestLimit -> the number of requests that can be made during the burst period
BurstTimePeriod -> length of the burst period in seconds
*/
type RateLimitConfig struct {
	Host                  string //may change to different data type later
	SustainedRequestLimit int
	SustainedTimePeriod   int64 //seconds
	BurstRequestLimit     int
	BurstTimePeriod       int64 //seconds
}

func NewRateLimitConfig(host string, sustainedRequestLimit int, sustainedTimePeriod int64, burstRequestLimit int, burstTimePeriod int64) RateLimitConfig {
	return RateLimitConfig{host, sustainedRequestLimit, sustainedTimePeriod, burstRequestLimit, burstTimePeriod}
}
