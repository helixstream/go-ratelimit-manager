package host

import (
	"fmt"
	"github.com/mediocregopher/radix"
	"strconv"
)

//RateLimitConfig struct contains the rate limit information for a specific host
type RateLimitConfig struct {
	host                  string //may change to different data type later
	requestLimit	int
	timePeriod		int64
	timeBetweenRequests int64
}

const (
	limit = "limit"
	timePeriod = "timePeriod"
	timeBetweenRequests = "timeBetween"
)

func NewRateLimitConfig(host string, sustainedRequestLimit int, sustainedTimePeriod int64, burstRequestLimit int, burstTimePeriod int64) RateLimitConfig {
	rl := RateLimitConfig{host, 0, 0, 0}

	if burstRequestLimit * int(sustainedTimePeriod) > sustainedRequestLimit * int(burstTimePeriod) {
		//TODO reduce fraction
		rl.requestLimit = sustainedRequestLimit
		rl.timePeriod = sustainedTimePeriod
	} else {
		rl.requestLimit = burstRequestLimit
		rl.timePeriod = burstTimePeriod
	}

	rl.setTimeBetweenRequests()

	return rl
}

func (rl *RateLimitConfig) setTimeBetweenRequests() {
	//requests per second
	var time int64

	if rl.timePeriod == 0 || rl.requestLimit == 0 {
		return
	}

	time = rl.timePeriod * 1000
	//percentage decrease time period by (9 = 10% decrease, 100 - 90 = 10%)

	rl.timeBetweenRequests = time/int64(rl.requestLimit)
}

func (rl *RateLimitConfig) updateConfigFromDatabase(c radix.Conn, key string) error {
	var values []string
	err := c.Do(radix.Cmd(&values, "HVALS", key))
	if err != nil {
		fmt.Print(err)
		return err
	}

	if len(values) != 2 {
		return nil
	}

	limit, _ := strconv.Atoi(values[0])
	timePeriod, _ := strconv.ParseInt(values[1], 10, 64)
	timeBetween, _ := strconv.ParseInt(values[2], 10, 64)

	*rl = RateLimitConfig{rl.host, limit, timePeriod, timeBetween}
	return nil
}

