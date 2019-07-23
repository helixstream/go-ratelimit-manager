package limiter

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func Test_Server(t *testing.T) {
	testPort := "8070"
	server := &http.Server{Addr: ":" + testPort, Handler: nil}

	go func() {
		http.HandleFunc("/hitLimit", serveHTTP)
		if err := http.ListenAndServe(":"+testPort, nil); err != nil {
			panic(err)
		}
	}()

	go func() {
		http.HandleFunc("/hitLimitWindow", serveWindowHTTP)
	}()

	time.Sleep(time.Duration(100) * time.Millisecond)

	serverHit := false

	for i := 0; i < sus+11; i++ {
		statusCode, err := getStatusCodeWithBadWeight("http://localhost:" + testPort + "/hitLimit")
		if err != nil {
			t.Error(err)
		}
		if statusCode == 419 {
			i = sus + 11
			serverHit = true
		}
	}

	if !serverHit {
		t.Errorf("Token server never hit 419")
	}

	for i := 0; i < sus+11; i++ {
		statusCode, err := getStatusCodeWithBadWeight("http://localhost:" + testPort + "/hitLimitWindow")
		if err != nil {
			t.Error(err)
		}
		if statusCode == 419 {
			if err := server.Shutdown(context.TODO()); err != nil {
				panic(err)
			}
			return
		}
	}

	t.Error("Window server never hit 419")

	if err := server.Shutdown(context.TODO()); err != nil {
		panic(err)
	}
}

func getStatusCodeWithBadWeight(url string) (int, error) {
	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	} else if resp != nil {
		return resp.StatusCode, resp.Body.Close()
	}

	return 0, nil
}
