package host

import (
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type myHandler struct{}

var (
	sustainedLimit    = 1200
	sustainedDuration = time.Minute
	burstLimit        = 35
	burstDuration     = time.Second

	sustainedLimiter = NewRateLimiter(sustainedLimit, sustainedDuration)
	burstLimiter     = NewRateLimiter(burstLimit, burstDuration)
	bannedLimiter    = NewRateLimiter(10, sustainedDuration)

	port = "8000"
)

func (h myHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if rand.Intn(200) == 5 {
		http.Error(w, "Internal Service Error", 500)
		return
	}

	if sustainedLimiter.Allow() && burstLimiter.Allow() {


		message := "Burst left: " + strconv.Itoa(burstLimiter.CurCount) + ". Sustained left: " + strconv.Itoa(sustainedLimiter.CurCount) + ". "
		_, err := w.Write([]byte(message))
		if err != nil {
			fmt.Print(err)
		}
	} else if bannedLimiter.Allow() {
		//w.WriteHeader(429)
		http.Error(w, "Too many requests", 429)
	} else {
		http.Error(w, "Banned for too many requests", 419)
	}
}

func main() {
	rand.Seed(time.Now().UTC().Unix())

	http.HandleFunc("/testRateLimit", myHandler{}.ServeHTTP)

	if err := http.ListenAndServe(":" + port, nil); err != nil {
		panic(err)
	}
}

//all code below taken from:
//https://github.com/vitessio/vitess/blob/master/go/ratelimiter/ratelimiter.go#L30
//modified so that I could access CurCount

//rate limit code
type RateLimiter struct {
	MaxCount int
	interval time.Duration

	mu       sync.Mutex
	CurCount int
	lastTime time.Time
}

// NewRateLimiter creates a new RateLimiter. maxCount is the max burst allowed
// while interval specifies the duration for a burst. The effective rate limit is
// equal to maxCount/interval. For example, if you want to a max QPS of 5000,
// and want to limit bursts to no more than 500, you'd specify a maxCount of 500
// and an interval of 100*time.Millilsecond.
func NewRateLimiter(maxCount int, interval time.Duration) *RateLimiter {
	return &RateLimiter{
		MaxCount: maxCount,
		interval: interval,
	}
}

// Allow returns true if a request is within the rate limit norms.
// Otherwise, it returns false.
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if time.Since(rl.lastTime) < rl.interval {
		if rl.CurCount > 0 {
			rl.CurCount--
			return true
		}
		return false
	}
	rl.CurCount = rl.MaxCount - 1
	rl.lastTime = time.Now()
	return true
}
