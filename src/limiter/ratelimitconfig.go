package limiter

import (
	"strconv"

	"github.com/mediocregopher/radix/v3"
)

//RateLimitConfig struct contains the rate limit information for a specific api.
//
//If you want to coordinate requests to one api across multiple threads, routines, containers, etc,
//it is imperative that each Limiter you create is initialized with a RateLimitConfig that has the same
//host name otherwise the Limiter structs will not be able to communicate and you will definitely hit
//the ratelimit.
type RateLimitConfig struct {
	host                string //may change to different data type later
	requestLimit        int    //how many requests can be made in the given timePeriod
	timePeriod          int64  //how long the period lasts in seconds
	timeBetweenRequests int64  //is the minimum number of milliseconds between requests
}

const (
	limit               = "limit"
	timePeriod          = "timePeriod"
	timeBetweenRequests = "timeBetween"
)

//NewRateLimitConfig creates a rate limit config for a Limiter struct.
//
//If you want to coordinate requests to one api across multiple threads, routines, containers, etc,
//it is imperative that each Limiter you create is initialized with a RateLimitConfig that has the same
//host name otherwise the Limiter structs will not be able to communicate and you will definitely hit
//the ratelimit
//
//NewRateLimitConfig takes in two rates: a sustained ratelimit and a burst ratelimit. If the api you are
//making requests to only uses one ratelimit, enter in that rate for both the sustained and burst ratelimit.
//
//	config := NewRateLimitConfig("exampleHostName", 1200, 60, 20, 1)
//The time periods of both rates are in terms of seconds so the config above has a sustained ratelimit of
//1200 requests per 60 seconds and a burst ratelimit of 20 requests per second.
func NewRateLimitConfig(host string, sustainedRequestLimit int, sustainedTimePeriod int64, burstRequestLimit int, burstTimePeriod int64) RateLimitConfig {
	rl := RateLimitConfig{host, 0, 0, 0}

	rl.requestLimit, rl.timePeriod = determineLowerRate(sustainedRequestLimit, sustainedTimePeriod, burstRequestLimit, burstTimePeriod)
	rl.setTimeBetweenRequests()

	return rl
}

func determineLowerRate(sustainedRequestLimit int, sustainedTimePeriod int64, burstRequestLimit int, burstTimePeriod int64) (int, int64) {
	if (sustainedRequestLimit == 0 || sustainedTimePeriod == 0) && (burstRequestLimit == 0 || burstTimePeriod == 0) {
		//both infinite rates
		return 0, 0
	}
	if sustainedRequestLimit == 0 || sustainedTimePeriod == 0 {
		//sustained is an infinite rate
		limit, time := reduceFraction(int64(burstRequestLimit), burstTimePeriod)
		return int(limit), time
	}
	if burstRequestLimit == 0 || burstTimePeriod == 0 {
		//burst is an infinite rate
		limit, time := reduceFraction(int64(sustainedRequestLimit), sustainedTimePeriod)
		return int(limit), time
	}
	//determines which is the lower effective rate
	if burstRequestLimit*int(sustainedTimePeriod) > sustainedRequestLimit*int(burstTimePeriod) {
		//sustained is the lower rate
		lim, period := reduceFraction(int64(sustainedRequestLimit), sustainedTimePeriod)
		return int(lim), period
	}

	//burst is the lower rate or they are equal
	lim, period := reduceFraction(int64(burstRequestLimit), burstTimePeriod)

	return int(lim), period
}

func (rl *RateLimitConfig) setTimeBetweenRequests() {
	//requests per second
	if rl.requestLimit == 0 {
		return
	}

	time := rl.timePeriod * 1000
	rl.timeBetweenRequests = time / int64(rl.requestLimit)
}

func (rl *RateLimitConfig) updateConfigFromDatabase(c radix.Conn, key string) error {
	var values []string

	err := c.Do(radix.Cmd(&values, "HVALS", key))
	if err != nil {
		return err
	}
  
	if len(values) != 3 {
		return nil
	}

	limit, _ := strconv.Atoi(values[0])
	timePeriod, _ := strconv.ParseInt(values[1], 10, 64)
	timeBetween, _ := strconv.ParseInt(values[2], 10, 64)

	*rl = RateLimitConfig{rl.host, limit, timePeriod, timeBetween}
	return nil
}

func gcd(a int64, b int64) int64 {
	//Calculate GCD
	c := a % b

	for c > 0 {
		a = b
		b = c
		c = a % b
	}
	return b
}

func reduceFraction(numerator int64, denominator int64) (int64, int64) {
	gcd := gcd(numerator, denominator)
	return numerator / gcd, denominator / gcd
}
