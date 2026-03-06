package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math/rand/v2"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

type httpReturnErr int

// error conditions I'll have as possibilities.
const (
	Err400 httpReturnErr = iota
	Err429
	Err500
)

type AlertsResponse struct {
	Alerts []*Alert `json:"alerts"`
}

type Alert struct {
	Source      string    `json:"source"`
	Severity    string    `json:"severity"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type config struct {
	host            string
	port            string
	alertMaxArrSize int     // used to adjust the max size of the returned array.
	errRate         float32 // used to randomly create an error on request.
}

func main() {
	initDataSources()
	conf := ConfigFromEnvVars()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /alerts", handleAlert(conf))
	addr := net.JoinHostPort(conf.host, conf.port)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Println("starting http alerts server", "address", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("failed to start http server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down alerts server...")
	shutdownCtx, fn := context.WithTimeout(context.Background(), time.Second*5)
	defer fn()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error, failed to shutdown server gracefully after 5 seconds: %v", err)
	}
	log.Print("alert service shut down, goodbye!")

}

func handleAlert(conf *config) http.HandlerFunc {
	log.Println("setting up handler alert")

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		//pretend long delay for some reason
		if rand.Float32() < 0.1 {
			time.Sleep(time.Second * 4)
		}

		// error based on errRate. If we're under it throw a random 400, 429,
		// or 500 error
		if rand.Float32() < conf.errRate {
			generateErr(w, r)
			return
		}

		query := r.URL.Query()
		timeSince := query.Get("since")

		since, err := time.Parse(time.RFC3339, timeSince)
		if err != nil {
			http.Error(w, "failed to parse since param", http.StatusBadRequest)
			return
		}

		alertsResponse := &AlertsResponse{
			Alerts: generateAlerts(conf.alertMaxArrSize, since),
		}

		marshalled, err := json.Marshal(alertsResponse)
		if err != nil {
			log.Printf("error creating alerts list: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		if _, err = w.Write(marshalled); err != nil {
			log.Printf("failed to write data to request: %v", err)
		}

	}
}

func generateErr(w http.ResponseWriter, _ *http.Request) {
	switch rand.IntN(3) {
	case 0: // 400
		http.Error(w, "400 error", http.StatusBadRequest)
	case 1: // 429 so we can test retry logic with headers
		w.Header().Set("Retry-After", strconv.Itoa(rand.IntN(5)+1))
		http.Error(w, "too many requests at once", http.StatusTooManyRequests)
	case 2: // 500 error
		http.Error(w, "something went wrong", http.StatusInternalServerError)
	}
}

func generateAlerts(numOf int, date time.Time) []*Alert {
	alerts := []*Alert{}
	diffFromDate := time.Now().Unix() - date.Unix()
	for range numOf {
		dateDiff := time.Duration(rand.Int64N(diffFromDate))
		alert := &Alert{
			Source:      sourceList.pickOne(),
			Severity:    severityTypeList.pickOne(),
			CreatedAt:   date.Add(dateDiff),
			Description: alertDescriptionList.pickOne(),
		}
		alerts = append(alerts, alert)
	}
	return alerts
}

// config may be mutable depending how much time I have. In practice you
// should probably always keep it immutable unless you need to dynamically
// change up values (which I may want to do to adjust error codes and whatnot)
func ConfigFromEnvVars() *config {
	conf := &config{
		host: os.Getenv("HOST"),
		port: os.Getenv("PORT"),
	}

	if envAlertSize := os.Getenv("ALERT_RESPONSE_SIZE"); envAlertSize != "" {
		size, err := strconv.ParseInt(envAlertSize, 10, 32)
		if err != nil {
			// just fatal out here. It's a mock service and you don't need to
			// clean anything up here anyway.
			log.Fatalf("failed to parse ALERT_RESPONSE_SIZE to an integer: %v", err)
		}
		conf.alertMaxArrSize = int(size)
	} else {
		conf.alertMaxArrSize = 20
	}

	if envErrRate := os.Getenv("ERROR_RATE"); envErrRate != "" {
		rate, err := strconv.ParseFloat(envErrRate, 32)
		if err != nil {
			// same as above.
			log.Fatalf("failed to parse ERROR_RATE to a float32 value: %v", err)
		}
		conf.errRate = float32(rate)
	} else {
		conf.errRate = 0
	}

	return conf
}
