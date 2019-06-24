package host

import (
	"golang.org/x/time/rate"
	"math/rand"
	"net/http"
	"time"
)

var (
	serverConfig = NewRateLimitConfig("transactionTestHost3", 500, 60, 20, 1)

	sustainedDuration = rate.Limit(float64(serverConfig.SustainedRequestLimit) / float64(serverConfig.SustainedTimePeriod))
	burstDuration     = rate.Limit(float64(serverConfig.BurstRequestLimit) / float64(serverConfig.BurstTimePeriod))

	sustainedLimiter = rate.NewLimiter(sustainedDuration, serverConfig.SustainedRequestLimit)
	burstLimiter     = rate.NewLimiter(burstDuration, serverConfig.BurstRequestLimit)
	bannedLimiter    = rate.NewLimiter(.1666666, 10)

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
