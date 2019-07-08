package host

import (
	"golang.org/x/time/rate"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

var (
	sus                = 600
	susPeriod    int64 = 60
	burst              = 10
	burstPeriod  int64 = 1
	serverConfig       = NewRateLimitConfig("localHost7", sus, int64(susPeriod), burst, int64(burstPeriod))

	//bucket/token rate limit
	sustainedDuration = rate.Limit(float64(sus) / float64(susPeriod))
	burstDuration     = rate.Limit(float64(burst) / float64(burstPeriod))

	sustainedLimiter = rate.NewLimiter(sustainedDuration, sus)
	burstLimiter     = rate.NewLimiter(burstDuration, burst)
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
