package victoria3

import (
	"fmt"
	"image/color"
	"math"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// The tags a colour can be written with.
const (
	tagRGB    = "rgb"
	tagHSV    = "hsv"
	tagHSV360 = "hsv360"
	tagList   = "list"
)

// ColorSlots are the colour slots a coat of arms and each of its layers can
// fill in. The games number them from one.
var ColorSlots = []string{
	"color1", "color2", "color3", "color4", "color5",
	"color6", "color7", "color8", "color9",
}

// IsColorSlot reports whether a key names a colour slot.
func IsColorSlot(key string) bool {
	return ColorSlotIndex(key) >= 0
}

// ColorSlotIndex returns the position of a colour slot, so that color1 is 0.
// It returns minus one for anything that is not a slot name.
func ColorSlotIndex(slot string) int {
	for index, name := range ColorSlots {
		if name == slot {
			return index
		}
	}

	return -1
}

// ColorValue is one of the ways a colour slot can be filled in.
type ColorValue interface {
	// Describe returns a short label for the value, for an interface to show.
	Describe() string

	colorValue()
}

// NamedColor refers to a colour defined in common/named_colors.
type NamedColor struct {
	Name string
}

// SlotColor refers to another colour slot, which is how a layer borrows a
// colour from the coat of arms it sits on, as in color1 = color2.
type SlotColor struct {
	Slot string
}

// RGBColor is a literal colour.
type RGBColor struct {
	R, G, B uint8
}

// HSVColor is a literal colour in hue, saturation and value. Hue is in
// degrees and the other two are fractions, whichever of the two spellings the
// file used: hsv { 0.5 0.8 0.3 } or hsv360 { 180 80 30 }.
type HSVColor struct {
	H, S, V float64
}

// ListColor picks a colour at random from a list defined in
// common/coat_of_arms/template_lists. The randomly generated coats of arms
// use it; there is no single colour behind it.
type ListColor struct {
	List string
}

func (NamedColor) colorValue() {}
func (SlotColor) colorValue()  {}
func (RGBColor) colorValue()   {}
func (HSVColor) colorValue()   {}
func (ListColor) colorValue()  {}

func (c NamedColor) Describe() string { return c.Name }
func (c SlotColor) Describe() string  { return c.Slot }
func (c RGBColor) Describe() string   { return fmt.Sprintf("rgb %d %d %d", c.R, c.G, c.B) }

func (c HSVColor) Describe() string {
	return fmt.Sprintf("hsv %.0f %.0f%% %.0f%%", c.H, c.S*100, c.V*100)
}

func (c ListColor) Describe() string { return "list " + c.List }

// Color is a colour slot and what fills it.
type Color struct {
	// Slot is the name the file used, one of ColorSlots.
	Slot string

	Value ColorValue
}

// Colors is the ordered set of colour slots of a coat of arms or of one of
// its layers.
type Colors []Color

// Get returns the colour in a slot.
func (c Colors) Get(slot string) (Color, bool) {
	for _, current := range c {
		if current.Slot == slot {
			return current, true
		}
	}

	return Color{}, false
}

// Set replaces the colour in a slot, or appends it when the slot is not
// filled in yet. Slots stay in the order they were first seen.
func (c *Colors) Set(color Color) {
	for index := range *c {
		if (*c)[index].Slot == color.Slot {
			(*c)[index] = color

			return
		}
	}

	*c = append(*c, color)
}

// NextFreeSlot returns the first colour slot that is not filled in yet, or an
// empty string when they all are.
func (c Colors) NextFreeSlot() string {
	for _, name := range ColorSlots {
		if _, taken := c.Get(name); !taken {
			return name
		}
	}

	return ""
}

// Palette maps the names defined in common/named_colors to colours.
type Palette map[string]color.RGBA

// Nearest returns the name of the palette colour closest to a colour, by
// distance in RGB.
func (p Palette) Nearest(value color.RGBA) (string, bool) {
	best, bestDistance := "", -1

	for name, candidate := range p {
		red := int(candidate.R) - int(value.R)
		green := int(candidate.G) - int(value.G)
		blue := int(candidate.B) - int(value.B)
		distance := red*red + green*green + blue*blue

		// Ties go to the alphabetically first name, so the answer does not
		// depend on the order of the map.
		if bestDistance < 0 || distance < bestDistance || distance == bestDistance && name < best {
			best, bestDistance = name, distance
		}
	}

	return best, bestDistance >= 0
}

// FallbackColor is what a colour resolves to when it cannot be worked out: an
// unknown name, a reference that leads nowhere, or a random pick. It is
// magenta, so that a tool drawing a coat of arms shows the gap rather than
// hiding it.
var FallbackColor = color.RGBA{R: 255, G: 0, B: 255, A: 255}

// maxSlotDepth bounds how far a chain of slot references is followed. A file
// can point color1 at color2 and color2 back at color1, and the games cope
// with that by giving up rather than looping.
const maxSlotDepth = 8

// Resolve works out the colour of a slot.
//
// slots is the set the value may refer back to, which for a layer is the
// colours of the coat of arms the layer belongs to, and for the coat of arms
// itself its own colours. It reports whether the colour could be worked out;
// when it could not, FallbackColor is returned, so that a coat of arms with
// one unknown colour still draws.
func (c Color) Resolve(palette Palette, slots Colors) (color.RGBA, bool) {
	return resolveColor(c.Value, palette, slots, 0)
}

func resolveColor(value ColorValue, palette Palette, slots Colors, depth int) (color.RGBA, bool) {
	if depth > maxSlotDepth {
		return FallbackColor, false
	}

	switch typed := value.(type) {
	case RGBColor:
		return color.RGBA{R: typed.R, G: typed.G, B: typed.B, A: 255}, true

	case HSVColor:
		return HSVToRGB(typed.H, typed.S, typed.V), true

	case NamedColor:
		if found, ok := palette[typed.Name]; ok {
			return found, true
		}

	case SlotColor:
		if referenced, ok := slots.Get(typed.Slot); ok {
			return resolveColor(referenced.Value, palette, slots, depth+1)
		}

	case ListColor:
		// A random pick has no single answer.
		return FallbackColor, false
	}

	return FallbackColor, false
}

// HSVToRGB converts a hue in degrees and a saturation and value between zero
// and one into a colour.
func HSVToRGB(hue, saturation, value float64) color.RGBA {
	hue = math.Mod(hue, 360)
	if hue < 0 {
		hue += 360
	}

	saturation = clampColor(saturation, 0, 1)
	value = clampColor(value, 0, 1)

	chroma := value * saturation
	sector := hue / 60
	second := chroma * (1 - math.Abs(math.Mod(sector, 2)-1))
	match := value - chroma

	var red, green, blue float64

	switch int(sector) {
	case 0:
		red, green, blue = chroma, second, 0
	case 1:
		red, green, blue = second, chroma, 0
	case 2:
		red, green, blue = 0, chroma, second
	case 3:
		red, green, blue = 0, second, chroma
	case 4:
		red, green, blue = second, 0, chroma
	default:
		red, green, blue = chroma, 0, second
	}

	return color.RGBA{
		R: channel(red + match),
		G: channel(green + match),
		B: channel(blue + match),
		A: 255,
	}
}

// RGBToHSV is the inverse of HSVToRGB: hue in degrees, saturation and value
// between zero and one.
func RGBToHSV(value color.RGBA) (hue, saturation, brightness float64) {
	red := float64(value.R) / 255
	green := float64(value.G) / 255
	blue := float64(value.B) / 255

	high := max(red, green, blue)
	low := min(red, green, blue)
	chroma := high - low

	brightness = high

	if high > 0 {
		saturation = chroma / high
	}

	if chroma == 0 {
		return 0, saturation, brightness
	}

	switch high {
	case red:
		hue = 60 * math.Mod((green-blue)/chroma, 6)
	case green:
		hue = 60 * ((blue-red)/chroma + 2)
	default:
		hue = 60 * ((red-green)/chroma + 4)
	}

	if hue < 0 {
		hue += 360
	}

	return hue, saturation, brightness
}

func channel(value float64) uint8 {
	return uint8(clampColor(value*255, 0, 255) + 0.5)
}

func clampColor(value, low, high float64) float64 {
	return min(max(value, low), high)
}

// FolderNamedColors is where the named colours are defined.
var FolderNamedColors = folders.Folder{Path: "common/named_colors"}

// LoadPalette reads common/named_colors, which is a set of files holding one
// colors block each:
//
//	colors = {
//		red    = hsv360 { 1 95 55 }
//		crimson = "red"
//	}
//
// A colour may be written as one defined before it, so the palette is filled
// in as it is read and a name that is not known yet is reported. Later files
// override earlier ones, the way the games read them.
func LoadPalette(set *folders.Set, collector *report.Collector) Palette {
	files, diagnostics := set.Files(FolderNamedColors)
	collector.AddAll(diagnostics)

	palette := Palette{}

	for _, parsed := range database.ParseFiles(files, collector) {
		for _, field := range parsed.Document.Fields {
			if field.Key != keyColors {
				collector.Addf(report.SeverityInfo, parsed.File.Source, parsed.File.Path, field.Line, field.Column, "named colors",
					"unknown field %s, kept but not interpreted", field.Key)

				continue
			}

			readColors(field.Value, palette, parsed.File, collector)
		}
	}

	return palette
}

// readColors adds the colours of one colors block to a palette.
func readColors(block script.Node, palette Palette, file folders.File, collector *report.Collector) {
	for _, field := range block.Fields {
		note := func(format string, args ...any) {
			collector.Addf(report.SeverityWarning, file.Source, file.Path, field.Line, field.Column,
				"named color "+field.Key, format, args...)
		}

		value, ok := DecodeColor(field.Value)
		if !ok {
			note("is %s, which is not a colour; ignored", field.Value.Describe())

			continue
		}

		resolved, ok := resolveColor(value, palette, nil, 0)
		if !ok {
			note("is %s, which is not defined; ignored", value.Describe())

			continue
		}

		palette[field.Key] = resolved
	}
}

// DecodeColor reads a colour as a file writes one: a name, a reference to a
// colour slot, rgb, hsv or hsv360 channels, an untagged triple of channels,
// or a random pick from a list.
//
// A colour written with a fourth channel, as the power bloc templates of
// Victoria 3 write one, is read from its first three.
func DecodeColor(node script.Node) (ColorValue, bool) {
	switch node.Kind {
	case script.KindString:
		// A quoted string always names a colour. An unquoted one naming a
		// slot is a reference to that slot, which is how a layer borrows a
		// colour from the coat of arms it sits on.
		if !node.Quoted && IsColorSlot(node.Text) {
			return SlotColor{Slot: node.Text}, true
		}

		return NamedColor{Name: node.Text}, true

	case script.KindList:
		// An untagged triple is a literal colour.
		return channels(tagRGB, node)

	case script.KindTagged:
		if node.Value == nil {
			return nil, false
		}

		if node.Tag == tagList {
			name, ok := node.Value.Str()
			if !ok {
				return nil, false
			}

			return ListColor{List: name}, true
		}

		return channels(node.Tag, *node.Value)
	}

	return nil, false
}

// channels turns a triple of numbers into a colour, in whichever of the three
// spellings the file used.
func channels(tag string, node script.Node) (ColorValue, bool) {
	numbers, ok := node.Numbers()
	if !ok || len(numbers) < 3 || len(numbers) > 4 {
		return nil, false
	}

	first, second, third := numbers[0], numbers[1], numbers[2]

	switch tag {
	case tagRGB:
		// Channels are written either as fractions or as whole numbers up to
		// 255, and which one it is can only be told from the values.
		if first <= 1 && second <= 1 && third <= 1 {
			return RGBColor{
				R: uint8(first*255 + 0.5),
				G: uint8(second*255 + 0.5),
				B: uint8(third*255 + 0.5),
			}, true
		}

		return RGBColor{R: channelOf(first), G: channelOf(second), B: channelOf(third)}, true

	case tagHSV360:
		// Degrees, then two percentages.
		return HSVColor{H: first, S: second / 100, V: third / 100}, true

	case tagHSV:
		// Three fractions.
		return HSVColor{H: first * 360, S: second, V: third}, true
	}

	return nil, false
}

func channelOf(value float64) uint8 {
	return uint8(clampColor(value, 0, 255))
}
