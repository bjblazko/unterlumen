package media

import (
	"regexp"
	"time"
)

// A photo without a date taken in its EXIF — WhatsApp strips it, a scan
// never had one — often still says when it was taken in its name: "WhatsApp
// Image 2026-09-20 at 17.17.36", "IMG_20240704_142518", "2026-10-03_…". That
// date is used instead, so the photo sorts among its neighbours rather than
// after every dated one. The EXIF's date always wins.

var namedDates = []*regexp.Regexp{
	// 2026-09-20 at 17.17.36, 2025-12-31_21-50-27, 2024-02-29 10:11:12
	regexp.MustCompile(`(?:^|\D)(\d{4})-(\d{2})-(\d{2})[ _T]+(?:at )?(\d{2})[-.:](\d{2})[-.:](\d{2})`),
	// IMG_20240704_142518, PXL_20231105_091500123
	regexp.MustCompile(`(?:^|\D)(\d{4})(\d{2})(\d{2})_(\d{2})(\d{2})(\d{2})`),
	// 2026-10-03_motorola: the day alone, taken as its first second
	regexp.MustCompile(`(?:^|\D)(\d{4})-(\d{2})-(\d{2})(?:\D|$)`),
}

// DateFromFileName is the date a file's name gives, as a date taken is
// written ("2006-01-02T15:04:05"), or "" when it gives none that exists.
func DateFromFileName(name string) string {
	for _, re := range namedDates {
		m := re.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		clock := "00:00:00"
		if len(m) == 7 {
			clock = m[4] + ":" + m[5] + ":" + m[6]
		}
		t, err := time.Parse("2006-01-02T15:04:05", m[1]+"-"+m[2]+"-"+m[3]+"T"+clock)
		if err != nil || t.Year() < 1900 || t.Year() > 2100 {
			continue
		}
		return t.Format("2006-01-02T15:04:05")
	}
	return ""
}
