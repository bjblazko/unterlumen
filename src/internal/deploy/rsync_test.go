package deploy

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSSHArgsNoOptionalFields(t *testing.T) {
	target := Target{Host: "example.com", User: "alice", RemotePath: "/var/www"}
	got := sshArgs(target, "true")
	want := []string{"-o", "BatchMode=yes", "-o", "ConnectTimeout=10", "alice@example.com", "true"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sshArgs() = %v, want %v", got, want)
	}
	assertNoAcceptNew(t, got)
}

func TestSSHArgsPortOnly(t *testing.T) {
	target := Target{Host: "example.com", User: "alice", RemotePath: "/var/www", Port: 2222}
	got := sshArgs(target, "true")
	want := []string{"-o", "BatchMode=yes", "-o", "ConnectTimeout=10", "-p", "2222", "alice@example.com", "true"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sshArgs() = %v, want %v", got, want)
	}
	assertNoAcceptNew(t, got)
}

func TestSSHArgsIdentityFileOnly(t *testing.T) {
	target := Target{Host: "example.com", User: "alice", RemotePath: "/var/www", IdentityFile: "/home/alice/.ssh/id_ed25519"}
	got := sshArgs(target, "true")
	want := []string{"-o", "BatchMode=yes", "-o", "ConnectTimeout=10", "-i", "/home/alice/.ssh/id_ed25519", "alice@example.com", "true"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sshArgs() = %v, want %v", got, want)
	}
	assertNoAcceptNew(t, got)
}

func TestSSHArgsBoth(t *testing.T) {
	target := Target{
		Host: "example.com", User: "alice", RemotePath: "/var/www",
		Port: 2222, IdentityFile: "/home/alice/.ssh/id_ed25519",
	}
	got := sshArgs(target, "true")
	want := []string{
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		"-i", "/home/alice/.ssh/id_ed25519",
		"-p", "2222",
		"alice@example.com", "true",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sshArgs() = %v, want %v", got, want)
	}
	assertNoAcceptNew(t, got)
}

func assertNoAcceptNew(t *testing.T, args []string) {
	t.Helper()
	for _, a := range args {
		if strings.Contains(a, "StrictHostKeyChecking") {
			t.Errorf("sshArgs() must never include StrictHostKeyChecking, got arg %q in %v", a, args)
		}
	}
}

func TestRsyncArgsNoOptionalFields(t *testing.T) {
	target := Target{Host: "example.com", User: "alice", RemotePath: "/var/www/site"}
	got := rsyncArgs(target, "/local/output")
	want := []string{
		"-az", "--delete",
		"--exclude=site.json",
		"--exclude=gallery.json",
		"-e", "ssh -o BatchMode=yes -o ConnectTimeout=10",
		"/local/output/",
		"alice@example.com:/var/www/site/",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("rsyncArgs() = %v, want %v", got, want)
	}
}

func TestRsyncArgsPortAndIdentity(t *testing.T) {
	target := Target{
		Host: "example.com", User: "alice", RemotePath: "/var/www/site",
		Port: 2222, IdentityFile: "/home/alice/.ssh/id_ed25519",
	}
	got := rsyncArgs(target, "/local/output")
	want := []string{
		"-az", "--delete",
		"--exclude=site.json",
		"--exclude=gallery.json",
		"-e", "ssh -o BatchMode=yes -o ConnectTimeout=10 -i /home/alice/.ssh/id_ed25519 -p 2222",
		"/local/output/",
		"alice@example.com:/var/www/site/",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("rsyncArgs() = %v, want %v", got, want)
	}
}

// TestRsyncArgsExcludesStatefiles guards finding #1: deploy must never push
// site.json/gallery.json to the remote, since site.json records every
// Unlisted album's supposedly-unguessable slug — publishing it defeats the
// whole point of the token.
func TestRsyncArgsExcludesStatefiles(t *testing.T) {
	target := Target{Host: "example.com", User: "alice", RemotePath: "/var/www/site"}
	got := rsyncArgs(target, "/local/output")

	for _, want := range []string{"--exclude=site.json", "--exclude=gallery.json"} {
		found := false
		for _, a := range got {
			if a == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("rsyncArgs() = %v, missing required flag %q", got, want)
		}
	}
}

func TestRsyncArgsTrailingSlashHandling(t *testing.T) {
	// Already-trailing-slash input should not become double-slashed.
	target := Target{Host: "example.com", User: "alice", RemotePath: "/var/www/site/"}
	got := rsyncArgs(target, "/local/output/")
	if got[len(got)-2] != "/local/output/" {
		t.Errorf("local dir = %q, want %q", got[len(got)-2], "/local/output/")
	}
	if got[len(got)-1] != "alice@example.com:/var/www/site/" {
		t.Errorf("remote dest = %q, want %q", got[len(got)-1], "alice@example.com:/var/www/site/")
	}
}

func TestTargetFromConfigValid(t *testing.T) {
	cfg := map[string]string{
		"host":         "example.com",
		"user":         "alice",
		"remotePath":   "/var/www/site",
		"port":         "2222",
		"identityFile": "/home/alice/.ssh/id_ed25519",
	}
	target, err := TargetFromConfig(cfg)
	if err != nil {
		t.Fatalf("TargetFromConfig() error = %v", err)
	}
	want := Target{
		Host: "example.com", User: "alice", RemotePath: "/var/www/site",
		Port: 2222, IdentityFile: "/home/alice/.ssh/id_ed25519",
	}
	if target != want {
		t.Errorf("TargetFromConfig() = %+v, want %+v", target, want)
	}
}

func TestTargetFromConfigMissingOptionalFields(t *testing.T) {
	cfg := map[string]string{
		"host":       "example.com",
		"user":       "alice",
		"remotePath": "/var/www/site",
	}
	target, err := TargetFromConfig(cfg)
	if err != nil {
		t.Fatalf("TargetFromConfig() error = %v", err)
	}
	if target.Port != 0 {
		t.Errorf("Port = %d, want 0", target.Port)
	}
	if target.IdentityFile != "" {
		t.Errorf("IdentityFile = %q, want empty", target.IdentityFile)
	}
}

func TestTargetFromConfigInvalidPortDefaultsToZero(t *testing.T) {
	cfg := map[string]string{
		"host":       "example.com",
		"user":       "alice",
		"remotePath": "/var/www/site",
		"port":       "not-a-number",
	}
	target, err := TargetFromConfig(cfg)
	if err != nil {
		t.Fatalf("TargetFromConfig() error = %v", err)
	}
	if target.Port != 0 {
		t.Errorf("Port = %d, want 0 for invalid input", target.Port)
	}
}

func TestTargetFromConfigMissingRequiredFields(t *testing.T) {
	cases := []struct {
		name string
		cfg  map[string]string
	}{
		{"missing host", map[string]string{"user": "alice", "remotePath": "/var/www"}},
		{"missing user", map[string]string{"host": "example.com", "remotePath": "/var/www"}},
		{"missing remotePath", map[string]string{"host": "example.com", "user": "alice"}},
		{"all missing", map[string]string{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := TargetFromConfig(c.cfg)
			if err == nil {
				t.Fatalf("TargetFromConfig(%v) expected error, got nil", c.cfg)
			}
		})
	}
}

func TestCheckLocalDirDeployableMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "does-not-exist")
	err := checkLocalDirDeployable(dir)
	if err == nil {
		t.Fatal("checkLocalDirDeployable() expected error for missing dir, got nil")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("checkLocalDirDeployable() error = %q, want mention of nonexistence", err.Error())
	}
}

func TestCheckLocalDirDeployableEmpty(t *testing.T) {
	dir := t.TempDir()
	err := checkLocalDirDeployable(dir)
	if err == nil {
		t.Fatal("checkLocalDirDeployable() expected error for empty dir, got nil")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("checkLocalDirDeployable() error = %q, want mention of emptiness", err.Error())
	}
}

func TestCheckLocalDirDeployableNonEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := checkLocalDirDeployable(dir); err != nil {
		t.Errorf("checkLocalDirDeployable() = %v, want nil for non-empty dir", err)
	}
}

// TestDeployRefusesEmptyLocalDir is an integration-style test for finding
// #3: Deploy must refuse to run (and must never reach the exec.CommandContext
// call) when localDir is missing/empty, since a real invocation would run
// rsync --delete against an effectively-empty source and wipe the remote.
func TestDeployRefusesEmptyLocalDir(t *testing.T) {
	dir := t.TempDir() // exists but empty
	target := Target{Host: "example.com", User: "alice", RemotePath: "/var/www/site"}

	_, err := Deploy(target, dir)
	if err == nil {
		t.Fatal("Deploy() expected error for empty local dir, got nil")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("Deploy() error = %q, want mention of emptiness", err.Error())
	}
}

func TestIsHostKeyVerificationFailure(t *testing.T) {
	cases := []struct {
		name   string
		output string
		want   bool
	}{
		{"host key failure", "Host key verification failed.\r\n", true},
		{"permission denied", "alice@example.com: Permission denied (publickey).\r\n", false},
		{"connection refused", "ssh: connect to host example.com port 22: Connection refused\r\n", false},
		{"empty", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isHostKeyVerificationFailure(c.output); got != c.want {
				t.Errorf("isHostKeyVerificationFailure(%q) = %v, want %v", c.output, got, c.want)
			}
		})
	}
}
