package asset

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// readEverything reads one input and asks for every value the package hands
// out, which is the most any input can ask of it.
func readEverything(input string) (*Assets, report.Diagnostics) {
	collector := &report.Collector{}
	assets := Read(script.Parse(input), testFile, collector)

	for _, mesh := range assets.Meshes.All() {
		Resolve(mesh.Origin(), mesh.File)

		for _, animation := range assets.AnimationsOf(mesh) {
			Resolve(animation.Origin, animation.File)
		}

		for _, animation := range mesh.Animations {
			Resolve(mesh.Origin(), animation.File)
		}
	}

	// Following a chain costs as much as the chain is long, which is a few
	// entities in any real game, so a sample is enough; following every
	// entity of a thirty thousand long chain would only measure the test.
	for index, name := range assets.Entities.Keys() {
		if index >= 100 {
			break
		}

		assets.CloneChain(name)
		assets.MeshOf(name)
	}

	return assets, collector.Diagnostics
}

func readWithin(t *testing.T, input string, limit time.Duration) (*Assets, report.Diagnostics) {
	t.Helper()

	type result struct {
		assets      *Assets
		diagnostics report.Diagnostics
	}

	done := make(chan result, 1)

	go func() {
		assets, diagnostics := readEverything(input)
		done <- result{assets, diagnostics}
	}()

	select {
	case read := <-done:
		return read.assets, read.diagnostics
	case <-time.After(limit):
		t.Fatalf("reading %d bytes took longer than %s", len(input), limit)

		return nil, nil
	}
}

// checkDiagnostics asserts that every diagnostic is complete enough to show.
func checkDiagnostics(t *testing.T, diagnostics report.Diagnostics) {
	t.Helper()

	for _, diagnostic := range diagnostics {
		if diagnostic.Message == "" {
			t.Errorf("diagnostic without a message: %+v", diagnostic)
		}

		if diagnostic.Severity > report.SeverityError {
			t.Errorf("diagnostic with an unknown severity: %+v", diagnostic)
		}

		if diagnostic.Path != testFile.Path {
			t.Errorf("diagnostic placed outside the file it is about: %+v", diagnostic)
		}
	}
}

// TestHostileShapes gives every field of both kinds of definition, and of the
// blocks inside them, a value of every wrong shape. Nothing may panic, and
// every definition must still come out.
func TestHostileShapes(t *testing.T) {
	meshFields := []string{
		"file", "scale", "cull_distance", "animation", "additive_animation", "blend_shape", "lod_percentages",
		"meshsettings", "meshsettings = { index", "meshsettings = { texture", "meshsettings = { additional_shader_defines",
		"meshsettings = { shadow_shader", "lod_percentages = { lod", "lod_percentages = { lod = { percent",
		"blend_shape = { id", "animation = { type", "import", "import = { type", "import = { name", "streaming",
		"meshsettings = { rasterizerstate",
	}

	setFields := []string{"reference_skeleton", "animation", "animation = { id", "animation = { type"}

	entityFields := []string{
		"pdxmesh", "clone", "scale", "cull_radius", "default_state", "get_state_from_parent", "meshsettings",
		"attribute", "attribute = { default", "state", "state = { chance", "state = { chance = { if_current_state",
		"state = { time_offset", "state = { propagate_state", "state = { event", "state = { start_event",
		"state = { event = { sound", "state = { event = { soundparameter", "locator", "locator = { position",
		"locator = { rotation", "attach", "attach = { name", "attach = { pos", "group", "group = { attach",
		"group = { attach = { chance", "game_data", "state = { chance = { default",
		"state = { event = { sound = { stop_on_state_change",
	}

	shapes := []string{
		`1`, `-5.5`, `0`, `1e300`, `yes`, `no`, `"text"`, `""`, `name`, `inf`, `{ }`, `{ a b c }`, `{ 1 2 3 }`,
		`{ 1 2 3 4 5 }`, `{ a = 1 }`, `{ a = { b = c } }`, `rgb { 1 2 3 }`, `{ { } { } }`, `{ x y = 1 z }`,
		`@[ 1 / 0 ]`, `1836.1.1`, `{ name = "x" }`,
	}

	// A field given as "outer = { inner" is closed with as many braces as it
	// opened.
	wrap := func(field, shape string) string {
		return field + " = " + shape + strings.Repeat(" }", strings.Count(field, "{"))
	}

	kinds := map[string][]string{"pdxmesh": meshFields, "entity": entityFields, "skeletal_animation_set": setFields}

	for kind, fields := range kinds {
		for _, field := range fields {
			for index, shape := range shapes {
				name := fmt.Sprintf("thing_%d", index)
				input := fmt.Sprintf("%s = { name = %q %s }\n%s = { name = \"next\" }", kind, name, wrap(field, shape), kind)

				assets, diagnostics := readWithin(t, input, 5*time.Second)
				checkDiagnostics(t, diagnostics)

				has := map[string]func(string) bool{
					"pdxmesh":                assets.Meshes.Has,
					"entity":                 assets.Entities.Has,
					"skeletal_animation_set": assets.AnimationSets.Has,
				}[kind]

				if !has(name) || !has("next") {
					t.Errorf("%s %s: a definition was lost", kind, wrap(field, shape))
				}
			}
		}
	}
}

// TestAtScale builds files far larger than any game or mod has, shaped to
// reach every loop in the decoders and the reference checks.
func TestAtScale(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}

	const count = 30_000

	var (
		chain       strings.Builder
		cycle       strings.Builder
		dangling    strings.Builder
		attachments strings.Builder
		states      strings.Builder
		shapes      strings.Builder
		settings    strings.Builder
		unknown     strings.Builder
	)

	for index := range count {
		fmt.Fprintf(&chain, "entity = { name = \"e_%d\" clone = \"e_%d\" }\n", index, index+1)
		fmt.Fprintf(&cycle, "entity = { name = \"c_%d\" clone = \"c_%d\" }\n", index, (index+1)%count)
		fmt.Fprintf(&dangling, "entity = { name = \"d_%d\" pdxmesh = \"missing_%d\" clone = \"missing_%d\" }\n", index, index, index)
		fmt.Fprintf(&attachments, "pos_%d = \"missing_%d\" ", index, index)
		fmt.Fprintf(&states, "state = { name = \"s_%d\" animation = \"missing_%d\" event = { entity = \"missing_%d\" } } ", index, index, index)
		fmt.Fprintf(&shapes, "blend_shape = { id = \"bs_%d\" type = \"bs_%d.mesh\" } ", index, index)
		fmt.Fprintf(&settings, "meshsettings = { name = \"part_%d\" index = %d texture = { index = 1 file = \"t.dds\" } } ", index, index)
		fmt.Fprintf(&unknown, "field_%d = %d ", index, index)
	}

	cases := map[string]string{
		"long clone chain":    chain.String() + "entity = { name = \"e_30000\" pdxmesh = \"m\" }\npdxmesh = { name = \"m\" file = \"m.mesh\" }",
		"long clone cycle":    cycle.String(),
		"dangling references": dangling.String(),
		"many attachments":    "entity = { name = \"big\" attach = { " + attachments.String() + "} }",
		"many states":         "pdxmesh = { name = \"m\" file = \"m.mesh\" }\nentity = { name = \"big\" pdxmesh = \"m\" " + states.String() + "}",
		"many blend shapes":   "pdxmesh = { name = \"m\" file = \"m.mesh\" " + shapes.String() + "}",
		"many mesh settings":  "pdxmesh = { name = \"m\" file = \"m.mesh\" " + settings.String() + "}",
		"many unknown fields": "entity = { name = \"big\" " + unknown.String() + "}",
		"many redefinitions":  strings.Repeat("pdxmesh = { name = \"same\" file = \"same.mesh\" }\n", count),
	}

	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			start := time.Now()
			_, diagnostics := readWithin(t, input, 30*time.Second)
			t.Logf("%d bytes in %s, %d diagnostics", len(input), time.Since(start).Round(time.Millisecond), len(diagnostics))
			checkDiagnostics(t, diagnostics)
		})
	}
}

// TestRandomDefinitions builds random definitions out of real field names and
// random values.
func TestRandomDefinitions(t *testing.T) {
	random := rand.New(rand.NewSource(1444))

	keys := []string{
		"name", "file", "scale", "pdxmesh", "entity", "clone", "meshsettings", "animation", "blend_shape", "id", "type",
		"lod_percentages", "lod", "index", "percent", "state", "event", "start_event", "attach", "group", "chance",
		"locator", "position", "rotation", "attribute", "propagate_state", "if_current_state", "time_offset", "sound",
		"texture", "game_data", "import", "reference_skeleton", "streaming", "rasterizerstate", "default",
		"stop_on_state_change", "skeletal_animation_set", "a", "b",
	}

	var value func(depth int) string

	value = func(depth int) string {
		switch choice := random.Intn(8); {
		case choice < 2 || depth > 4:
			return []string{"1", "-2", "yes", "no", "a", "b", `"a"`, `"b"`, "inf", "1836.1.1"}[random.Intn(10)]
		case choice < 4:
			return "{ " + strings.Repeat([]string{"1 ", "a ", `"b" `}[random.Intn(3)], random.Intn(5)) + "}"
		default:
			var block strings.Builder

			block.WriteString("{ ")

			for range random.Intn(6) {
				block.WriteString(keys[random.Intn(len(keys))] + " = " + value(depth+1) + " ")
			}

			block.WriteString("}")

			return block.String()
		}
	}

	for range 3000 {
		var input strings.Builder

		for range random.Intn(6) {
			kind := []string{"pdxmesh", "entity", "skeletal_animation_set", "arrowType"}[random.Intn(4)]
			input.WriteString(kind + " = " + value(0) + "\n")
		}

		_, diagnostics := readWithin(t, input.String(), 5*time.Second)
		checkDiagnostics(t, diagnostics)

		if t.Failed() {
			t.Fatalf("failed on input %q", input.String())
		}
	}
}

// TestLoadOfNothing reads a set with no asset files at all.
func TestLoadOfNothing(t *testing.T) {
	assets := Load(folders.Open(nil))

	if assets.Meshes.Len() != 0 || assets.Entities.Len() != 0 || len(assets.Diagnostics) != 0 {
		t.Errorf("assets of nothing = %+v", assets)
	}
}

// FuzzRead runs the package over inputs the fuzzer makes up. Run it with
//
//	go test ./asset -run '^$' -fuzz FuzzRead -fuzztime 60s
func FuzzRead(f *testing.F) {
	for _, seed := range []string{
		`pdxmesh = { name = "m" file = "m.mesh" lod_percentages = { lod = { index = 1 percent = inf } } }`,
		`entity = { name = "e" pdxmesh = "m" clone = "e" attach = { name = w hand = "e" } }`,
		`entity = { name = "e" state = { name = "s" chance = { if_current_state = { "s" = 1 } } propagate_state = "s" } }`,
		`entity = { name = "e" group = { name = "g" attach = { chance = 1 pos = "e" } } locator = { name = l position = { 1 2 } } }`,
		`pdxmesh = { name = "m" meshsettings = { index = 1.5 texture = { index = 1 file = "t.dds" srgb = yes } } }`,
		`pdxmesh = { file = "nameless.mesh" } entity = "e" arrowType = { name = "a" }`,
		`skeletal_animation_set = { name = "s" reference_skeleton = "m" animation = { id = "a" type = "a.anim" } } pdxmesh = { name = "m" import = { type = skeletal_animation_set name = "s" } }`,
		`entity = { name = "e" state = { name = "s" chance = 1 chance = { if_current_state = { s = 1 } default = 0 } } }`,
		`@x = 2 entity = { name = "e" scale = @x locator = { name = "l" position = { @[x * 2] 0 0 } } }`,
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		_, diagnostics := readEverything(input)
		checkDiagnostics(t, diagnostics)
	})
}
