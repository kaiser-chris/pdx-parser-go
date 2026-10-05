package database

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// thing is a made-up kind of definition, so that these tests show the package
// works without any particular game.
type thing struct {
	Definition

	Size    float64
	Label   string
	Enabled Opt[bool]
	Tags    []string
	Parts   []Amount
	Effects []Modifier
	Formula ScriptValue
}

func decodeThing(r *Reader, _ *Definition) *thing {
	return &thing{
		Size:    r.NumberOr("size", 1),
		Label:   r.Text("label"),
		Enabled: r.Flag("enabled"),
		Tags:    r.Names("tags"),
		Parts:   r.NumberMap("parts"),
		Effects: r.Modifiers("effects"),
		Formula: r.ScriptValue("formula"),
	}
}

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()

	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))

		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// load reads common/things from a game and a mod.
func load(t *testing.T, files map[string]string) (Table[thing], report.Diagnostics) {
	t.Helper()

	root := t.TempDir()
	writeTree(t, root, files)

	set := folders.Open([]folders.Source{
		{Name: "game", Path: filepath.Join(root, "game")},
		{Name: "mod", Path: filepath.Join(root, "mod")},
	})

	collector := &report.Collector{}
	store := Collect(set, folders.Folder{Path: "common/things"}, "thing", collector)

	return Decode(store, collector, decodeThing), collector.Diagnostics
}

func mentions(diagnostics report.Diagnostics, severity report.Severity, fragment string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == severity && strings.Contains(diagnostic.String(), fragment) {
			return true
		}
	}

	return false
}

func expect(t *testing.T, diagnostics report.Diagnostics, severity report.Severity, fragment string) {
	t.Helper()

	if !mentions(diagnostics, severity, fragment) {
		var lines []string
		for _, diagnostic := range diagnostics {
			lines = append(lines, "  "+diagnostic.String())
		}

		t.Errorf("no %s mentioning %q; got:\n%s", severity, fragment, strings.Join(lines, "\n"))
	}
}

func TestDecode(t *testing.T) {
	things, diagnostics := load(t, map[string]string{
		"game/common/things/00_things.txt": `
first = { # The first one
	size = 3
	label = "First"
	enabled = yes
	tags = { a b }
	parts = { wheel = 4 axle = 2 wheel = 6 }
	effects = { speed_add = 0.5 flag = yes target = key }
	formula = named_value
}
second = { }
`,
	})

	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diagnostics)
	}

	if keys := things.Keys(); !slices.Equal(keys, []string{"first", "second"}) {
		t.Fatalf("keys = %v", keys)
	}

	first, _ := things.Get("first")
	if first.Size != 3 || first.Label != "First" || !first.Enabled.Value || !first.Enabled.Set ||
		!slices.Equal(first.Tags, []string{"a", "b"}) || first.Formula.Name != "named_value" {
		t.Errorf("first = %+v", first)
	}

	if !slices.Equal(first.Parts, []Amount{{"wheel", 6}, {"axle", 2}}) {
		t.Errorf("parts = %v, want the order of first mention and the last value", first.Parts)
	}

	if len(first.Effects) != 3 || first.Effects[1].Value != 1 || first.Effects[2].Text != "key" {
		t.Errorf("effects = %+v", first.Effects)
	}

	if first.Comment != "The first one" || first.Origin().Line != 2 || first.Origin().File != "common/things/00_things.txt" {
		t.Errorf("definition = %+v", first.Definition)
	}

	second, _ := things.Get("second")
	if second.Size != 1 || second.Enabled.Set || second.Formula.Set() {
		t.Errorf("second = %+v, want defaults", second)
	}
}

func TestModificationModes(t *testing.T) {
	things, diagnostics := load(t, map[string]string{
		"game/common/things/00_things.txt": `
listed = { tags = { a b } size = 2 }
replaced = { size = 5 label = "old" }
plain = { size = 1 }
`,
		"mod/common/things/zz_mod.txt": `
INJECT:listed = { tags = { b c } size = 9 }
REPLACE:replaced = { size = 6 }
plain = { size = 7 }
TRY_INJECT:nothing = { }
TRY_REPLACE:nothing = { }
INJECT_OR_CREATE:created = { size = 8 }
REPLACE_OR_CREATE:also_created = { }
INJECT:missing = { }
REPLACE:missing_too = { }
NEW_MODE:future = { }
not_a_block = 5
comparison >= { }
`,
	})

	listed, _ := things.Get("listed")
	if !slices.Equal(listed.Tags, []string{"a", "b", "c"}) || listed.Size != 9 {
		t.Errorf("an injection should join lists and override single values: %+v", listed)
	}

	if len(listed.Origins) != 2 || listed.Origins[1].Mode != ModeInject || listed.Origins[1].Source != "mod" {
		t.Errorf("origins = %+v", listed.Origins)
	}

	replaced, _ := things.Get("replaced")
	if replaced.Size != 6 || replaced.Label != "" || len(replaced.Overridden) != 1 {
		t.Errorf("a replacement should drop the old fields and remember them: %+v", replaced)
	}

	plain, _ := things.Get("plain")
	if plain.Size != 7 {
		t.Errorf("a plain definition of the same key overrides: %+v", plain)
	}

	for _, key := range []string{"created", "also_created", "future"} {
		if !things.Has(key) {
			t.Errorf("%s should exist", key)
		}
	}

	for _, key := range []string{"nothing", "missing", "missing_too", "not_a_block"} {
		if things.Has(key) {
			t.Errorf("%s should not exist", key)
		}
	}

	expect(t, diagnostics, report.SeverityInfo, "overrides the definition in common/things/00_things.txt from game")
	expect(t, diagnostics, report.SeverityInfo, "TRY_INJECT: skipped")
	expect(t, diagnostics, report.SeverityInfo, "TRY_REPLACE: skipped")
	expect(t, diagnostics, report.SeverityError, "INJECT: has nothing to modify")
	expect(t, diagnostics, report.SeverityError, "REPLACE: has nothing to modify")
	expect(t, diagnostics, report.SeverityWarning, "unknown modification NEW_MODE:")
	expect(t, diagnostics, report.SeverityWarning, "not a block")
	expect(t, diagnostics, report.SeverityWarning, "uses >= instead of =")
}

func TestReaderTolerance(t *testing.T) {
	things, diagnostics := load(t, map[string]string{
		"game/common/things/00_things.txt": `
odd = {
	size = "big"
	enabled = maybe
	label = { }
	tags = { a 5 "quoted" }
	parts = { wheel = four }
	effects = 7
	formula = rgb { 1 2 3 }
	colour = blue
	stray
}
`,
		"mod/common/things/zz_mod.txt": "INJECT:odd = { shiny = yes }\n",
	})

	odd, ok := things.Get("odd")
	if !ok {
		t.Fatal("a definition full of wrong values should still be read")
	}

	if odd.Size != 1 || odd.Enabled.Set || odd.Label != "" || !slices.Equal(odd.Tags, []string{"a", "quoted"}) || len(odd.Parts) != 0 {
		t.Errorf("odd = %+v, want defaults where the values were wrong", odd)
	}

	for _, fragment := range []string{
		"size should be a number", "enabled should be yes or no", "label should be a name or a string",
		"tags should list names", "parts.wheel should be a number", "effects should be a block",
		"formula should be a number, a script value or a block", "ignored stray, which has no key",
	} {
		expect(t, diagnostics, report.SeverityWarning, fragment)
	}

	expect(t, diagnostics, report.SeverityInfo, "unknown field colour")

	// A field injected by the mod is reported in the mod's file.
	var shiny report.Diagnostic
	for _, diagnostic := range diagnostics {
		if strings.Contains(diagnostic.Message, "unknown field shiny") {
			shiny = diagnostic
		}
	}

	if !strings.HasSuffix(filepath.ToSlash(shiny.Path), "mod/common/things/zz_mod.txt") || shiny.Source != "mod" {
		t.Errorf("the injected field is reported at %s from %s, want the mod's file", shiny.Path, shiny.Source)
	}

	if colour, ok := odd.Field("colour"); !ok || colour.Text != "blue" {
		t.Error("an unknown field should stay readable through the raw script")
	}
}

func TestChildReaders(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"game/common/things/a.txt": "holder = { entry = { size = 1 } entry = { size = x extra = 2 } }\n",
	})

	set := folders.Open([]folders.Source{{Name: "game", Path: filepath.Join(root, "game")}})
	collector := &report.Collector{}
	store := Collect(set, folders.Folder{Path: "common/things"}, "thing", collector)

	var sizes []float64

	Decode(store, collector, func(r *Reader, _ *Definition) *thing {
		for _, block := range r.Blocks("entry") {
			child := r.Child(block.Index, block.Value, "entry")
			sizes = append(sizes, child.NumberOr("size", -1))
			child.Finish()
		}

		return &thing{}
	})

	if !slices.Equal(sizes, []float64{1, -1}) {
		t.Errorf("sizes = %v", sizes)
	}

	expect(t, collector.Diagnostics, report.SeverityWarning, "thing holder, entry: size should be a number")
	expect(t, collector.Diagnostics, report.SeverityInfo, "thing holder, entry: unknown field extra")
}

func TestDefines(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"game/common/defines/00_defines.txt":        "NEconomy = { PRICE_RANGE = 0.75 MIN = 1 }\nNEconomy = { EXTRA = 3 }\nbroken = 5\n",
		"game/common/defines/jomini/00_tooltip.txt": "NTooltip = { DELAY = 0.3f; }\n",
		"mod/common/defines/zz_mod.txt":             "NEconomy = { PRICE_RANGE = 0.5 }\n",
	})

	set := folders.Open([]folders.Source{
		{Name: "game", Path: filepath.Join(root, "game")},
		{Name: "mod", Path: filepath.Join(root, "mod")},
	})

	collector := &report.Collector{}
	defines := LoadDefines(set, collector)

	checks := map[string]float64{"PRICE_RANGE": 0.5, "MIN": 1, "EXTRA": 3}
	for key, want := range checks {
		if got, _ := defines.Number("NEconomy", key); got != want {
			t.Errorf("NEconomy.%s = %v, want %v", key, got, want)
		}
	}

	if got, _ := defines.Number("NTooltip", "DELAY"); got != 0.3 {
		t.Errorf("defines in subfolders should be read, got %v", got)
	}

	if defines.NumberOr("NEconomy", "MISSING", 42) != 42 || defines.Len() != 4 {
		t.Errorf("fallback or count wrong: %d", defines.Len())
	}

	if !slices.Equal(defines.Namespaces(), []string{"NEconomy", "NTooltip"}) {
		t.Errorf("namespaces = %v", defines.Namespaces())
	}

	if define, _ := defines.Get("NEconomy", "PRICE_RANGE"); define.Origin.Source != "mod" {
		t.Errorf("PRICE_RANGE should come from the mod: %+v", define.Origin)
	}

	expect(t, collector.Diagnostics, report.SeverityWarning, "define namespace broken: ignored")
}

func TestSplitKey(t *testing.T) {
	cases := []struct {
		raw   string
		mode  Mode
		key   string
		known bool
	}{
		{"steel", ModeCreate, "steel", true},
		{"INJECT:steel", ModeInject, "steel", true},
		{"REPLACE_OR_CREATE:steel", ModeReplaceOrCreate, "steel", true},
		{"cu:north_german", ModeCreate, "cu:north_german", true},
		{"FUTURE:steel", ModeCreate, "steel", false},
		{"INJECT:", ModeCreate, "INJECT:", true},
	}

	for _, test := range cases {
		mode, key, known := SplitKey(test.raw)
		if mode != test.mode || key != test.key || known != test.known {
			t.Errorf("SplitKey(%q) = %v %q %v", test.raw, mode, key, known)
		}
	}

	if ModeInjectOrCreate.String() != "INJECT_OR_CREATE" || ModeCreate.String() != "CREATE" || !ModeTryInject.Injects() || ModeReplace.Injects() {
		t.Error("mode names or kinds are wrong")
	}
}

// TestDefinitionsThatAreNoBlocks covers definitions whose value is a number or
// a list, as the script values of Europa Universalis 5 are, and a scripted
// effect whose body is a parameter: stability_bonus = 7, call = { $effect$ }.
// The store keeps them all; only decoding into a typed definition, which reads
// fields, needs a block.
func TestDefinitionsThatAreNoBlocks(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"game/common/things/00_things.txt": "number = 7\nnegative = -0.5\ntext = hello\nparameter = { $effect$ }\nempty = { }\nblock = { size = 2 }\n",
		"mod/common/things/01_mod.txt":     "INJECT:number = { size = 3 }\nINJECT:block = 4\nREPLACE:text = { size = 5 }\n",
	})

	set := folders.Open([]folders.Source{
		{Name: "game", Path: filepath.Join(root, "game")},
		{Name: "mod", Path: filepath.Join(root, "mod")},
	})

	collector := &report.Collector{}
	store := Collect(set, folders.Folder{Path: "common/things"}, "thing", collector)

	if want := []string{"number", "negative", "text", "parameter", "empty", "block"}; !slices.Equal(store.Keys(), want) {
		t.Errorf("keys = %v, want %v", store.Keys(), want)
	}

	if definition, _ := store.Get("negative"); definition.Raw.Number != -0.5 {
		t.Errorf("negative = %+v", definition.Raw)
	}

	if definition, _ := store.Get("parameter"); len(definition.Raw.Items) != 1 || definition.Raw.Items[0].Text != "$effect$" {
		t.Errorf("parameter = %+v", definition.Raw)
	}

	// Only blocks can be injected into, and only blocks injected; anything
	// else replaces what was there, with a warning.
	if definition, _ := store.Get("number"); !definition.Raw.IsBlock() || len(definition.Overridden) != 1 {
		t.Errorf("number = %+v", definition)
	}

	if definition, _ := store.Get("block"); definition.Raw.Number != 4 {
		t.Errorf("block = %+v", definition.Raw)
	}

	if definition, _ := store.Get("text"); !definition.Raw.IsBlock() {
		t.Errorf("text = %+v", definition.Raw)
	}

	expect(t, collector.Diagnostics, report.SeverityWarning, "INJECT: cannot inject into 7, which is not a block; replaced it instead")
	expect(t, collector.Diagnostics, report.SeverityWarning, "INJECT: cannot inject 4, which is not a block; replaced it instead")

	decoded := Decode(store, collector, decodeThing)

	if want := []string{"number", "text", "empty"}; !slices.Equal(decoded.Keys(), want) {
		t.Errorf("decoded keys = %v, want %v", decoded.Keys(), want)
	}

	expect(t, collector.Diagnostics, report.SeverityWarning, "thing negative: ignored, because its value -0.5 is not a block")
}

// TestAddNamed covers definitions that carry their name inside, all under one
// field key, the way the .asset files write theirs. The name given is the
// key, without a modification prefix read from it, and a second definition
// of a name overrides the first completely.
func TestAddNamed(t *testing.T) {
	document := script.Parse(`
thing = { name = "INJECT:odd" size = 1 }
thing = { name = "plain" size = 2 }
thing = { name = "plain" label = "Again" }
`)

	file := folders.File{Relative: "gfx/things.asset", Path: "things.asset", Source: "game"}
	collector := &report.Collector{}
	store := NewStore("thing")

	for _, field := range document.Fields {
		name, _ := field.Value.Get("name")
		store.AddNamed(name.Text, field, "", file, collector)
	}

	if want := []string{"INJECT:odd", "plain"}; !slices.Equal(store.Keys(), want) {
		t.Errorf("keys = %v, want %v", store.Keys(), want)
	}

	odd, _ := store.Get("INJECT:odd")
	if odd.Origin().Mode != ModeCreate {
		t.Errorf("INJECT:odd was read with mode %v, want a plain definition", odd.Origin().Mode)
	}

	plain, _ := store.Get("plain")
	if _, ok := plain.Field("size"); ok || len(plain.Overridden) != 1 || plain.Origin().Line != 4 {
		t.Errorf("plain = %+v, want the second definition alone, overriding the first", plain)
	}

	expect(t, collector.Diagnostics, report.SeverityInfo, "thing plain: overrides the definition in gfx/things.asset from game")
}
