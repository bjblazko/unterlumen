package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"huepattl.de/unterlumen/internal/api"
)

func main() {
	port := flag.Int("port", 8080, "HTTP server port")
	bind := flag.String("bind", "localhost", "Address to bind to (use 0.0.0.0 for remote access)")
	flag.Parse()

	root := "."
	if flag.NArg() > 0 {
		root = flag.Arg(0)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving root path: %v\n", err)
		os.Exit(1)
	}

	info, err := os.Stat(absRoot)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Root path is not a valid directory: %s\n", absRoot)
		os.Exit(1)
	}

	mux := api.NewRouter(absRoot)

	addr := fmt.Sprintf("%s:%d", *bind, *port)
	log.Printf("Serving photos from %s", absRoot)
	log.Printf("Listening on http://%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
