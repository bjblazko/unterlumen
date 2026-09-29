//go:build !darwin

package desktop

import "errors"

// WriteMacBundle makes Unterlumen.app for the release's .dmg; the icon tools
// it needs (sips, iconutil) exist only on macOS.
func WriteMacBundle(_, _ string, _ []byte, _ string) error {
	return errors.New("an app bundle can only be made on macOS")
}
