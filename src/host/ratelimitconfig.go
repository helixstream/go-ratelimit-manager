package host

//RateLimitConfig struct contains the rate limit information for a specific host
type RateLimitConfig struct {
	host                  string //may change to different data type later
	sustainedRequestLimit int    //the number of requests that can be made during the sustained period
	sustainedTimePeriod   int64  //length of sustained period in seconds
	burstRequestLimit     int    //the number of requests that can be made during the burst period
	burstTimePeriod       int64  //length of the burst period in seconds
	timeBetweenRequests int64
}

func NewRateLimitConfig(host string, sustainedRequestLimit int, sustainedTimePeriod int64, burstRequestLimit int, burstTimePeriod int64) RateLimitConfig {
	rl := RateLimitConfig{host, sustainedRequestLimit, sustainedTimePeriod, burstRequestLimit, burstTimePeriod, 0}
	rl.setTimeBetweenRequests(60)
	return rl
}

func (rl *RateLimitConfig) setTimeBetweenRequests(percentage int64) {
	//requests per second
	var time int64

	if rl.sustainedTimePeriod == 0 || rl.burstTimePeriod == 0{
		return
	}

	//use sustained limit
	if rl.burstRequestLimit * int(rl.sustainedTimePeriod) > rl.sustainedRequestLimit {
		//converts to milliseconds
		time = rl.sustainedTimePeriod * 1000
		//percentage decrease time period by (9 = 10% decrease, 100 - 90 = 10%)
		time *= percentage
		time = time/100

		rl.timeBetweenRequests = time/int64(rl.sustainedRequestLimit)
	} else if rl.burstRequestLimit * int(rl.sustainedTimePeriod) <= rl.sustainedRequestLimit {
		//converts to milliseconds
		time = rl.burstTimePeriod * 1000
		//percentage decrease time period by (9 = 10% decrease, 100 - 90 = 10%)
		time *= percentage
		time = time/100
		rl.timeBetweenRequests = time/int64(rl.burstRequestLimit)
	}

}
