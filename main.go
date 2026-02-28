package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"huepattl.de/unterlumen/internal/api"
)

//go:embed web
var webFS embed.FS

func main() {
	portDefault := 8080
	if v := os.Getenv("UNTERLUMEN_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			portDefault = n
		} else {
			fmt.Fprintf(os.Stderr, "Invalid UNTERLUMEN_PORT value %q, using default %d\n", v, portDefault)
		}
	}
	bindDefault := "localhost"
	if v := os.Getenv("UNTERLUMEN_BIND"); v != "" {
		bindDefault = v
	}

	port := flag.Int("port", portDefault, "HTTP server port (env: UNTERLUMEN_PORT)")
	bind := flag.String("bind", bindDefault, "Address to bind to (env: UNTERLUMEN_BIND)")
	flag.Parse()

	// Priority: cmdline arg > UNTERLUMEN_ROOT_PATH env > user home dir
	var startDir, boundary string

	if flag.NArg() > 0 {
		// cmdline arg: start there, no navigation restriction
		startDir = flag.Arg(0)
		boundary = "/"
	} else if envPath := os.Getenv("UNTERLUMEN_ROOT_PATH"); envPath != "" {
		// ENV var: start there, restrict navigation to that directory
		startDir = envPath
		boundary = envPath
	} else {
		// Default: user home directory, no navigation restriction
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error resolving home directory: %v\n", err)
			os.Exit(1)
		}
		startDir = home
		boundary = "/"
	}

	absStart, err := filepath.Abs(startDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving start path: %v\n", err)
		os.Exit(1)
	}
	info, err := os.Stat(absStart)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Start path is not a valid directory: %s\n", absStart)
		os.Exit(1)
	}

	var absBoundary string
	if boundary == "/" {
		absBoundary = "/"
	} else {
		absBoundary, err = filepath.Abs(boundary)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error resolving boundary path: %v\n", err)
			os.Exit(1)
		}
		info, err = os.Stat(absBoundary)
		if err != nil || !info.IsDir() {
			fmt.Fprintf(os.Stderr, "Boundary path is not a valid directory: %s\n", absBoundary)
			os.Exit(1)
		}
	}

	// Compute startPath relative to boundary (for the frontend's initial navigation)
	var relStart string
	if absBoundary == "/" {
		relStart = strings.TrimPrefix(absStart, "/")
	} else {
		relStart, err = filepath.Rel(absBoundary, absStart)
		if err != nil || relStart == "." {
			relStart = ""
		}
	}

	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("Failed to sub web FS: %v", err)
	}

	mux := api.NewRouter(absBoundary, relStart, sub)

	addr := fmt.Sprintf("%s:%d", *bind, *port)
	log.Printf("Serving photos from %s (boundary: %s)", absStart, absBoundary)
	log.Printf("Listening on http://%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
