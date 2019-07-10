package limiter

import (
	"testing"

	"github.com/go-test/deep"
)

func Test_CanMakeRequestLogic(t *testing.T) {
	host := NewRateLimitConfig("test_host_1", 1200, 60, 20, 1)

	type HostStatusTest struct {
		name                   string
		host                   RateLimitConfig
		requestWeight          int
		status                 RequestsStatus
		expectedStatus         RequestsStatus
		expectedCanMakeRequest bool
	}
	//runs the test 10 times so ensure that firstBurstRequest is always within an acceptable range
	for i := 0; i < 10; i++ {

		now := getUnixTimeMilliseconds()

		testCases := []HostStatusTest{
			{
				"in period, will hit limit",
				host,
				1,
				newRequestsStatus(10, 10, now, 0),
				newRequestsStatus(10, 10, now, 0),
				false,
			},
			{
				"in period, will not hit limit, enough time has passed",
				host,
				5,
				newRequestsStatus(0, 0, now, 0),
				newRequestsStatus(0, 5, now, 0),
				true,
			},
			{
				"in period, will not hit limit, not enough time has passed",
				host,
				1,
				newRequestsStatus(10, 3, now, 0),
				newRequestsStatus(10, 3, now, 0),
				false,
			},
			{
				"not in period",
				host,
				3,
				newRequestsStatus(16, 0, 0, 0),
				newRequestsStatus(0, 0, now, 0),
				true,
			},
		}

		for i := 0; i < len(testCases); i++ {
			t.Run(testCases[i].name, func(t *testing.T) {

				status := testCases[i].status
				expected := testCases[i].expectedStatus
				canMake, _ := status.canMakeRequestLogic(testCases[i].requestWeight, testCases[i].host)

				if canMake != testCases[i].expectedCanMakeRequest {
					//error if the boolean the function returns does not match the expected value
					t.Errorf("Loop: %v. Expected ability to make request: %v, got: %v", i, testCases[i].expectedCanMakeRequest, canMake)
				}

				if diff := deep.Equal(status, expected); diff != nil {
					//because firstBurstRequest and firstSustainedRequest are millisecond timestamps, they are too small of a unit to predict exactly
					//this line makes sure that firstBurstRequest and firstSustainedRequest is within a range of 20ms
					if status.firstRequest-expected.firstRequest > 20 {
						t.Errorf("Loop: %v. %v", i, diff)
					}
				}

			})

		}
	}
}

func Test_IsInSustainedPeriod(t *testing.T) {
	hosts := []RateLimitConfig{
		NewRateLimitConfig("test_host_1", 20, 60, 20, 1),
		NewRateLimitConfig("test_host_2", 30, 60, 20, 1),
		NewRateLimitConfig("test_host_3", 20, 45, 20, 1),
	}

	type HostStatusTest struct {
		host     RateLimitConfig
		status   RequestsStatus
		expected bool
	}

	now := getUnixTimeMilliseconds()

	testCases := []HostStatusTest{
		{
			hosts[0],
			newRequestsStatus(0, 0, now-(hosts[0].timePeriod*1000), 0),
			false,
		},
		{
			hosts[0],
			newRequestsStatus(0, 0, now, 0),
			true,
		},
		{
			hosts[0],
			newRequestsStatus(0, 0, now-(hosts[0].timePeriod*1000), 0),
			false,
		},
		{
			hosts[0],
			newRequestsStatus(0, 0, now-(hosts[0].timePeriod*1000)-100, 0),
			false,
		},
		{
			hosts[1],
			newRequestsStatus(0, 0, now-(hosts[1].timePeriod*1000)-50, 0),
			false,
		},
		{
			hosts[1],
			newRequestsStatus(0, 0, now-(hosts[1].timePeriod*1000)/7, 0),
			true,
		},
		{
			hosts[2],
			newRequestsStatus(0, 0, now-(hosts[0].timePeriod*1000)/2, 0),
			true,
		},
	}

	for i := 0; i < len(testCases); i++ {
		result := testCases[i].status.isInPeriod(now, testCases[i].host)
		if result != testCases[i].expected {
			t.Errorf("Loop: %v. Expected %v for is in sustained period, got: %v.", i, testCases[i].expected, result)
		}
	}
}

func Test_WillHitLimit(t *testing.T) {
	hosts := []RateLimitConfig{
		NewRateLimitConfig("test_host_1", 1200, 60, 30, 1),
		NewRateLimitConfig("test_host_2", 600, 60, 30, 1),
		NewRateLimitConfig("test_host_3", 100, 10, 300, 1),
	}

	type TestHostStatus struct {
		host     RateLimitConfig
		weight   int
		status   RequestsStatus
		expected bool
	}

	testCases := []TestHostStatus{
		{
			hosts[0],
			1,
			newRequestsStatus(1200, 0, 0, 0),
			true,
		},
		{
			hosts[1],
			7,
			newRequestsStatus(595, 1, 1, 0),
			true,
		},
		{
			hosts[2],
			9,
			newRequestsStatus(90, 2, 2, 0),
			true,
		},
		{
			hosts[0],
			7,
			newRequestsStatus(2, 6, 80, 0),
			false,
		},
		{
			hosts[1],
			1,
			newRequestsStatus(5, 4, 40, 0),
			false,
		},
		{
			hosts[2],
			3,
			newRequestsStatus(3, 3, 0, 0),
			false,
		},
	}

	for i := 0; i < len(testCases); i++ {
		result := testCases[i].status.willHitLimit(testCases[i].weight, testCases[i].host)
		if result != testCases[i].expected {
			t.Errorf("Loop: %v. Expected %v for will hit sustained limit, got: %v.", i, testCases[i].expected, result)
		}
	}
}
