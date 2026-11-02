// Package metadata reads and writes what the metadata editor changes about a
// photo or a video — when it was taken, its title, description, keywords,
// creator, rights and rating — always into a named target: the file itself
// (EXIF, IPTC and XMP inside it, through exiftool) or what lies beside it
// (the XMP sidecar of a photo, the NFO file of a video). Reading gives each
// value with where it came from; what lies beside a file wins over the file
// (ADR-0060).
package metadata

// Field is one value the editor changes. Keywords are a list of their own.
type Field string

const (
	DateTaken   Field = "dateTaken"
	Title       Field = "title"
	Description Field = "description"
	Creator     Field = "creator"
	Rights      Field = "rights"
	Rating      Field = "rating"
)

// Fields are every field, in the order the editor shows them.
var Fields = []Field{DateTaken, Title, Description, Creator, Rights, Rating}

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
// and the keywords.
type Current struct {
	Fields       map[Field]Value `json:"fields"`
	Keywords     []string        `json:"keywords"`
	KeywordsFrom Source          `json:"keywordsFrom,omitempty"`
	Video        bool            `json:"video"`
}

// Change is what to write: fields to set ("" removes one), keywords to add
// and remove, and a correction of the date taken, which is worked out per
// file from its own date.
type Change struct {
	Set            map[Field]string `json:"set,omitempty"`
	KeywordsAdd    []string         `json:"keywordsAdd,omitempty"`
	KeywordsRemove []string         `json:"keywordsRemove,omitempty"`
	Date           *DateFix         `json:"date,omitempty"`
}
