package victoria3

import (
	"image/color"
	"strings"
	"testing"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// readArms reads coats of arms from script, without any folder to read it
// from.
func readArms(input string) (database.Table[CoatOfArms], report.Diagnostics) {
	collector := &report.Collector{}

	arms, _ := ReadCoatOfArms(script.Parse(input),
		folders.File{Relative: "common/test.txt", Path: "test.txt", Source: "test"}, collector)

	return arms, collector.Diagnostics
}

// oneArms reads a single coat of arms and fails the test over any warning,
// which keeps every test that is not about diagnostics honest.
func oneArms(t *testing.T, input string) *CoatOfArms {
	t.Helper()

	table, diagnostics := readArms(input)

	for _, diagnostic := range diagnostics.Filter(report.SeverityWarning) {
		t.Errorf("unexpected diagnostic: %s", diagnostic)
	}

	if table.Len() != 1 {
		t.Fatalf("read %d coats of arms, want 1", table.Len())
	}

	arms, _ := table.Get(table.Keys()[0])

	return arms
}

func TestLayersKeepTheOrderTheyAreWrittenIn(t *testing.T) {
	arms := oneArms(t, `
TST = {
	pattern = "pattern_solid.tga"
	colored_emblem = { texture = "first.dds" }
	sub = { parent = "OTHER" }
	textured_emblem = { texture = "second.dds" }
	colored_emblem = { texture = "third.dds" }
}
`)

	var kinds []string

	for _, layer := range arms.Layers {
		switch typed := layer.(type) {
		case *ColoredEmblem:
			kinds = append(kinds, "colored "+typed.Texture.File)
		case *TexturedEmblem:
			kinds = append(kinds, "textured "+typed.Texture.File)
		case *SubCoatOfArms:
			kinds = append(kinds, "sub "+typed.Parent)
		}
	}

	want := "colored first.dds, sub OTHER, textured second.dds, colored third.dds"
	if got := strings.Join(kinds, ", "); got != want {
		t.Errorf("layers = %s, want %s", got, want)
	}
}

func TestInstancesAndTheirDefaults(t *testing.T) {
	arms := oneArms(t, `
TST = {
	colored_emblem = { texture = "a.dds" }
	colored_emblem = {
		texture = "b.dds"
		instance = { position = { 0.25 0.75 } scale = { -0.5 0.5 } rotation = 90 }
		instance = { scale = { 0.07 } }
	}
	sub = { parent = "OTHER" instance = { offset = { 0.5 0 } scale = { 0.5 1 } } }
}
`)

	// A layer without an instance is drawn once, centred and at full size.
	first := arms.Layers[0].(*ColoredEmblem)
	if len(first.Instances) != 0 {
		t.Errorf("instances = %+v, want none kept for a layer that has none", first.Instances)
	}

	if placements := first.Placements(); len(placements) != 1 || placements[0] != DefaultInstance() {
		t.Errorf("placements = %+v, want the one default placement", placements)
	}

	second := arms.Layers[1].(*ColoredEmblem)

	want := Instance{Position: Vec2{X: 0.25, Y: 0.75}, Scale: Vec2{X: -0.5, Y: 0.5}, Rotation: 90}
	if second.Instances[0] != want {
		t.Errorf("instance = %+v, want %+v", second.Instances[0], want)
	}

	// A scale of one value sets the width and leaves the height whole, which
	// is how the games read the hoist stripes of Sulu.
	if got := second.Instances[1]; got.Scale != (Vec2{X: 0.07, Y: 1}) || got.Position != (Vec2{X: 0.5, Y: 0.5}) {
		t.Errorf("instance = %+v, want a narrow full height bar", got)
	}

	sub := arms.Layers[2].(*SubCoatOfArms)
	if got := sub.Instances[0]; got.Offset != (Vec2{X: 0.5}) || got.Scale != (Vec2{X: 0.5, Y: 1}) {
		t.Errorf("sub instance = %+v, want the right half", got)
	}
}

func TestColorsAndTextures(t *testing.T) {
	arms := oneArms(t, `
TST = {
	pattern = list "pattern_anarchy"
	color1 = "red"
	color2 = rgb { 0 128 255 }
	color3 = hsv360 { 180 50 100 }
	color4 = hsv { 0.5 0.5 1 }
	color5 = list "normal_colors"

	colored_emblem = {
		texture = list "ce_stars"
		color1 = color2
		mask = { 2 }
	}
}
`)

	if arms.Pattern.List != "pattern_anarchy" || arms.Pattern.File != "" {
		t.Errorf("pattern = %+v, want a random pick from pattern_anarchy", arms.Pattern)
	}

	want := []string{"red", "rgb 0 128 255", "hsv 180 50% 100%", "hsv 180 50% 100%", "list normal_colors"}
	for index, slot := range ColorSlots[:len(want)] {
		found, ok := arms.Colors.Get(slot)
		if !ok {
			t.Fatalf("%s is missing", slot)
		}

		if got := found.Value.Describe(); got != want[index] {
			t.Errorf("%s = %s, want %s", slot, got, want[index])
		}
	}

	emblem := arms.Layers[0].(*ColoredEmblem)

	if emblem.Texture.List != "ce_stars" {
		t.Errorf("emblem texture = %+v, want a random pick", emblem.Texture)
	}

	if len(emblem.Masks) != 1 || emblem.Masks[0] != 2 {
		t.Errorf("masks = %v, want the second pattern colour", emblem.Masks)
	}

	// An emblem borrows a colour of the coat of arms it sits on.
	borrowed, _ := emblem.Colors.Get("color1")
	if _, isSlot := borrowed.Value.(SlotColor); !isSlot {
		t.Fatalf("emblem color1 = %T, want a reference to a slot", borrowed.Value)
	}

	resolved, ok := borrowed.Resolve(Palette{}, arms.Colors)
	if !ok || resolved != (color.RGBA{R: 0, G: 128, B: 255, A: 255}) {
		t.Errorf("borrowed colour = %v, %v; want the flag's second colour", resolved, ok)
	}
}

func TestColorsResolveThroughThePalette(t *testing.T) {
	palette := Palette{"red": {R: 200, A: 255}}

	known := Color{Slot: "color1", Value: NamedColor{Name: "red"}}
	if got, ok := known.Resolve(palette, nil); !ok || got != (color.RGBA{R: 200, A: 255}) {
		t.Errorf("red = %v, %v", got, ok)
	}

	// An unknown name, a random pick and a reference that leads nowhere all
	// come out as the fallback, so that a coat of arms still draws.
	for _, value := range []ColorValue{NamedColor{Name: "puce"}, ListColor{List: "any"}, SlotColor{Slot: "color2"}} {
		if got, ok := (Color{Value: value}).Resolve(palette, nil); ok || got != FallbackColor {
			t.Errorf("%s = %v, %v; want the fallback", value.Describe(), got, ok)
		}
	}

	// A file can point two slots at each other; resolving gives up rather
	// than looping.
	loop := Colors{
		{Slot: "color1", Value: SlotColor{Slot: "color2"}},
		{Slot: "color2", Value: SlotColor{Slot: "color1"}},
	}

	if _, ok := loop[0].Resolve(palette, loop); ok {
		t.Error("a loop of slot references should not resolve")
	}
}

func TestProblemsAreReportedAndTheRestIsKept(t *testing.T) {
	table, diagnostics := readArms(`
TST = {
	pattern = 5
	color1 = { 1 2 }
	colored_emblem = {
		texture = "a.dds"
		instance = { position = { 0.1 0.2 0.3 } scale = yes }
		hat = yes
	}
}
`)

	arms, ok := table.Get("TST")
	if !ok {
		t.Fatal("the coat of arms was dropped over its mistakes")
	}

	// What could be read is still there.
	emblem := arms.Layers[0].(*ColoredEmblem)
	if emblem.Texture.File != "a.dds" {
		t.Errorf("texture = %+v, want it read in spite of the rest", emblem.Texture)
	}

	if got := emblem.Instances[0]; got.Position != (Vec2{X: 0.1, Y: 0.2}) || got.Scale != (Vec2{X: 1, Y: 1}) {
		t.Errorf("instance = %+v, want the position read and the scale left at its default", got)
	}

	want := []string{
		"pattern should be a file name or a list",
		"color1 should be a colour",
		"position has 3 values",
		"scale should be a pair of numbers",
		"unknown field hat",
	}

	for _, message := range want {
		found := false

		for _, diagnostic := range diagnostics {
			if strings.Contains(diagnostic.Message, message) {
				found = true

				break
			}
		}

		if !found {
			t.Errorf("no diagnostic about %q, got %v", message, diagnostics)
		}
	}
}

func TestTemplatesAreKeptApartAndEveryFileKeepsThem(t *testing.T) {
	collector := &report.Collector{}
	set := folders.Open([]folders.Source{{Name: "Victoria 3", Path: "../testdata/game"}})

	arms, templates := collectCoatOfArms(set, collector)

	if _, ok := arms.Get(keyTemplate); ok {
		t.Error("the template block was read as a coat of arms of its own")
	}

	if _, ok := templates.Get("template_plain"); !ok {
		t.Errorf("templates = %v, want the file's template", templates.Keys())
	}

	// Every file writes its own template block. Read as ordinary
	// definitions, the later block would override the earlier one and the
	// first file's templates would be gone.
	document := script.Parse(`template = { template_second = { pattern = "second.tga" } }`)
	file := folders.File{Relative: "common/second.txt", Path: "second.txt", Source: "second"}

	for _, field := range document.Fields {
		for _, template := range field.Value.Fields {
			templates.Add(template, "", file, collector)
		}
	}

	if keys := templates.Keys(); len(keys) != 2 {
		t.Errorf("templates = %v, want both files' templates", keys)
	}
}

func TestNearestPaletteColor(t *testing.T) {
	palette := Palette{
		"red":   {R: 200, A: 255},
		"blue":  {B: 200, A: 255},
		"azure": {B: 200, A: 255},
	}

	if name, ok := palette.Nearest(color.RGBA{R: 180, G: 20, A: 255}); !ok || name != "red" {
		t.Errorf("nearest to a dark red = %q, %v", name, ok)
	}

	// Ties go to the first name in alphabetical order, so the answer does not
	// depend on the order of the map.
	if name, _ := palette.Nearest(color.RGBA{B: 200, A: 255}); name != "azure" {
		t.Errorf("nearest to blue = %q, want the alphabetically first of the two", name)
	}

	if _, ok := (Palette{}).Nearest(color.RGBA{}); ok {
		t.Error("an empty palette has no nearest colour")
	}
}

func TestHSVRoundTrip(t *testing.T) {
	for _, value := range []color.RGBA{
		{R: 255, A: 255}, {G: 128, B: 64, A: 255}, {R: 12, G: 34, B: 56, A: 255},
		{R: 255, G: 255, B: 255, A: 255}, {A: 255},
	} {
		hue, saturation, brightness := RGBToHSV(value)

		if back := HSVToRGB(hue, saturation, brightness); back != value {
			t.Errorf("%v came back as %v", value, back)
		}
	}
}
