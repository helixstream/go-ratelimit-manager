package host

import (
	"context"
	"fmt"
	"github.com/go-test/deep"
	"github.com/mediocregopher/radix"
	"math/rand"
	"net/http"
	url2 "net/url"
	"strconv"
	"testing"
	"time"
)

//pool of connections to redis database
var pool, err = radix.NewPool("tcp", "127.0.0.1:6379", 500)

func Test_CanMakeRequest(t *testing.T) {
	//handles creating new pool error
	if err != nil {
		panic(err)
	}

	rand.Seed(time.Now().Unix())
	channel := make(chan string)

	numOfRoutines := 200

	server := getServer()

	fmt.Print("testing concurrent requests ")

	testConfig := NewRateLimitConfig(
		serverConfig.host,
		serverConfig.sustainedRequestLimit-1,
		serverConfig.sustainedTimePeriod,
		serverConfig.burstRequestLimit-1,
		serverConfig.burstTimePeriod)

	limiter, err := NewLimiter(testConfig, pool)
	if err != nil {
		t.Error(err)
	}

	for i := 0; i < numOfRoutines; i++ {
		go makeRequests(t, limiter, i, channel)
	}

	for i := 0; i < numOfRoutines; i++ {
		<-channel
	}

	if err := server.Shutdown(context.TODO()); err != nil {
		panic(err)
	}

	fmt.Print("done")
}

func makeRequests(t *testing.T, limiter Limiter, id int, c chan<- string) {
	numOfRequests := rand.Intn(3) + 1

	for numOfRequests > 0 {

		requestWeight := rand.Intn(2) + 1

		canMake, sleepTime := limiter.CanMakeRequest(requestWeight)

		if canMake {
			//fmt.Printf("Can Make: %v \n", limiter.status)
			statusCode, err := getStatusCode("http://127.0.0.1:"+port+"/testRateLimit", requestWeight)
			if err != nil {
				t.Errorf("Error on getting Status Code: %v. ", err)
			}

			if statusCode == 500 {
				if err := limiter.RequestCancelled(requestWeight); err != nil {
					t.Errorf("Error on Request Cancelled: %v. ", err)
				}

			} else if statusCode == 200 {
				if err := limiter.RequestFinished(requestWeight); err != nil {
					t.Errorf("Error on Request Finished: %v. ", err)
				}
				numOfRequests -= requestWeight
			} else {
				if err := limiter.RequestFinished(requestWeight); err != nil {
					t.Errorf("Error on Request Finished: %v. ", err)
				}
				fmt.Printf("Routine: %v. %v. %v, \n", id, statusCode, limiter.status)
				t.Errorf("Routine: %v. %v. ", id, statusCode)
			}

		} else if sleepTime != 0 {
			time.Sleep(time.Duration(sleepTime) * time.Millisecond)
		}
	}

	fmt.Print(".")
	c <- "done"
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
		expected      requestsStatus
	}

	config := NewRateLimitConfig("testHost", 0, 0, 0, 0)

	testCases := []TestRequestStatus{
		{
			2,
			Limiter{newRequestsStatus(0, 0, 10, 0, 0), config, pool},
			newRequestsStatus(0, 0, 8, 0, 0),
		},
		{
			1,
			Limiter{newRequestsStatus(0, 0, 3, 0, 0), config, pool},
			newRequestsStatus(0, 0, 2, 0, 0),
		},
		{
			5,
			Limiter{newRequestsStatus(0, 0, 10, 0, 0), config, pool},
			newRequestsStatus(0, 0, 5, 0, 0),
		},
	}

	key := testCases[0].limiter.getStatusKey()

	for i := 0; i < len(testCases); i++ {
		l := testCases[i].limiter

		err := pool.Do(radix.WithConn(key, func(c radix.Conn) error {
			err = pool.Do(radix.FlatCmd(nil, "HSET",
				key,
				sustainedRequests, l.status.sustainedRequests,
				burstRequests, l.status.burstRequests,
				pendingRequests, l.status.pendingRequests,
				firstSustainedRequest, l.status.firstSustainedRequest,
				firstBurstRequest, l.status.firstBurstRequest,
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
		expected      requestsStatus
	}

	config := NewRateLimitConfig("testHost", 0, 0, 0, 0)

	testCases := []TestRequestStatus{
		{
			2,
			Limiter{newRequestsStatus(5, 2, 10, 0, 0), config, pool},
			newRequestsStatus(7, 4, 8, 0, 0),
		},
		{
			1,
			Limiter{newRequestsStatus(0, 40, 3, 0, 0), config, pool},
			newRequestsStatus(1, 41, 2, 0, 0),
		},
		{
			5,
			Limiter{newRequestsStatus(35, 0, 10, 0, 0), config, pool},
			newRequestsStatus(40, 5, 5, 0, 0),
		},
	}

	key := testCases[0].limiter.getStatusKey()

	for i := 0; i < len(testCases); i++ {
		l := testCases[i].limiter

		err := pool.Do(radix.WithConn(key, func(c radix.Conn) error {
			err = pool.Do(radix.FlatCmd(nil, "HSET",
				key,
				sustainedRequests, l.status.sustainedRequests,
				burstRequests, l.status.burstRequests,
				pendingRequests, l.status.pendingRequests,
				firstSustainedRequest, l.status.firstSustainedRequest,
				firstBurstRequest, l.status.firstBurstRequest,
			))
			if err != nil {
				return err
			}

			if err := l.RequestFinished(testCases[i].requestWeight); err != nil {
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
	config := NewRateLimitConfig("testHost", 0, 0, 0, 0)

	testCases := []Limiter{
		{newRequestsStatus(5, 2, 10, 2126523, 2343), config, pool},
		{newRequestsStatus(0, 40, 3, 236436, 0), config, pool},
		{newRequestsStatus(35, 0, 10, 0, 9545456), config, pool},
	}

	key := testCases[0].getStatusKey()

	for i := 0; i < len(testCases); i++ {
		l := testCases[i]
		newLimiter, err := NewLimiter(config, pool)

		err = pool.Do(radix.WithConn(key, func(c radix.Conn) error {
			err = c.Do(radix.FlatCmd(nil, "HSET",
				key,
				sustainedRequests, l.status.sustainedRequests,
				burstRequests, l.status.burstRequests,
				pendingRequests, l.status.pendingRequests,
				firstSustainedRequest, l.status.firstSustainedRequest,
				firstBurstRequest, l.status.firstBurstRequest,
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
