package host

import (
	"github.com/go-test/deep"
	"github.com/gomodule/redigo/redis"
	"log"
	"testing"
	"time"
)

//pool of connections to redis database
var pool = &redis.Pool{
	MaxIdle:   80,
	MaxActive: 12000,
	Dial: func() (redis.Conn, error) {
		c, err := redis.Dial("tcp", ":6379")
		if err != nil {
			panic(err.Error())
		}
		return c, err
	},
	IdleTimeout: 240 * time.Second,
}

func Test_PING(t *testing.T) {
	resp := isConnectedToRedis(pool)

	if resp != true {
		t.Error("Could not connect to Redis.")
	}
}

func Test_DoesKeyExist(t *testing.T) {

	type TestRequestStatus struct {
		status   RequestsStatus
		expected bool
	}

	testCases := []TestRequestStatus{
		{
			NewRequestsStatus("testHost1", 0, 0, 0, 0, 0),
			false,
		},
		{
			NewRequestsStatus("testHost2", 0, 0, 0, 0, 0),
			true,
		},
	}

	c := pool.Get()

	_, err := c.Do("SET", "status:testHost2", "value")
	if err != nil {
		log.Fatal(err)
	}

	err = c.Close()
	if err != nil {
		log.Fatal(err)
	}

	for i := 0; i < len(testCases); i++ {
		result := testCases[i].status.DoesKeyExist(pool)

		if result != testCases[i].expected {
			t.Errorf("Loop: %v. Expected key to exist: %v, got: %v. ", i, testCases[i].expected, result)
		}
	}
}

func Test_RequestCancelled(t *testing.T) {
	c := pool.Get()
	key := "status:" + "testHost"

	type TestRequestStatus struct {
		requestWeight int
		status        RequestsStatus
		expected      RequestsStatus
	}

	testCases := []TestRequestStatus{
		{
			2,
			NewRequestsStatus("testHost", 0, 0, 10, 0, 0),
			NewRequestsStatus("testHost", 0, 0, 8, 0, 0),
		},
		{
			1,
			NewRequestsStatus("testHost", 0, 0, 3, 0, 0),
			NewRequestsStatus("testHost", 0, 0, 2, 0, 0),
		},
		{
			5,
			NewRequestsStatus("testHost", 0, 0, 10, 0, 0),
			NewRequestsStatus("testHost", 0, 0, 5, 0, 0),
		},
	}

	for i := 0; i < len(testCases); i++ {
		s := testCases[i].status
		_, err := c.Do("HSET", key, host, s.Host, sustainedRequests, s.SustainedRequests, burstRequests, s.BurstRequests, pendingRequests, s.PendingRequests, firstSustainedRequest, s.FirstSustainedRequest, firstBurstRequest, s.FirstBurstRequest)
		if err != nil {
			t.Errorf("Loop: %v. %v. ", i, err)
		}

		s.RequestCancelled(testCases[i].requestWeight, pool)

		s.getStatus(c, key)

		if diff := deep.Equal(s, testCases[i].expected); diff != nil {
			t.Errorf("Loop: %v. %v. ", i, diff)
		}
	}

}

func Test_RequestFinished(t *testing.T) {
	c := pool.Get()
	key := "status:" + "testHost"

	type TestRequestStatus struct {
		requestWeight int
		status        RequestsStatus
		expected      RequestsStatus
	}

	testCases := []TestRequestStatus{
		{
			2,
			NewRequestsStatus("testHost", 5, 2, 10, 0, 0),
			NewRequestsStatus("testHost", 7, 4, 8, 0, 0),
		},
		{
			1,
			NewRequestsStatus("testHost", 0, 40, 3, 0, 0),
			NewRequestsStatus("testHost", 1, 41, 2, 0, 0),
		},
		{
			5,
			NewRequestsStatus("testHost", 35, 0, 10, 0, 0),
			NewRequestsStatus("testHost", 40, 5, 5, 0, 0),
		},
	}

	for i := 0; i < len(testCases); i++ {
		s := testCases[i].status
		_, err := c.Do("HSET", key, host, s.Host, sustainedRequests, s.SustainedRequests, burstRequests, s.BurstRequests, pendingRequests, s.PendingRequests, firstSustainedRequest, s.FirstSustainedRequest, firstBurstRequest, s.FirstBurstRequest)
		if err != nil {
			t.Errorf("Loop: %v. %v. ", i, err)
		}

		s.RequestFinished(testCases[i].requestWeight, pool)

		s.getStatus(c, key)

		if diff := deep.Equal(s, testCases[i].expected); diff != nil {
			t.Errorf("Loop: %v. %v. ", i, diff)
		}
	}
}

func Test_getStatus(t *testing.T) {
	c := pool.Get()
	key := "status:" + "testHost"

	testCases := []RequestsStatus{
		NewRequestsStatus("testHost", 5, 2, 10, 2126523, 2343),
		NewRequestsStatus("testHost", 0, 40, 3, 236436, 0),
		NewRequestsStatus("testHost", 35, 0, 10, 0, 9545456),
	}

	for i := 0; i < len(testCases); i++ {
		s := testCases[i]
		_, err := c.Do("HSET", key, host, s.Host, sustainedRequests, s.SustainedRequests, burstRequests, s.BurstRequests, pendingRequests, s.PendingRequests, firstSustainedRequest, s.FirstSustainedRequest, firstBurstRequest, s.FirstBurstRequest)
		if err != nil {
			t.Errorf("Loop: %v. %v. ", i, err)
		}
		newStatus := NewRequestsStatus("testHost", 0, 0, 0, 0, 0)
		newStatus.getStatus(c, key)

		if diff := deep.Equal(s, newStatus); diff != nil {
			t.Errorf("Loop: %v. %v. ", i, diff)
		}
	}
}
