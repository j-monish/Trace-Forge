package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type Experiment struct {
	Service     string  `json:"service"`
	Failure     string  `json:"failure"`
	Intensity   int     `json:"intensity"`
	Probability float64 `json:"probability"`
	Duration    int     `json:"duration"`
}

var serviceURLs = map[string]string{
	"db": "http://localhost:8002",
}

func main() {
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/experiment", experimentHandler)

	log.Println("Chaos Controller running on :8007")

	if err := http.ListenAndServe(":8007", nil); err != nil {
		log.Fatal(err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]string{
		"status":  "healthy",
		"service": "chaos-controller",
	})
}

func experimentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	var experiment Experiment

	if err := json.NewDecoder(r.Body).Decode(&experiment); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	baseURL, exists := serviceURLs[experiment.Service]
	if !exists {
		http.Error(w, "unsupported service", http.StatusBadRequest)
		return
	}

	// Build failure injection URL.
	injectURL := fmt.Sprintf(
		"%s/inject-failure?type=%s&intensity=%d&probability=%f&duration=%d",
		baseURL,
		experiment.Failure,
		experiment.Intensity,
		experiment.Probability,
		experiment.Duration,
	)

	log.Printf(
		"Injecting %s failure into %s for %ds",
		experiment.Failure,
		experiment.Service,
		experiment.Duration,
	)

	// Inject failure.
	resp, err := http.Post(injectURL, "", nil)
	if err != nil {
		http.Error(w, "failed to inject failure", http.StatusBadGateway)
		return
	}
	resp.Body.Close()

	// Schedule automatic reset.
	go func() {
		time.Sleep(time.Duration(experiment.Duration) * time.Second)

		resetURL := baseURL + "/reset"

		resp, err := http.Post(resetURL, "", nil)
		if err != nil {
			log.Printf("Failed to reset %s: %v", experiment.Service, err)
			return
		}
		resp.Body.Close()

		log.Printf("Automatically reset %s", experiment.Service)
	}()

	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "experiment started",
		"service":    experiment.Service,
		"failure":    experiment.Failure,
		"duration":   experiment.Duration,
		"auto_reset": true,
	})
}
