package limiter

//example function for godoc purposes
func ExampleLimiter_WaitForRatelimit() {
	config := NewRateLimitConfig("testhost", 60, 60, 1, 1)
	limiter, err := NewLimiter(config, pool)
	if err != nil {
		//handle err
	}

	for {
		limiter.WaitForRatelimit(1)

		//make api request
		statusCode, err := getStatusCode("www.example.com", 1)
		if err != nil {
			if err := limiter.RequestCancelled(1); err != nil {
				//handle error
			}

		}

		if statusCode == 429 {
			if err := limiter.HitRateLimit(1); err != nil {
				//handle error
			}
		} else {
			if err := limiter.RequestSuccessful(1); err != nil {
				//handle error
			}
		}
	}
}
