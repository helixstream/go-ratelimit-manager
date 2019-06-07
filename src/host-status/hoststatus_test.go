package host_status

import (
	"../host-config"
	"testing"
	"time"

	//"time"
)

func Test_CanMakeRequest(t *testing.T) {
	hosts := []host_config.HostConfig{
		//type ==
		host_config.NewHostConfig("test_host_1", 1200, 60, 20, 1),
		//type burst/period > sustained/period
		host_config.NewHostConfig("test_host_2", 600, 60, 20, 1),
		//type burst/period < sustained/period
		//host_config.NewHostConfig("test_host_3", 1200, 60, 10, 1),
	}

	type HostStatusTest struct {
		host host_config.HostConfig
		requestWeight int
		status HostStatus
		expectedStatus HostStatus
		expectedCanMakeRequest bool
	}

	now := int(time.Now().UTC().Unix())


	statuses := []HostStatusTest {
		{
			hosts[0],
			1,
			NewHostStatus("test_host", 500, 500, 0, now - hosts[0].SustainedTimePeriod - 5, now - hosts[0].BurstTimePeriod - 1),
			NewHostStatus("test_host", 0, 0, 1, now, now),
			true,
		},
		{
			hosts[1],
			5,
			NewHostStatus("test_host", 500, 500, 0, now - hosts[0].SustainedTimePeriod - 5, now - hosts[0].BurstTimePeriod - 1),
			NewHostStatus("test_host", 0, 0, 5, now, now),
			true,
		},
		{
			hosts[0],
			1,
			NewHostStatus("test_host", 500, 500, 20, now, now - hosts[0].BurstTimePeriod - 1),
			NewHostStatus("test_host", 500, 0, 21, now, now),
			true,
		},
		{
			hosts[0],
			1,
			NewHostStatus("test_host", 1195, 5, 5, now - (hosts[0].SustainedTimePeriod/2), now - hosts[0].BurstTimePeriod - 1),
			NewHostStatus("test_host",  1195, 0, 5, now - (hosts[0].SustainedTimePeriod/2), now),
			false,
		},
		{
			hosts[0],
			1,
			NewHostStatus("test_host", 1000, 18, 2, now - (hosts[0].SustainedTimePeriod/2), now),
			NewHostStatus("test_host", 1000, 18, 2, now - (hosts[0].SustainedTimePeriod/2), now),
			false,
		},
		{
			hosts[0],
			1,
			NewHostStatus("test_host", 1195, 8, 6, now - (hosts[0].SustainedTimePeriod/2), now),
			NewHostStatus("test_host", 1195, 8, 6, now - (hosts[0].SustainedTimePeriod/2), now),
			false,
		},
		{
			hosts[0],
			1,
			NewHostStatus("test_host", 1000, 10, 4, now - (hosts[0].SustainedTimePeriod/2), now),
			NewHostStatus("test_host", 1000, 10, 5, now - (hosts[0].SustainedTimePeriod/2), now),
			true,
		},
	}

	for i := 0; i < len(statuses); i++ {
		canMake, _ := statuses[i].status.CanMakeRequest(statuses[i].requestWeight, now, statuses[i].host)
		if canMake != statuses[i].expectedCanMakeRequest {
			//error if the boolean the function returns does not match the expected value
			t.Errorf("Loop: %v. Expected ability to make request: %v, got: %v", i, statuses[i].expectedCanMakeRequest, canMake)
		}
		if statuses[i].status != statuses[i].expectedStatus {
			expected := statuses[i].expectedStatus
			status := statuses[i].status
			//causes an error if the expected host status is different than the modified host status
			t.Errorf("Loop: %v, Expected %v sustained requests, got: %v. Expected %v burst requests, got: %v. Expected %v pending requests, got: %v. Expected %v first sustained request, got: %v, Expected %v first burst request, got; %v",
				i, expected.GetSustainedRequests(), status.GetSustainedRequests(), expected.GetBurstRequests(), status.GetBurstRequests(), expected.GetPendingRequests(), status.GetPendingRequests(), expected.GetFirstSustainedRequest(),
				status.GetFirstSustainedRequest(), expected.GetFirstBurstRequest(), status.GetFirstBurstRequest())
		}

	}
}



/*func TestIsInSustainedPeriod(t *testing.T) {

	type SustainedPeriodTest struct {
		Input          HostStatus
		ExpectedOutput bool
	}

	now := time.Now().UTC().Unix()
	nowUnit := int(now)

	host := host_config.HostConfig{"test_host", 600, 10, 60, 1}

	testCases := []SustainedPeriodTest{
		{
			HostStatus{"test_host", 300, 100, 100, nowUnit - (host.SustainedTimePeriod / 2), 0},
			true,
		},
		{
			HostStatus{"test_host", 300, 100, 100, nowUnit - host.SustainedTimePeriod - 5, 0},
			false,
		},
	}

	for i := 0; i < len(testCases)-1; i++ {
		isInPeriod := testCases[i].Input.IsInSustainedPeriod(host, nowUnit)

		if isInPeriod != testCases[i].ExpectedOutput {
			t.Errorf("Expected request to be in sustained period: %v, got: %v", testCases[i].ExpectedOutput, isInPeriod)
		}
	}
}

func TestIsInBurstPeriod(t *testing.T) {
	type BurstPeriodTest struct {
		Input          HostStatus
		ExpectedOutput bool
	}

	now := time.Now().UTC().Unix()
	nowUnit := int(now)

	host := host_config.HostConfig{"test_host", 600, 10, 60, 1}

	testCases := []BurstPeriodTest{
		{
			HostStatus{"test_host", 300, 100, 100, 0, nowUnit-1},
			true,
		},
		{
			HostStatus{"test_host", 300, 100, 100, 0, nowUnit-2},
			false,
		},
	}

	for i := 0; i < len(testCases); i++ {
		isInPeriod := testCases[i].Input.IsInBurstPeriod(host, nowUnit)

		if isInPeriod != testCases[i].ExpectedOutput {
			t.Errorf("Expected request to be in burst period: %v, got: %v", testCases[i].ExpectedOutput, isInPeriod)
		}
	}
}

func Test_recordOutOfPeriodSustainedRequest(t *testing.T) {
	const currentTime int = 12345

	type PeriodSustainedTest struct {
		Input          HostStatus
		ExpectedOutput HostStatus
	}

	testCases := []PeriodSustainedTest{
		{
			NewHostStatus("test_host", 100, 56, 35, 0, 0),
			NewHostStatus("test_host", 0, 56, 36, currentTime, 0),
		},
		{
			NewHostStatus("test_host", 200, 0, 0, 2345, 0),
			NewHostStatus("test_host", 0, 0, 1, currentTime, 0),
		},
	}

	for i := 0; i < len(testCases); i++ {
		testCases[i].Input.recordOutOfPeriodSustainedRequests(currentTime)
		result := testCases[i].Input
		expected := testCases[i].ExpectedOutput
		if result != expected {
			t.Errorf("Expected %v sustained requests, got: %v. Expected %v pending got: %v. Expected time of: %v, got: %v.", expected.GetSustainedRequests(), result.GetSustainedRequests(), expected.GetPendingRequests(), result.GetPendingRequests(), expected.GetFirstSustainedRequest(), result.GetFirstSustainedRequest())
		}
	}
}*/

/*func Test_recordOutOfPeriodBurstRequest(t *testing.T) {
	const currentTime int = 12345

	type PeriodBurstTest struct {
		Input          HostStatus
		ExpectedOutput HostStatus
	}

	testCases := []PeriodBurstTest{
		{
			NewHostStatus("test_host", 100, 56, 35, 0, 0),
			NewHostStatus("test_host", 100, 0, 36, 0, currentTime),
		},
		{
			NewHostStatus("test_host", 200, 0, 0, 0, 2345),
			NewHostStatus("test_host", 200, 0, 1, 0, currentTime),
		},
	}

	for i := 0; i < len(testCases); i++ {
		testCases[i].Input.recordOutOfPeriodBurstRequests(currentTime)
		result := testCases[i].Input
		expected := testCases[i].ExpectedOutput
		if result != testCases[i].ExpectedOutput {
			t.Errorf("Expected %v burst requests, got: %v. Expected %v pending got: %v. Expected time of: %v, got: %v.", expected.GetBurstRequests(), result.GetBurstRequests(), expected.GetPendingRequests(), result.GetPendingRequests(), expected.GetFirstBurstRequest(), result.GetFirstBurstRequest())
		}
	}
}*/
