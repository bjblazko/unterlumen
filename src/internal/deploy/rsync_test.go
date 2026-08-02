package deploy

import (
	"reflect"
	"strings"
	"testing"
)

func TestSSHArgsNoOptionalFields(t *testing.T) {
	target := Target{Host: "example.com", User: "alice", RemotePath: "/var/www"}
	got := sshArgs(target, "true")
	want := []string{"-o", "BatchMode=yes", "alice@example.com", "true"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sshArgs() = %v, want %v", got, want)
	}
	assertNoAcceptNew(t, got)
}

func TestSSHArgsPortOnly(t *testing.T) {
	target := Target{Host: "example.com", User: "alice", RemotePath: "/var/www", Port: 2222}
	got := sshArgs(target, "true")
	want := []string{"-o", "BatchMode=yes", "-p", "2222", "alice@example.com", "true"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sshArgs() = %v, want %v", got, want)
	}
	assertNoAcceptNew(t, got)
}

func TestSSHArgsIdentityFileOnly(t *testing.T) {
	target := Target{Host: "example.com", User: "alice", RemotePath: "/var/www", IdentityFile: "/home/alice/.ssh/id_ed25519"}
	got := sshArgs(target, "true")
	want := []string{"-o", "BatchMode=yes", "-i", "/home/alice/.ssh/id_ed25519", "alice@example.com", "true"}
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
		"-e", "ssh -o BatchMode=yes",
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
		"-e", "ssh -o BatchMode=yes -i /home/alice/.ssh/id_ed25519 -p 2222",
		"/local/output/",
		"alice@example.com:/var/www/site/",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("rsyncArgs() = %v, want %v", got, want)
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
