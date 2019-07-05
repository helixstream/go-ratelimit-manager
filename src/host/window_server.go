package host

import (
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"
)

var (
	windowSustainedDuration = time.Minute
	windowBurstDuration     = time.Second

	windowSustainedLimiter = NewRateLimiter(sus, windowSustainedDuration)
	windowBurstLimiter     = NewRateLimiter(burst, windowBurstDuration)
	windowBannedLimiter    = NewRateLimiter(10, 60)

	windowPort = "8080"
)

func serveWindowHTTP(w http.ResponseWriter, r *http.Request) {
	weight, err := strconv.Atoi(r.FormValue("weight"))
	if err != nil {
		weight = 1
	}
	//simulates random server errors
	if rand.Intn(200) == 5 {
		http.Error(w, "Internal Service Error", 500)
		return
	}

	if windowSustainedLimiter.Allow(weight) && windowBurstLimiter.Allow(weight) {
		w.WriteHeader(200)
	} else if windowBannedLimiter.Allow(weight) {
		http.Error(w, "Too many requests", 429)
	} else {
		http.Error(w, "Banned for too many requests", 419)
	}
}

func getWindowServer() *http.Server {
	rand.Seed(time.Now().UTC().Unix())

	server := &http.Server{Addr: ":" + windowPort, Handler: nil}

	go func() {
		http.HandleFunc("/testWindowRateLimit", serveWindowHTTP)
		if err := http.ListenAndServe(":"+windowPort, nil); err != nil {
			panic(err)
		}
	}()

	return server
}

//rate limiting code taken from https://github.com/vitessio/vitess/blob/master/go/ratelimiter/ratelimiter.go#L30
//modified to allow different request weights
type RateLimiter struct {
	maxCount int
	interval time.Duration

	mu       sync.Mutex
	curCount int
	lastTime time.Time
}

func NewRateLimiter(maxCount int, interval time.Duration) *RateLimiter {
	return &RateLimiter{
		maxCount: maxCount,
		interval: interval,
	}
}

// Allow returns true if a request is within the rate limit norms.
// Otherwise, it returns false.
func (rl *RateLimiter) Allow(weight int) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if time.Since(rl.lastTime) < rl.interval {
		if rl.curCount >= weight {
			rl.curCount -= weight
			return true
		}
		return false
	}
	rl.curCount = rl.maxCount - weight
	rl.lastTime = time.Now()
	return true
}
