package host

import (
	"context"
	"fmt"
	"github.com/go-test/deep"
	"github.com/gomodule/redigo/redis"
	"math/rand"
	"net/http"
	"testing"
	"time"
)

//pool of connections to redis database
var pool = &redis.Pool{
	MaxIdle:   700,
	MaxActive: 3000,
	Dial: func() (redis.Conn, error) {
		c, err := redis.Dial("tcp", ":6379")
		if err != nil {
			panic(err.Error())
		}
		return c, err

	},
	IdleTimeout: 240 * time.Second,
}

func Test_CanMakeTestTransaction(t *testing.T) {
	rand.Seed(time.Now().Unix())
	channel := make(chan string)

	numOfRoutines := 100

	server := server()

	fmt.Print("testing concurrent requests")

	for i := 0; i < numOfRoutines; i++ {
		//ServerConfig is a global variable declared in server.go
		go makeRequests(t, serverConfig, i, channel)
	}

	for i := 0; i < numOfRoutines; i++ {
		<-channel
	}

	if err := server.Shutdown(context.TODO()); err != nil {
		panic(err)
	}

	fmt.Print("done")
}

func makeRequests(t *testing.T, hostConfig RateLimitConfig, id int, c chan<- string) {
	requestStatus := NewRequestsStatus(hostConfig.Host, 0, 0, 0, 0, 0)

	numOfRequests := rand.Intn(10) + 1
	//sleep to make sure that we do not flood redis with requests at the start of the program
	time.Sleep(time.Duration(rand.Intn(200)) * time.Millisecond)

	for numOfRequests > 0 {
		requestWeight := rand.Intn(5) + 1

		canMake, sleepTime := requestStatus.CanMakeRequest(pool, requestWeight, hostConfig)

		if canMake {
			statusCode, err := getStatusCode("http://127.0.0.1:" + port + "/testRateLimit")
			if err != nil {
				t.Error(err)
			}

			if statusCode == 500 {
				requestStatus.RequestCancelled(requestWeight, pool)

			} else if statusCode == 200 {
				requestStatus.RequestFinished(requestWeight, pool)
				numOfRequests--

			} else {
				t.Errorf("Routine: %v. %v", id, statusCode)
			}

		} else {
			time.Sleep(time.Duration(sleepTime) * time.Millisecond)
		}
	}

	fmt.Print(".")
	c <- "done"
}

func getStatusCode(url string) (int, error) {
	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	} else if resp != nil {
		return resp.StatusCode, resp.Body.Close()
	}

	return 0, nil
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
		t.Error(err)
	}

	closeConnection(c)

	for i := 0; i < len(testCases); i++ {
		result := testCases[i].status.DoesKeyExist(pool)

		if result != testCases[i].expected {
			t.Errorf("Loop: %v. Expected key to exist: %v, got: %v. ", i, testCases[i].expected, result)
		}
	}
}

func Test_RequestCancelled(t *testing.T) {

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

		c := pool.Get()
		_, err := c.Do("HSET",
			key,
			host, s.Host,
			sustainedRequests, s.SustainedRequests,
			burstRequests, s.BurstRequests,
			pendingRequests, s.PendingRequests,
			firstSustainedRequest, s.FirstSustainedRequest,
			firstBurstRequest, s.FirstBurstRequest,
		)
		if err != nil {
			t.Errorf("Loop: %v. %v. ", i, err)
		}

		closeConnection(c)

		s.RequestCancelled(testCases[i].requestWeight, pool)

		err = s.updateStatusFromDatabase(pool, key)
		if err != nil {
			t.Error(err)
		}

		if diff := deep.Equal(s, testCases[i].expected); diff != nil {
			t.Errorf("Loop: %v. %v. ", i, diff)
		}
	}

}

func Test_RequestFinished(t *testing.T) {
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

		c := pool.Get()
		_, err := c.Do("HSET",
			key,
			host, s.Host,
			sustainedRequests, s.SustainedRequests,
			burstRequests, s.BurstRequests,
			pendingRequests, s.PendingRequests,
			firstSustainedRequest, s.FirstSustainedRequest,
			firstBurstRequest, s.FirstBurstRequest,
		)
		if err != nil {
			t.Errorf("Loop: %v. %v. ", i, err)
		}

		closeConnection(c)

		s.RequestFinished(testCases[i].requestWeight, pool)

		err = s.updateStatusFromDatabase(pool, key)
		if err != nil {
			t.Error(err)
		}

		if diff := deep.Equal(s, testCases[i].expected); diff != nil {
			t.Errorf("Loop: %v. %v. ", i, diff)
		}
	}
}

func Test_updateStatusFromDatabase(t *testing.T) {

	key := "status:" + "testHost"

	testCases := []RequestsStatus{
		NewRequestsStatus("testHost", 5, 2, 10, 2126523, 2343),
		NewRequestsStatus("testHost", 0, 40, 3, 236436, 0),
		NewRequestsStatus("testHost", 35, 0, 10, 0, 9545456),
	}

	for i := 0; i < len(testCases); i++ {
		c := pool.Get()
		s := testCases[i]
		_, err := c.Do("HSET",
			key,
			host, s.Host,
			sustainedRequests, s.SustainedRequests,
			burstRequests, s.BurstRequests,
			pendingRequests, s.PendingRequests,
			firstSustainedRequest, s.FirstSustainedRequest,
			firstBurstRequest, s.FirstBurstRequest,
		)
		if err != nil {
			t.Errorf("Loop: %v. %v. ", i, err)
		}

		closeConnection(c)

		newStatus := NewRequestsStatus("testHost", 0, 0, 0, 0, 0)
		err = newStatus.updateStatusFromDatabase(pool, key)
		if err != nil {
			t.Error(err)
		}

		if diff := deep.Equal(s, newStatus); diff != nil {
			t.Errorf("Loop: %v. %v. ", i, diff)
		}
	}
}
