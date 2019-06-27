package host

import (
	"golang.org/x/time/rate"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

var (
	serverConfig = NewRateLimitConfig("transactionTestHost5", 1200, 60, 30, 1)

	sustainedDuration = rate.Limit(float64(serverConfig.sustainedRequestLimit) / float64(serverConfig.sustainedTimePeriod))
	burstDuration     = rate.Limit(float64(serverConfig.burstRequestLimit) / float64(serverConfig.burstTimePeriod))

	sustainedLimiter = rate.NewLimiter(sustainedDuration, serverConfig.sustainedRequestLimit)
	burstLimiter     = rate.NewLimiter(burstDuration, serverConfig.burstRequestLimit)
	bannedLimiter    = rate.NewLimiter(.1666666, 10)

	port = "8090"
)

func serveHTTP(w http.ResponseWriter, r *http.Request) {
	weight, err := strconv.Atoi(r.FormValue("weight"))
	if err != nil {
		weight = 1
	}
	//simulates random server errors
	if rand.Intn(200) == 5 {
		http.Error(w, "Internal Service Error", 500)
		return
	}

	if sustainedLimiter.AllowN(time.Now(), weight) && burstLimiter.AllowN(time.Now(), weight) {
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
