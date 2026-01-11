package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

var (
	excuses          []string
	clientLimiter    = make(map[string]*rate.Limiter)
	clientLimiterMux sync.Mutex
)

func clientIP(r *http.Request) string {
	ip := r.RemoteAddr
	if strings.Contains(ip, ":") {
		ip, _, _ = net.SplitHostPort(ip)
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ip = strings.Split(xff, ",")[0]
	}
	return ip
}

func getClientLimiter(ip string) *rate.Limiter {

	clientLimiterMux.Lock()
	defer clientLimiterMux.Unlock()
	limiter, exists := clientLimiter[ip]
	if !exists {
		limiter = rate.NewLimiter(rate.Every(time.Minute/30), 10)
	}
	return limiter
}

func loadExcuses() {

	fdata, err := os.ReadFile("reasons.json")
	if err != nil {
		log.Fatalf("reasons.json not found or unable to load: %v", err)
	}
	if err = json.Unmarshal(fdata, &excuses); err != nil {
		log.Fatalf("JSON file not parsed, corrupt?: %v", err)
	}
}

func excuseHandler(w http.ResponseWriter, req *http.Request) {
	ip := clientIP(req)
	if !getClientLimiter(ip).Allow() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":"Rate limit hit, try again later."}`))
		return
	}
	excuse := excuses[rand.Intn(len(excuses))]
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"reason": excuse})
}

func main() {

	loadExcuses()
	http.HandleFunc("/no", excuseHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
