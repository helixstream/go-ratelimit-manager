package host

//RateLimitConfig struct contains the rate limit information for a specific host
type RateLimitConfig struct {
	host                  string //may change to different data type later
	sustainedRequestLimit int    //the number of requests that can be made during the sustained period
	sustainedTimePeriod   int64  //length of sustained period in seconds
	burstRequestLimit     int    //the number of requests that can be made during the burst period
	burstTimePeriod       int64  //length of the burst period in seconds
}

func NewRateLimitConfig(host string, sustainedRequestLimit int, sustainedTimePeriod int64, burstRequestLimit int, burstTimePeriod int64) RateLimitConfig {
	return RateLimitConfig{host, sustainedRequestLimit, sustainedTimePeriod, burstRequestLimit, burstTimePeriod}
}
