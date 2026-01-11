package main

import (
	"encoding/json"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
)

var excuses []string

func loadExuces() {
	fdata, err := os.ReadFile("reasons.json")
	if err != nil {
		log.Fatalf("reasons.json not found or unable to load: %v", err)
	}
	if err = json.Unmarshal(fdata, &excuses); err != nil {
		log.Fatalf("JSON file not parsed, corrupt?: %v", err)
	}
}

func exucseHandler(w http.ResponseWriter, req *http.Request) {
	loadExuces()

	io.WriteString(w, excuses[rand.Intn(len(excuses))])
}

func main() {

	http.HandleFunc("/hello", exucseHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
