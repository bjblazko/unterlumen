package toolinstall

import "os/exec"

// linuxCommand is the install command for this distribution's package
// manager, the same packages install/install.sh installs; "" when there is
// none it knows.
func linuxCommand() string {
	switch {
	case has("apt-get"):
		return "sudo apt-get install -y ffmpeg libimage-exiftool-perl libheif-examples webp"
	case has("dnf"):
		return "sudo dnf install -y ffmpeg-free perl-Image-ExifTool libheif-tools libwebp-tools"
	case has("pacman"):
		return "sudo pacman -S --needed ffmpeg perl-image-exiftool libheif libwebp"
	}
	return ""
}

func has(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
