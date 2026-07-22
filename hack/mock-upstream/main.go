// Mock upstream server for manual live-updates testing.
// Endpoints:
//   GET  /data  — returns JSON with a counter value
//   POST /bump  — increments the counter and returns the new value
//   GET  /flaky — alternates between 200 and 503 on each request
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
)

var counter atomic.Int64
var flakyState atomic.Bool

func main() {
	addr := flag.String("addr", ":9090", "listen address")
	flag.Parse()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /data", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		v := counter.Load()
		log.Printf("GET /data → %d", v)
		json.NewEncoder(w).Encode(map[string]int64{"count": v})
	})

	mux.HandleFunc("POST /bump", func(w http.ResponseWriter, _ *http.Request) {
		v := counter.Add(1)
		log.Printf("POST /bump → %d", v)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int64{"count": v})
	})

	mux.HandleFunc("GET /flaky", func(w http.ResponseWriter, _ *http.Request) {
		if flakyState.Load() {
			flakyState.Store(false)
			log.Printf("GET /flaky → 503")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		flakyState.Store(true)
		log.Printf("GET /flaky → 200")
		w.WriteHeader(http.StatusOK)
	})

	log.Printf("mock upstream listening on %s", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		fmt.Println(err)
	}
}
