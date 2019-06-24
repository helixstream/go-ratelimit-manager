package host

import (
	"context"
	"fmt"
	"github.com/go-test/deep"
	"github.com/mediocregopher/radix"
	"math/rand"
	"net/http"
	url2 "net/url"
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

	err = pool.Do(radix.FlatCmd(nil, "HSET",
		"status:"+serverConfig.Host,
		host, serverConfig.Host,
		sustainedRequests, 0,
		burstRequests, 0,
		pendingRequests, 0,
		firstSustainedRequest, 0,
		firstBurstRequest, 0,
	))

	rand.Seed(time.Now().Unix())
	channel := make(chan string)

	numOfRoutines := 200

	server := getServer()

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

	numOfRequests := 2

	for numOfRequests > 0 {

		requestWeight := rand.Intn(4) + 1

		canMake, sleepTime := requestStatus.CanMakeRequest(pool, requestWeight, hostConfig)

		if canMake {
			statusCode, err := getStatusCode("http://127.0.0.1:" + port + "/testRateLimit")
			if err != nil {
				t.Errorf("Error on getting Status Code: %v. ", err)
			}

			if statusCode == 500 {
				if err := requestStatus.RequestCancelled(requestWeight, pool); err != nil {
					t.Errorf("Error on Request Cancelled: %v. ", err)
				}

			} else if statusCode == 200 {
				if err := requestStatus.RequestFinished(requestWeight, pool); err != nil {
					t.Errorf("Error on Request Finished: %v. ", err)
				}
				numOfRequests--
			} else {
				if err := requestStatus.RequestFinished(requestWeight, pool); err != nil {
					t.Errorf("Error on Request Finished: %v. ", err)
				}
				fmt.Printf("Routine: %v. %v. %v, ", id, statusCode, requestStatus)
				t.Errorf("Routine: %v. %v. ", id, statusCode)
			}

		} else {
			time.Sleep(time.Duration(sleepTime) * time.Millisecond)
		}
	}

	fmt.Print(".")
	c <- "done"
}

func getStatusCode(url string) (int, error) {
	resp, err := http.PostForm(url, url2.Values{"weight": {"1"}})
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

		err := pool.Do(radix.WithConn(key, func(c radix.Conn) error {
			err = pool.Do(radix.FlatCmd(nil, "HSET",
				key,
				host, s.Host,
				sustainedRequests, s.SustainedRequests,
				burstRequests, s.BurstRequests,
				pendingRequests, s.PendingRequests,
				firstSustainedRequest, s.FirstSustainedRequest,
				firstBurstRequest, s.FirstBurstRequest,
			))
			if err != nil {
				return err
			}

			if err := s.RequestCancelled(testCases[i].requestWeight, pool); err != nil {
				t.Error(err)
			}

			err = s.updateStatusFromDatabase(c, key)
			if err != nil {
				return err
			}

			return nil
		}))

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

		err := pool.Do(radix.WithConn(key, func(c radix.Conn) error {
			err = pool.Do(radix.FlatCmd(nil, "HSET",
				key,
				host, s.Host,
				sustainedRequests, s.SustainedRequests,
				burstRequests, s.BurstRequests,
				pendingRequests, s.PendingRequests,
				firstSustainedRequest, s.FirstSustainedRequest,
				firstBurstRequest, s.FirstBurstRequest,
			))
			if err != nil {
				return err
			}

			if err := s.RequestFinished(testCases[i].requestWeight, pool); err != nil {
				t.Error(err)
			}

			err = s.updateStatusFromDatabase(c, key)
			if err != nil {
				return err
			}

			return nil
		}))

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
		s := testCases[i]
		newStatus := NewRequestsStatus("testHost", 0, 0, 0, 0, 0)

		err := pool.Do(radix.WithConn(key, func(c radix.Conn) error {
			err = c.Do(radix.FlatCmd(nil, "HSET",
				key,
				host, s.Host,
				sustainedRequests, s.SustainedRequests,
				burstRequests, s.BurstRequests,
				pendingRequests, s.PendingRequests,
				firstSustainedRequest, s.FirstSustainedRequest,
				firstBurstRequest, s.FirstBurstRequest,
			))
			if err != nil {
				return err
			}

			err = newStatus.updateStatusFromDatabase(c, key)
			if err != nil {
				return err
			}

			return nil
		}))
		if err != nil {
			t.Error(err)
		}

		if diff := deep.Equal(s, newStatus); diff != nil {
			t.Errorf("Loop: %v. %v. ", i, diff)
		}
	}
}
