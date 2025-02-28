package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	resp, err := http.Get("http://localhost:3000/health")
	if err != nil {
		log.Fatalf("Error performing health check: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Health check failed: Status code %d", resp.StatusCode)
		os.Exit(1)
	}

	fmt.Println("OK")
	os.Exit(0)
}
