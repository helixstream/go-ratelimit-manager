# go-ratelimit-manager [![Documentation](https://godoc.org/github.com/helixstream/go-ratelimit-manager/src/host?status.svg)](http://godoc.org/github.com/helixstream/go-ratelimit-manager/src/host) [![Build Status](https://travis-ci.org/helixstream/go-ratelimit-manager.svg?branch=master)](https://travis-ci.org/helixstream/go-ratelimit-manager) [![Coverage Status](https://coveralls.io/repos/github/helixstream/go-ratelimit-manager/badge.svg?branch=master)](https://coveralls.io/github/helixstream/go-ratelimit-manager?branch=master)


## Motivations
We needed a way to coordinate concurrent requests to web APIs so that we would not hit their
rate limits. In addition the posted rate limits on some web APIs differ from how the rate limits
actually operate. 

By using redis to keep track of requests, this library allows you to respect the rate limits 
from different hosts across multiple containers, threads, or processes. In addition, it 
optimizes based on true rate limits versus published rate limits

## Redis 
This library uses the [Redis](https://redis.io/) database to help coordinate requests. In order to use this library you
need to import the Redis client: [Radix](https://github.com/mediocregopher/radix)


## Rate Limit Config
The `RateLimitConfig` struct contains the relevant rate limit information for a specific host. 

How to create a new RateLimitConfig:
```go
config := NewRateLimitConfig(
	    "myExampleHostName",  
	    1200, //number of requests allowed in the sustained period
	    60,   //length of the sustained period in seconds
	    20,   //number of requests allowed in the burst period
	    1,    //length of the burst period in seconds
	)
```

If a host only has one posted rate limit, enter the same rate limit for both the sustained and burst
limits. Entering a 0 for the time period or number of requests will result in the rate being considered an 
infinite rate. The library will ignore this rate and only use the non-infinite rate.

## Limiter
The `Limiter` struct contains the main functionality of determining whether a request can me made.

The NewLimiter function requires a RateLimitConfig struct and a [Radix pool](https://godoc.org/github.com/mediocregopher/radix/#Pool).
```go
pool, err := radix.NewPool("tcp", "127.0.0.1:6379", 100)
if err != nil {
    //handle error
}

limiter, err := NewLimiter(config, pool)
if err != nil {
    //handle error
}
```

#### Can Make Request
`CanMakeRequest` returns bool, int64. If a request can be made it returns true, 0. 
If a request cannot be made it returns false, and the time in milliseconds the program should 
wait before calling `CanMakeRequest` again
```go
canMake, sleepTime := limiter.CanMakeRequest(requestWeight)
```
The requestWeight represents how much a request counts against the rate limit.
In most cases the requestWeight is 1.
#### Full Example
To coordinate requests to the same api across multiple threads/containers, it is imperative that each
`Limiter` is initialized with a `RateLimitConfig` with the same host name. In addition the radix pool
must be connected to the same redis database.  

```go
canMake, sleepTime := limiter.CanMakeRequest(requestWeight)
//if a request can be made
if canMake {
    //make some api request
    statusCode, err := makeApiRequest(url)
    if err != nil {
        //if the request did not occur, it is imperative to call RequestCancelled()
        err := limiter.RequestCancelled(requestWeight)
        if err != nil {
            //handle error
        }
    }
    //if the request hit the rate limit
    if statusCode == 429 {
        err := limiter.HitRateLimit(requestWeight)
        if err != nil {
            //handle error
        }
    } else {
        //if the request was successful and did not hit the rate limit
        err := limiter.RequestSuccessful(requestWeight)
        if err != nil {
            //handle error
        }
    }

} else if sleepTime != 0 {
    //if the request cannot be made, sleep for the time specified 
    time.Sleep(time.Duration(sleepTime) * time.Millisecond)
}
```

