package host_status

import (
	"../host-config"
	"testing"
	"time"
)

func TestIsInSustainedPeriod(t *testing.T) {

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
}

func Test_recordOutOfPeriodBurstRequest(t *testing.T) {
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
}