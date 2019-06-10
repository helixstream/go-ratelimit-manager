package host

//RateLimitConfig struct contains the rate limit information for a specific host
type RateLimitConfig struct {
	Host                  string //may change to different data type later
	SustainedRequestLimit int    //the number of requests that can be made during the sustained period
	SustainedTimePeriod   int64  //length of sustained period in seconds
	BurstRequestLimit     int    //the number of requests that can be made during the burst period
	BurstTimePeriod       int64  //length of the burst period in seconds
}

func NewRateLimitConfig(host string, sustainedRequestLimit int, sustainedTimePeriod int64, burstRequestLimit int, burstTimePeriod int64) RateLimitConfig {
	return RateLimitConfig{host, sustainedRequestLimit, sustainedTimePeriod, burstRequestLimit, burstTimePeriod}
}
