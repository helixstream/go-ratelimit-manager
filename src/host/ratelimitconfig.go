package host

import (
	"fmt"
	"github.com/mediocregopher/radix"
	"strconv"
)

//RateLimitConfig struct contains the rate limit information for a specific host
type RateLimitConfig struct {
	host                  string //may change to different data type later
	sustainedRequestLimit int    //the number of requests that can be made during the sustained period
	sustainedTimePeriod   int64  //length of sustained period in seconds
	burstRequestLimit     int    //the number of requests that can be made during the burst period
	burstTimePeriod       int64  //length of the burst period in seconds
	timeBetweenRequests int64
}

var percentage int64 = 100

func NewRateLimitConfig(host string, sustainedRequestLimit int, sustainedTimePeriod int64, burstRequestLimit int, burstTimePeriod int64) RateLimitConfig {
	rl := RateLimitConfig{host, sustainedRequestLimit, sustainedTimePeriod, burstRequestLimit, burstTimePeriod, 0}
	rl.setTimeBetweenRequests(percentage)
	return rl
}

func (rl *RateLimitConfig) setTimeBetweenRequests(percentage int64) {
	//requests per second
	var time int64

	if rl.sustainedTimePeriod == 0 || rl.burstTimePeriod == 0{
		return
	}
	limit, timePeriod := rl.getEffectiveLimit()

	time = timePeriod * 1000
	//percentage decrease time period by (9 = 10% decrease, 100 - 90 = 10%)
	time *= percentage
	time = time/100

	rl.timeBetweenRequests = time/int64(limit)
}

func (rl *RateLimitConfig) getEffectiveLimit() (int, int64) {
	if rl.burstRequestLimit * int(rl.sustainedTimePeriod) > rl.sustainedRequestLimit * int(rl.burstTimePeriod) {
		return rl.sustainedRequestLimit, rl.sustainedTimePeriod
	} else {
		return rl.burstRequestLimit, rl.burstTimePeriod
	}
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

	sus, _ := strconv.Atoi(values[0])
	burst, _ := strconv.Atoi(values[1])

	*rl = NewRateLimitConfig(rl.host, sus, rl.sustainedTimePeriod, burst, rl.burstTimePeriod)
	return nil
}
