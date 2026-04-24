package media

import (
	"encoding/binary"

	"github.com/rwcarlsen/goexif/exif"
)

// extractFujiFilmSimulation parses the raw Fujifilm MakerNote IFD and returns
// the film simulation name, or "" if not a Fujifilm image or tag not found.
func extractFujiFilmSimulation(x *exif.Exif) string {
	mknTag, err := x.Get(exif.MakerNote)
	if err != nil || len(mknTag.Val) < 12 {
		return ""
	}
	data := mknTag.Val
	if string(data[:8]) != "FUJIFILM" {
		return ""
	}

	ifdOffset := int(binary.LittleEndian.Uint32(data[8:12]))
	if ifdOffset+2 > len(data) {
		return ""
	}

	numEntries := int(binary.LittleEndian.Uint16(data[ifdOffset : ifdOffset+2]))
	var filmMode, saturation int = -1, -1
	for i := 0; i < numEntries; i++ {
		off := ifdOffset + 2 + i*12
		if off+12 > len(data) {
			break
		}
		tagID := binary.LittleEndian.Uint16(data[off:])
		val := int(binary.LittleEndian.Uint16(data[off+8:]))
		switch tagID {
		case 0x1003:
			saturation = val
		case 0x1401:
			filmMode = val
		}
	}

	// B&W/Acros simulations live in Saturation (values >= 0x300, except 0x8000)
	if saturation >= 0x300 && saturation != 0x8000 {
		if name := fujiBWSimName(saturation); name != "" {
			return name
		}
	}
	if filmMode >= 0 {
		return fujiColorSimName(filmMode)
	}
	return ""
}

func fujiColorSimName(v int) string {
	switch v {
	case 0x000:
		return "Provia"
	case 0x120:
		return "Astia"
	case 0x200:
		return "Velvia"
	case 0x500:
		return "Pro Neg. Std"
	case 0x501:
		return "Pro Neg. Hi"
	case 0x600:
		return "Classic Chrome"
	case 0x700:
		return "Eterna"
	case 0x800:
		return "Classic Neg."
	case 0x900:
		return "Bleach Bypass"
	case 0xa00:
		return "Nostalgic Neg."
	case 0xb00:
		return "Reala Ace"
	}
	return ""
}

func fujiBWSimName(v int) string {
	switch v {
	case 0x300:
		return "Monochrome"
	case 0x301:
		return "Monochrome + R"
	case 0x302:
		return "Monochrome + Ye"
	case 0x303:
		return "Monochrome + G"
	case 0x310:
		return "Sepia"
	case 0x500:
		return "Acros"
	case 0x501:
		return "Acros + R"
	case 0x502:
		return "Acros + Ye"
	case 0x503:
		return "Acros + G"
	}
	return ""
}
