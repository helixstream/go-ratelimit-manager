package host

import (
	"github.com/go-test/deep"
	"testing"
)

func Test_CanMakeRequestLogic(t *testing.T) {
	hosts := []RateLimitConfig{
		NewRateLimitConfig("test_host_1", 1200, 60, 20, 1),
		NewRateLimitConfig("test_host_2", 600, 60, 20, 1),
	}

	type HostStatusTest struct {
		name                   string
		host                   RateLimitConfig
		requestWeight          int
		status                 requestsStatus
		expectedStatus         requestsStatus
		expectedCanMakeRequest bool
	}
	//runs the test 10 times so ensure that firstBurstRequest is always within an acceptable range
	for i := 0; i < 10; i++ {

		now := getUnixTimeMilliseconds()

		testCases := []HostStatusTest{
			{
				"not is sustained, not in burst, no sus limit, no burst limit",
				hosts[0],
				1,
				newRequestsStatus(500, 500, 0, now-(hosts[0].sustainedTimePeriod*1000)-50, now-(hosts[0].burstTimePeriod*1000)-100),
				newRequestsStatus(0, 0, 1, now, now),
				true,
			},
			{
				"not is sustained, not in burst, no sus limit, no burst limit",
				hosts[1],
				5,
				newRequestsStatus(500, 500, 0, now-(hosts[1].sustainedTimePeriod*1000)-500, now-(hosts[0].burstTimePeriod*1000)-100),
				newRequestsStatus(0, 0, 5, now, now),
				true,
			},
			{
				"is in sustained, not in burst, no sustained limit, burst limit does not matter",
				hosts[0],
				1,
				newRequestsStatus(500, 500, 20, now, now-(hosts[0].burstTimePeriod*1000)-100),
				newRequestsStatus(500, 0, 21, now, now),
				true,
			},
			{
				"is in sustained, not in burst, no sustained limit, burst limit does not matter",
				hosts[1],
				3,
				newRequestsStatus(500, 10, 20, now-30000, now-(hosts[0].burstTimePeriod*1000)),
				newRequestsStatus(500, 0, 23, now-30000, now),
				true,
			},
			{
				"is in sustained, not in burst, will hit sustained limit, burst limit does not matter",
				hosts[0],
				1,
				newRequestsStatus(1195, 5, 5, now-((hosts[0].sustainedTimePeriod*1000)/2), now-(hosts[0].burstTimePeriod*1000)-100),
				newRequestsStatus(1195, 0, 5, now-((hosts[0].sustainedTimePeriod*1000)/2), now),
				false,
			},
			{
				"is in sustained, not in burst, will hit sustained limit, burst limit does not matter",
				hosts[1],
				5,
				newRequestsStatus(597, 5, 5, now-(hosts[1].sustainedTimePeriod*1000)+100, now-(hosts[0].burstTimePeriod*1000)-100),
				newRequestsStatus(597, 0, 5, now-(hosts[0].sustainedTimePeriod*1000)+100, now),
				false,
			},
			{
				"is in sustained, is in burst, will hit burst limit, sustained limit does not matter",
				hosts[0],
				1,
				newRequestsStatus(1000, 18, 2, now-((hosts[0].sustainedTimePeriod*1000)/2), now),
				newRequestsStatus(1000, 18, 2, now-((hosts[0].sustainedTimePeriod*1000)/2), now),
				false,
			},
			{
				"is in sustained, is in burst, will hit burst limit, sustained limit does not matter",
				hosts[1],
				5,
				newRequestsStatus(1000, 15, 1, now-((hosts[1].sustainedTimePeriod*1000)/2), now),
				newRequestsStatus(1000, 15, 1, now-((hosts[0].sustainedTimePeriod*1000)/2), now),
				false,
			},
			{
				"is in sustained, is in burst, will hit sustained limit, will not hit burst limit",
				hosts[0],
				1,
				newRequestsStatus(1195, 8, 6, now-((hosts[0].sustainedTimePeriod*1000)/2), now),
				newRequestsStatus(1195, 8, 6, now-((hosts[0].sustainedTimePeriod*1000)/2), now),
				false,
			},
			{
				"is in sustained, is in burst, will hit sustained limit, will not hit burst limit",
				hosts[1],
				1,
				newRequestsStatus(1195, 8, 6, now-((hosts[1].sustainedTimePeriod*1000)/2), now),
				newRequestsStatus(1195, 8, 6, now-((hosts[0].sustainedTimePeriod*1000)/2), now),
				false,
			},
			{
				"is in sustained, is in burst, will not hit either limit",
				hosts[0],
				1,
				newRequestsStatus(1000, 10, 4, now-((hosts[0].sustainedTimePeriod*1000)/2), now),
				newRequestsStatus(1000, 10, 5, now-((hosts[0].sustainedTimePeriod*1000)/2), now),
				true,
			},
			{
				"is in sustained, is in burst, will not hit either limit",
				hosts[1],
				3,
				newRequestsStatus(400, 10, 4, now-((hosts[0].sustainedTimePeriod*1000)/2), now),
				newRequestsStatus(400, 10, 7, now-((hosts[1].sustainedTimePeriod*1000)/2), now),
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
					if status.firstBurstRequest-expected.firstBurstRequest > 20 && status.firstSustainedRequest-expected.firstSustainedRequest > 20 {
						t.Errorf("Loop: %v. %v", i, diff)
					}
				}

			})

		}
	}
}

func Test_IsInSustainedPeriod(t *testing.T) {
	hosts := []RateLimitConfig{
		NewRateLimitConfig("test_host_1", 0, 60, 0, 0),
		NewRateLimitConfig("test_host_2", 0, 60, 0, 0),
		NewRateLimitConfig("test_host_3", 0, 45, 0, 0),
	}

	type HostStatusTest struct {
		host     RateLimitConfig
		status   requestsStatus
		expected bool
	}

	now := getUnixTimeMilliseconds()

	testCases := []HostStatusTest{
		{
			hosts[0],
			newRequestsStatus(0, 0, 0, now-(hosts[0].sustainedTimePeriod*1000), 0),
			false,
		},
		{
			hosts[0],
			newRequestsStatus(0, 0, 0, now, 0),
			true,
		},
		{
			hosts[0],
			newRequestsStatus(0, 0, 0, now-(hosts[0].sustainedTimePeriod*1000), 0),
			false,
		},
		{
			hosts[0],
			newRequestsStatus(0, 0, 0, now-(hosts[0].sustainedTimePeriod*1000)-100, 0),
			false,
		},
		{
			hosts[1],
			newRequestsStatus(0, 0, 0, now-(hosts[1].sustainedTimePeriod*1000)-50, 0),
			false,
		},
		{
			hosts[1],
			newRequestsStatus(0, 0, 0, now-(hosts[1].sustainedTimePeriod*1000)/7, 0),
			true,
		},
		{
			hosts[2],
			newRequestsStatus(0, 0, 0, now-(hosts[0].sustainedTimePeriod*1000)/2, 0),
			true,
		},
	}

	for i := 0; i < len(testCases); i++ {
		result := testCases[i].status.isInSustainedPeriod(now, testCases[i].host)
		if result != testCases[i].expected {
			t.Errorf("Loop: %v. Expected %v for is in sustained period, got: %v.", i, testCases[i].expected, result)
		}
	}
}

func Test_IsInBurstPeriod(t *testing.T) {
	hosts := []RateLimitConfig{
		NewRateLimitConfig("test_host_1", 0, 0, 0, 1),
		NewRateLimitConfig("test_host_2", 0, 0, 0, 5),
		NewRateLimitConfig("test_host_3", 0, 0, 0, 2),
	}

	type HostStatusTest struct {
		host     RateLimitConfig
		status   requestsStatus
		expected bool
	}

	now := getUnixTimeMilliseconds()

	testCases := []HostStatusTest{
		{
			hosts[0],
			newRequestsStatus(0, 0, 0, 0, now-(hosts[0].burstTimePeriod*1000)-300),
			false,
		},
		{
			hosts[0],
			newRequestsStatus(0, 0, 0, 0, now-(hosts[0].burstTimePeriod*1000)-100),
			false,
		},
		{
			hosts[0],
			newRequestsStatus(0, 0, 0, 0, now),
			true,
		},
		{
			hosts[0],
			newRequestsStatus(0, 0, 0, 0, now),
			true,
		},
		{
			hosts[1],
			newRequestsStatus(0, 0, 0, 0, now-3000),
			true,
		},
		{
			hosts[1],
			newRequestsStatus(0, 0, 0, 0, now-(hosts[1].burstTimePeriod*1000)-100),
			false,
		},
		{
			hosts[2],
			newRequestsStatus(0, 0, 0, 0, now-1000),
			true,
		},
	}

	for i := 0; i < len(testCases); i++ {
		result := testCases[i].status.isInBurstPeriod(now, testCases[i].host)

		if result != testCases[i].expected {
			t.Errorf("Loop: %v. Expected %v for is in burst period, got: %v.", i, testCases[i].expected, result)
		}
	}
}

func Test_WillHitSustainedLimit(t *testing.T) {
	hosts := []RateLimitConfig{
		NewRateLimitConfig("test_host_1", 1200, 0, 0, 0),
		NewRateLimitConfig("test_host_2", 600, 0, 0, 0),
		NewRateLimitConfig("test_host_3", 100, 0, 0, 0),
	}

	type TestHostStatus struct {
		host     RateLimitConfig
		weight   int
		status   requestsStatus
		expected bool
	}

	testCases := []TestHostStatus{
		{
			hosts[0],
			1,
			newRequestsStatus(1200, 0, 0, 0, 0),
			true,
		},
		{
			hosts[1],
			7,
			newRequestsStatus(595, 0, 1, 0, 0),
			true,
		},
		{
			hosts[2],
			9,
			newRequestsStatus(90, 0, 2, 0, 0),
			true,
		},
		{
			hosts[0],
			20,
			newRequestsStatus(1100, 0, 80, 0, 0),
			false,
		},
		{
			hosts[1],
			1,
			newRequestsStatus(35, 0, 40, 0, 0),
			false,
		},
		{
			hosts[2],
			15,
			newRequestsStatus(85, 0, 0, 0, 0),
			false,
		},
	}

	for i := 0; i < len(testCases); i++ {
		result := testCases[i].status.willHitSustainedLimit(testCases[i].weight, testCases[i].host)
		if result != testCases[i].expected {
			t.Errorf("Loop: %v. Expected %v for will hit sustained limit, got: %v.", i, testCases[i].expected, result)
		}
	}
}

func Test_WillHitBurstLimit(t *testing.T) {
	hosts := []RateLimitConfig{
		NewRateLimitConfig("test_host_1", 0, 0, 20, 0),
		NewRateLimitConfig("test_host_2", 0, 0, 5, 0),
		NewRateLimitConfig("test_host_3", 0, 0, 10, 0),
	}

	type TestHostStatus struct {
		host     RateLimitConfig
		weight   int
		status   requestsStatus
		expected bool
	}

	testCases := []TestHostStatus{
		{
			hosts[0],
			1,
			newRequestsStatus(0, 18, 2, 0, 0),
			true,
		},
		{
			hosts[1],
			3,
			newRequestsStatus(0, 3, 0, 0, 0),
			true,
		},
		{
			hosts[2],
			5,
			newRequestsStatus(0, 5, 1, 0, 0),
			true,
		},
		{
			hosts[0],
			1,
			newRequestsStatus(0, 10, 8, 0, 0),
			false,
		},
		{
			hosts[1],
			1,
			newRequestsStatus(0, 0, 0, 0, 0),
			false,
		},
		{
			hosts[2],
			1,
			newRequestsStatus(0, 9, 0, 0, 0),
			false,
		},
	}

	for i := 0; i < len(testCases); i++ {
		result := testCases[i].status.willHitBurstLimit(testCases[i].weight, testCases[i].host)
		if result != testCases[i].expected {
			t.Errorf("Loop: %v. Expected %v for will hit burst limit, got %v. ", i, testCases[i].expected, result)
		}
	}
}
