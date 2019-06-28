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
		sus + 0,
		int64(susPeriod),
		burst + 0,
		int64(burstPeriod),
	)

	limiter, err := NewLimiter(testConfig, pool)
	if err != nil {
		t.Error(err)
	}

	fmt.Print(limiter.config)

	for i := 0; i < numOfRoutines; i++ {
		go makeRequests(t, limiter, i, channel)
		time.Sleep(time.Duration(10) * time.Millisecond)
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

	numOfRequests := 1//rand.Intn(3) + 1
	for numOfRequests > 0 {
		requestWeight := 1//rand.Intn(2) + 1
		canMake, sleepTime := limiter.CanMakeRequest(requestWeight)
		if canMake {
			fmt.Printf("Can Make: %v, %v, %v \n", id, limiter.config, limiter.status)

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
		expected      RequestsStatus
	}

	config := NewRateLimitConfig("testHost1", 0, 0, 0, 0)

	testCases := []TestRequestStatus{
		{
			2,
			Limiter{newRequestsStatus(0, 10, 0), config, pool },
			newRequestsStatus(0, 8, 0),
		},
		{
			1,
			Limiter{newRequestsStatus(0, 3, 0), config, pool},
			newRequestsStatus(0, 2, 0),
		},
		{
			5,
			Limiter{newRequestsStatus(0, 10, 0), config, pool},
			newRequestsStatus(0, 5, 0),
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
			Limiter{newRequestsStatus(5, 2, 0), config, pool},
			newRequestsStatus(7, 0, 0),
		},
		{
			1,
			Limiter{newRequestsStatus(0, 40, 0), config, pool},
			newRequestsStatus(1, 39, 0),
		},
		{
			5,
			Limiter{newRequestsStatus(35, 5, 0), config, pool},
			newRequestsStatus(40, 0, 0),
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
	config := NewRateLimitConfig("testHost1", 1, 1, 1, 1)

	testCases := []Limiter{
		{newRequestsStatus(5, 2, 23564), config, pool},
		{newRequestsStatus(0, 40, 3454345), config, pool},
		{newRequestsStatus(35, 0, 266256), config, pool},
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
