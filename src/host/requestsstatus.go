package host

import (
	"encoding/json"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"github.com/youtube/vitess/go/vt/log"
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

//SaveStatus saves the values of the RequestStatus struct to
//the redis database
func (h *RequestsStatus) SaveStatus(p *redis.Pool) {
	c := p.Get()
	json, err := h.ConvertToJSON()
	if err != nil {
		log.Fatal(err)
	}
	_, err = c.Do("SET", "status:"+h.Host, json)
	if err != nil {
		log.Fatal(err)
	}

	err = c.Close()
	if err != nil {
		log.Fatal(err)
	}
}

//GetStatus sets the struct's values to the values in the database
func (h *RequestsStatus) GetStatus(p *redis.Pool) {
	c := p.Get()

	resp, err := c.Do("GET", "status:"+h.Host)
	//converts resp to a slice of bytes
	bytes, err := redis.Bytes(resp, err)
	if err != nil {
		log.Fatal(err)
	}

	err = h.ConvertFromJSON(bytes)
	if err != nil {
		log.Fatal(err)
	}

	err = c.Close()
	if err != nil {
		log.Fatal(err)
	}
}

//ConvertToJSON converts the contents of the struct to json and
//returns a slice of bytes to be saved to the database
func (h *RequestsStatus) ConvertToJSON() ([]byte, error) {
	return json.Marshal(h)
}

//ConvertFromJSON takes a slice of bytes and converts it to JSON
//and then sets the values of the struct based on that json. Used
//to update struct data from database
func (h *RequestsStatus) ConvertFromJSON(data []byte) error {
	if err := json.Unmarshal(data, &h); err != nil {
		return err
	}

	return nil
}

//isConnectedToRedis pings the redis database and returns
//whether there is a successful connection
func isConnectedToRedis(p *redis.Pool) bool {
	c := p.Get()
	resp, err := c.Do("PING")
	pingResp, err := redis.String(resp, err)
	if err != nil {
		fmt.Errorf("%v", err)
	}

	if pingResp != "PONG" {
		return false
	}

	return true
}

//CheckRequest calls CanMakeRequest until a request can be  made
//returns true when a request can be made
//if a request cannot be made it waits the correct amount of time and check again to see if a request can be made
func (h *RequestsStatus) CheckRequest(requestWeight int, host RateLimitConfig) bool {
	canMake, wait := h.CanMakeRequest(requestWeight, host)

	if wait != 0 {
		//sleeps out of burst or sustained period where limit has been reached
		time.Sleep(time.Duration(wait) * time.Millisecond)

		canMake, _ = h.CanMakeRequest(requestWeight, host)
		return canMake
	}

	return true
}

//CanMakeRequest checks to see if a request can be made
//returns true, 0 if request can be made
//returns false and the number of milliseconds to wait if a request cannot be made
func (h *RequestsStatus) CanMakeRequest(requestWeight int, host RateLimitConfig) (bool, int64) {

	//if request is in the current burst period
	if h.isInBurstPeriod(host) {
		//will the request push us over the burst limit
		if h.willHitBurstLimit(requestWeight, host) {
			//is so do not make the request and wait
			return false, h.timeUntilEndOfBurst(host)
		}

		//determines if the request will go over the sustained limit
		if h.willHitSustainedLimit(requestWeight, host) {
			//is so do not make the request and wait
			return false, h.timeUntilEndOfSustained(host)
		}

		//did not hit either burst or sustained limit
		//so can make request and increments pending requests
		h.IncrementPendingRequests(requestWeight)
		return true, 0

		//not in burst period, but in sustained period
	}

	if h.isInSustainedPeriod(host) {
		//reset burst to 0 and sets start of new burst period to now
		h.SetBurstRequests(0)
		h.SetFirstBurstRequest(GetUnixTimeMilliseconds())

		if h.willHitSustainedLimit(requestWeight, host) {
			return false, h.timeUntilEndOfSustained(host)
		}

		//can make request because did not hit sustained limit
		h.IncrementPendingRequests(requestWeight)
		return true, 0

	}
	//out of burst and sustained, able to make request

	//reset number of sustained and burst in new time period
	h.SetSustainedRequests(0)
	h.SetBurstRequests(0)
	//set start of both sustained and burst to now
	h.SetFirstSustainedRequest(GetUnixTimeMilliseconds())
	h.SetFirstBurstRequest(GetUnixTimeMilliseconds())
	//increment the number of pending requests by the weight of the request
	h.IncrementPendingRequests(requestWeight)

	return true, 0

}

//isInSustainedPeriod checks if the current request is in the sustained period
func (h *RequestsStatus) isInSustainedPeriod(hostConfig RateLimitConfig) bool {
	timeSincePeriodStart := GetUnixTimeMilliseconds() - h.GetFirstSustainedRequest()
	//								converts seconds to milliseconds
	return timeSincePeriodStart < hostConfig.SustainedTimePeriod*1000 && timeSincePeriodStart >= 0
}

//isInBurstPeriod checks if the current request is in the burst period
func (h *RequestsStatus) isInBurstPeriod(hostConfig RateLimitConfig) bool {
	timeSincePeriodStart := GetUnixTimeMilliseconds() - h.GetFirstBurstRequest()
	//								converts seconds to milliseconds
	return timeSincePeriodStart < hostConfig.BurstTimePeriod*1000 && timeSincePeriodStart >= 0
}

//willHitSustainedLimit checks if the current request will hit the sustained rate limit
//if the total number of requests plus the weight of the requested request is greater than the limit
//than the requested request should not occur because it would cause us to go over the limit
func (h *RequestsStatus) willHitSustainedLimit(requestWeight int, host RateLimitConfig) bool {
	totalRequests := h.GetSustainedRequests() + h.GetPendingRequests()

	return totalRequests+requestWeight > host.SustainedRequestLimit
}

//willHitBurstLimit checks if the current request will hit the burst rate limit
//if the total number of requests plus the weight of the requested request is greater than the limit
//than the requested request should not occur because it would cause us to go over the limit
func (h *RequestsStatus) willHitBurstLimit(requestWeight int, host RateLimitConfig) bool {
	totalRequests := h.GetBurstRequests() + h.GetPendingRequests()

	return totalRequests+requestWeight > host.BurstRequestLimit
}

//timeUntilEndOfSustained calculates the time in milliseconds until the end of the sustained period
func (h *RequestsStatus) timeUntilEndOfSustained(host RateLimitConfig) (millisecondsToWait int64) {
	//milliseconds  							converts from seconds to milliseconds
	endOfPeriod := h.GetFirstSustainedRequest() + (host.SustainedTimePeriod * 1000)
	now := GetUnixTimeMilliseconds()

	return endOfPeriod - now
}

//timeUntilEndOfBurst calculates the time in milliseconds until the end of the burst period
func (h *RequestsStatus) timeUntilEndOfBurst(host RateLimitConfig) (millisecondsToWait int64) {
	//milliseconds  						converts from seconds to milliseconds
	endOfPeriod := h.GetFirstBurstRequest() + (host.BurstTimePeriod * 1000)
	now := GetUnixTimeMilliseconds()

	return endOfPeriod - now
}

func GetUnixTimeMilliseconds() int64 {
	return time.Now().UTC().UnixNano() / int64(time.Millisecond)
}

func NewRequestsStatus(host string, sustainedRequests int, burstRequests int, pending int, firstSustainedRequests int64, firstBurstRequest int64) RequestsStatus {
	return RequestsStatus{host, sustainedRequests, burstRequests, pending, firstSustainedRequests, firstBurstRequest}
}

func (h *RequestsStatus) IncrementSustainedRequests(increment int) {
	h.SustainedRequests += increment
}

func (h *RequestsStatus) SetSustainedRequests(value int) {
	h.SustainedRequests = value
}

func (h *RequestsStatus) IncrementBurstRequests(increment int) {
	h.BurstRequests += increment
}

func (h *RequestsStatus) SetBurstRequests(value int) {
	h.BurstRequests = value
}

func (h *RequestsStatus) IncrementPendingRequests(increment int) {
	h.PendingRequests += increment
}

func (h *RequestsStatus) DecrementPendingRequests(increment int) {
	h.PendingRequests -= increment
}

func (h *RequestsStatus) SetFirstSustainedRequest(value int64) {
	h.FirstSustainedRequest = value
}

func (h *RequestsStatus) SetFirstBurstRequest(value int64) {
	h.FirstBurstRequest = value
}

func (h *RequestsStatus) GetSustainedRequests() int {
	return (*h).SustainedRequests
}

func (h *RequestsStatus) GetBurstRequests() int {
	return h.BurstRequests
}

func (h *RequestsStatus) GetPendingRequests() int {
	return h.PendingRequests
}

func (h *RequestsStatus) GetFirstSustainedRequest() int64 {
	return h.FirstSustainedRequest
}

func (h *RequestsStatus) GetFirstBurstRequest() int64 {
	return h.FirstBurstRequest
}
