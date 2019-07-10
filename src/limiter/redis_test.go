package limiter

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	url2 "net/url"
	"strconv"
	"testing"
	"time"

	"github.com/go-test/deep"
	"github.com/mediocregopher/radix"
)

//pool of connections to redis database
var (
	pool, err     = radix.NewPool("tcp", "127.0.0.1:6379", 1000)
	numOfRoutines = 100

	testConfig = NewRateLimitConfig(
		serverConfig.host,
		sus,
		susPeriod,
		burst,
		burstPeriod,
	)
)

func Test_CanMakeRequestTokenServer(t *testing.T) {
	//handles creating new pool error
	if err != nil {
		panic(err)
	}

	rand.Seed(time.Now().Unix())
	channel := make(chan string)

	server := getServer()

	fmt.Print("testing token rate limit server")

	limiter, err := NewLimiter(testConfig, pool)
	if err != nil {
		t.Error(err)
	}

	for i := 0; i < numOfRoutines; i++ {
		go makeRequests(t, limiter, i, channel, "http://localhost:"+port+"/testRateLimit")
		time.Sleep(time.Duration(10) * time.Millisecond)
	}

	for i := 0; i < numOfRoutines; i++ {
		<-channel
	}

	if err := server.Shutdown(context.TODO()); err != nil {
		panic(err)
	}

	fmt.Print("done \n")
}

func Test_CanMakeRequestWindowServer(t *testing.T) {
	fmt.Print("testing window rate limit server")

	windowServer := getWindowServer()
	channel := make(chan string)

	limiter, err := NewLimiter(testConfig, pool)
	if err != nil {
		t.Error(err)
	}

	for i := 0; i < numOfRoutines; i++ {
		go makeRequests(t, limiter, i, channel, "http://localhost:"+windowPort+"/testWindowRateLimit")
		time.Sleep(time.Duration(10) * time.Millisecond)
	}

	for i := 0; i < numOfRoutines; i++ {
		<-channel
	}

	if err := windowServer.Shutdown(context.TODO()); err != nil {
		panic(err)
	}

	fmt.Print("done ")
}

func makeRequests(t *testing.T, limiter Limiter, id int, c chan<- string, url string) {
	numOfRequests := 3
	for numOfRequests > 0 {
		requestWeight := 1

		limiter.WaitForRatelimit(requestWeight)

		statusCode, err := getStatusCode(url, requestWeight)
		if err != nil {
			t.Errorf("Error on getting Status Code: %v. ", err)
			if err := limiter.RequestCancelled(requestWeight); err != nil {
				t.Errorf("Error on Request Cancelled: %v. ", err)
			}
		}

		if statusCode == 500 {
			if err := limiter.RequestCancelled(requestWeight); err != nil {
				t.Errorf("Error on Request Cancelled: %v. ", err)
			}

		} else if statusCode == 200 {
			if err := limiter.RequestSuccessful(requestWeight); err != nil {
				t.Errorf("Error on Request Finished: %v. ", err)
			}
			numOfRequests -= requestWeight
		} else {
			if getUnixTimeMilliseconds()-limiter.status.lastErrorTime < 5000 {
				t.Errorf("Multiple 429s too close together \n")
			}

			if err := limiter.HitRateLimit(requestWeight); err != nil {
				t.Errorf("Error on HitRateLimit: %v. ", err)
			}

			fmt.Printf("Routine: %v. %v. %v, %v \n", id, statusCode, limiter.status, limiter.config)
		}

		fmt.Print(".")
	}

	//if the time since the last error is less than one minute
	if getUnixTimeMilliseconds()-limiter.status.lastErrorTime < 60*1000 {
		go makeRequests(t, limiter, id, c, url)
	} else {
		c <- "done"
	}
}

func getStatusCode(url string, weight int) (int, error) {
	resp, err := http.PostForm(url, url2.Values{"weight": {strconv.Itoa(weight)}})
	if err != nil {
		return 0, err
	} else if resp != nil {
		return resp.StatusCode, resp.Body.Close()
	}

	return 0, nil
}

func Test_RequestCancelled(t *testing.T) {

	type TestRequestStatus struct {
		requestWeight int
		limiter       Limiter
		expected      RequestsStatus
	}

	config := NewRateLimitConfig("testHost1", 0, 0, 0, 0)

	testCases := []TestRequestStatus{
		{
			2,
			Limiter{newRequestsStatus(0, 10, 0, 0), config, pool},
			newRequestsStatus(0, 8, 0, 0),
		},
		{
			1,
			Limiter{newRequestsStatus(0, 3, 0, 0), config, pool},
			newRequestsStatus(0, 2, 0, 0),
		},
		{
			5,
			Limiter{newRequestsStatus(0, 10, 0, 0), config, pool},
			newRequestsStatus(0, 5, 0, 0),
		},
	}

	key := testCases[0].limiter.getStatusKey()

	for i := 0; i < len(testCases); i++ {
		l := testCases[i].limiter

		err := pool.Do(radix.WithConn(key, func(c radix.Conn) error {
			err = pool.Do(radix.FlatCmd(nil, "HSET",
				key,
				requests, l.status.requests,
				pendingRequests, l.status.pendingRequests,
				firstRequest, l.status.firstRequest,
				lastErrorTime, l.status.lastErrorTime,
			))

			if err != nil {
				return err
			}

			if err := l.RequestCancelled(testCases[i].requestWeight); err != nil {
				t.Error(err)
			}

			err = l.status.updateStatusFromDatabase(c, key)
			if err != nil {
				return err
			}

			return nil
		}))

		if err != nil {
			t.Error(err)
		}

		if diff := deep.Equal(l.status, testCases[i].expected); diff != nil {
			t.Errorf("Loop: %v. %v. ", i, diff)
		}
	}

}

func Test_RequestFinished(t *testing.T) {

	type TestRequestStatus struct {
		requestWeight int
		limiter       Limiter
		expected      RequestsStatus
	}

	config := NewRateLimitConfig("testHost1", 0, 0, 0, 0)

	testCases := []TestRequestStatus{
		{
			2,
			Limiter{newRequestsStatus(5, 2, 0, 0), config, pool},
			newRequestsStatus(7, 0, 0, 0),
		},
		{
			1,
			Limiter{newRequestsStatus(0, 40, 0, 0), config, pool},
			newRequestsStatus(1, 39, 0, 0),
		},
		{
			5,
			Limiter{newRequestsStatus(35, 5, 0, 0), config, pool},
			newRequestsStatus(40, 0, 0, 0),
		},
	}

	key := testCases[0].limiter.getStatusKey()

	for i := 0; i < len(testCases); i++ {
		l := testCases[i].limiter

		err := pool.Do(radix.WithConn(key, func(c radix.Conn) error {
			err = pool.Do(radix.FlatCmd(nil, "HSET",
				key,
				requests, l.status.requests,
				pendingRequests, l.status.pendingRequests,
				firstRequest, l.status.firstRequest,
				lastErrorTime, l.status.lastErrorTime,
			))
			if err != nil {
				return err
			}

			if err := l.RequestSuccessful(testCases[i].requestWeight); err != nil {
				t.Error(err)
			}

			err = l.status.updateStatusFromDatabase(c, key)
			if err != nil {
				return err
			}

			return nil
		}))

		if err != nil {
			t.Error(err)
		}
		if diff := deep.Equal(l.status, testCases[i].expected); diff != nil {
			t.Errorf("Loop: %v. %v. ", i, diff)
		}
	}
}

func Test_updateStatusFromDatabase(t *testing.T) {
	config := NewRateLimitConfig("testHost1", 1, 1, 1, 1)

	testCases := []Limiter{
		{newRequestsStatus(5, 2, 23564, 0), config, pool},
		{newRequestsStatus(0, 40, 3454345, 0), config, pool},
		{newRequestsStatus(35, 0, 266256, 0), config, pool},
	}

	key := testCases[0].getStatusKey()

	for i := 0; i < len(testCases); i++ {
		l := testCases[i]
		newLimiter, err := NewLimiter(config, pool)

		err = pool.Do(radix.WithConn(key, func(c radix.Conn) error {
			err = c.Do(radix.FlatCmd(nil, "HSET",
				key,
				requests, l.status.requests,
				pendingRequests, l.status.pendingRequests,
				firstRequest, l.status.firstRequest,
				lastErrorTime, l.status.lastErrorTime,
			))
			if err != nil {
				return err
			}

			err = newLimiter.status.updateStatusFromDatabase(c, key)
			if err != nil {
				return err
			}

			return nil
		}))
		if err != nil {
			t.Error(err)
		}

		if diff := deep.Equal(l, newLimiter); diff != nil {
			t.Errorf("Loop: %v. %v. ", i, diff)
		}
	}
}

func ExampleLimiter_WaitForRatelimit() {
	config := NewRateLimitConfig("testhost", 60, 60, 1, 1)
	limiter, err := NewLimiter(config, pool)
	if err != nil {
		//handle err
	}

	for {
		limiter.WaitForRatelimit(1)

		//make api request
		statusCode, err := getStatusCode("www.example.com", 1)
		if err != nil {
			if err := limiter.RequestCancelled(1); err != nil {
				//handle error
			}

		}

		if statusCode == 429 {
			if err := limiter.HitRateLimit(1); err != nil {
				//handle error
			}
		} else {
			if err := limiter.RequestSuccessful(1); err != nil {
				//handle error
			}
		}
	}
}

func Test_GetStatus(t *testing.T) {

	testCases := []Limiter{
		{
			newRequestsStatus(10, 45, getUnixTimeMilliseconds(), 42350232),
			NewRateLimitConfig("host1", 45, 3, 452, 4),
			pool,
		},
		{
			newRequestsStatus(52, 85, 23542636, 34534),
			NewRateLimitConfig("host2", 25453, 2343, 234, 3243),
			pool,
		},
		{
			newRequestsStatus(0, 0, 0, 0),
			NewRateLimitConfig("host3", 0, 0, 0, 0),
			pool,
		},
		{
			newRequestsStatus(1, 23, 324, 423362),
			NewRateLimitConfig("host4", 23523, 324, 23, 1),
			pool,
		},
		{
			newRequestsStatus(120, 32, 455635435, 1246564566),
			NewRateLimitConfig("host5", 1200, 60, 20, 10),
			pool,
		},
	}

	for i := 0; i < len(testCases); i++ {
		if diff := deep.Equal(testCases[i].GetStatus(), testCases[i].status); diff != nil {
			t.Error(diff)
		}
	}
}
