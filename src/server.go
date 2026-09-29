package main

import (
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"

	"huepattl.de/unterlumen/internal/api"
	apisetup "huepattl.de/unterlumen/internal/api/setup"
	"huepattl.de/unterlumen/internal/installation"
)

// server serves the app and can replace everything behind it — browse root,
// libraries, destinations — when the setup saves a new configuration, so a
// first start needs no restart after the photo folder is chosen.
type server struct {
	web     fs.FS
	flags   config          // the command line, with defaults from the environment
	set     map[string]bool // the flags given on the command line
	managed bool            // configured by config.json and #setup
	handler atomic.Pointer[http.Handler]

	mu            sync.Mutex // serialises apply
	saved         installation.Config
	channelsDir   string
	defaultLibDir string
	problem       string // why the saved photo folder is not shown, for the setup
}

func newServer(web fs.FS, flags config, fset *flag.FlagSet) *server {
	s := &server{web: web, flags: flags, set: map[string]bool{}}
	fset.Visit(func(f *flag.Flag) { s.set[f.Name] = true })
	// A folder on the command line or UNTERLUMEN_ROOT_PATH decides the photo
	// folder, as dev runs, the e2e tests and the container do; config.json
	// is the desktop app's, started by its launcher without one.
	s.managed = len(flags.args) == 0 && os.Getenv("UNTERLUMEN_ROOT_PATH") == ""
	return s
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	(*s.handler.Load()).ServeHTTP(w, r)
}

// start loads config.json when the installation is managed and builds the
// app. It returns the effective configuration, e.g. for the port.
func (s *server) start() (config, error) {
	var saved installation.Config
	if s.managed {
		loaded, _, err := installation.Load()
		if err != nil {
			log.Printf("Warning: %v; starting with the setup", err)
		}
		saved = loaded
		if s.defaultLibDir, err = installation.DefaultLibDir(); err != nil {
			s.defaultLibDir = s.flags.libDir
		}
	}
	cfg, err := s.build(saved)
	if err != nil && s.managed && saved.PhotosDir != "" {
		// A photo folder on a disk or a NAS that is not connected must not
		// keep the app from starting: it opens the setup and says why.
		log.Printf("Warning: %v; opening the setup", err)
		s.problem = "The photo folder " + saved.PhotosDir + " is not there. If it is on a disk or a NAS, connect it and start Unterlumen again, or choose another folder."
		without := saved
		without.PhotosDir = ""
		return s.build(without)
	}
	return cfg, err
}

// apply saves a configuration from the setup and starts using it.
func (s *server) apply(next installation.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Built first, so a configuration that cannot start is never saved.
	if _, err := s.build(next); err != nil {
		return err
	}
	s.problem = ""
	return installation.Save(next)
}

// build makes the app for a saved configuration and puts it behind the server.
func (s *server) build(saved installation.Config) (config, error) {
	cfg := s.flags
	if s.managed {
		cfg = withInstallation(cfg, s.set, saved, s.defaultLibDir)
	}
	app, err := buildApp(cfg)
	if err != nil {
		return cfg, err
	}
	var setup func() apisetup.Installation
	if s.managed {
		s.saved, s.channelsDir = saved, app.channelsDir
		setup = s.installation
	}
	h := api.NewRouter(app.boundary, app.start, app.home, s.web, app.serverRole, app.libMgr, app.chStore, Version, setup)
	s.handler.Store(&h)
	return cfg, nil
}

func (s *server) installation() apisetup.Installation {
	return apisetup.Installation{
		Saved:         s.saved,
		ChannelsDir:   s.channelsDir,
		DefaultLibDir: s.defaultLibDir,
		Problem:       s.problem,
		Apply:         s.apply,
	}
}

// withInstallation fills in from config.json what neither a flag nor the
// environment decides: flag > environment > config.json > default.
func withInstallation(cfg config, set map[string]bool, saved installation.Config, defaultLibDir string) config {
	unset := func(flagName, env string) bool { return !set[flagName] && os.Getenv(env) == "" }
	if unset("port", "UNTERLUMEN_PORT") {
		cfg.port = installation.DesktopPort
		if saved.Port != 0 {
			cfg.port = saved.Port
		}
	}
	if unset("lib-dir", "UNTERLUMEN_LIB_DIR") {
		cfg.libDir = defaultLibDir
		if saved.LibDir != "" {
			cfg.libDir = saved.LibDir
		}
	}
	if unset("channels-dir", "UNTERLUMEN_CHANNELS_DIR") && saved.ChannelsDir != "" {
		cfg.channelsDir = saved.ChannelsDir
	}
	if saved.PhotosDir != "" {
		cfg.args = []string{saved.PhotosDir}
	}
	return cfg
}
