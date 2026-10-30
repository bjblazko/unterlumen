package media

import "testing"

func TestDateFromFileName(t *testing.T) {
	cases := map[string]string{
		"WhatsApp Image 2026-09-20 at 17.17.36.jpeg":           "2026-09-20T17:17:36",
		"marinaWhatsApp Image 2026-09-20 at 17.17.36 (1).jpeg": "2026-09-20T17:17:36",
		"2025-12-31_21-50-27_X-T50_DSCF0150.heic":              "2025-12-31T21:50:27",
		"IMG_20240704_142518.jpg":                              "2024-07-04T14:25:18",
		"PXL_20231105_091500123.jpg":                           "2023-11-05T09:15:00",
		"2026-10-03_motorola-edge-60-neo_015.jpg":              "2026-10-03T00:00:00",
		"Screenshot 2024-02-29 at 10.11.12.png":                "2024-02-29T10:11:12",
	}
	for name, want := range cases {
		if got := DateFromFileName(name); got != want {
			t.Errorf("DateFromFileName(%q) = %q, want %q", name, got, want)
		}
	}
	for _, name := range []string{"AIDA 1.heic", "Scan 172.jpg", "IMG_1234.jpg", "2026-13-40_x.jpg", "1850-01-01_x.jpg", "DSCF20240704.jpg"} {
		if got := DateFromFileName(name); got != "" {
			t.Errorf("DateFromFileName(%q) = %q, want none", name, got)
		}
	}
}
