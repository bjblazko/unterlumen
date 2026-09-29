package media

import (
	"os/exec"
	"sync"
)

// toolCheck remembers whether a helper program is there until RecheckTools:
// looking costs a process start, and the answer changes only when a program
// is installed, which the app can now do itself.
type toolCheck[T any] struct {
	mu   sync.Mutex
	done bool
	v    T
	look func() T
}

func (c *toolCheck[T]) get() T {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.done {
		c.v, c.done = c.look(), true
	}
	return c.v
}

func (c *toolCheck[T]) reset() {
	c.mu.Lock()
	c.done = false
	c.mu.Unlock()
}

func onPath(name string) func() bool {
	return func() bool {
		path, err := exec.LookPath(name)
		return err == nil && path != ""
	}
}

var (
	ffmpegCheck      = &toolCheck[FFmpegStatus]{look: lookFFmpeg}
	sipsCheck        = &toolCheck[bool]{look: onPath("sips")}
	cwebpCheck       = &toolCheck[bool]{look: onPath("cwebp")}
	heifConvertCheck = &toolCheck[bool]{look: onPath("heif-convert")}
	exiftoolCheck    = &toolCheck[bool]{look: onPath("exiftool")}
)

// RecheckTools forgets what was found, so the next check looks again — after
// helper programs were installed while the app runs.
func RecheckTools() {
	ffmpegCheck.reset()
	sipsCheck.reset()
	cwebpCheck.reset()
	heifConvertCheck.reset()
	exiftoolCheck.reset()
}
