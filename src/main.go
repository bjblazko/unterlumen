package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"huepattl.de/unterlumen/internal/api"
	"huepattl.de/unterlumen/internal/channels"
	"huepattl.de/unterlumen/internal/desktop"
	"huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/media"
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

	libDirDefault := ""
	if v := os.Getenv("UNTERLUMEN_LIB_DIR"); v != "" {
		libDirDefault = v
	} else if home, err := os.UserHomeDir(); err == nil {
		libDirDefault = filepath.Join(home, ".unterlumen")
	}

	cacheDirDefault := os.Getenv("UNTERLUMEN_CACHE_DIR")

	port := flag.Int("port", portDefault, "HTTP server port (env: UNTERLUMEN_PORT)")
	bind := flag.String("bind", bindDefault, "Address to bind to (env: UNTERLUMEN_BIND)")
	libDir := flag.String("lib-dir", libDirDefault, "Library data directory (env: UNTERLUMEN_LIB_DIR)")
	cacheDir := flag.String("cache-dir", cacheDirDefault, "Thumbnail and conversion cache directory (env: UNTERLUMEN_CACHE_DIR)")
	desktopMode := flag.Bool("desktop", false, "Open in a Chrome app window (no URL bar); server shuts down when the window is closed")
	desktopInstall := flag.Bool("desktop-install", false, "Install as a native app launcher (macOS .app, Linux .desktop, Windows Start Menu)")
	flag.Parse()

	if *desktopInstall {
		iconData, _ := webFS.ReadFile("web/logo.png")
		execPath, _ := os.Executable()
		if err := desktop.Install(execPath, iconData); err != nil {
			log.Fatalf("Install failed: %v", err)
		}
		return
	}

	if *cacheDir != "" {
		media.SetCacheDir(*cacheDir)
	}

	// Priority: cmdline arg > UNTERLUMEN_ROOT_PATH env > user home dir
	var startDir, boundary string

	if flag.NArg() > 0 {
		// cmdline arg: use as both start dir and navigation boundary
		startDir = flag.Arg(0)
		boundary = flag.Arg(0)
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

	// Server mode: explicitly deployed via UNTERLUMEN_ROOT_PATH (multi-user, restricted UI).
	// Local mode: cmdline arg or default home dir; boundary still restricts navigation.
	serverRole := os.Getenv("UNTERLUMEN_ROOT_PATH") != ""

	var libMgr *library.Manager
	var chStore *channels.Store
	if *libDir != "" {
		if mgr, err := library.NewManager(*libDir); err != nil {
			log.Printf("Warning: library manager init failed: %v", err)
		} else {
			libMgr = mgr
		}
		chStore = channels.NewStore(*libDir)
	}

	mux := api.NewRouter(absBoundary, relStart, sub, serverRole, libMgr, chStore)

	addr := fmt.Sprintf("%s:%d", *bind, *port)
	log.Printf("Serving photos from %s (boundary: %s)", absStart, absBoundary)
	log.Printf("Listening on http://%s", addr)

	if !*desktopMode {
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Fatalf("Server error: %v", err)
		}
		return
	}

	// Desktop mode: bind the port first so Chrome can connect immediately.
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to bind %s: %v", addr, err)
	}
	srv := &http.Server{Handler: mux}
	go func() {
		if serveErr := srv.Serve(ln); serveErr != nil && serveErr != http.ErrServerClosed {
			log.Fatalf("Server error: %v", serveErr)
		}
	}()

	appURL := fmt.Sprintf("http://%s", addr)
	instance, err := desktop.LaunchApp(appURL)
	if err != nil {
		log.Fatalf("Failed to open browser: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	if instance != nil {
		done := make(chan struct{})
		go func() { instance.Wait(); close(done) }()
		select {
		case <-done:
			log.Println("Browser window closed, shutting down")
		case <-sigCh:
			log.Println("Signal received, shutting down")
		}
	} else {
		<-sigCh
		log.Println("Signal received, shutting down")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Shutdown error: %v", err)
	}
}
