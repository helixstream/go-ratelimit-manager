package limiter

import "testing"

func Test_DetermineLowerRate(t *testing.T) {
	type testConfig struct {
		susLimit    int
		susPeriod   int64
		burstLimit  int
		burstPeriod int64

		expectedLim  int
		expectedRate int64
	}

	testCases := []testConfig{
		{
			0, 0, 0, 0,
			0,
			0,
		},
		{
			10, 0, 35, 1,
			35,
			1,
		},
		{
			1000, 1, 20, 0,
			1000,
			1,
		},
		{
			1300, 60, 20, 1,
			20,
			1,
		},
		{
			1100, 60, 20, 1,
			55,
			3,
		},
	}

	for i := 0; i < len(testCases); i++ {
		limit, rate := determineLowerRate(testCases[i].susLimit, testCases[i].susPeriod, testCases[i].burstLimit, testCases[i].burstPeriod)

		if limit != testCases[i].expectedLim {
			t.Errorf("Expected: %v, got %v", testCases[i].expectedLim, limit)
		}

		if rate != testCases[i].expectedRate {
			t.Errorf("Expected: %v, got %v", testCases[i].expectedRate, rate)
		}
	}
}

func Test_ReduceFraction(t *testing.T) {
	type TestFraction struct {
		numerator   int64
		denominator int64
		expectedNum int64
		expectedDem int64
	}

	testCases := []TestFraction{
		{
			10,
			6,
			5,
			3,
		},
		{
			19,
			5,
			19,
			5,
		},
		{
			600,
			20,
			30,
			1,
		},
	}

	for i := 0; i < len(testCases); i++ {
		num, dem := reduceFraction(testCases[i].numerator, testCases[i].denominator)
		if num != testCases[i].expectedNum {
			t.Errorf("Numerator: expected %v, got %v", testCases[i].expectedNum, num)
		}
		if dem != testCases[i].expectedDem {
			t.Errorf("Numerator: expected %v, got %v", testCases[i].expectedDem, dem)
		}
	}
}

func Test_GCD(t *testing.T) {

	type TestFraction struct {
		numerator   int64
		denominator int64
		gcd         int64
	}

	testCases := []TestFraction{
		{
			10,
			6,
			2,
		},
		{
			19,
			5,
			1,
		},
		{
			600,
			20,
			20,
		},
	}

	for i := 0; i < len(testCases); i++ {
		g := gcd(testCases[i].numerator, testCases[i].denominator)
		if g != testCases[i].gcd {
			t.Errorf("Expected %v, got %v", testCases[i].gcd, g)
		}
	}
}
