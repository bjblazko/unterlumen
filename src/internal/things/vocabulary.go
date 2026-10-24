package things

import "fmt"

// VocabularyVersion names the word list and its thresholds. Photos scored
// with an older list are scored again from their stored vectors.
const VocabularyVersion = 1

// A Category groups things in the filter and the statistics.
type Category string

const (
	Animals   Category = "Animals"
	Vehicles  Category = "Vehicles"
	Buildings Category = "Buildings"
	Nature    Category = "Nature"
	Water     Category = "Water"
	Sky       Category = "Sky and weather"
	Plants    Category = "Plants"
	Food      Category = "Food and drink"
	Objects   Category = "Objects"
	Activity  Category = "Activities"
	Occasions Category = "Occasions"
	// Quality is what the photo is like rather than what it shows: blurred,
	// too dark, too bright. A photo whose best thing is one of these gets
	// no other things — it shows nothing clearly enough.
	Quality Category = "Photo quality"
	// none is the category of a name the list does not know.
	none Category = ""
)

// A Thing is one word of the list; Name is its ID, kept in the index and
// the sidecars whatever the language. Prompts are the phrases its text
// vector is the average of; empty means the default phrases around Name.
// Coco names the detector's class that also finds it. Min is the
// probability it needs, when not the default.
type Thing struct {
	Name     string
	Category Category
	Prompts  []string
	Coco     string
	Min      float32
}

// Vocabulary is the fixed, curated list of things looked for.
var Vocabulary = []Thing{
	// Animals
	{Name: "dog", Category: Animals, Coco: "dog"},
	{Name: "cat", Category: Animals, Coco: "cat"},
	{Name: "bird", Category: Animals, Coco: "bird"},
	{Name: "horse", Category: Animals, Coco: "horse"},
	{Name: "cow", Category: Animals, Coco: "cow"},
	{Name: "sheep", Category: Animals, Coco: "sheep"},
	{Name: "goat", Category: Animals},
	{Name: "pig", Category: Animals},
	{Name: "chicken", Category: Animals},
	{Name: "duck", Category: Animals},
	{Name: "swan", Category: Animals},
	{Name: "seagull", Category: Animals},
	{Name: "fish", Category: Animals},
	{Name: "insect", Category: Animals, Prompts: []string{"a photo of an insect", "a close-up photo of a bug"}},
	{Name: "butterfly", Category: Animals},
	{Name: "bee", Category: Animals},
	{Name: "spider", Category: Animals},
	{Name: "snail", Category: Animals},
	{Name: "squirrel", Category: Animals},
	{Name: "rabbit", Category: Animals},
	{Name: "deer", Category: Animals},
	{Name: "fox", Category: Animals},
	{Name: "hedgehog", Category: Animals},
	{Name: "frog", Category: Animals},
	{Name: "lizard", Category: Animals},
	{Name: "elephant", Category: Animals, Coco: "elephant"},
	{Name: "giraffe", Category: Animals, Coco: "giraffe"},
	{Name: "zebra", Category: Animals, Coco: "zebra"},
	{Name: "bear", Category: Animals, Coco: "bear"},
	{Name: "monkey", Category: Animals},
	{Name: "lion", Category: Animals},
	{Name: "seal", Category: Animals},
	// Vehicles
	{Name: "car", Category: Vehicles, Coco: "car"},
	{Name: "vintage car", Category: Vehicles, Prompts: []string{"a photo of a vintage car", "a photo of an oldtimer car"}},
	{Name: "camper van", Category: Vehicles, Prompts: []string{"a photo of a camper van", "a photo of a VW bus"}},
	{Name: "bus", Category: Vehicles, Coco: "bus"},
	{Name: "truck", Category: Vehicles, Coco: "truck"},
	{Name: "fire engine", Category: Vehicles},
	{Name: "tractor", Category: Vehicles},
	{Name: "motorcycle", Category: Vehicles, Coco: "motorcycle"},
	{Name: "bicycle", Category: Vehicles, Coco: "bicycle"},
	{Name: "train", Category: Vehicles, Coco: "train"},
	{Name: "tram", Category: Vehicles},
	{Name: "airplane", Category: Vehicles, Coco: "airplane"},
	{Name: "helicopter", Category: Vehicles},
	{Name: "boat", Category: Vehicles, Coco: "boat"},
	{Name: "sailboat", Category: Vehicles},
	{Name: "ship", Category: Vehicles, Prompts: []string{"a photo of a ship", "a photo of a ferry"}},
	{Name: "canoe", Category: Vehicles, Prompts: []string{"a photo of a canoe", "a photo of a kayak"}},
	// Buildings and places
	{Name: "house", Category: Buildings},
	{Name: "half-timbered house", Category: Buildings},
	{Name: "church", Category: Buildings},
	{Name: "mosque", Category: Buildings},
	{Name: "castle", Category: Buildings},
	{Name: "ruin", Category: Buildings, Prompts: []string{"a photo of a ruin", "a photo of old ruins"}},
	{Name: "tower", Category: Buildings},
	{Name: "lighthouse", Category: Buildings},
	{Name: "windmill", Category: Buildings},
	{Name: "wind turbine", Category: Buildings},
	{Name: "ferris wheel", Category: Buildings},
	{Name: "crane", Category: Buildings, Prompts: []string{"a photo of a construction crane", "a photo of harbour cranes"}},
	{Name: "industry", Category: Buildings, Prompts: []string{"a photo of an industrial plant", "a photo of a factory"}},
	{Name: "bridge", Category: Buildings},
	{Name: "skyscraper", Category: Buildings},
	{Name: "city", Category: Buildings, Prompts: []string{"a photo of a city", "a photo of a city skyline"}},
	{Name: "village", Category: Buildings},
	{Name: "street", Category: Buildings},
	{Name: "market", Category: Buildings, Prompts: []string{"a photo of a market", "a photo of market stalls"}},
	{Name: "train station", Category: Buildings},
	{Name: "harbour", Category: Buildings, Prompts: []string{"a photo of a harbour", "a photo of a marina with boats"}},
	{Name: "playground", Category: Buildings},
	{Name: "funfair", Category: Buildings, Prompts: []string{"a photo of a funfair", "a photo of a fairground ride"}},
	{Name: "garden", Category: Buildings},
	{Name: "interior", Category: Buildings, Prompts: []string{"a photo of a room inside a house", "an indoor photo of a living room"}},
	{Name: "kitchen", Category: Buildings},
	{Name: "museum", Category: Buildings, Prompts: []string{"a photo of an exhibition in a museum"}},
	{Name: "stairs", Category: Buildings},
	{Name: "door", Category: Buildings},
	{Name: "window", Category: Buildings},
	// Nature and landscape
	{Name: "nature", Category: Nature, Prompts: []string{"a photo of nature", "a landscape photo of untouched nature"}},
	{Name: "landscape", Category: Nature},
	{Name: "mountains", Category: Nature, Prompts: []string{"a photo of mountains", "a photo of a mountain landscape"}},
	{Name: "snow", Category: Nature, Prompts: []string{"a photo of snow", "a photo of a snowy landscape"}},
	{Name: "forest", Category: Nature, Prompts: []string{"a photo of a forest", "a photo taken in the woods"}},
	{Name: "tree", Category: Nature},
	{Name: "meadow", Category: Nature, Prompts: []string{"a photo of a meadow", "a photo of a grassy field"}},
	{Name: "field", Category: Nature, Prompts: []string{"a photo of farmland", "a photo of a wheat field"}},
	{Name: "vineyard", Category: Nature},
	{Name: "rocks", Category: Nature, Prompts: []string{"a photo of rocks", "a photo of a rocky cliff"}},
	{Name: "desert", Category: Nature},
	{Name: "cave", Category: Nature},
	{Name: "path", Category: Nature, Prompts: []string{"a photo of a hiking path", "a photo of a trail"}},
	// Water
	{Name: "sea", Category: Water, Prompts: []string{"a photo of the sea", "a photo of the ocean"}},
	{Name: "beach", Category: Water},
	{Name: "dunes", Category: Water, Prompts: []string{"a photo of sand dunes"}},
	{Name: "lake", Category: Water},
	{Name: "river", Category: Water},
	{Name: "waterfall", Category: Water},
	{Name: "canal", Category: Water},
	{Name: "swimming pool", Category: Water},
	{Name: "fountain", Category: Water},
	// Sky and weather
	{Name: "sunset", Category: Sky, Prompts: []string{"a photo of a sunset", "a photo of a sunrise"}},
	{Name: "clouds", Category: Sky, Prompts: []string{"a photo of clouds in the sky"}},
	{Name: "blue sky", Category: Sky, Prompts: []string{"a photo with a clear blue sky"}},
	{Name: "night", Category: Sky, Prompts: []string{"a photo taken at night"}},
	{Name: "starry sky", Category: Sky, Prompts: []string{"a photo of the stars at night"}},
	{Name: "moon", Category: Sky},
	{Name: "rain", Category: Sky, Prompts: []string{"a photo taken in the rain"}},
	{Name: "fog", Category: Sky, Prompts: []string{"a photo of fog", "a misty photo"}},
	{Name: "rainbow", Category: Sky},
	{Name: "lightning", Category: Sky, Prompts: []string{"a photo of lightning in a thunderstorm"}},
	{Name: "fireworks", Category: Sky},
	{Name: "fire", Category: Objects, Prompts: []string{"a photo of a fire", "a photo of a campfire", "a photo of a fireplace"}},
	// Plants
	{Name: "flower", Category: Plants},
	{Name: "rose", Category: Plants},
	{Name: "sunflower", Category: Plants},
	{Name: "tulip", Category: Plants},
	{Name: "potted plant", Category: Plants, Coco: "potted plant"},
	{Name: "mushroom", Category: Plants},
	{Name: "autumn leaves", Category: Plants, Prompts: []string{"a photo of autumn leaves", "a photo of colourful autumn foliage"}},
	{Name: "cactus", Category: Plants},
	{Name: "palm tree", Category: Plants},
	// Food and drink
	{Name: "food", Category: Food, Prompts: []string{"a photo of food", "a photo of a meal on a plate"}},
	{Name: "cake", Category: Food, Coco: "cake"},
	{Name: "pizza", Category: Food, Coco: "pizza"},
	{Name: "ice cream", Category: Food},
	{Name: "fruit", Category: Food},
	{Name: "vegetables", Category: Food},
	{Name: "bread", Category: Food},
	{Name: "coffee", Category: Food, Prompts: []string{"a photo of a cup of coffee"}},
	{Name: "wine", Category: Food, Prompts: []string{"a photo of a glass of wine"}},
	{Name: "beer", Category: Food, Prompts: []string{"a photo of a glass of beer"}},
	// Objects
	{Name: "toy", Category: Objects, Prompts: []string{"a photo of a toy", "a photo of children's toys"}},
	{Name: "teddy bear", Category: Objects, Coco: "teddy bear"},
	{Name: "book", Category: Objects, Coco: "book"},
	{Name: "computer", Category: Objects, Prompts: []string{"a photo of a computer", "a photo of a laptop"}, Coco: "laptop"},
	{Name: "phone", Category: Objects, Prompts: []string{"a photo of a mobile phone"}, Coco: "cell phone"},
	{Name: "camera", Category: Objects},
	{Name: "television", Category: Objects, Coco: "tv"},
	{Name: "guitar", Category: Objects},
	{Name: "piano", Category: Objects},
	{Name: "bench", Category: Objects, Coco: "bench"},
	{Name: "table", Category: Objects, Coco: "dining table"},
	{Name: "chair", Category: Objects, Coco: "chair"},
	{Name: "sofa", Category: Objects, Coco: "couch"},
	{Name: "bed", Category: Objects, Coco: "bed"},
	{Name: "umbrella", Category: Objects, Coco: "umbrella"},
	{Name: "ball", Category: Objects, Coco: "sports ball"},
	{Name: "kite", Category: Objects, Coco: "kite"},
	{Name: "tent", Category: Objects},
	{Name: "sign", Category: Objects, Prompts: []string{"a photo of a sign with writing on it"}},
	{Name: "graffiti", Category: Objects},
	{Name: "statue", Category: Objects, Prompts: []string{"a photo of a statue", "a photo of a sculpture"}},
	{Name: "painting", Category: Objects, Prompts: []string{"a photo of a painting on a wall"}},
	{Name: "document", Category: Objects, Prompts: []string{"a photo of a document", "a scan of a page of text"}},
	{Name: "screenshot", Category: Objects, Prompts: []string{"a screenshot of a computer screen"}},
	// Activities
	{Name: "hiking", Category: Activity, Prompts: []string{"a photo of people hiking"}},
	{Name: "swimming", Category: Activity, Prompts: []string{"a photo of people swimming"}},
	{Name: "cycling", Category: Activity, Prompts: []string{"a photo of people riding bicycles"}},
	{Name: "skiing", Category: Activity, Prompts: []string{"a photo of people skiing"}},
	{Name: "sailing", Category: Activity, Prompts: []string{"a photo of people sailing a boat"}},
	{Name: "playing", Category: Activity, Prompts: []string{"a photo of children playing"}},
	{Name: "eating", Category: Activity, Prompts: []string{"a photo of people eating at a table"}},
	{Name: "concert", Category: Activity, Prompts: []string{"a photo of a concert", "a photo of musicians on stage"}},
	{Name: "sport", Category: Activity, Prompts: []string{"a photo of people playing sports"}},
	{Name: "selfie", Category: Activity},
	{Name: "group photo", Category: Activity, Prompts: []string{"a group photo of many people"}},
	{Name: "portrait", Category: Activity, Prompts: []string{"a portrait photo of a person"}},
	{Name: "baby", Category: Activity, Prompts: []string{"a photo of a baby"}},
	// Occasions
	{Name: "birthday", Category: Occasions, Prompts: []string{"a photo of a birthday party", "a photo of a birthday cake with candles"}},
	{Name: "wedding", Category: Occasions},
	{Name: "christmas", Category: Occasions, Prompts: []string{"a photo of christmas", "a photo of a christmas tree"}},
	{Name: "easter", Category: Occasions, Prompts: []string{"a photo of easter eggs"}},
	{Name: "halloween", Category: Occasions, Prompts: []string{"a photo of halloween", "a photo of a carved pumpkin"}},
	{Name: "carnival", Category: Occasions, Prompts: []string{"a photo of people in carnival costumes"}},
	// Photo quality
	{Name: "blurred", Category: Quality, Prompts: []string{"a blurry out of focus photo", "a photo with motion blur"}, Min: 0.015},
	{Name: "underexposed", Category: Quality, Prompts: []string{"a very dark underexposed photo", "an almost black photo"}, Min: 0.015},
	{Name: "overexposed", Category: Quality, Prompts: []string{"an overexposed washed out photo", "an almost white photo"}, Min: 0.015},
}

// prompts are the phrases a thing's text vector is the average of.
func (t Thing) prompts() []string {
	if len(t.Prompts) > 0 {
		return t.Prompts
	}
	article := "a "
	if len(t.Name) > 0 && (t.Name[0] == 'a' || t.Name[0] == 'e' || t.Name[0] == 'i' || t.Name[0] == 'o' || t.Name[0] == 'u') {
		article = "an "
	}
	return []string{"a photo of " + article + t.Name, "a photo with " + article + t.Name + " in it"}
}

// RecordVersion is what a sidecar's things record names: the models and
// the word list, as "version.vocabulary".
func RecordVersion() string {
	return fmt.Sprintf("%d.%d", Version, VocabularyVersion)
}

// Current reports whether a record of version v is as new as this one's, or
// newer: what it found is taken over rather than looked for again.
func Current(v string) bool {
	var ver, voc int
	if _, err := fmt.Sscanf(v, "%d.%d", &ver, &voc); err != nil {
		return false
	}
	return ver > Version || ver == Version && voc >= VocabularyVersion
}

// CategoryOf is the category of the thing called name, "" for a name not
// in the list — one the other installation's newer list knows.
func CategoryOf(name string) Category {
	for _, t := range Vocabulary {
		if t.Name == name {
			return t.Category
		}
	}
	return none
}
