// Package deploy pushes a channel's built output to a remote host over SSH,
// by shelling out to the system ssh/rsync binaries. Authentication is
// SSH-key only — there is deliberately no password support anywhere in this
// package: HandlerConfig is a map[string]string persisted in plaintext to
// channels.json, and storing a password there would put a secret on disk in
// the clear.
package deploy

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Target describes a remote rsync-over-SSH destination, built from a
// Channel's HandlerConfig map (keys: "host", "user", "port", "remotePath",
// "identityFile" — identityFile and port are optional).
type Target struct {
	Host         string
	User         string
	Port         int    // 0 means "use ssh's default" (22)
	RemotePath   string
	IdentityFile string // optional; empty means use the default key/agent
}

// TargetFromConfig builds a Target from a channel's HandlerConfig map.
// "host", "user", and "remotePath" are required; a missing or blank value
// for any of them is a configuration mistake and returns a clear error.
// "port" and "identityFile" are optional: a missing/blank/unparseable port
// defaults to 0 (ssh's default) rather than erroring.
func TargetFromConfig(cfg map[string]string) (Target, error) {
	host := strings.TrimSpace(cfg["host"])
	user := strings.TrimSpace(cfg["user"])
	remotePath := strings.TrimSpace(cfg["remotePath"])

	var missing []string
	if host == "" {
		missing = append(missing, "host")
	}
	if user == "" {
		missing = append(missing, "user")
	}
	if remotePath == "" {
		missing = append(missing, "remotePath")
	}
	if len(missing) > 0 {
		return Target{}, fmt.Errorf("rsync deploy config missing required field(s): %s", strings.Join(missing, ", "))
	}

	port := 0
	if p := strings.TrimSpace(cfg["port"]); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			port = parsed
		}
	}

	return Target{
		Host:         host,
		User:         user,
		Port:         port,
		RemotePath:   remotePath,
		IdentityFile: strings.TrimSpace(cfg["identityFile"]),
	}, nil
}

// sshArgs builds the argument list for invoking ssh against t, running
// remoteCommand. BatchMode=yes is always present, so ssh fails immediately
// instead of hanging forever on a prompt it has no TTY to answer (e.g. an
// untrusted host key). It never includes StrictHostKeyChecking=accept-new
// or any other host-key-bypassing option — an untrusted host key must cause
// a failure, not a silent auto-trust.
func sshArgs(t Target, remoteCommand string) []string {
	args := []string{"-o", "BatchMode=yes"}
	if t.IdentityFile != "" {
		args = append(args, "-i", t.IdentityFile)
	}
	if t.Port != 0 {
		args = append(args, "-p", strconv.Itoa(t.Port))
	}
	args = append(args, fmt.Sprintf("%s@%s", t.User, t.Host), remoteCommand)
	return args
}

// sshTransport builds the -e "ssh ..." transport string passed to rsync,
// using the same BatchMode/identity/port rules as sshArgs (minus the
// destination and remote command, which rsync appends itself).
func sshTransport(t Target) string {
	parts := []string{"ssh", "-o", "BatchMode=yes"}
	if t.IdentityFile != "" {
		parts = append(parts, "-i", t.IdentityFile)
	}
	if t.Port != 0 {
		parts = append(parts, "-p", strconv.Itoa(t.Port))
	}
	return strings.Join(parts, " ")
}

// rsyncArgs builds the argument list for invoking rsync to push the
// contents of localDir to t. Trailing slashes on the local source and
// remote destination matter for rsync semantics — they make rsync copy the
// *contents* of localDir/remotePath rather than the directory itself.
func rsyncArgs(t Target, localDir string) []string {
	local := strings.TrimRight(localDir, "/") + "/"
	remote := strings.TrimRight(t.RemotePath, "/") + "/"
	return []string{
		"-az", "--delete",
		"-e", sshTransport(t),
		local,
		fmt.Sprintf("%s@%s:%s", t.User, t.Host, remote),
	}
}

// isHostKeyVerificationFailure reports whether combined ssh/rsync output
// indicates an untrusted/unverified host key, as opposed to some other
// connection failure (permission denied, connection refused, unknown host,
// etc.).
func isHostKeyVerificationFailure(output string) bool {
	return strings.Contains(output, "Host key verification failed")
}

// TestConnection runs a harmless remote command (`true`) over ssh to verify
// that t is reachable and authenticates correctly. Returns nil on success.
// On failure, it returns a clear, actionable error: if the host key isn't
// trusted yet, the error explains that and gives the exact `ssh user@host`
// command to run once, interactively, to establish trust. Otherwise the
// error includes the raw command output so the user can see what actually
// went wrong.
func TestConnection(t Target) error {
	cmd := exec.Command("ssh", sshArgs(t, "true")...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	output := string(out)
	if isHostKeyVerificationFailure(output) {
		return fmt.Errorf(
			"host key verification failed: the host key for %s is not yet trusted. "+
				"Run `ssh %s@%s` manually once to verify and accept its host key, then try again.\n%s",
			t.Host, t.User, t.Host, output,
		)
	}
	return fmt.Errorf("ssh connection test failed: %w\n%s", err, output)
}

// Deploy runs rsync (over the same ssh transport used by TestConnection) to
// push the contents of localDir to t. It returns the combined stdout+stderr
// output even on success, since it's useful to show what rsync actually
// did, alongside any error.
func Deploy(t Target, localDir string) (string, error) {
	cmd := exec.Command("rsync", rsyncArgs(t, localDir)...)
	out, err := cmd.CombinedOutput()
	output := string(out)
	if err != nil {
		return output, fmt.Errorf("rsync deploy failed: %w\n%s", err, output)
	}
	return output, nil
}
