// Package metadata reads and writes what the metadata editor changes about a
// photo or a video — when it was taken, its title, description, keywords,
// creator, rights, rating and the IPTC fields — always into a named target: the file itself
// (EXIF, IPTC and XMP inside it, through exiftool) or what lies beside it
// (the XMP sidecar of a photo, the NFO file of a video). Reading gives each
// value with where it came from; what lies beside a file wins over the file
// (ADR-0060).
package metadata

// Field is one value the editor changes. Keywords are a list of their own.
// Where each is kept is the field table (table.go).
type Field string

const (
	DateTaken   Field = "dateTaken"
	Title       Field = "title"
	Description Field = "description"
	Creator     Field = "creator"
	Rights      Field = "rights"
	Rating      Field = "rating"

	// The IPTC Photo Metadata Standard's fields (ADR-0061).
	Sublocation       Field = "sublocation"
	City              Field = "city"
	State             Field = "state"
	Country           Field = "country"
	CountryCode       Field = "countryCode"
	Headline          Field = "headline"
	Event             Field = "event"
	Credit            Field = "credit"
	IPTCSource        Field = "source"
	UsageTerms        Field = "usageTerms"
	WebStatement      Field = "webStatement"
	Instructions      Field = "instructions"
	JobID             Field = "jobId"
	PersonShown       Field = "personShown"
	OrganisationShown Field = "organisationShown"
)

// Source says where a value was read.
type Source string

const (
	FromFile     Source = "file"
	FromSidecar  Source = "sidecar"
	FromNFO      Source = "nfo"
	FromFileName Source = "file name"
)

// Target is where a change is written.
type Target string

const (
	// TargetFile writes into the photo itself; its content, and so its ID
	// in a library, changes (ADR-0052). Videos are never written into.
	TargetFile Target = "file"
	// TargetSidecar writes the XMP sidecar of a photo, the NFO of a video.
	TargetSidecar Target = "sidecar"
)

// Value is a field's value and where it came from.
type Value struct {
	Value string `json:"value"`
	From  Source `json:"from,omitempty"`
}

// Current is what a file says now: the value of each field that has one,
// the keywords, and its own fields (key and value, only ever in the sidecar
// of a photo; ADR-0048, ADR-0061).
type Current struct {
	Fields       map[Field]Value   `json:"fields"`
	Keywords     []string          `json:"keywords"`
	KeywordsFrom Source            `json:"keywordsFrom,omitempty"`
	Own          map[string]string `json:"own"`
	Video        bool              `json:"video"`
}

// Change is what to write: fields to set ("" removes one), entries to add
// to and remove from a list field (Person Shown, …) and the keywords, and a
// correction of the date taken. The lists and the date are worked out per
// file from what it has.
type Change struct {
	Set            map[Field]string     `json:"set,omitempty"`
	Add            map[Field][]string   `json:"add,omitempty"`
	Remove         map[Field][]string   `json:"remove,omitempty"`
	KeywordsAdd    []string             `json:"keywordsAdd,omitempty"`
	KeywordsRemove []string             `json:"keywordsRemove,omitempty"`
	Date           *DateFix             `json:"date,omitempty"`
	Own            map[string]OwnChange `json:"own,omitempty"`
}

// OwnChange is what to do with one own field: a new value ("" removes the
// field), and whether it is also a keyword (a machine tag, key=value); nil
// leaves either as it is.
type OwnChange struct {
	Value  *string `json:"value,omitempty"`
	Mirror *bool   `json:"mirror,omitempty"`
}
