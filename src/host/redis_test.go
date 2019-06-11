package host

import (
	"github.com/go-test/deep"
	"github.com/gomodule/redigo/redis"
	"log"
	"math/rand"
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

	if resp != true{
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

func Test_SaveGetStatus(t *testing.T) {

	type TestRequestStatus struct {
		host   string
		status RequestsStatus
	}

	rand.Seed(time.Now().UnixNano())

	min := 1
	max := 200

	testCases := []TestRequestStatus{
		{
			"testHost4",
			NewRequestsStatus("testHost4", rand.Intn(max-min)+min, rand.Intn(max-min)+min, rand.Intn(max-min)+min, 40, 50),
		},
		{
			"testHost5",
			NewRequestsStatus("testHost5", rand.Intn(max-min)+min, rand.Intn(max-min)+min, rand.Intn(max-min)+min, 0, 90),
		},
		{
			"testHost6",
			NewRequestsStatus("testHost6", rand.Intn(max-min)+min, rand.Intn(max-min)+min, rand.Intn(max-min)+min, 450, 0),
		},
	}

	for i := 0; i < len(testCases); i++ {
		testCases[i].status.SaveStatus(pool)

		newStatus := NewRequestsStatus(testCases[i].host, 0, 0, 0, 0, 0)

		newStatus.GetStatus(pool)

		if diff := deep.Equal(testCases[i].status, newStatus); diff != nil {
			t.Errorf("Loop: %v. %v", i, diff)
		}
	}

}
