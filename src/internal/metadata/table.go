package metadata

import "huepattl.de/unterlumen/internal/media"

// The field table (ADR-0061): for each field, where it is kept beside a
// photo, in the file, beside a video, and under which name the index and the
// info panel show it. A field is one line here and nowhere else.
//
// The date taken and the rating have rules of their own besides: the date
// also goes into photoshop:DateCreated and into EXIF's date tags with their
// offset, and a video keeps its day in premiered; an NFO counts stars in
// tens.

type fieldSpec struct {
	field Field
	// prop is the sidecar's property; the title has its own (media.WriteTitle).
	prop media.Prop
	// file are the exiftool tags written into the file, the first one read.
	file []string
	// nfo is the element of a video's NFO; nfoChild the element inside it
	// that holds the value (Kodi's <actor><name>).
	nfo, nfoChild string
	// tag is the name the index and the info panel show the value under.
	tag string
}

func prop(prefix, local string) media.Prop { return media.NewProp(prefix, local, media.PropSimple) }
func alt(prefix, local string) media.Prop  { return media.NewProp(prefix, local, media.PropAlt) }
func bag(prefix, local string) media.Prop  { return media.NewProp(prefix, local, media.PropBag) }

var table = []fieldSpec{
	{field: DateTaken, prop: prop("exif", "DateTimeOriginal"), nfo: "ul:datetaken"},
	{field: Title, file: []string{"XMP-dc:Title", "IPTC:ObjectName"}, nfo: "title"},
	{field: Description, prop: alt("dc", "description"), file: []string{"EXIF:ImageDescription", "XMP-dc:Description", "IPTC:Caption-Abstract"}, nfo: "plot", tag: "ImageDescription"},
	{field: Creator, prop: media.NewProp("dc", "creator", media.PropSeq), file: []string{"EXIF:Artist", "XMP-dc:Creator", "IPTC:By-line"}, nfo: "credits", tag: "Artist"},
	{field: Rights, prop: alt("dc", "rights"), file: []string{"EXIF:Copyright", "XMP-dc:Rights", "IPTC:CopyrightNotice"}, nfo: "ul:rights", tag: "Copyright"},
	{field: Rating, prop: media.PropRating, file: []string{"XMP-xmp:Rating"}, nfo: "userrating", tag: "Rating"},

	{field: Sublocation, prop: prop("Iptc4xmpCore", "Location"), file: []string{"XMP-iptcCore:Location", "IPTC:Sub-location"}, nfo: "ul:sublocation", tag: "Sublocation"},
	{field: City, prop: prop("photoshop", "City"), file: []string{"XMP-photoshop:City", "IPTC:City"}, nfo: "ul:city", tag: "City"},
	{field: State, prop: prop("photoshop", "State"), file: []string{"XMP-photoshop:State", "IPTC:Province-State"}, nfo: "ul:state", tag: "State"},
	{field: Country, prop: prop("photoshop", "Country"), file: []string{"XMP-photoshop:Country", "IPTC:Country-PrimaryLocationName"}, nfo: "country", tag: "Country"},
	{field: CountryCode, prop: prop("Iptc4xmpCore", "CountryCode"), file: []string{"XMP-iptcCore:CountryCode", "IPTC:Country-PrimaryLocationCode"}, nfo: "ul:countrycode", tag: "CountryCode"},
	{field: Headline, prop: prop("photoshop", "Headline"), file: []string{"XMP-photoshop:Headline", "IPTC:Headline"}, nfo: "tagline", tag: "Headline"},
	{field: Event, prop: alt("Iptc4xmpExt", "Event"), file: []string{"XMP-iptcExt:Event"}, nfo: "ul:event", tag: "Event"},
	{field: Credit, prop: prop("photoshop", "Credit"), file: []string{"XMP-photoshop:Credit", "IPTC:Credit"}, nfo: "ul:credit", tag: "Credit"},
	{field: IPTCSource, prop: prop("photoshop", "Source"), file: []string{"XMP-photoshop:Source", "IPTC:Source"}, nfo: "ul:source", tag: "Source"},
	{field: UsageTerms, prop: alt("xmpRights", "UsageTerms"), file: []string{"XMP-xmpRights:UsageTerms"}, nfo: "ul:usageterms", tag: "UsageTerms"},
	{field: WebStatement, prop: prop("xmpRights", "WebStatement"), file: []string{"XMP-xmpRights:WebStatement"}, nfo: "ul:webstatement", tag: "WebStatement"},
	{field: Instructions, prop: prop("photoshop", "Instructions"), file: []string{"XMP-photoshop:Instructions", "IPTC:SpecialInstructions"}, nfo: "ul:instructions", tag: "Instructions"},
	{field: JobID, prop: prop("photoshop", "TransmissionReference"), file: []string{"XMP-photoshop:TransmissionReference", "IPTC:OriginalTransmissionReference"}, nfo: "ul:jobid", tag: "TransmissionReference"},
	{field: PersonShown, prop: bag("Iptc4xmpExt", "PersonInImage"), file: []string{"XMP-iptcExt:PersonInImage"}, nfo: "actor", nfoChild: "name", tag: "PersonInImage"},
	{field: OrganisationShown, prop: bag("Iptc4xmpExt", "OrganisationInImageName"), file: []string{"XMP-iptcExt:OrganisationInImageName"}, nfo: "ul:organisation", tag: "OrganisationInImageName"},
}

// propDateCreated is where Photoshop and Lightroom keep the date taken; it
// is written with exif:DateTimeOriginal and read when that is missing.
var propDateCreated = prop("photoshop", "DateCreated")

// Fields are every field, in the order of the table.
var Fields = func() []Field {
	out := make([]Field, len(table))
	for i, s := range table {
		out[i] = s.field
	}
	return out
}()

// sidecarProps are the sidecar's properties, in the order of Unterlumen's
// own description.
var sidecarProps = func() []media.Prop {
	out := []media.Prop{}
	for _, s := range table {
		if s.prop != (media.Prop{}) {
			out = append(out, s.prop)
		}
		if s.field == DateTaken {
			out = append(out, propDateCreated)
		}
	}
	return out
}()

func specOf(f Field) (fieldSpec, bool) {
	for _, s := range table {
		if s.field == f {
			return s, true
		}
	}
	return fieldSpec{}, false
}

// IsList says whether a field holds several entries, joined by "; ".
func IsList(f Field) bool {
	s, ok := specOf(f)
	return ok && s.prop.IsList()
}
