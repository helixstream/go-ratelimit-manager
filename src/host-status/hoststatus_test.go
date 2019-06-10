package host_status

import (
	"../host-config"
	"testing"
	"time"
	//"time"
)

func Test_CanMakeRequestRecursive(t *testing.T) {
	hosts := []host_config.HostConfig{
		host_config.NewHostConfig("test_host_1", 1200, 60, 20, 1),
		host_config.NewHostConfig("test_host_2", 600, 60, 20, 1),
	}

	type HostStatusTest struct {
		host          host_config.HostConfig
		requestWeight int
		status        HostStatus
	}

	now := time.Now().UTC().Unix()

	testCases := []HostStatusTest{
		//not in sustained, not in burst
		{
			hosts[0],
			1,
			NewHostStatus("test_host", 500, 500, 0, now-hosts[0].SustainedTimePeriod-5, now-hosts[0].BurstTimePeriod-1),
		},
		{
			hosts[1],
			5,
			NewHostStatus("test_host", 500, 500, 0, now-hosts[0].SustainedTimePeriod-5, now-hosts[0].BurstTimePeriod-1),
		},
		//is in sustained, not in burst, no sustained limit, burst limit does not matter
		{
			hosts[0],
			1,
			NewHostStatus("test_host", 500, 500, 20, now, now-hosts[0].BurstTimePeriod-1),
		},
		{
			hosts[1],
			3,
			NewHostStatus("test_host", 500, 10, 20, now-30, now-hosts[0].BurstTimePeriod),
		},
		//is in sustained, not in burst, will hit sustained limit, burst limit does not matter
		{
			hosts[0],
			1,
			NewHostStatus("test_host", 1195, 5, 5, now-(hosts[0].SustainedTimePeriod/2), now-hosts[0].BurstTimePeriod-1),
		},
		{
			hosts[1],
			5,
			NewHostStatus("test_host", 597, 5, 5, now-hosts[0].SustainedTimePeriod+1, now-hosts[0].BurstTimePeriod-1),
		},
		//is in sustained, is in burst, will hit burst limit, sustained limit does not matter
		{
			hosts[0],
			1,
			NewHostStatus("test_host", 1000, 18, 2, now-(hosts[0].SustainedTimePeriod/2), now),
		},
		{
			hosts[1],
			5,
			NewHostStatus("test_host", 1000, 15, 1, now-(hosts[0].SustainedTimePeriod/2), now),
		},
		//is in sustained, is in burst, will hit sustained limit, will not hit burst limit
		{
			hosts[0],
			1,
			NewHostStatus("test_host", 1195, 8, 6, now-(hosts[0].SustainedTimePeriod/2), now),
		},
		{
			hosts[1],
			1,
			NewHostStatus("test_host", 1195, 8, 6, now-(hosts[0].SustainedTimePeriod/2), now),
		},
		//is in sustained, is in burst, will not hit either limit
		{
			hosts[0],
			1,
			NewHostStatus("test_host", 1000, 10, 4, now-(hosts[0].SustainedTimePeriod/2), now),
		},
		{
			hosts[1],
			3,
			NewHostStatus("test_host", 400, 10, 4, now-(hosts[0].SustainedTimePeriod/2), now),
		},
	}

	for i := 0; i < len(testCases); i++ {
		canMake := testCases[i].status.CheckRequest(testCases[i].requestWeight, testCases[i].host)

		if !canMake {
			t.Errorf("Loop: %v. Expected true for Can Make Request Recursive, got: %v", i, canMake)
		}

	}
}

func Test_CanMakeRequest(t *testing.T) {
	hosts := []host_config.HostConfig{
		host_config.NewHostConfig("test_host_1", 1200, 60, 20, 1),
		host_config.NewHostConfig("test_host_2", 600, 60, 20, 1),
	}

	type HostStatusTest struct {
		host                   host_config.HostConfig
		requestWeight          int
		status                 HostStatus
		expectedStatus         HostStatus
		expectedCanMakeRequest bool
	}

	now := time.Now().UTC().Unix()

	testCases := []HostStatusTest{
		//not is sustained, not in burst, no sus limit, no burst limit
		{
			hosts[0],
			1,
			NewHostStatus("test_host", 500, 500, 0, now-hosts[0].SustainedTimePeriod-5, now-hosts[0].BurstTimePeriod-1),
			NewHostStatus("test_host", 0, 0, 1, now, now),
			true,
		},
		{
			hosts[1],
			5,
			NewHostStatus("test_host", 500, 500, 0, now-hosts[0].SustainedTimePeriod-5, now-hosts[0].BurstTimePeriod-1),
			NewHostStatus("test_host", 0, 0, 5, now, now),
			true,
		},
		//is in sustained, not in burst, no sustained limit, burst limit does not matter
		{
			hosts[0],
			1,
			NewHostStatus("test_host", 500, 500, 20, now, now-hosts[0].BurstTimePeriod-1),
			NewHostStatus("test_host", 500, 0, 21, now, now),
			true,
		},
		{
			hosts[1],
			3,
			NewHostStatus("test_host", 500, 10, 20, now-30, now-hosts[0].BurstTimePeriod),
			NewHostStatus("test_host", 500, 0, 23, now-30, now),
			true,
		},
		//is in sustained, not in burst, will hit sustained limit, burst limit does not matter
		{
			hosts[0],
			1,
			NewHostStatus("test_host", 1195, 5, 5, now-(hosts[0].SustainedTimePeriod/2), now-hosts[0].BurstTimePeriod-1),
			NewHostStatus("test_host", 1195, 0, 5, now-(hosts[0].SustainedTimePeriod/2), now),
			false,
		},
		{
			hosts[1],
			5,
			NewHostStatus("test_host", 597, 5, 5, now-hosts[0].SustainedTimePeriod+1, now-hosts[0].BurstTimePeriod-1),
			NewHostStatus("test_host", 597, 0, 5, now-hosts[0].SustainedTimePeriod+1, now),
			false,
		},
		//is in sustained, is in burst, will hit burst limit, sustained limit does not matter
		{
			hosts[0],
			1,
			NewHostStatus("test_host", 1000, 18, 2, now-(hosts[0].SustainedTimePeriod/2), now),
			NewHostStatus("test_host", 1000, 18, 2, now-(hosts[0].SustainedTimePeriod/2), now),
			false,
		},
		{
			hosts[1],
			5,
			NewHostStatus("test_host", 1000, 15, 1, now-(hosts[0].SustainedTimePeriod/2), now),
			NewHostStatus("test_host", 1000, 15, 1, now-(hosts[0].SustainedTimePeriod/2), now),
			false,
		},
		//is in sustained, is in burst, will hit sustained limit, will not hit burst limit
		{
			hosts[0],
			1,
			NewHostStatus("test_host", 1195, 8, 6, now-(hosts[0].SustainedTimePeriod/2), now),
			NewHostStatus("test_host", 1195, 8, 6, now-(hosts[0].SustainedTimePeriod/2), now),
			false,
		},
		{
			hosts[1],
			1,
			NewHostStatus("test_host", 1195, 8, 6, now-(hosts[0].SustainedTimePeriod/2), now),
			NewHostStatus("test_host", 1195, 8, 6, now-(hosts[0].SustainedTimePeriod/2), now),
			false,
		},
		//is in sustained, is in burst, will not hit either limit
		{
			hosts[0],
			1,
			NewHostStatus("test_host", 1000, 10, 4, now-(hosts[0].SustainedTimePeriod/2), now),
			NewHostStatus("test_host", 1000, 10, 5, now-(hosts[0].SustainedTimePeriod/2), now),
			true,
		},
		{
			hosts[1],
			3,
			NewHostStatus("test_host", 400, 10, 4, now-(hosts[0].SustainedTimePeriod/2), now),
			NewHostStatus("test_host", 400, 10, 7, now-(hosts[0].SustainedTimePeriod/2), now),
			true,
		},
	}

	for i := 0; i < len(testCases); i++ {
		canMake, _ := testCases[i].status.CanMakeRequest(testCases[i].requestWeight, now, testCases[i].host)
		if canMake != testCases[i].expectedCanMakeRequest {
			//error if the boolean the function returns does not match the expected value
			t.Errorf("Loop: %v. Expected ability to make request: %v, got: %v", i, testCases[i].expectedCanMakeRequest, canMake)
		}
		if testCases[i].status != testCases[i].expectedStatus {
			expected := testCases[i].expectedStatus
			status := testCases[i].status
			//causes an error if the expected host status is different than the modified host status
			t.Errorf("Loop: %v, Expected %v sustained requests, got: %v. Expected %v burst requests, got: %v. Expected %v pending requests, got: %v. Expected %v first sustained request, got: %v, Expected %v first burst request, got; %v",
				i, expected.GetSustainedRequests(), status.GetSustainedRequests(), expected.GetBurstRequests(), status.GetBurstRequests(), expected.GetPendingRequests(), status.GetPendingRequests(), expected.GetFirstSustainedRequest(),
				status.GetFirstSustainedRequest(), expected.GetFirstBurstRequest(), status.GetFirstBurstRequest())
		}

	}
}

func TestHostStatus_IsInSustainedPeriod(t *testing.T) {
	hosts := []host_config.HostConfig{
		host_config.NewHostConfig("test_host_1", 0, 60, 0, 0),
		host_config.NewHostConfig("test_host_2", 0, 60, 0, 0),
		host_config.NewHostConfig("test_host_3", 0, 45, 0, 0),
	}

	type HostStatusTest struct {
		host     host_config.HostConfig
		status   HostStatus
		expected bool
	}

	now := time.Now().UTC().Unix()

	testCases := []HostStatusTest{
		{
			hosts[0],
			NewHostStatus("test_host", 0, 0, 0, now-hosts[0].SustainedTimePeriod-5, 0),
			false,
		},
		{
			hosts[0],

			NewHostStatus("test_host", 0, 0, 0, now, 0),
			true,
		},
		{
			hosts[0],

			NewHostStatus("test_host", 0, 0, 0, now-(hosts[0].SustainedTimePeriod), 0),
			false,
		},
		{
			hosts[0],

			NewHostStatus("test_host", 0, 0, 0, now-(hosts[0].SustainedTimePeriod)-1, 0),
			false,
		},
		{
			hosts[1],

			NewHostStatus("test_host", 0, 0, 0, now-hosts[0].SustainedTimePeriod-5, 0),
			false,
		},
		{
			hosts[1],

			NewHostStatus("test_host", 0, 0, 0, now-(hosts[0].SustainedTimePeriod/7), 0),
			true,
		},
		{
			hosts[2],

			NewHostStatus("test_host", 0, 0, 0, now-(hosts[0].SustainedTimePeriod/2), 0),
			true,
		},
	}

	for i := 0; i < len(testCases); i++ {
		result := testCases[i].status.IsInSustainedPeriod(testCases[i].host, now)
		if result != testCases[i].expected {
			t.Errorf("Loop: %v. Expected %v for is in sustained period, got: %v.", i, testCases[i].expected, result)
		}
	}
}

func TestHostStatus_IsInBurstPeriod(t *testing.T) {
	hosts := []host_config.HostConfig{
		host_config.NewHostConfig("test_host_1", 0, 0, 0, 1),
		host_config.NewHostConfig("test_host_2", 0, 0, 0, 5),
		host_config.NewHostConfig("test_host_3", 0, 0, 0, 2),
	}

	type HostStatusTest struct {
		host     host_config.HostConfig
		status   HostStatus
		expected bool
	}

	now := time.Now().UTC().Unix()

	testCases := []HostStatusTest{
		{
			hosts[0],
			NewHostStatus("test_host", 0, 0, 0, 0, now-hosts[0].BurstTimePeriod-1),
			false,
		},
		{
			hosts[0],
			NewHostStatus("test_host", 0, 0, 0, 0, now-hosts[0].BurstTimePeriod-1),
			false,
		},
		{
			hosts[0],
			NewHostStatus("test_host", 0, 0, 0, 0, now),
			true,
		},
		{
			hosts[0],
			NewHostStatus("test_host", 0, 0, 0, 0, now),
			true,
		},
		{
			hosts[1],
			NewHostStatus("test_host", 0, 0, 0, 0, now-3),
			true,
		},
		{
			hosts[1],
			NewHostStatus("test_host", 0, 0, 0, 0, now-hosts[1].BurstTimePeriod-1),
			false,
		},
		{
			hosts[2],
			NewHostStatus("test_host", 0, 0, 0, 0, now-1),
			true,
		},
	}

	for i := 0; i < len(testCases); i++ {
		result := testCases[i].status.IsInBurstPeriod(testCases[i].host, now)
		if result != testCases[i].expected {
			t.Errorf("Loop: %v. Expected %v for is in burst period, got: %v.", i, testCases[i].expected, result)
		}
	}
}

func TestHostStatus_WillHitSustainedLimit(t *testing.T) {
	hosts := []host_config.HostConfig{
		host_config.NewHostConfig("test_host_1", 1200, 0, 0, 0),
		host_config.NewHostConfig("test_host_2", 600, 0, 0, 0),
		host_config.NewHostConfig("test_host_3", 100, 0, 0, 0),
	}

	type TestHostStatus struct {
		host     host_config.HostConfig
		weight   int
		status   HostStatus
		expected bool
	}

	testCases := []TestHostStatus{
		{
			hosts[0],
			1,
			NewHostStatus("test_host", 1200, 0, 0, 0, 0),
			true,
		},
		{
			hosts[1],
			7,
			NewHostStatus("test_host", 595, 0, 1, 0, 0),
			true,
		},
		{
			hosts[2],
			9,
			NewHostStatus("test_host", 90, 0, 2, 0, 0),
			true,
		},
		{
			hosts[0],
			20,
			NewHostStatus("test_host", 1100, 0, 80, 0, 0),
			false,
		},
		{
			hosts[1],
			1,
			NewHostStatus("test_host", 35, 0, 40, 0, 0),
			false,
		},
		{
			hosts[2],
			15,
			NewHostStatus("test_host", 85, 0, 0, 0, 0),
			false,
		},
	}

	for i := 0; i < len(testCases); i++ {
		result := testCases[i].status.WillHitSustainedLimit(testCases[i].weight, testCases[i].host)
		if result != testCases[i].expected {
			t.Errorf("Loop: %v. Expected %v for will hit sustained limit, got: %v.", i, testCases[i].expected, result)
		}
	}
}

func TestHostStatus_WillHitBurstLimit(t *testing.T) {
	hosts := []host_config.HostConfig{
		host_config.NewHostConfig("test_host_1", 0, 0, 20, 0),
		host_config.NewHostConfig("test_host_2", 0, 0, 5, 0),
		host_config.NewHostConfig("test_host_3", 0, 0, 10, 0),
	}

	type TestHostStatus struct {
		host     host_config.HostConfig
		weight   int
		status   HostStatus
		expected bool
	}

	testCases := []TestHostStatus{
		{
			hosts[0],
			1,
			NewHostStatus("testHost", 0, 18, 2, 0, 0),
			true,
		},
		{
			hosts[1],
			3,
			NewHostStatus("testHost", 0, 3, 0, 0, 0),
			true,
		},
		{
			hosts[2],
			5,
			NewHostStatus("testHost", 0, 5, 1, 0, 0),
			true,
		},
		{
			hosts[0],
			1,
			NewHostStatus("testHost", 0, 10, 8, 0, 0),
			false,
		},
		{
			hosts[1],
			1,
			NewHostStatus("testHost", 0, 0, 0, 0, 0),
			false,
		},
		{
			hosts[2],
			1,
			NewHostStatus("testHost", 0, 9, 0, 0, 0),
			false,
		},
	}

	for i := 0; i < len(testCases); i++ {
		result := testCases[i].status.WillHitBurstLimit(testCases[i].weight, testCases[i].host)
		if result != testCases[i].expected {
			t.Errorf("Loop: %v. Expected %v for will hit burst limit, got %v. ", i, testCases[i].expected, result)
		}
	}
}

