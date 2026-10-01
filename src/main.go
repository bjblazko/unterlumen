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

	"huepattl.de/unterlumen/internal/channels"
	"huepattl.de/unterlumen/internal/desktop"
	"huepattl.de/unterlumen/internal/installation"
	"huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/media"
)

//go:embed web
var webFS embed.FS

var Version = "dev"

func main() {
	cfg := parseConfig(flag.CommandLine, desktop.WithoutProcessSerial(os.Args[1:]))
	installation.AddToolsToPath()
	if execPath, err := os.Executable(); err == nil && desktop.LaunchedAsMacApp(execPath) && !cfg.desktopInstall && cfg.macBundle == "" {
		cfg.desktop = true
	}

	if packaged(cfg) {
		return
	}
	if cfg.cacheDir != "" {
		media.SetCacheDir(cfg.cacheDir)
	}

	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("Failed to sub web FS: %v", err)
	}
	srv := newServer(sub, cfg, flag.CommandLine)
	cfg, err = srv.start()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	addr := fmt.Sprintf("%s:%d", cfg.bind, cfg.port)
	log.Printf("Listening on http://%s", addr)
	if !cfg.desktop {
		if err := http.ListenAndServe(addr, srv); err != nil {
			log.Fatalf("Server error: %v", err)
		}
		return
	}
	serveDesktop(addr, srv)
}

// packaged runs the packaging commands, which exit instead of serving:
// -desktop-install and -macos-bundle. It says whether one ran.
func packaged(cfg config) bool {
	if !cfg.desktopInstall && cfg.macBundle == "" {
		return false
	}
	iconData, _ := webFS.ReadFile("web/logo.png")
	execPath, _ := os.Executable()
	if cfg.desktopInstall {
		if err := desktop.Install(execPath, iconData, Version); err != nil {
			log.Fatalf("Install failed: %v", err)
		}
		return true
	}
	if err := desktop.WriteMacBundle(cfg.macBundle, execPath, iconData, Version); err != nil {
		log.Fatalf("Making the app bundle failed: %v", err)
	}
	return true
}

// app is what one configuration serves: the browse root, the folder to start
// in, the libraries and the destinations.
type app struct {
	boundary, start, home string // start and home relative to boundary
	serverRole            bool
	libMgr                *library.Manager
	chStore               *channels.Store
	channelsDir           string
}

// buildApp resolves the photo folder of cfg and opens its stores.
func buildApp(cfg config) (app, error) {
	startDir, boundary, err := browseRoots(cfg.args, os.Getenv("UNTERLUMEN_ROOT_PATH"))
	var absStart, absBoundary string
	if err == nil {
		absStart, absBoundary, err = absoluteRoots(startDir, boundary)
	}
	if err != nil {
		return app{}, err
	}
	a := app{boundary: absBoundary, start: relativeStart(absBoundary, absStart)}
	// The home folder relative to the boundary is the frontend's home button;
	// "" (the boundary root) when home is outside the boundary.
	if homeDir, err := os.UserHomeDir(); err == nil {
		a.home = homeRelative(absBoundary, homeDir)
	}
	// Server mode: explicitly deployed via UNTERLUMEN_ROOT_PATH (multi-user, restricted UI).
	// Local mode: cmdline arg, config.json or default home dir; boundary still restricts navigation.
	a.serverRole = os.Getenv("UNTERLUMEN_ROOT_PATH") != ""
	a.channelsDir = channelsDir(cfg, absStart)
	a.libMgr, a.chStore = openStores(cfg.libDir, a.channelsDir, absBoundary)
	log.Printf("Serving photos from %s (boundary: %s)", absStart, absBoundary)
	return a, nil
}

// config is the command line, with defaults from the environment.
type config struct {
	port           int
	bind           string
	libDir         string
	cacheDir       string
	channelsDir    string
	desktop        bool
	desktopInstall bool
	macBundle      string   // where to make Unterlumen.app, for the release's .dmg
	args           []string // the folder to browse, if given
}

// parseConfig defines the flags on fs, with defaults from the UNTERLUMEN_*
// environment, and parses args.
func parseConfig(fs *flag.FlagSet, args []string) config {
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

	var cfg config
	fs.IntVar(&cfg.port, "port", portDefault, "HTTP server port (env: UNTERLUMEN_PORT)")
	fs.StringVar(&cfg.bind, "bind", bindDefault, "Address to bind to (env: UNTERLUMEN_BIND)")
	fs.StringVar(&cfg.libDir, "lib-dir", libDirDefault, "Library data directory (env: UNTERLUMEN_LIB_DIR)")
	fs.StringVar(&cfg.cacheDir, "cache-dir", os.Getenv("UNTERLUMEN_CACHE_DIR"), "Thumbnail and conversion cache directory (env: UNTERLUMEN_CACHE_DIR)")
	fs.StringVar(&cfg.channelsDir, "channels-dir", os.Getenv("UNTERLUMEN_CHANNELS_DIR"), "Directory for channels.json; override to share channel config across installations (defaults to lib-dir; env: UNTERLUMEN_CHANNELS_DIR)")
	fs.BoolVar(&cfg.desktop, "desktop", false, "Open in a Chrome app window (no URL bar); server shuts down when the window is closed")
	fs.BoolVar(&cfg.desktopInstall, "desktop-install", false, "Install as a native app launcher (macOS .app, Linux .desktop, Windows Start Menu)")
	fs.StringVar(&cfg.macBundle, "macos-bundle", "", "Make Unterlumen.app at this path around this binary and exit (used to build the .dmg; macOS only)")
	fs.Parse(args) //nolint:errcheck // flag.CommandLine exits on error
	cfg.args = fs.Args()
	return cfg
}

// browseRoots returns the start folder and the navigation boundary. Priority:
// the command-line folder (start and boundary), else envRoot
// (UNTERLUMEN_ROOT_PATH, start and boundary), else the home folder with no
// restriction.
func browseRoots(args []string, envRoot string) (startDir, boundary string, err error) {
	if len(args) > 0 {
		return args[0], args[0], nil
	}
	if envRoot != "" {
		return envRoot, envRoot, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("Error resolving home directory: %v", err)
	}
	return home, "/", nil
}

// absoluteRoots makes both roots absolute, with symlinks resolved, and checks
// they are folders. A boundary of "/" stays as it is. Resolved, because a
// library's folder is stored resolved (pathguard.SafePath), and a boundary
// reached through a symlink (/var → /private/var on macOS) would otherwise
// never contain it.
func absoluteRoots(startDir, boundary string) (absStart, absBoundary string, err error) {
	absStart, err = realAbs(startDir)
	if err != nil {
		return "", "", fmt.Errorf("Error resolving start path: %v", err)
	}
	if info, err := os.Stat(absStart); err != nil || !info.IsDir() {
		return "", "", fmt.Errorf("Start path is not a valid directory: %s", absStart)
	}
	if boundary == "/" {
		return absStart, "/", nil
	}
	absBoundary, err = realAbs(boundary)
	if err != nil {
		return "", "", fmt.Errorf("Error resolving boundary path: %v", err)
	}
	if info, err := os.Stat(absBoundary); err != nil || !info.IsDir() {
		return "", "", fmt.Errorf("Boundary path is not a valid directory: %s", absBoundary)
	}
	return absStart, absBoundary, nil
}

// realAbs is dir as an absolute path with its symlinks resolved; a folder
// that is not there stays as filepath.Abs makes it, for the caller to report.
func realAbs(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		return real, nil
	}
	return abs, nil
}

// relativeStart is the start folder relative to the boundary, for the
// frontend's first navigation; "" for the boundary itself.
func relativeStart(absBoundary, absStart string) string {
	if absBoundary == "/" {
		return strings.TrimPrefix(absStart, "/")
	}
	rel, err := filepath.Rel(absBoundary, absStart)
	if err != nil || rel == "." {
		return ""
	}
	return rel
}

// homeRelative is homeDir relative to the boundary, or "" when it is the
// boundary itself or outside it.
func homeRelative(absBoundary, homeDir string) string {
	if absBoundary == "/" {
		return strings.TrimPrefix(homeDir, "/")
	}
	if strings.HasPrefix(homeDir+"/", absBoundary+"/") || homeDir == absBoundary {
		if rel, err := filepath.Rel(absBoundary, homeDir); err == nil && rel != "." {
			return rel
		}
	}
	return ""
}

// channelsDir is where the destinations are kept: -channels-dir, else the
// shared folder in the photo folder when there is one (installations that
// show the same photos find each other there), else the lib dir.
func channelsDir(cfg config, absStart string) string {
	if cfg.channelsDir != "" {
		return cfg.channelsDir
	}
	if shared := installation.FindShared(absStart); shared != "" {
		return shared
	}
	return cfg.libDir
}

// openStores opens the library manager and the destination store, or leaves
// them nil without a lib dir.
func openStores(libDir, channelsDir, absBoundary string) (*library.Manager, *channels.Store) {
	if libDir == "" {
		return nil, nil
	}
	var libMgr *library.Manager
	if mgr, err := library.NewManager(libDir); err != nil {
		log.Printf("Warning: library manager init failed: %v", err)
	} else {
		libMgr = mgr
	}
	return libMgr, channels.NewStore(channelsDir, libDir).WithBoundary(absBoundary)
}

// serveDesktop serves in a Chrome app window and shuts down when the window
// closes or on a signal. The port is bound first so Chrome can connect
// immediately.
func serveDesktop(addr string, mux http.Handler) {
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

	instance, err := desktop.LaunchApp(fmt.Sprintf("http://%s", addr))
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
