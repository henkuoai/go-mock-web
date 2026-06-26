package main

import (
	"log"
	"os"
	"path/filepath"

	"go-mock-web/internal"
	"go-mock-web/internal/server"
)

const (
	defaultAddr = ":8080"
	dataFile    = "data/mocks.json"
	webDir      = "web"
)

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = defaultAddr
	}

	store, err := internal.NewStore(dataFile)
	if err != nil {
		log.Fatalf("init store failed: %v", err)
	}

	abs, _ := filepath.Abs(webDir)
	srv := server.New(store, abs)

	log.Printf("go-mock-web listening on %s (web: %s, data: %s)", addr, abs, dataFile)
	if err := srv.Engine().Run(addr); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
