package host

import (
	"fmt"
	"strconv"

	"github.com/mediocregopher/radix"
)

//RateLimitConfig struct contains the rate limit information for a specific host
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

//NewRateLimitConfig creates a rate limit config for the limiter struct based on the sustained rate and burst rate in requests per second
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
		fmt.Print(err)
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
