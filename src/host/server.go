package host

import (
	"github.com/youtube/vitess/go/ratelimiter"
	"math/rand"
	"net/http"
	"time"
)

var (
	serverConfig      = NewRateLimitConfig("transactionTestHost3", 1200, 60, 35, 1)
	sustainedDuration = time.Minute
	burstDuration     = time.Second

	sustainedLimiter = ratelimiter.NewRateLimiter(serverConfig.SustainedRequestLimit, sustainedDuration)
	burstLimiter     = ratelimiter.NewRateLimiter(serverConfig.BurstRequestLimit, burstDuration)
	bannedLimiter    = ratelimiter.NewRateLimiter(10, sustainedDuration)

	port = "8080"
)

func serveHTTP(w http.ResponseWriter, r *http.Request) {
	//simulates random server errors
	if rand.Intn(200) == 5 {
		http.Error(w, "Internal Service Error", 500)
		return
	}

	if sustainedLimiter.Allow() && burstLimiter.Allow() {
		w.WriteHeader(200)
	} else if bannedLimiter.Allow() {
		http.Error(w, "Too many requests", 429)
	} else {
		http.Error(w, "Banned for too many requests", 419)
	}
}

func server() *http.Server {
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
