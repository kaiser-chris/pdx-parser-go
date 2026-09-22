package victoria3

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"math/rand"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// decodeEverything reads one document as every kind of definition at once and
// links the result, which is the most any input can ask of the decoders.
func decodeEverything(input string) *Data {
	document := script.Parse(input)
	file := folders.File{Relative: "common/test.txt", Path: "test.txt", Source: "test"}
	collector := &report.Collector{}
	data := &Data{}

	data.Defines.AddDocument(document, file, collector)

	store := func(kind string) *database.Store {
		s := database.NewStore(kind)
		s.AddDocument(document, file, collector)

		return s
	}

	data.Goods = database.Decode(store("good"), collector, func(r *database.Reader, _ *database.Definition) *Good { return decodeGood(r, data.Defines) })
	data.BuildingGroups = database.Decode(store("building group"), collector, func(r *database.Reader, _ *database.Definition) *BuildingGroup { return decodeBuildingGroup(r) })
	data.Buildings = database.Decode(store("building"), collector, func(r *database.Reader, _ *database.Definition) *Building { return decodeBuilding(r) })
	data.ProductionMethodGroups = database.Decode(store("production method group"), collector, func(r *database.Reader, _ *database.Definition) *ProductionMethodGroup {
		return decodeProductionMethodGroup(r)
	})
	data.ProductionMethods = database.Decode(store("production method"), collector, func(r *database.Reader, _ *database.Definition) *ProductionMethod { return decodeProductionMethod(r) })
	data.PopTypes = database.Decode(store("pop type"), collector, func(r *database.Reader, _ *database.Definition) *PopType { return decodePopType(r) })
	data.PopNeeds = database.Decode(store("pop need"), collector, func(r *database.Reader, _ *database.Definition) *PopNeed { return decodePopNeed(r) })
	data.BuyPackages = database.Decode(store("buy package"), collector, func(r *database.Reader, d *database.Definition) *BuyPackage { return decodeBuyPackage(r, d.Key) })
	data.Companies = database.Decode(store("company"), collector, func(r *database.Reader, _ *database.Definition) *Company { return decodeCompany(r) })
	data.Technologies = database.Decode(store("technology"), collector, func(r *database.Reader, _ *database.Definition) *Technology { return decodeTechnology(r) })

	index := 0
	data.Eras = database.Decode(store("era"), collector, func(r *database.Reader, d *database.Definition) *Era {
		index++

		return decodeEra(r, index-1, d.Comment)
	})

	link(data, collector)

	// Resolving costs as much as the group is deep, which is a few levels in
	// any real game, so a sample is enough; resolving every group of a
	// thirty thousand deep chain would only measure the test.
	for index, key := range data.BuildingGroups.Keys() {
		if index >= 100 {
			break
		}

		ResolveGroup(data, key, func(g *BuildingGroup) database.Opt[bool] { return g.EconomyOfScale })
	}

	for _, method := range data.ProductionMethods.All() {
		method.Inputs()
		method.Outputs()
		method.Employment()
	}

	data.Diagnostics = collector.Diagnostics

	return data
}

func decodeWithin(t *testing.T, input string, limit time.Duration) *Data {
	t.Helper()

	done := make(chan *Data, 1)

	go func() { done <- decodeEverything(input) }()

	select {
	case data := <-done:
		return data
	case <-time.After(limit):
		t.Fatalf("decoding %d bytes took longer than %s", len(input), limit)

		return nil
	}
}

// checkDiagnostics asserts that every diagnostic is complete enough to show.
func checkDiagnostics(t *testing.T, data *Data) {
	t.Helper()

	for _, diagnostic := range data.Diagnostics {
		if diagnostic.Message == "" {
			t.Errorf("diagnostic without a message: %+v", diagnostic)
		}

		if diagnostic.Severity > report.SeverityError {
			t.Errorf("diagnostic with an unknown severity: %+v", diagnostic)
		}
	}
}

// TestDecodeHostileShapes gives every field of every kind a value of every
// wrong shape. Nothing may panic, and every definition must still come out.
func TestDecodeHostileShapes(t *testing.T) {
	fields := []string{
		"cost", "category", "local", "texture", "building_group", "production_method_groups", "unlocking_technologies",
		"production_methods", "building_modifiers", "state_modifiers", "country_modifiers", "profession_ratio",
		"entry", "default", "goods", "political_strength", "building_types", "prosperity_modifier", "modifier", "era",
		"technology_cost", "parent_group", "required_construction", "ai_value", "color", "wage_weight", "replacement_if_valid",
		"unlocking_identity", "required_input_goods", "economy_of_scale", "cash_reserves_max",
	}

	shapes := []string{
		`1`, `-5.5`, `yes`, `no`, `"text"`, `name`, `{ }`, `{ a b c }`, `{ 1 2 3 }`, `{ a = 1 }`,
		`{ a = { b = c } }`, `rgb { 1 2 3 }`, `{ workforce_scaled = 5 }`, `{ workforce_scaled = { x = { } } }`,
		`{ { } { } }`, `{ x y = 1 z }`, `{ goods = { } weight = no }`, `@[ 1 / 0 ]`, `1836.1.1`,
	}

	for _, field := range fields {
		for index, shape := range shapes {
			input := fmt.Sprintf("thing_%d = { %s = %s }\nnext = { texture = \"x\" }", index, field, shape)

			data := decodeWithin(t, input, 5*time.Second)
			checkDiagnostics(t, data)

			if !data.Goods.Has(fmt.Sprintf("thing_%d", index)) || !data.Goods.Has("next") {
				t.Errorf("%s = %s: a definition was lost", field, shape)
			}
		}
	}
}

// TestDecodeAtScale builds definitions far larger than any game or mod has,
// shaped to reach every loop in the decoders and the reference checks.
func TestDecodeAtScale(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}

	const count = 30_000

	var (
		manyNames     strings.Builder
		manyNeeds     strings.Builder
		manyModifiers strings.Builder
		manyInjects   strings.Builder
		manyGroups    strings.Builder
		manyTechs     strings.Builder
	)

	for index := range count {
		fmt.Fprintf(&manyNames, "pm_missing_%d ", index)
		fmt.Fprintf(&manyNeeds, "popneed_%d = %d ", index, index)
		fmt.Fprintf(&manyModifiers, "goods_input_good_%d_add = 1 goods_input_iron_add = 1 ", index)
		fmt.Fprintf(&manyInjects, "INJECT:pmg_big = { production_methods = { pm_%d } }\n", index)
		fmt.Fprintf(&manyGroups, "bg_%d = { parent_group = bg_%d }\n", index, index+1)
		fmt.Fprintf(&manyTechs, "tech_%d = { era = era_1 unlocking_technologies = { tech_%d } }\n", index, index+1)
	}

	cases := map[string]string{
		"dangling names":      "pmg_big = { production_methods = { " + manyNames.String() + "} }",
		"large buy package":   "wealth_1 = { goods = { " + manyNeeds.String() + "} }",
		"many modifiers":      "pm_big = { building_modifiers = { workforce_scaled = { " + manyModifiers.String() + "} } }",
		"many injections":     "pmg_big = { production_methods = { pm_a } }\n" + manyInjects.String(),
		"long group chain":    manyGroups.String(),
		"long tech chain":     manyTechs.String(),
		"many unknown fields": "good = { " + strings.ReplaceAll(manyNeeds.String(), "popneed_", "field_") + "}",
	}

	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			start := time.Now()
			data := decodeWithin(t, input, 30*time.Second)
			t.Logf("%d bytes in %s, %d diagnostics", len(input), time.Since(start).Round(time.Millisecond), len(data.Diagnostics))
			checkDiagnostics(t, data)
		})
	}
}

// TestDecodeRandomDefinitions builds random definitions out of real field names
// and random values.
func TestDecodeRandomDefinitions(t *testing.T) {
	random := rand.New(rand.NewSource(1936))

	keys := []string{
		"cost", "category", "entry", "goods", "building_modifiers", "workforce_scaled", "level_scaled", "production_methods",
		"parent_group", "unlocking_technologies", "era", "modifier", "INJECT:a", "REPLACE:b", "a", "b", "weight",
		"goods_input_a_add", "building_employment_b_add", "profession_ratio", "default", "texture",
	}

	var value func(depth int) string

	value = func(depth int) string {
		switch choice := random.Intn(8); {
		case choice < 2 || depth > 4:
			return []string{"1", "-2", "yes", "no", "a", "b", `"s"`, "1836.1.1"}[random.Intn(8)]
		case choice < 4:
			return "{ " + strings.Repeat(keys[random.Intn(len(keys))]+" ", random.Intn(4)) + "}"
		default:
			var block strings.Builder

			block.WriteString("{ ")

			for range random.Intn(5) {
				block.WriteString(keys[random.Intn(len(keys))] + " = " + value(depth+1) + " ")
			}

			block.WriteString("}")

			return block.String()
		}
	}

	for range 3000 {
		var input strings.Builder

		for range random.Intn(6) {
			input.WriteString(keys[random.Intn(len(keys))] + " = " + value(0) + "\n")
		}

		data := decodeWithin(t, input.String(), 5*time.Second)
		checkDiagnostics(t, data)

		if t.Failed() {
			t.Fatalf("failed on input %q", input.String())
		}
	}
}

// FuzzDecode runs every decoder over inputs the fuzzer makes up. Run it with
//
//	go test ./victoria3 -run '^$' -fuzz FuzzDecode -fuzztime 60s
func FuzzDecode(f *testing.F) {
	for _, seed := range []string{
		"steel = { cost = 50 category = industrial }",
		"pm = { building_modifiers = { workforce_scaled = { goods_input_iron_add = 40 } level_scaled = { building_employment_laborers_add = 1 } } }",
		"popneed_x = { default = a entry = { goods = a weight = 1 } }",
		"wealth_1 = { political_strength = 1 goods = { popneed_x = 1 } }",
		"INJECT:a = { x = 1 } a = { } REPLACE:a = { y = 2 }",
		"bg_a = { parent_group = bg_b } bg_b = { parent_group = bg_a }",
		"t1 = { era = era_1 unlocking_technologies = { t2 } } t2 = { unlocking_technologies = { t1 } }",
		"NEconomy = { PRICE_RANGE = 0.75 }",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		checkDiagnostics(t, decodeEverything(input))
	})
}
