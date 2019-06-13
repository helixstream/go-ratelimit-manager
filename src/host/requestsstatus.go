package host

import (
	"fmt"
	"github.com/gomodule/redigo/redis"
	"github.com/youtube/vitess/go/vt/log"
	"strconv"
	"time"
)

//RequestsStatus struct contains all info pertaining to the cumulative requests made to a specific host
type RequestsStatus struct {
	Host                  string
	SustainedRequests     int   //total number of completed requests made during the current sustained period
	BurstRequests         int   //total number of completed requests made during the current burst period
	PendingRequests       int   //number of requests that have started but have not completed
	FirstSustainedRequest int64 //timestamp in milliseconds that represents when the sustained period began
	FirstBurstRequest     int64 //timestamp in milliseconds that represents when the burst period began
}

const (
	host                  = "host"
	sustainedRequests     = "sustainedRequests"
	burstRequests         = "burstRequests"
	pendingRequests       = "pendingRequests"
	firstSustainedRequest = "firstSustainedRequest"
	firstBurstRequest     = "firstBurstRequest"
)

//key convention redis: struct:host
//example: status:com.binance.api
//example: config:com.binance.api

//DoesKeyExist checks the database to see if a non nil value is
//stored at the specific RequestStatus key
func (h *RequestsStatus) DoesKeyExist(p *redis.Pool) bool {
	c := p.Get()

	resp, err := c.Do("GET", "status:"+h.Host)
	if err != nil {
		log.Fatal(err)
	}
	//converts resp to a string
	stringResp, err := redis.String(resp, err)
	if stringResp != "" {
		return true
	}

	return false
}

//isConnectedToRedis pings the redis database and returns
//whether there is a successful connection
func isConnectedToRedis(p *redis.Pool) bool {
	c := p.Get()
	resp, err := c.Do("PING")
	pingResp, err := redis.String(resp, err)
	if err != nil {
		log.Fatal(err)
	}

	if pingResp != "PONG" {
		return false
	}

	return true
}

//RequestFinished updates the RequestsStatus struct by removing a pending request into the sustained and burst categories
//should be called directly after the request has finished
func (h *RequestsStatus) RequestFinished(requestWeight int, p *redis.Pool) {
	key := "status:" + h.Host
	c := p.Get()

	_, err := c.Do("MULTI")
	if err != nil {
		log.Fatal(err)
	}
	_, err = c.Do("HINCRBY", key, sustainedRequests, requestWeight)
	if err != nil {
		log.Fatal(err)
	}

	_, err = c.Do("HINCRBY", key, burstRequests, requestWeight)
	if err != nil {
		log.Fatal(err)
	}

	_, err = c.Do("HINCRBY", key, pendingRequests, -requestWeight)
	if err != nil {
		log.Fatal(err)
	}

	_, err = c.Do("EXEC")
	if err != nil {
		log.Fatal(err)
	}
}

//RequestCancelled updates the RequestStatus struct by removing a pending request as the request did not complete
//and so does not could against the rate limit. Should be called directly after the request was cancelled/failed
func (h *RequestsStatus) RequestCancelled(requestWeight int, p *redis.Pool) {
	key := "status:" + h.Host
	c := p.Get()
	//QUESTION: does this need to be a transaction since I am only updating one value?
	_, err := c.Do("MULTI")
	if err != nil {
		log.Fatal(err)
	}

	_, err = c.Do("HINCRBY", key, pendingRequests, -requestWeight)
	if err != nil {
		log.Fatal(err)
	}

	_, err = c.Do("EXEC")
	if err != nil {
		log.Fatal(err)
	}
}

func (h *RequestsStatus) CanMakeRequestTransaction(p *redis.Pool, requestWeight int, config RateLimitConfig) (bool, int64, error) {
	c := p.Get()
	key := "status:" + h.Host

	//if another client modifies the val in the time between our call to WATCH and our call to EXEC the transaction will fail.
	_, err := c.Do("WATCH", key)

	h.getStatus(c, key)

	canMake, wait := h.CanMakeRequest(requestWeight, config)

	_, err = c.Do("MULTI")
	if err != nil {
		fmt.Print(err)
		return false, 0, err
	}

	_, err = c.Do("HSET", key, host, h.Host, sustainedRequests, h.SustainedRequests, burstRequests, h.BurstRequests, pendingRequests, h.PendingRequests, firstSustainedRequest, h.SustainedRequests, firstBurstRequest, h.FirstBurstRequest)
	if err != nil {
		fmt.Print(err)
		return false, 0, err
	}

	_, err = c.Do("EXEC")
	if err != nil {
		fmt.Print(err)
		return false, 0, err
	}

	return canMake, wait, nil
}

func (h *RequestsStatus) getStatus(c redis.Conn, key string) {
	list, err := c.Do("HVALS", key)
	values, err := redis.Strings(list, err)
	if err != nil && len(values) != 6 {
		log.Fatal(err)
	}

	host := values[0]
	sus, _ := strconv.Atoi(values[1])
	burst, _ := strconv.Atoi(values[2])
	pending, _ := strconv.Atoi(values[3])
	firstSus, _ := strconv.ParseInt(values[4], 10, 64)
	firstBurst, _ := strconv.ParseInt(values[5], 10, 64)

	*h = NewRequestsStatus(host, sus, burst, pending, firstSus, firstBurst)
}

//CheckRequest calls CanMakeRequest until a request can be  made
//returns true when a request can be made
//if a request cannot be made it waits the correct amount of time and check again to see if a request can be made
func (h *RequestsStatus) CheckRequest(requestWeight int, conifig RateLimitConfig) bool {
	canMake, wait := h.CanMakeRequest(requestWeight, conifig)

	if wait != 0 {
		//sleeps out of burst or sustained period where limit has been reached
		time.Sleep(time.Duration(wait) * time.Millisecond)

		canMake, _ = h.CanMakeRequest(requestWeight, conifig)
		return canMake
	}

	return true
}

//CanMakeRequest checks to see if a request can be made
//returns true, 0 if request can be made
//returns false and the number of milliseconds to wait if a request cannot be made
func (h *RequestsStatus) CanMakeRequest(requestWeight int, config RateLimitConfig) (bool, int64) {
	now := GetUnixTimeMilliseconds()
	//if request is in the current burst period
	if h.isInBurstPeriod(now, config) {
		//will the request push us over the burst limit
		if h.willHitBurstLimit(requestWeight, config) {
			//is so do not make the request and wait
			return false, h.timeUntilEndOfBurst(now, config)
		}

		//determines if the request will go over the sustained limit
		if h.willHitSustainedLimit(requestWeight, config) {
			//is so do not make the request and wait
			return false, h.timeUntilEndOfSustained(now, config)
		}

		//did not hit either burst or sustained limit
		//so can make request and increments pending requests
		h.incrementPendingRequests(requestWeight)
		return true, 0

		//not in burst period, but in sustained period
	}

	if h.isInSustainedPeriod(now, config) {
		//reset burst to 0 and sets start of new burst period to now
		h.setBurstRequests(0)
		h.setFirstBurstRequest(now)

		if h.willHitSustainedLimit(requestWeight, config) {
			return false, h.timeUntilEndOfSustained(now, config)
		}

		//can make request because did not hit sustained limit
		h.incrementPendingRequests(requestWeight)
		return true, 0

	}
	//out of burst and sustained, able to make request

	//reset number of sustained and burst in new time period
	h.setSustainedRequests(0)
	h.setBurstRequests(0)
	//set start of both sustained and burst to now
	h.setFirstSustainedRequest(now)
	h.setFirstBurstRequest(now)
	//increment the number of pending requests by the weight of the request
	h.incrementPendingRequests(requestWeight)

	return true, 0

}

//isInSustainedPeriod checks if the current request is in the sustained period
func (h *RequestsStatus) isInSustainedPeriod(currentTime int64, hostConfig RateLimitConfig) bool {
	timeSincePeriodStart := currentTime - h.getFirstSustainedRequest()
	//								converts seconds to milliseconds
	return timeSincePeriodStart < hostConfig.SustainedTimePeriod*1000 && timeSincePeriodStart >= 0
}

//isInBurstPeriod checks if the current request is in the burst period
func (h *RequestsStatus) isInBurstPeriod(currentTime int64, hostConfig RateLimitConfig) bool {
	timeSincePeriodStart := currentTime - h.getFirstBurstRequest()
	//								converts seconds to milliseconds
	return timeSincePeriodStart < hostConfig.BurstTimePeriod*1000 && timeSincePeriodStart >= 0
}

//willHitSustainedLimit checks if the current request will hit the sustained rate limit
//if the total number of requests plus the weight of the requested request is greater than the limit
//than the requested request should not occur because it would cause us to go over the limit
func (h *RequestsStatus) willHitSustainedLimit(requestWeight int, host RateLimitConfig) bool {
	totalRequests := h.getSustainedRequests() + h.getPendingRequests()

	return totalRequests+requestWeight > host.SustainedRequestLimit
}

//willHitBurstLimit checks if the current request will hit the burst rate limit
//if the total number of requests plus the weight of the requested request is greater than the limit
//than the requested request should not occur because it would cause us to go over the limit
func (h *RequestsStatus) willHitBurstLimit(requestWeight int, host RateLimitConfig) bool {
	totalRequests := h.getBurstRequests() + h.getPendingRequests()

	return totalRequests+requestWeight > host.BurstRequestLimit
}

//timeUntilEndOfSustained calculates the time in milliseconds until the end of the sustained period
func (h *RequestsStatus) timeUntilEndOfSustained(currentTime int64, host RateLimitConfig) (millisecondsToWait int64) {
	// 											converts from seconds to milliseconds
	endOfPeriod := h.getFirstSustainedRequest() + (host.SustainedTimePeriod * 1000)

	return endOfPeriod - currentTime
}

//timeUntilEndOfBurst calculates the time in milliseconds until the end of the burst period
func (h *RequestsStatus) timeUntilEndOfBurst(currentTime int64, host RateLimitConfig) (millisecondsToWait int64) {
	//  								converts from seconds to milliseconds
	endOfPeriod := h.getFirstBurstRequest() + (host.BurstTimePeriod * 1000)

	return endOfPeriod - currentTime
}

func GetUnixTimeMilliseconds() int64 {
	return time.Now().UTC().UnixNano() / int64(time.Millisecond)
}

func NewRequestsStatus(host string, sustainedRequests int, burstRequests int, pending int, firstSustainedRequests int64, firstBurstRequest int64) RequestsStatus {
	return RequestsStatus{host, sustainedRequests, burstRequests, pending, firstSustainedRequests, firstBurstRequest}
}

func (h *RequestsStatus) incrementSustainedRequests(increment int) {
	h.SustainedRequests += increment
}

func (h *RequestsStatus) setSustainedRequests(value int) {
	h.SustainedRequests = value
}

func (h *RequestsStatus) incrementBurstRequests(increment int) {
	h.BurstRequests += increment
}

func (h *RequestsStatus) setBurstRequests(value int) {
	h.BurstRequests = value
}

func (h *RequestsStatus) incrementPendingRequests(increment int) {
	h.PendingRequests += increment
}

func (h *RequestsStatus) decrementPendingRequests(increment int) {
	h.PendingRequests -= increment
}

func (h *RequestsStatus) setFirstSustainedRequest(value int64) {
	h.FirstSustainedRequest = value
}

func (h *RequestsStatus) setFirstBurstRequest(value int64) {
	h.FirstBurstRequest = value
}

func (h *RequestsStatus) getSustainedRequests() int {
	return (*h).SustainedRequests
}

func (h *RequestsStatus) getBurstRequests() int {
	return h.BurstRequests
}

func (h *RequestsStatus) getPendingRequests() int {
	return h.PendingRequests
}

func (h *RequestsStatus) getFirstSustainedRequest() int64 {
	return h.FirstSustainedRequest
}

func (h *RequestsStatus) getFirstBurstRequest() int64 {
	return h.FirstBurstRequest
}
