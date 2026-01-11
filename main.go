package main

import (
	"context"
	"encoding/json"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/time/rate"
)

var (
	excuses    []string
	rng        = rand.New(rand.NewSource(time.Now().UnixNano()))
	rateLimit  = rate.Every(time.Minute / 30)
	burst      = 10
	limiterMgr *limiterManager
)

type client struct {
	lastSeen time.Time
	limiter  *rate.Limiter
}

type limiterManager struct {
	clients map[string]*client
	mux     sync.Mutex
}

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

func newClientLimiter(ctx context.Context) *limiterManager {
	lm := &limiterManager{
		clients: make(map[string]*client),
	}
	go lm.cleanupClients(ctx)
	return lm
}

func (lm *limiterManager) getClientLimiter(ip string) {
	lm.mux.Lock()
	defer lm.mux.Unlock()
	if c, exists := lm.clients[ip]; exists {
		c.lastSeen = time.Now()
	} else {
		lm.clients[ip] = &client{
			lastSeen: time.Now(),
			limiter:  rate.NewLimiter(rateLimit, burst),
		}
	}
}

func (lm *limiterManager) cleanupClients(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			lm.mux.Lock()
			for ip, c := range lm.clients {
				if time.Since(c.lastSeen) > 3*time.Minute {
					delete(lm.clients, ip)
				}
			}
			lm.mux.Unlock()
		}
	}

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
	limiterMgr.getClientLimiter(ip)
	if !limiterMgr.clients[ip].limiter.Allow() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]string{"error": "Rate limit exceeded"})
		return
	}
	excuse := excuses[rng.Intn(len(excuses))]
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"reason": excuse})
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	loadExcuses()
	limiterMgr = newClientLimiter(ctx)

	server := &http.Server{
		Addr:    ":8080",
		Handler: http.DefaultServeMux,
	}

	http.HandleFunc("/no", excuseHandler)
	go func() {
		log.Fatal(server.ListenAndServe())
	}()

	<-ctx.Done()

	log.Println("Shutting down server...")
	server.Shutdown(context.Background())
	log.Println("...server shutdown gracefully")
}
