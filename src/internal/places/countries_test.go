package places

import "testing"

func TestCountryAt(t *testing.T) {
	for _, tc := range []struct {
		name     string
		lat, lon float64
		code, de string
	}{
		{"Kiel", 54.3233, 10.1228, "DE", "Deutschland"},
		{"Paris", 48.8566, 2.3522, "FR", "Frankreich"},
		{"Basel, by the border", 47.5596, 7.5886, "CH", "Schweiz"},
		{"Lesotho, inside South Africa", -29.31, 27.48, "LS", "Lesotho"},
		{"Johannesburg, around Lesotho", -26.2041, 28.0473, "ZA", "Südafrika"},
		{"Tokyo", 35.6762, 139.6503, "JP", "Japan"},
		{"a pier off Sylt", 54.9, 8.29, "DE", "Deutschland"},
		// Disputed areas as Germany sees them (the _deu point of view).
		{"Simferopol", 44.95, 34.10, "UA", "Ukraine"},
		{"Kyrenia", 35.34, 33.32, "CY", "Republik Zypern"},
		{"Laayoune", 27.15, -13.2, "EH", "Westsahara"},
	} {
		c, ok := CountryAt(tc.lat, tc.lon)
		if !ok || c.Code != tc.code || c.Name("de") != tc.de {
			t.Errorf("%s: %v %v", tc.name, c, ok)
		}
	}
	if c, ok := CountryAt(0, -30); ok {
		t.Errorf("the Atlantic is %s", c.Code)
	}
	if c, _ := CountryAt(54.3233, 10.1228); c.Name("xx") != "Germany" {
		t.Errorf("no English name for an unknown language: %s", c.Name("xx"))
	}
}

func BenchmarkCountryAt(b *testing.B) {
	load()
	for b.Loop() {
		CountryAt(54.3233, 10.1228)
	}
}
