package host

import (
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

var (
	serverConfig      = NewRateLimitConfig("transactionTestHost3", 6000, 60, 100, 1)
	sustainedDuration = time.Minute
	burstDuration     = time.Second

	sustainedLimiter = NewRateLimiter(serverConfig.SustainedRequestLimit, sustainedDuration)
	burstLimiter     = NewRateLimiter(serverConfig.BurstRequestLimit, burstDuration)
	bannedLimiter    = NewRateLimiter(10, sustainedDuration)

	port = "8000"
)

func serveHTTP(w http.ResponseWriter, r *http.Request) {
	//simulates random server errors
	if rand.Intn(200) == 5 {
		http.Error(w, "Internal Service Error", 500)
		return
	}

	if sustainedLimiter.Allow() && burstLimiter.Allow() {
		fmt.Printf(" %v ", burstLimiter.curCount)
		w.WriteHeader(200)
	} else if bannedLimiter.Allow() {
		http.Error(w, "Too many requests", 429)
	} else {
		http.Error(w, "Banned for too many requests", 419)
	}
}

func getServer() *http.Server {
	rand.Seed(time.Now().UTC().Unix())

	server := &http.Server{Addr: ":" + port, Handler: nil}

	go func() {
		http.HandleFunc("/testRateLimit", serveHTTP)

		if err := http.ListenAndServe(":"+port, nil); err != nil {
			panic(err)
		}
	}()

	return server
}

type RateLimiter struct {
	maxCount int
	interval time.Duration

	mu       sync.Mutex
	curCount int
	lastTime time.Time
}

// NewRateLimiter creates a new RateLimiter. maxCount is the max burst allowed
// while interval specifies the duration for a burst. The effective rate limit is
// equal to maxCount/interval. For example, if you want to a max QPS of 5000,
// and want to limit bursts to no more than 500, you'd specify a maxCount of 500
// and an interval of 100*time.Millilsecond.
func NewRateLimiter(maxCount int, interval time.Duration) *RateLimiter {
	return &RateLimiter{
		maxCount: maxCount,
		interval: interval,
	}
}

// Allow returns true if a request is within the rate limit norms.
// Otherwise, it returns false.
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if time.Since(rl.lastTime) < rl.interval {
		if rl.curCount > 0 {
			rl.curCount--
			return true
		}
		return false
	}
	rl.curCount = rl.maxCount - 1
	rl.lastTime = time.Now()
	return true
}
