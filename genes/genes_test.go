package genes

import (
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

var testFile = folders.File{Relative: "common/genes/test.txt", Path: "test.txt", Source: "test"}

// read reads one file of genes.
func read(t *testing.T, input string) (*Genes, report.Diagnostics) {
	t.Helper()

	collector := &report.Collector{}
	document := script.Parse(input)

	for _, warning := range document.Warnings {
		collector.Addf(report.SeverityWarning, testFile.Source, testFile.Path, 0, 0, "", "%s", warning.Message)
	}

	return Read(document, testFile, collector), collector.Diagnostics
}

func expect(t *testing.T, diagnostics report.Diagnostics, severity report.Severity, fragment string) {
	t.Helper()

	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == severity && strings.Contains(diagnostic.String(), fragment) {
			return
		}
	}

	var lines []string
	for _, diagnostic := range diagnostics {
		lines = append(lines, "  "+diagnostic.String())
	}

	t.Errorf("no %s mentioning %q; got:\n%s", severity, fragment, strings.Join(lines, "\n"))
}

func expectClean(t *testing.T, diagnostics report.Diagnostics) {
	t.Helper()

	for _, diagnostic := range diagnostics {
		if diagnostic.Severity >= report.SeverityWarning || strings.Contains(diagnostic.Message, "unknown field") {
			t.Errorf("unexpected: %s", diagnostic)
		}
	}
}

func near(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

// A morph gene with everything the shipped files write in one.
func TestMorphGene(t *testing.T) {
	genes, diagnostics := read(t, `
@max = 0.8

age_presets = {
	age_preset_old = { mode = add curve = { { 0.7 0.0 } { 0.95 0.4 } } }
}

morph_genes = {
	gene_age = {
		group = eyes
		old_1 = {
			index = 2
			male = {
				setting = { attribute = "head_bs_old_1" value = { min = 0 max = @max } age = age_preset_old }
				setting = { attribute = "body_bs_shrink" required_tags = "shrink_arms, shrink_legs" value = { min = 0 max = 1 } }
				decal = {
					body_part = head
					textures = { diffuse = "a_diffuse.dds" normal = "a_normal.dds" properties = "a_properties.dds" }
					blend_modes = { diffuse = multiply normal = overlay properties = replace }
					alpha_curve = { { 0.0 0.0 } { 1.0 0.5 } }
					age = { mode = multiply curve = { { 0.0 0.0 } { 0.6 1.0 } } }
					priority = 3
				}
				texture_override = { weight = 100 diffuse = "d.dds" normal = "n.dds" specular = "s.dds" }
				hair_hsv_shift_curve = {
					curve = { { 0.0 { 0.0 -1.0 0.8 } } }
					age = age_preset_old
				}
			}
			female = { }
			boy = male
			girl = female
		}
	}
}
`)
	expectClean(t, diagnostics)

	gene, ok := genes.MorphGenes.Get("gene_age")
	if !ok || gene.Group != "eyes" || len(gene.Templates) != 1 {
		t.Fatalf("gene_age = %+v", gene)
	}

	template, _ := gene.Template("old_1")
	if template.Index != 2 {
		t.Errorf("index = %d", template.Index)
	}

	male := template.For(Male)
	if len(male.Settings) != 2 || len(male.Decals) != 1 || len(male.TextureOverrides) != 1 || len(male.HSVShifts) != 1 {
		t.Fatalf("male = %+v", male)
	}

	old := male.Settings[0]
	if old.Attribute != "head_bs_old_1" || old.Age.Preset != "age_preset_old" || !near(old.At(0.5), 0.4) {
		t.Errorf("setting = %+v, at 0.5 %g", old, old.At(0.5))
	}

	if tags := male.Settings[1].RequiredTags; len(tags) != 2 || tags[0] != "shrink_arms" || tags[1] != "shrink_legs" {
		t.Errorf("required tags = %q", tags)
	}

	decal := male.Decals[0]
	if decal.BodyPart != "head" || decal.Textures.Properties != "a_properties.dds" || decal.BlendModes.Diffuse != "multiply" ||
		decal.Priority != 3 || len(decal.AlphaCurve) != 2 || decal.Age.Mode != ModeMultiply || len(decal.Age.Curve) != 2 {
		t.Errorf("decal = %+v", decal)
	}

	if override := male.TextureOverrides[0]; override.Weight != 100 || override.Specular != "s.dds" {
		t.Errorf("texture override = %+v", override)
	}

	if shift := male.HSVShifts[0]; shift.Color != "hair" || len(shift.Curve) != 1 || shift.Curve[0].Shift != [3]float64{0, -1, 0.8} {
		t.Errorf("hsv shift = %+v", shift)
	}

	if boy := template.For(Boy); boy.SameAs != "male" || len(boy.Settings) != 2 {
		t.Errorf("boy = %+v", boy)
	}

	if girl := template.For(Girl); girl.SameAs != "female" || len(girl.Settings) != 0 {
		t.Errorf("girl = %+v", girl)
	}

	mode, curve, ok := genes.Age(old.Age)
	if !ok || mode != ModeAdd || !near(curve.At(0.95), 0.4) {
		t.Errorf("age = %s %v %v", mode, curve, ok)
	}
}

// A sex the same as one that is not written, as itself, or as no sex at all
// is reported and left empty, and a chain is followed to its end.
func TestSameAs(t *testing.T) {
	genes, diagnostics := read(t, `
morph_genes = {
	gene = {
		chained = {
			index = 0
			male = { setting = { attribute = a value = { min = 0 max = 1 } } }
			female = male
			girl = female
			boy = girl
		}
		broken = {
			index = 1
			male = male
			female = robot
			boy = girl
		}
	}
}
`)

	gene, _ := genes.MorphGenes.Get("gene")

	chained, _ := gene.Template("chained")
	for _, sex := range Sexes {
		if morph := chained.For(sex); len(morph.Settings) != 1 || sex != Male && morph.SameAs != "male" {
			t.Errorf("%s = %+v", sex, morph)
		}
	}

	broken, _ := gene.Template("broken")
	for _, sex := range Sexes {
		if morph := broken.For(sex); len(morph.Settings) != 0 || morph.SameAs != "" {
			t.Errorf("%s = %+v", sex, morph)
		}
	}

	expect(t, diagnostics, report.SeverityWarning, "male is the same as male, which is not given")
	expect(t, diagnostics, report.SeverityWarning, "female is the same as robot, which is no sex")
	expect(t, diagnostics, report.SeverityWarning, "boy is the same as girl, which is not given")
}

// A value written as one number is the value at full strength, from nothing
// at none, as Europa Universalis 5 writes some of its ageing settings.
func TestSingleValue(t *testing.T) {
	genes, diagnostics := read(t, `
morph_genes = {
	gene = {
		aged = {
			index = 0
			male = {
				setting = { attribute = a value = 0.7 }
				setting = { attribute = b value = -0.2 required_tags = "x, y" }
			}
		}
	}
}
`)

	for _, diagnostic := range diagnostics {
		t.Errorf("unexpected diagnostic: %s", diagnostic)
	}

	gene, _ := genes.MorphGenes.Get("gene")
	aged, _ := gene.Template("aged")

	settings := aged.For(Male).Settings
	if len(settings) != 2 {
		t.Fatalf("settings = %+v", settings)
	}

	if !near(settings[0].At(0), 0) || !near(settings[0].At(0.5), 0.35) || !near(settings[0].At(1), 0.7) {
		t.Errorf("a = %+v", settings[0])
	}

	if !near(settings[1].At(1), -0.2) || len(settings[1].RequiredTags) != 2 {
		t.Errorf("b = %+v", settings[1])
	}
}

// What does not fit is reported and read as far as it goes.
func TestBrokenMorphGene(t *testing.T) {
	genes, diagnostics := read(t, `
morph_genes = {
	gene = {
		size = {
			male = {
				setting = { value = { min = 0 } curve = { { 0 0 } { 1 } { 1 1 } nonsense } age = 5 }
				hair_hsv_shift_curve = { curve = { { 0 { 1 2 } } } }
				colour = blue
			}
			female = 3
		}
		nonsense = yes
	}
	gene_aged = {
		aged = { index = 0 male = { setting = { attribute = a age = age_preset_missing } } }
	}
}
special_genes = { decal_genes = { } }
`)

	gene, _ := genes.MorphGenes.Get("gene")
	size, _ := gene.Template("size")

	setting := size.For(Male).Settings[0]
	if len(setting.Curve) != 2 || setting.Value.Set || setting.Age.Preset != "" || setting.Age.Mode != "" {
		t.Errorf("setting = %+v", setting)
	}

	if shifts := size.For(Male).HSVShifts; len(shifts) != 1 || len(shifts[0].Curve) != 0 {
		t.Errorf("hsv shifts = %+v", shifts)
	}

	expect(t, diagnostics, report.SeverityWarning, "template size: has no index")
	expect(t, diagnostics, report.SeverityWarning, "has no attribute")
	expect(t, diagnostics, report.SeverityWarning, "has no min or max")
	expect(t, diagnostics, report.SeverityWarning, "a point of curve should be two numbers, found a list")
	expect(t, diagnostics, report.SeverityWarning, "a point of curve should be two numbers, found nonsense")
	expect(t, diagnostics, report.SeverityWarning, "age should name an age preset or be a block, found 5")
	expect(t, diagnostics, report.SeverityWarning, "a point of curve should be a number and three numbers")
	expect(t, diagnostics, report.SeverityInfo, "unknown field colour")
	expect(t, diagnostics, report.SeverityWarning, "female should be a block or the name of another sex, found 3")
	expect(t, diagnostics, report.SeverityInfo, "unknown field nonsense")
	expect(t, diagnostics, report.SeverityWarning, "morph gene gene_aged: age preset age_preset_missing does not exist")
	expect(t, diagnostics, report.SeverityInfo, "decal_genes is no kind of gene this package reads")
}

// An accessory gene picks among its accessories by weight.
func TestAccessoryGene(t *testing.T) {
	genes, diagnostics := read(t, `
special_genes = {
	accessory_genes = {
		hats = {
			inheritable = no
			tall_hats = {
				index = 3
				male = { 1 = top_hat 3 = "bowler" 1 = 1918 2 = { } }
				female = { }
				boy = male
				girl = female
			}
		}
	}
}
`)
	expect(t, diagnostics, report.SeverityWarning, "the accessory of weight 2 should be a name, found a list")

	if _, ok := genes.AccessoryGenes.Get("hats"); ok {
		t.Error("a special gene is among the genes of the DNA")
	}

	gene, ok := genes.AccessoryGene("hats")
	if !ok || gene.Inheritable.Value || !gene.Inheritable.Set {
		t.Fatalf("hats = %+v", gene)
	}

	template, _ := gene.Template("tall_hats")
	boy := template.For(Boy)

	if template.Index != 3 || boy.SameAs != "male" || len(boy.Choices) != 3 || boy.Choices[2].Accessory != "1918" {
		t.Fatalf("tall_hats = %+v", template)
	}

	for _, test := range []struct {
		strength float64
		want     string
	}{{0, "top_hat"}, {0.19, "top_hat"}, {0.2, "bowler"}, {0.79, "bowler"}, {0.8, "1918"}, {1, "1918"}, {-3, "top_hat"}, {7, "1918"}} {
		if choice, _ := boy.Pick(test.strength); choice.Accessory != test.want {
			t.Errorf("pick %g = %s, want %s", test.strength, choice.Accessory, test.want)
		}
	}

	if _, ok := template.For(Girl).Pick(0.5); ok {
		t.Error("picked from nothing")
	}
}

// Colour genes and age presets.
func TestColorGenesAndAgePresets(t *testing.T) {
	genes, diagnostics := read(t, `
age_presets = { preset = { mode = squared curve = { } } }
color_genes = {
	skin_color = { index = 1 sync_inheritance_with = hair_color color = skin blend_range = { 0.55 0.65 } }
	eye_color = { color = eye blend_range = { 0.5 } }
}
`)
	expect(t, diagnostics, report.SeverityInfo, "mode squared is not one this package knows of")
	expect(t, diagnostics, report.SeverityWarning, "blend_range should be two numbers")

	skin, _ := genes.ColorGenes.Get("skin_color")
	if skin.Color != "skin" || skin.Index.Value != 1 || skin.SyncInheritanceWith != "hair_color" || skin.BlendRange.Value != (Range{0.55, 0.65}) {
		t.Errorf("skin = %+v", skin)
	}

	if eye, _ := genes.ColorGenes.Get("eye_color"); eye.BlendRange.Set {
		t.Errorf("eye = %+v", eye)
	}

	if preset, _ := genes.AgePresets.Get("preset"); preset.Mode != "squared" || len(preset.Curve) != 0 {
		t.Errorf("preset = %+v", preset)
	}
}

func TestCurve(t *testing.T) {
	curve := Curve{{0, 1}, {0.5, 0}, {1, 1}}

	for _, test := range []struct{ x, want float64 }{{-1, 1}, {0, 1}, {0.25, 0.5}, {0.5, 0}, {0.75, 0.5}, {1, 1}, {2, 1}} {
		if got := curve.At(test.x); !near(got, test.want) {
			t.Errorf("at %g = %g, want %g", test.x, got, test.want)
		}
	}

	// Out of order, and a step where two points share an X.
	shuffled := Curve{{0.7, 0}, {-1, -1}, {0.6, 1}, {0.6, 2}}
	for _, test := range []struct{ x, want float64 }{{-1, -1}, {0.6, 1}, {0.61, 1.8}, {0.65, 1}, {0.7, 0}} {
		if got := shuffled.At(test.x); !near(got, test.want) {
			t.Errorf("shuffled at %g = %g, want %g", test.x, got, test.want)
		}
	}

	if got := (Curve{}).At(0.5); got != 0 {
		t.Errorf("empty curve = %g", got)
	}

	if got := (&Setting{}).At(0.3); got != 0.3 {
		t.Errorf("a setting without curve or value = %g", got)
	}
}

func TestDNA(t *testing.T) {
	collector := &report.Collector{}
	document := script.Parse(`
dna_one = {
	portrait_info = {
		genes = {
			hair_color = { 14 244 25 255 }
			gene_nose = { "nose_size" 300 nose_size 2.5 }
			gene_chin = { "chin_width" 127 }
			gene_jaw = { 1 2 "jaw" 3 }
			hairstyles = { "" 0 "short" 255 }
		}
		entity = { 0 0 }
	}
}
dna_two = { dna = "QfBB8FKg" enabled = no }
`)

	table := ReadDNA(document, folders.File{Relative: "common/dna_data/test.txt", Path: "test.txt", Source: "test"}, collector)
	diagnostics := collector.Diagnostics

	one, _ := table.Get("dna_one")
	if !one.Enabled || len(one.Genes) != 3 {
		t.Fatalf("dna_one = %+v", one)
	}

	if hair := one.Genes[0]; hair.Dominant != (Allele{Value: 14, Second: 244}) || hair.Recessive != (Allele{Value: 25, Second: 255}) {
		t.Errorf("hair = %+v", hair)
	}

	nose, _ := one.Gene("gene_nose")
	if nose.Dominant != (Allele{Template: "nose_size", Value: 255}) || nose.Recessive.Value != 2 || !near(nose.Dominant.Strength(), 1) {
		t.Errorf("nose = %+v", nose)
	}

	if hair, _ := one.Gene("hairstyles"); hair.Dominant.Template != "" || hair.Recessive.Template != "short" {
		t.Errorf("hairstyles = %+v", hair)
	}

	expect(t, diagnostics, report.SeverityWarning, "gene gene_nose: 300 should be a whole number from 0 to 255; read as 255")
	expect(t, diagnostics, report.SeverityWarning, "gene gene_nose: 2.5 should be a whole number from 0 to 255; read as 2")
	expect(t, diagnostics, report.SeverityWarning, "gene gene_chin should be four values")
	expect(t, diagnostics, report.SeverityWarning, "gene gene_jaw should be four numbers, or a template and a strength twice")

	two, _ := table.Get("dna_two")
	if two.Enabled || two.String != "QfBB8FKg" || len(two.Genes) != 0 {
		t.Errorf("dna_two = %+v", two)
	}
}

// Random and hostile input never makes the package panic.
func FuzzRead(f *testing.F) {
	f.Add(`morph_genes = { g = { t = { index = 0 male = { setting = { attribute = a curve = { { 0 0 } } } } boy = male } } }`)
	f.Add(`accessory_genes = { g = { t = { male = { 1 = a } female = female girl = boy boy = girl } } }`)
	f.Add(`e = { template = "f" skin_color = { 1 = { 0 0 1 1 } } gene = { 2 = { name = a range = { 0 1 } } } } f = { template = "e" }`)
	f.Add(`d = { portrait_info = { genes = { g = { "a" 1 "b" 2 } c = { 1 2 3 4 } } } }`)
	f.Add(`special_genes = { morph_genes = { g = { t = { male = { hair_hsv_shift_curve = { curve = { { 0 { 1 2 3 } } } } } } } } }`)
	f.Add(`idle = { male = { default = { head = "a" torso = "b" } v = { animation = { head = "c" } weight = { base = 1 } } } girl = male }`)

	f.Fuzz(func(t *testing.T, input string) {
		collector := &report.Collector{}
		document := script.Parse(input)
		genes := Read(document, testFile, collector)
		ReadDNA(document, testFile, collector)
		ReadAccessories(document, testFile, collector)
		ReadAnimations(document, testFile, collector)

		ethnicities := ReadEthnicities(document, testFile, collector)
		for key := range ethnicities.All() {
			if flat, err := Flatten(ethnicities, key); err == nil {
				genes.Preview(flat, PreviewLikeliest)
			}
		}

		for _, gene := range genes.MorphGenes.All() {
			for _, template := range gene.Templates {
				for _, sex := range Sexes {
					for _, setting := range template.For(sex).Settings {
						setting.At(0.5)
						genes.Age(setting.Age)
					}
				}
			}
		}

		for _, gene := range genes.AccessoryGenes.All() {
			for _, template := range gene.Templates {
				template.For(Male).Pick(0.5)
			}
		}
	})
}

// A morph sets the attributes of its settings, and two settings of one
// attribute add up.
func TestSetAttributes(t *testing.T) {
	morph := &Morph{Settings: []Setting{
		{Attribute: "a", Curve: Curve{{0, 0}, {1, 1}}},
		{Attribute: "b", Value: database.Some(Range{Min: -0.5, Max: 0.5})},
		{Attribute: "a", Value: database.Some(Range{Min: 1, Max: 1})},
		{Value: database.Some(Range{Min: 1, Max: 1})},
	}}

	attributes := map[string]float64{"c": 3}
	morph.SetAttributes(0.25, attributes)

	if len(attributes) != 3 || !near(attributes["a"], 1.25) || !near(attributes["b"], -0.25) || attributes["c"] != 3 {
		t.Errorf("attributes = %v", attributes)
	}
}

// Portrait accessories, and the entity a portrait draws of one by its tags.
func TestAccessories(t *testing.T) {
	collector := &report.Collector{}
	document := script.Parse(`
male_hair_01 = {
	entity = { required_tags = "no_hair" shared_pose_entity = head }
	entity = { required_tags = "cap1, bonnet" shared_pose_entity = head entity = "male_hair_01_cap1_entity" }
	entity = { required_tags = "" shared_pose_entity = head entity = "male_hair_01_entity" }
}
prop_saber = {
	set_tags = "armed,proud"
	entity = { node = "bn_l_prop" entity = "prop_saber_entity" colour = red }
}
`)

	table := ReadAccessories(document, folders.File{Relative: "gfx/portraits/accessories/test.txt", Path: "test.txt", Source: "test"}, collector)
	expect(t, collector.Diagnostics, report.SeverityInfo, "unknown field colour")

	hair, _ := table.Get("male_hair_01")
	if len(hair.Entities) != 3 || hair.Entities[1].SharedPoseEntity != "head" || len(hair.Entities[1].RequiredTags) != 2 {
		t.Fatalf("hair = %+v", hair)
	}

	for _, test := range []struct {
		tags []string
		want string
	}{{nil, "male_hair_01_entity"}, {[]string{"no_hair"}, ""}, {[]string{"cap1"}, "male_hair_01_entity"}, {[]string{"bonnet", "cap1"}, "male_hair_01_cap1_entity"}} {
		if chosen, ok := hair.Choose(test.tags); !ok || chosen.Entity != test.want {
			t.Errorf("with tags %v chose %+v, want %q", test.tags, chosen, test.want)
		}
	}

	saber, _ := table.Get("prop_saber")
	if len(saber.SetTags) != 2 || saber.SetTags[1] != "proud" || saber.Entities[0].Node != "bn_l_prop" {
		t.Errorf("saber = %+v", saber)
	}

	if _, ok := (&Accessory{Entities: []AccessoryEntity{{RequiredTags: []string{"x"}}}}).Choose(nil); ok {
		t.Error("chose an entity whose tags the portrait does not have")
	}
}

// A tag written as not(tag) is one the portrait must not have, as Europa
// Universalis 5 draws its hair where no hat covers it.
func TestAccessoryWithoutATag(t *testing.T) {
	hair := &Accessory{Entities: []AccessoryEntity{
		{RequiredTags: []string{"no_hair"}},
		{RequiredTags: []string{"not(hair_covered)"}, Entity: "bob_entity"},
		{RequiredTags: []string{"not(hair_covered)"}, Entity: "curly_entity"},
		{RequiredTags: []string{"hair_covered"}, Entity: "hat_hair_entity"},
	}}

	for _, test := range []struct {
		tags []string
		want string
	}{{nil, "bob_entity"}, {[]string{"hair_covered"}, "hat_hair_entity"}, {[]string{"no_hair"}, ""}, {[]string{"other"}, "bob_entity"}} {
		if chosen, ok := hair.Choose(test.tags); !ok || chosen.Entity != test.want {
			t.Errorf("with tags %v chose %+v, want %q", test.tags, chosen, test.want)
		}
	}
}

// Age changes a value by the curve of its preset or its own.
func TestAtAge(t *testing.T) {
	genes, _ := read(t, `age_presets = {
		grown = { mode = multiply curve = { { 0.0 0.0 } { 0.2 1.0 } } }
		aged = { mode = add curve = { { 0.5 0.0 } { 1.0 0.5 } } }
	}`)

	for _, test := range []struct {
		age  Age
		at   float64
		want float64
	}{
		{Age{Preset: "grown"}, 0.1, 0.25},
		{Age{Preset: "grown"}, 0.5, 0.5},
		{Age{Preset: "aged"}, 1, 1},
		{Age{Curve: Curve{{0, 2}}}, 0.3, 1},
		{Age{Preset: "missing"}, 0.3, 0.5},
		{Age{}, 0.3, 0.5},
	} {
		if got := genes.AtAge(0.5, test.age, test.at); !near(got, test.want) {
			t.Errorf("%+v at %g = %g, want %g", test.age, test.at, got, test.want)
		}
	}

	morph := &Morph{Settings: []Setting{{Attribute: "a", Value: database.Some(Range{Min: 1, Max: 1}), Age: Age{Preset: "grown"}}}}
	attributes := map[string]float64{}
	genes.SetAttributes(morph, 0.5, 0.1, attributes)

	if !near(attributes["a"], 0.5) {
		t.Errorf("attributes = %v", attributes)
	}
}

// A decal is as opaque as its alpha curve says, or as the gene is strong
// without one, changed by its age and kept between 0 and 1.
func TestDecalAlpha(t *testing.T) {
	genes, _ := read(t, `age_presets = {
		grown = { mode = multiply curve = { { 0.0 0.0 } { 0.2 1.0 } } }
		aged = { mode = add curve = { { 0.5 0.0 } { 1.0 1.0 } } }
	}`)

	for _, test := range []struct {
		decal    Decal
		strength float64
		at       float64
		want     float64
	}{
		{Decal{}, 0.4, 0.5, 0.4},
		{Decal{AlphaCurve: Curve{{0, 0}, {0.75, 1}, {1, 1}}}, 0.375, 0.5, 0.5},
		{Decal{Age: Age{Preset: "grown"}}, 0.8, 0.1, 0.4},
		{Decal{Age: Age{Mode: ModeMultiply, Curve: Curve{{0.25, 0}, {0.6, 1}}}}, 1, 0.25, 0},
		{Decal{Age: Age{Preset: "aged"}}, 0.6, 1, 1},
		{Decal{AlphaCurve: Curve{{0, -1}}}, 0.5, 0.5, 0},
	} {
		if got := genes.DecalAlpha(&test.decal, test.strength, test.at); !near(got, test.want) {
			t.Errorf("%+v at strength %g, age %g = %g, want %g", test.decal, test.strength, test.at, got, test.want)
		}
	}
}

// A decal says whether it is laid before the skin colour or after, and an
// order the package does not know is reported and laid before.
func TestDecalApplyOrder(t *testing.T) {
	genes, diagnostics := read(t, `morph_genes = {
		gene_brows = {
			brows = {
				index = 0
				male = {
					decal = { body_part = head decal_apply_order = post_skin_color }
					decal = { body_part = head decal_apply_order = pre_skin_color }
					decal = { body_part = head }
					decal = { body_part = head decal_apply_order = sideways }
				}
			}
		}
	}`)
	expect(t, diagnostics, report.SeverityWarning, "decal_apply_order sideways")

	gene, _ := genes.MorphGenes.Get("gene_brows")
	decals := gene.Templates[0].For(Male).Decals

	if len(decals) != 4 {
		t.Fatalf("decals = %+v", decals)
	}

	for index, want := range []bool{true, false, false, false} {
		if decals[index].PostSkinColor() != want {
			t.Errorf("decal %d = %+v, after the skin colour %v", index, decals[index], !want)
		}
	}
}

// An ethnicity builds on the one its template names, replacing the genes it
// lists, and previews either neutral or at its likeliest choices.
func TestEthnicities(t *testing.T) {
	genes, _ := read(t, `
color_genes = { skin_color = { color = skin } }
morph_genes = {
	gene_nose = {
		big = { index = 1 male = { } }
		small = { index = 0 male = { } }
	}
	gene_chin = { chin = { index = 0 male = { } } }
}
accessory_genes = {
	hairstyles = {
		no_hair = { index = 0 male = { } }
		short = { index = 1 male = { 1 = short_01 } }
	}
}`)

	collector := &report.Collector{}
	table := ReadEthnicities(script.Parse(`
base = {
	visible = no
	skin_color = { 10 = { 0.2 0.4 0.6 0.8 } 30 = { 0.0 0.0 0.5 0.5 } }
	gene_nose = {
		20 = { name = small range = { 0.4 0.6 } }
		20 = { name = big range = { 0.7 0.9 } }
	}
	gene_chin = { 1 = { name = chin range = { 0.0 1.0 } } }
}
child = {
	template = "base"
	using = "saxon"
	gene_nose = { 5 = { name = big range = { 0.8 1.0 } } }
	hairstyles = { 10 = { name = short } }
	gene_odd = { heavy = { name = x } 3 = nonsense 4 = { range = { 0 1 } } }
	skin = 5
}
loop_a = { template = "loop_b" }
loop_b = { template = "loop_a" }
lost = { template = "nowhere" }
`), folders.File{Relative: "common/ethnicities/test.txt", Path: "test.txt", Source: "test"}, collector)

	diagnostics := collector.Diagnostics
	expect(t, diagnostics, report.SeverityWarning, "heavy should be the weight of a choice")
	expect(t, diagnostics, report.SeverityWarning, "a choice should be a block of a name and a range, found nonsense")
	expect(t, diagnostics, report.SeverityWarning, "has no name; the choice is left out")
	expect(t, diagnostics, report.SeverityWarning, "has no range; it is taken as 0 to 1")
	expect(t, diagnostics, report.SeverityWarning, "gene skin should be a block of weighted choices")
	expect(t, diagnostics, report.SeverityWarning, "its chain of templates goes round in a loop at ethnicity loop_a")
	expect(t, diagnostics, report.SeverityWarning, "builds on ethnicity nowhere, which does not exist")

	base, _ := table.Get("base")
	if base.Visible || len(base.Colors) != 1 || len(base.Genes) != 2 {
		t.Errorf("base = %+v", base)
	}

	child, err := Flatten(table, "child")
	if err != nil {
		t.Fatal(err)
	}

	if child.Key != "child" || child.Using != "saxon" || !child.Visible || len(child.Colors) != 1 {
		t.Errorf("child = %+v", child)
	}

	var names []string
	for _, gene := range child.Genes {
		names = append(names, gene.Gene)
	}

	// The base's genes in its order, the child's nose in its place, then
	// what the child adds.
	if strings.Join(names, " ") != "gene_nose gene_chin hairstyles gene_odd" {
		t.Errorf("genes = %v", names)
	}

	nose, _ := child.Gene("gene_nose")
	if choice, _ := nose.Likeliest(); choice.Template != "big" || choice.Range != (Range{0.8, 1.0}) {
		t.Errorf("the child's nose = %+v", choice)
	}

	// The first of two of the same weight is the likeliest.
	baseNose, _ := base.Gene("gene_nose")
	if choice, _ := baseNose.Likeliest(); choice.Template != "small" {
		t.Errorf("the base's nose = %+v", choice)
	}

	for _, test := range []struct {
		preview Preview
		want    []GeneValue
	}{
		{PreviewNeutral, []GeneValue{
			{Gene: "skin_color", Colour: true},
			{Gene: "gene_nose", Dominant: Allele{Template: "small", Value: 127}, Recessive: Allele{Template: "small", Value: 127}},
			{Gene: "gene_chin", Dominant: Allele{Template: "chin", Value: 127}, Recessive: Allele{Template: "chin", Value: 127}},
			{Gene: "hairstyles", Dominant: Allele{Template: "no_hair"}, Recessive: Allele{Template: "no_hair"}},
		}},
		{PreviewLikeliest, []GeneValue{
			{Gene: "skin_color", Colour: true, Dominant: Allele{Value: 63, Second: 63}, Recessive: Allele{Value: 63, Second: 63}},
			{Gene: "gene_nose", Dominant: Allele{Template: "big", Value: 229}, Recessive: Allele{Template: "big", Value: 229}},
			{Gene: "gene_chin", Dominant: Allele{Template: "chin", Value: 127}, Recessive: Allele{Template: "chin", Value: 127}},
			{Gene: "hairstyles", Dominant: Allele{Template: "short", Value: 127}, Recessive: Allele{Template: "short", Value: 127}},
		}},
	} {
		dna := genes.Preview(child, test.preview)
		if len(dna.Genes) != len(test.want) {
			t.Fatalf("preview %d: %+v", test.preview, dna.Genes)
		}

		for index, want := range test.want {
			if dna.Genes[index] != want {
				t.Errorf("preview %d, gene %d: %+v, want %+v", test.preview, index, dna.Genes[index], want)
			}
		}
	}

	if _, err := Flatten(table, "missing"); err == nil {
		t.Error("flattened an ethnicity that does not exist")
	}
}

// A DNA the game writes out holds its genes at the top, with the sex, age and
// numbers of the portrait beside them.
func TestWrittenDNA(t *testing.T) {
	collector := &report.Collector{}
	table := ReadDNA(script.Parse(`key = {
		type = male
		id = 1076145720
		random_seed = 1076145720
		age = 0.5
		genes = { gene_nose = { "small" 127 "small" 127 } }
		entity = { 0 0 }
	}`), testFile, collector)

	for _, diagnostic := range collector.Diagnostics {
		t.Errorf("unexpected: %s", diagnostic)
	}

	dna, _ := table.Get("key")
	if dna.Sex != "male" || dna.Age.Value != 0.5 || dna.ID != "1076145720" || dna.RandomSeed != "1076145720" || len(dna.Genes) != 1 {
		t.Errorf("dna = %+v", dna)
	}
}

// An animation set gives each sex a default pair of states and its variants;
// a sex the same as another gets its animations, and a variant's weight and
// props are known but not read.
func TestReadAnimations(t *testing.T) {
	collector := &report.Collector{}
	table := ReadAnimations(script.Parse(`
idle = {
	male = {
		default = { head = "idle_entry" torso = "idle_entry" }
		smoker = {
			animation = { head = "smoke_head" torso = "smoke_torso" }
			weight = { base = 0 }
			portrait_modifier = { animations_props = smoking_attachment }
		}
	}
	female = { default = { head = "idle3" torso = "idle3" } portrait_modifier = { animations = expression_angry } }
	boy = { default = { head = "idle1" torso = "idle1" } }
	girl = boy
	infant = boy
	adolescent_boy = { default = { head = "idle1" torso = "idle1" } }
}
`), testFile, collector)

	idle, ok := table.Get("idle")
	if !ok {
		t.Fatalf("idle not read; diagnostics %v", collector.Diagnostics)
	}

	male := idle.For(Male)
	if male.Default != (StatePair{Head: "idle_entry", Torso: "idle_entry"}) {
		t.Errorf("male default = %+v", male.Default)
	}

	if len(male.Variants) != 1 || male.Variants[0].Name != "smoker" || male.Variants[0].Animation != (StatePair{Head: "smoke_head", Torso: "smoke_torso"}) {
		t.Errorf("male variants = %+v", male.Variants)
	}

	if modifiers := male.Variants[0].Modifiers; !slices.Equal(modifiers, []ModifierChoice{{Group: "animations_props", Modifier: "smoking_attachment"}}) {
		t.Errorf("the smoker's modifiers are %+v, want its props", modifiers)
	}

	if girl := idle.For(Girl); girl.SameAs != "boy" || girl.Default.Torso != "idle1" {
		t.Errorf("girl = %+v, want the boy's", girl)
	}

	if female := idle.For(Female); female.Default.Head != "idle3" || len(female.Variants) != 0 {
		t.Errorf("female = %+v", female)
	}

	// A portrait_modifier beside the default is the sex's own, not a variant.
	if modifiers := idle.For(Female).Modifiers; !slices.Equal(modifiers, []ModifierChoice{{Group: "animations", Modifier: "expression_angry"}}) {
		t.Errorf("the woman's modifiers are %+v, want her expression", modifiers)
	}

	for _, diagnostic := range collector.Diagnostics {
		t.Errorf("unexpected diagnostic: %s", diagnostic)
	}
}

// A sex without a default, a variant that is no block, and one without an
// animation are reported, and what can be read still is.
func TestReadAnimationsReportsProblems(t *testing.T) {
	collector := &report.Collector{}
	table := ReadAnimations(script.Parse(`idle = { male = { odd = 3 bare = { weight = 1 } } }`), testFile, collector)

	idle, ok := table.Get("idle")
	if !ok {
		t.Fatal("idle not read")
	}

	if male := idle.For(Male); male.Default != (StatePair{}) || len(male.Variants) != 1 || male.Variants[0].Name != "bare" {
		t.Errorf("male = %+v", male)
	}

	if len(collector.Diagnostics) != 3 {
		t.Errorf("diagnostics = %v, want the missing default, the odd variant and the bare one's missing animation", collector.Diagnostics)
	}
}

// A group of portrait modifiers has its settings and its modifiers, each
// with what it does to genes: a template at a strength, or at one picked
// from a range, which a portrait drawn without chance takes the middle of.
func TestReadModifiers(t *testing.T) {
	collector := &report.Collector{}
	table := ReadModifiers(script.Parse(`
animations_props = {
	usage = none
	saber_attachment = {
		dna_modifiers = {
			accessory = { mode = add gene = props template = gene_saber_01 value = 1.0 accessory = prop_saber type = male }
		}
	}
	smoking_attachment = {
		dna_modifiers = {
			accessory = { mode = add gene = props template = cigar_and_pipes range = { 0.2 0.6 } }
			morph = { mode = modify gene = gene_mouth_open template = mouth_open value = 0.25 }
		}
		weight = { base = 0 modifier = { add = 10 is_female = no } }
	}
}
beards = {
	usage = game
	selection_behavior = weighted_random
	fallback = no_beard
	priority = 3
	gruesome = no
	no_beard = { dna_modifiers = { accessory = { mode = add gene = beards template = no_beard range = { 0 1 } } } outfit_tags = { no_clothes } }
}
`), testFile, collector)

	for _, diagnostic := range collector.Diagnostics {
		t.Errorf("unexpected: %s", diagnostic)
	}

	props, ok := table.Get("animations_props")
	if !ok || props.Usage != "none" || len(props.Modifiers) != 2 || props.Priority.Set || props.Gruesome.Set {
		t.Fatalf("animations_props = %+v", props)
	}

	saber, ok := props.Modifier("saber_attachment")
	if !ok || len(saber.Genes) != 1 {
		t.Fatalf("saber_attachment = %+v", saber)
	}

	if gene := saber.Genes[0]; gene.Kind != "accessory" || gene.Mode != "add" || gene.Gene != "props" || gene.Template != "gene_saber_01" || gene.Strength() != 1 || gene.Accessory != "prop_saber" || gene.Type != "male" {
		t.Errorf("the saber's gene = %+v at %g", gene, gene.Strength())
	}

	smoking, _ := props.Modifier("smoking_attachment")
	if len(smoking.Genes) != 2 || smoking.Genes[0].Strength() != 0.4 || smoking.Genes[1].Kind != "morph" || smoking.Genes[1].Mode != "modify" {
		t.Errorf("smoking_attachment = %+v", smoking)
	}

	beards, _ := table.Get("beards")
	if beards.SelectionBehavior != "weighted_random" || beards.Fallback != "no_beard" || beards.Usage != "game" || beards.Priority.Or(0) != 3 || beards.Gruesome.Or(true) || len(beards.Modifiers) != 1 || !slices.Equal(beards.Modifiers[0].OutfitTags, []string{"no_clothes"}) {
		t.Errorf("beards = %+v", beards)
	}

	if _, ok := props.Modifier("missing"); ok {
		t.Error("found a modifier that is not there")
	}
}

// A modifier of the wrong shape, a gene modifier of a kind the package does
// not know and one that names no gene are reported, and what can be read
// stays.
func TestReadModifiersReportsProblems(t *testing.T) {
	collector := &report.Collector{}
	table := ReadModifiers(script.Parse(`
group = {
	odd = 3
	broken = {
		dna_modifiers = {
			colour = { gene = hair_color }
			accessory = { mode = add template = nothing range = { 1 } }
		}
	}
}
`), testFile, collector)

	group, _ := table.Get("group")
	if len(group.Modifiers) != 1 || len(group.Modifiers[0].Genes) != 1 || group.Modifiers[0].Genes[0].Strength() != 0 {
		t.Errorf("group = %+v", group)
	}

	expect(t, collector.Diagnostics, report.SeverityWarning, "modifier odd should be a block")
	expect(t, collector.Diagnostics, report.SeverityWarning, "range should be two numbers")

	if len(collector.Diagnostics) != 4 {
		t.Errorf("diagnostics = %v, want the odd modifier, the unknown colour, the missing gene and the range", collector.Diagnostics)
	}
}

// Europa Universalis 5's persistent DNA is a DNA of the same shape, with a
// priority and the tags of the characters it is for.
func TestPersistentDNA(t *testing.T) {
	collector := &report.Collector{}
	table := ReadDNA(script.Parse(`
magnus_eriksson = {
	priority = 1
	tags = { swe_magnus_eriksson }
	portrait_info = {
		genes = {
			hair_color={ 62 59 203 6 }
			gene_cheek_fat={ "template_2" 119 "template_2" 174 }
		}
	}
}
`), folders.File{Relative: "common/persistent_dna/test.txt", Path: "test.txt", Source: "test"}, collector)

	for _, diagnostic := range collector.Diagnostics {
		t.Errorf("unexpected diagnostic: %s", diagnostic)
	}

	dna, ok := table.Get("magnus_eriksson")
	if !ok || dna.Priority.Or(0) != 1 || !slices.Equal(dna.Tags, []string{"swe_magnus_eriksson"}) || len(dna.Genes) != 2 || !dna.Enabled {
		t.Errorf("dna = %+v", dna)
	}
}

// The DNA of a game is read from whichever folder it keeps it in.
func TestLoadDNAFromEitherFolder(t *testing.T) {
	root := t.TempDir()

	for _, file := range []struct{ path, text string }{
		{"victoria/common/dna_data/a.txt", `dna_a = { portrait_info = { genes = { hair_color = { 1 2 3 4 } } } }`},
		{"europa/common/persistent_dna/b.txt", `dna_b = { priority = 1 portrait_info = { genes = { hair_color = { 1 2 3 4 } } } }`},
	} {
		path := filepath.Join(root, filepath.FromSlash(file.path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(file.text), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	for game, key := range map[string]string{"victoria": "dna_a", "europa": "dna_b"} {
		table, _ := LoadDNA(folders.Open([]folders.Source{{Name: game, Path: filepath.Join(root, game)}}))
		if !table.Has(key) || table.Len() != 1 {
			t.Errorf("%s: %v, want %s", game, table.Keys(), key)
		}
	}
}
