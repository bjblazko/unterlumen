// Package deploy pushes a channel's built output to a remote host over SSH,
// by shelling out to the system ssh/rsync binaries. Authentication is
// SSH-key only — there is deliberately no password support anywhere in this
// package: HandlerConfig is a map[string]string persisted in plaintext to
// channels.json, and storing a password there would put a secret on disk in
// the clear.
package deploy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// testConnectionTimeout bounds the whole `ssh ... true` connectivity check.
// A success should be near-instant (a single round-trip plus auth), so this
// is kept short — long enough to tolerate a slow network, short enough that
// an HTTP request handler calling TestConnection doesn't hang for minutes
// waiting on a black-holed TCP connection.
const testConnectionTimeout = 15 * time.Second

// deployTimeout bounds a full rsync deploy. A real photo-site sync can take
// a while (many files/large albums), so this is generous compared to
// testConnectionTimeout, but it still guarantees the HTTP handler calling
// Deploy eventually gets an answer instead of blocking forever on a
// hung/black-holed connection.
const deployTimeout = 5 * time.Minute

// sshConnectTimeout bounds the SSH-level TCP connection attempt itself (as
// opposed to the outer process timeout above). Without this, a
// slow/black-holed TCP handshake can take minutes to fail at the OS level,
// regardless of any context timeout wrapping the process.
const sshConnectTimeoutSeconds = 10

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
	args := []string{"-o", "BatchMode=yes", "-o", fmt.Sprintf("ConnectTimeout=%d", sshConnectTimeoutSeconds)}
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
	parts := []string{"ssh", "-o", "BatchMode=yes", "-o", fmt.Sprintf("ConnectTimeout=%d", sshConnectTimeoutSeconds)}
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
//
// Deliberately no --delete: this project supports pointing several channels'
// HandlerConfig at the same RemotePath (e.g. multiple single-gallery
// channels sharing one domain, each gallery living in its own postID
// subfolder), and --delete mirrors the destination to exactly what one
// channel's local output contains — deploying channel A would delete
// channel B's already-deployed content the moment they share a remote
// path. The tradeoff is that a gallery deleted locally isn't automatically
// removed from the remote; that's an acceptable cost against silently
// destroying a sibling channel's live content.
//
// site.json and gallery.json are always excluded from what gets pushed:
// they're local statefiles used to rebuild/regenerate the site (and, for
// site.json, they record every album's Slug/Unlisted fields — pushing them
// would publish the supposedly-unguessable folder name of every "Unlisted"
// album, defeating the point of the token). They are never needed on the
// remote, which only serves the already-generated static HTML. .DS_Store is
// also excluded — harmless but pointless clutter on the remote.
func rsyncArgs(t Target, localDir string) []string {
	local := strings.TrimRight(localDir, "/") + "/"
	remote := strings.TrimRight(t.RemotePath, "/") + "/"
	return []string{
		"-az",
		"--exclude=site.json",
		"--exclude=gallery.json",
		"--exclude=.DS_Store",
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
	ctx, cancel := context.WithTimeout(context.Background(), testConnectionTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ssh", sshArgs(t, "true")...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	output := string(out)
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("ssh connection test timed out after %s\n%s", testConnectionTimeout, output)
	}
	if isHostKeyVerificationFailure(output) {
		return fmt.Errorf(
			"host key verification failed: the host key for %s is not yet trusted. "+
				"Run `ssh %s@%s` manually once to verify and accept its host key, then try again.\n%s",
			t.Host, t.User, t.Host, output,
		)
	}
	return fmt.Errorf("ssh connection test failed: %w\n%s", err, output)
}

// checkLocalDirDeployable verifies that dir exists and contains at least one
// entry before it's used as an rsync source. A missing, empty, or stale
// local directory (e.g. the site was never built, or a build failed partway
// through) would otherwise waste a deploy round-trip pushing nothing, or —
// on a misconfigured OutputPath — silently succeed while deploying the
// wrong (empty) thing instead of failing loudly.
func checkLocalDirDeployable(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("deploy source directory %q does not exist", dir)
		}
		return fmt.Errorf("deploy source directory %q: %w", dir, err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("deploy source directory %q is empty; refusing to deploy nothing", dir)
	}
	return nil
}

// Deploy runs rsync (over the same ssh transport used by TestConnection) to
// push the contents of localDir to t. It returns the combined stdout+stderr
// output even on success, since it's useful to show what rsync actually
// did, alongside any error.
func Deploy(t Target, localDir string) (string, error) {
	if err := checkLocalDirDeployable(localDir); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), deployTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "rsync", rsyncArgs(t, localDir)...)
	out, err := cmd.CombinedOutput()
	output := string(out)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return output, fmt.Errorf("deploy timed out after %s", deployTimeout)
		}
		return output, fmt.Errorf("rsync deploy failed: %w\n%s", err, output)
	}

	fixOut, fixErr := fixRemotePermissions(t)
	output += fixOut
	if fixErr != nil {
		return output, fmt.Errorf("deploy succeeded but making the uploaded files world-readable failed — your web server may not be able to serve them: %w", fixErr)
	}
	return output, nil
}

// fixRemotePermissions makes the just-deployed directory tree readable by a
// web server that isn't the SSH user rsync ran as (e.g. Caddy running as its
// own service account). rsync -a preserves the local machine's file modes by
// default, and a typical local output directory is not world-readable (e.g.
// macOS commonly creates directories as 700) — without this step, a deploy
// can report success while the content stays completely unreadable on the
// remote. Ownership is deliberately left as whatever the SSH user already
// owns (usually itself) — only the permission bits are widened, which is the
// minimum needed for a different-user process to read the files.
func fixRemotePermissions(t Target) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), testConnectionTimeout)
	defer cancel()
	remotePath := shellQuoteSingle(strings.TrimRight(t.RemotePath, "/"))
	remoteCmd := fmt.Sprintf(
		"find %s -type d -exec chmod 755 {} + && find %s -type f -exec chmod 644 {} +",
		remotePath, remotePath,
	)
	cmd := exec.CommandContext(ctx, "ssh", sshArgs(t, remoteCmd)...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// shellQuoteSingle wraps s in single quotes for safe inclusion in a POSIX
// shell command string, escaping any single quotes it already contains.
// Needed because RemotePath is user-supplied configuration that ends up
// inside a remote shell command string built by fixRemotePermissions.
func shellQuoteSingle(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
