package installation

import (
	"strconv"
	"strings"
)

// FromLauncher reads the settings out of a launcher written by an earlier
// -desktop-install (launch, launch.sh or launch.bat), so that installing a new
// version keeps them. ok is false when the script holds no Unterlumen command.
func FromLauncher(script string) (cfg Config, ok bool) {
	for _, line := range strings.Split(script, "\n") {
		args := splitCommand(strings.TrimSpace(line))
		for i, a := range args {
			if a == "-desktop" {
				return fromArgs(args[i+1:]), true
			}
		}
	}
	return Config{}, false
}

// fromArgs reads the flags a launcher passed after -desktop; the last plain
// argument is the photo folder of that time, which Migrate turns into a
// shared folder on the next start.
func fromArgs(args []string) Config {
	var cfg Config
	for i := 0; i < len(args); i++ {
		value := func() string {
			if i+1 < len(args) {
				i++
				return args[i]
			}
			return ""
		}
		switch args[i] {
		case "-port":
			cfg.Port, _ = strconv.Atoi(value())
		case "-lib-dir":
			cfg.LibDir = value()
		case "-channels-dir":
			cfg.ChannelsDir = value()
		default:
			if !strings.HasPrefix(args[i], "-") {
				cfg.PhotosDir = args[i]
			}
		}
	}
	return cfg
}

// splitCommand splits one command line the way the launchers quote it: POSIX
// single quotes, where a quote inside is closed, escaped and reopened, and
// Windows double quotes.
func splitCommand(line string) []string {
	var args []string
	var cur strings.Builder
	inArg, escaped, quote := false, false, rune(0)
	for _, r := range line {
		switch {
		case escaped:
			cur.WriteRune(r) // the quote in '\''
			escaped = false
		case quote != 0 && r == quote:
			quote = 0
		case quote != 0:
			cur.WriteRune(r)
		case r == '\'' || r == '"':
			quote, inArg = r, true
		case r == '\\':
			escaped, inArg = true, true
		case r == ' ' || r == '\t' || r == '\r':
			if inArg {
				args = append(args, cur.String())
				cur.Reset()
				inArg = false
			}
		default:
			cur.WriteRune(r)
			inArg = true
		}
	}
	if inArg {
		args = append(args, cur.String())
	}
	return args
}
