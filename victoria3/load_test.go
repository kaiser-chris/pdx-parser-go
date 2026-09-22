package victoria3

import (
	"os"
	"strings"
	"testing"

	"path/filepath"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
)

// writeTree creates files below root from a map of slash-separated paths to
// contents.
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

// vanilla is a small but complete game: enough of every kind of definition
// for all references to resolve.
var vanilla = map[string]string{
	"game/common/defines/00_defines.txt": `
NEconomy = {
	PRICE_RANGE = 0.75
	GOODS_DEFAULT_TRADE_QUANTITY = 10
}
NPops = { POP_SIZE_PACKAGE = 10000 }
NEconomy = { MIN_GOODS_PRICE = 1 }
`,
	"game/common/defines/jomini/00_tooltips.txt": "NTooltip = {\n\tOPEN_DELAYED_TIME = 0.3f;\n}\n",
	"game/common/goods/00_goods.txt": `
iron = { texture = "iron.dds" cost = 40 category = industrial }
coal = { texture = "coal.dds" cost = 30 category = industrial }
steel = { texture = "steel.dds" cost = 50 category = industrial traded_quantity = 4 }
electricity = { cost = 30 category = staple local = yes tradeable = no }
`,
	"game/common/building_groups/00_building_groups.txt": `
bg_manufacturing = { category = urban economy_of_scale = yes cash_reserves_max = 25000 }
bg_heavy_industry = { parent_group = bg_manufacturing lens = heavy_industry urbanization = 20 }
`,
	"game/common/buildings/01_industry.txt": `
building_steel_mill = {
	building_group = bg_heavy_industry
	unlocking_technologies = { steelworking }
	production_method_groups = { pmg_steelmaking }
	required_construction = construction_cost_high
	city_type = city
}
`,
	"game/common/production_method_groups/01_industry.txt": `
pmg_steelmaking = { production_methods = { pm_blister pm_bessemer } }
`,
	"game/common/production_methods/01_industry.txt": `
pm_blister = {
	texture = "blister.dds"
	building_modifiers = {
		workforce_scaled = { goods_input_iron_add = 40 goods_input_coal_add = 30 goods_output_steel_add = 65 }
		level_scaled = { building_employment_laborers_add = 4500 }
	}
}
pm_bessemer = {
	unlocking_technologies = { bessemer_process }
	building_modifiers = {
		workforce_scaled = { goods_input_iron_add = 60 goods_input_coal_add = 30 goods_output_steel_add = 90 }
		level_scaled = { building_employment_laborers_add = 3000 building_employment_machinists_add = 1000 }
	}
}
`,
	"game/common/pop_types/laborers.txt":   "laborers = { texture = \"l.dds\" color = hsv{ 0.02 0.38 0.37 } wage_weight = 1 }\n",
	"game/common/pop_types/machinists.txt": "machinists = { wage_weight = 2 paid_private_wage = yes }\n",
	"game/common/pop_needs/00_pop_needs.txt": `
popneed_heating = {
	default = coal
	entry = { goods = coal weight = 1 max_supply_share = 0.5 min_supply_share = 0 }
	entry = { goods = electricity weight = 2 max_supply_share = 1 min_supply_share = 0 }
}
`,
	"game/common/buy_packages/00_buy_packages.txt": "wealth_1 = { political_strength = 0.03 goods = { popneed_heating = 15 } }\n",
	"game/common/company_types/00_companies.txt": `
company_steel = {
	category = industrialist
	building_types = { building_steel_mill }
	prosperity_modifier = { building_steel_mill_throughput_add = 0.1 }
	possible = { any_scope_building = { level >= 5 } }
}
`,
	"game/common/technology/eras/00_eras.txt": "era_1 = { #Pre-1836\n\ttechnology_cost = 7500\n}\nera_2 = { #1836-1861\n\ttechnology_cost = 10000\n}\n",
	"game/common/technology/technologies/10_production.txt": `
steelworking = { era = era_1 category = production }
bessemer_process = {
	era = era_2
	category = production
	unlocking_technologies = { steelworking }
	modifier = { building_steel_mill_throughput_add = 0.1 }
}
`,
}

func loadTree(t *testing.T, files map[string]string, sources ...string) *Data {
	t.Helper()

	root := t.TempDir()
	writeTree(t, root, files)

	var list []folders.Source
	for _, source := range sources {
		list = append(list, folders.Source{Name: filepath.Base(source), Path: filepath.Join(root, filepath.FromSlash(source))})
	}

	return Load(folders.Open(list))
}

// merge returns a copy of base with extra added.
func merge(base, extra map[string]string) map[string]string {
	merged := map[string]string{}

	for name, content := range base {
		merged[name] = content
	}

	for name, content := range extra {
		merged[name] = content
	}

	return merged
}

func findDiagnostic(data *Data, severity report.Severity, fragment string) (report.Diagnostic, bool) {
	for _, diagnostic := range data.Diagnostics {
		if diagnostic.Severity == severity && strings.Contains(diagnostic.String(), fragment) {
			return diagnostic, true
		}
	}

	return report.Diagnostic{}, false
}

func expectDiagnostic(t *testing.T, data *Data, severity report.Severity, fragment string) report.Diagnostic {
	t.Helper()

	diagnostic, ok := findDiagnostic(data, severity, fragment)
	if !ok {
		t.Errorf("no %s mentioning %q; got:\n%s", severity, fragment, dump(data.Diagnostics))
	}

	return diagnostic
}

func dump(diagnostics report.Diagnostics) string {
	lines := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		lines = append(lines, "  "+diagnostic.String())
	}

	return strings.Join(lines, "\n")
}

func TestLoadVanilla(t *testing.T) {
	data := loadTree(t, vanilla, "game")

	if len(data.Diagnostics) > 0 {
		t.Fatalf("a clean game should load without diagnostics:\n%s", dump(data.Diagnostics))
	}

	steel, _ := data.Goods.Get("steel")
	if steel.Cost != 50 || steel.TradedQuantity != 4 || !steel.Tradeable || steel.Local {
		t.Errorf("steel = %+v", steel)
	}

	iron, _ := data.Goods.Get("iron")
	if iron.TradedQuantity != 10 {
		t.Errorf("iron traded quantity = %v, want the define's default of 10", iron.TradedQuantity)
	}

	electricity, _ := data.Goods.Get("electricity")
	if !electricity.Local || electricity.Tradeable {
		t.Errorf("electricity = %+v", electricity)
	}

	if keys := strings.Join(data.Goods.Keys(), ","); keys != "iron,coal,steel,electricity" {
		t.Errorf("goods keep file order, got %s", keys)
	}

	mill, _ := data.Buildings.Get("building_steel_mill")
	if mill.RequiredConstruction.Name != "construction_cost_high" || !mill.Buildable || mill.CityType != "city" {
		t.Errorf("steel mill = %+v", mill)
	}

	if origin := mill.Origin(); origin.File != "common/buildings/01_industry.txt" || origin.Line != 2 || origin.Source != "game" {
		t.Errorf("steel mill origin = %+v", origin)
	}

	bessemer, _ := data.ProductionMethods.Get("pm_bessemer")
	if inputs := bessemer.Inputs(); len(inputs) != 2 || inputs[0] != (Flow{Key: "iron", Amount: 60, Scaling: WorkforceScaled}) {
		t.Errorf("bessemer inputs = %+v", inputs)
	}

	if outputs := bessemer.Outputs(); len(outputs) != 1 || outputs[0].Amount != 90 {
		t.Errorf("bessemer outputs = %+v", outputs)
	}

	if jobs := bessemer.Employment(); len(jobs) != 2 || jobs[1] != (Flow{Key: "machinists", Amount: 1000, Scaling: LevelScaled}) {
		t.Errorf("bessemer jobs = %+v", jobs)
	}

	laborers, _ := data.PopTypes.Get("laborers")
	if laborers.WageWeight != 1 || laborers.Color.Tag != "hsv" || laborers.ConsumptionMult != 1 {
		t.Errorf("laborers = %+v", laborers)
	}

	heating, _ := data.PopNeeds.Get("popneed_heating")
	if len(heating.Entries) != 2 || heating.Entries[1].Goods != "electricity" || heating.Entries[1].Weight.Value != 2 {
		t.Errorf("heating = %+v", heating)
	}

	pack, _ := data.BuyPackages.Get("wealth_1")
	if pack.Wealth != 1 || len(pack.Needs) != 1 || pack.Needs[0] != (database.Amount{Key: "popneed_heating", Amount: 15}) {
		t.Errorf("wealth_1 = %+v", pack)
	}

	company, _ := data.Companies.Get("company_steel")
	if len(company.ProsperityModifier) != 1 || company.ProsperityModifier[0].Value != 0.1 {
		t.Errorf("company = %+v", company)
	}

	if _, ok := company.Field("possible"); !ok {
		t.Error("triggers stay readable through the raw script")
	}

	technology, _ := data.Technologies.Get("bessemer_process")
	if technology.Era != "era_2" || !technology.CanResearch || len(technology.Modifiers) != 1 {
		t.Errorf("bessemer_process = %+v", technology)
	}

	era, _ := data.Eras.Get("era_2")
	if era.Index != 1 || era.TechnologyCost != 10000 || era.Label != "1836-1861" {
		t.Errorf("era_2 = %+v", era)
	}

	if value, _ := data.Defines.Number("NEconomy", "PRICE_RANGE"); value != 0.75 {
		t.Errorf("PRICE_RANGE = %v", value)
	}

	if value, _ := data.Defines.Number("NEconomy", "MIN_GOODS_PRICE"); value != 1 {
		t.Error("a namespace opened twice should keep the defines of both")
	}

	if value, _ := data.Defines.Number("NTooltip", "OPEN_DELAYED_TIME"); value != 0.3 {
		t.Error("defines in subfolders should be read")
	}
}

func TestFileLayering(t *testing.T) {
	data := loadTree(t, merge(vanilla, map[string]string{
		// Same name as the game's file: replaces it whole.
		"mod/common/goods/00_goods.txt": `
iron = { cost = 45 category = industrial }
coal = { cost = 30 category = industrial }
steel = { cost = 50 category = industrial }
electricity = { cost = 30 category = staple }
`,
		// Sorts before the game's file, so the game's definition, read later,
		// overrides it.
		"mod/common/pop_types/aaa_laborers.txt": "laborers = { wage_weight = 9 }\n",
	}), "game", "mod")

	iron, _ := data.Goods.Get("iron")
	if iron.Cost != 45 || iron.Origin().Source != "mod" {
		t.Errorf("iron = %+v, want the mod's file to replace the game's", iron)
	}

	expectDiagnostic(t, data, report.SeverityInfo, "replaces the file of the same name from game")

	laborers, _ := data.PopTypes.Get("laborers")
	if laborers.WageWeight != 1 {
		t.Errorf("laborers wage weight = %v; files are read in name order, so the game's laborers.txt comes last and wins", laborers.WageWeight)
	}

	expectDiagnostic(t, data, report.SeverityInfo, "overrides the definition in common/pop_types/aaa_laborers.txt")
}

func TestReplacePaths(t *testing.T) {
	data := loadTree(t, merge(vanilla, map[string]string{
		"mod/.metadata/metadata.json":                `{ "name": "mod", "game_custom_data": { "replace_paths": [ "common/company_types" ] } }`,
		"mod/common/company_types/mod_companies.txt": "company_mod = { building_types = { building_steel_mill } }\n",
	}), "game", "mod")

	if data.Companies.Has("company_steel") {
		t.Error("replace_paths should drop the game's companies")
	}

	if !data.Companies.Has("company_mod") {
		t.Error("the mod's own companies should be read")
	}

	if !data.Goods.Has("iron") {
		t.Error("replace_paths should leave other folders alone")
	}

	expectDiagnostic(t, data, report.SeverityInfo, "replaces every file")
}

func TestModificationModes(t *testing.T) {
	data := loadTree(t, merge(vanilla, map[string]string{
		"mod/common/production_method_groups/zz_mod.txt": `
INJECT:pmg_steelmaking = { production_methods = { pm_mod_steel pm_bessemer } }
`,
		"mod/common/production_methods/zz_mod.txt": `
pm_mod_steel = {
	building_modifiers = { workforce_scaled = { goods_output_steel_add = 200 } }
}
REPLACE:pm_blister = {
	building_modifiers = { workforce_scaled = { goods_output_steel_add = 70 } }
}
TRY_REPLACE:pm_nothing = { }
TRY_INJECT:pm_nothing = { }
REPLACE_OR_CREATE:pm_created = { texture = "a.dds" }
INJECT_OR_CREATE:pm_bessemer = { texture = "new.dds" }
INJECT:pm_missing = { texture = "x.dds" }
REPLACE:pm_also_missing = { }
FUTURE_MODE:pm_future = { }
`,
		"mod/common/goods/zz_mod.txt": "INJECT:steel = { cost = 55 }\n",
	}), "game", "mod")

	group, _ := data.ProductionMethodGroups.Get("pmg_steelmaking")
	if got := strings.Join(group.ProductionMethods, ","); got != "pm_blister,pm_bessemer,pm_mod_steel" {
		t.Errorf("injected group = %s, want the game's methods followed by the new one, without duplicates", got)
	}

	if len(group.Origins) != 2 || group.Origins[1].Mode != database.ModeInject || group.Origins[1].Source != "mod" {
		t.Errorf("group origins = %+v", group.Origins)
	}

	blister, _ := data.ProductionMethods.Get("pm_blister")
	if outputs := blister.Outputs(); len(outputs) != 1 || outputs[0].Amount != 70 || len(blister.Inputs()) != 0 {
		t.Errorf("replaced blister = %+v", blister.BuildingModifiers)
	}

	if len(blister.Overridden) != 1 || blister.Overridden[0].Source != "game" {
		t.Errorf("replaced blister should remember what it replaced: %+v", blister.Overridden)
	}

	if blister.Texture != "" {
		t.Error("REPLACE should drop the fields of the old definition")
	}

	bessemer, _ := data.ProductionMethods.Get("pm_bessemer")
	if bessemer.Texture != "new.dds" || len(bessemer.Inputs()) != 2 {
		t.Errorf("INJECT_OR_CREATE into an existing method should add to it: %+v", bessemer)
	}

	steel, _ := data.Goods.Get("steel")
	if steel.Cost != 55 || steel.TradedQuantity != 4 {
		t.Errorf("injected steel = %+v, want the injected cost and the original traded quantity", steel)
	}

	for _, key := range []string{"pm_created", "pm_future"} {
		if !data.ProductionMethods.Has(key) {
			t.Errorf("%s should exist", key)
		}
	}

	for _, key := range []string{"pm_nothing", "pm_missing", "pm_also_missing"} {
		if data.ProductionMethods.Has(key) {
			t.Errorf("%s should not exist", key)
		}
	}

	expectDiagnostic(t, data, report.SeverityInfo, "TRY_REPLACE: skipped")
	expectDiagnostic(t, data, report.SeverityInfo, "TRY_INJECT: skipped")
	expectDiagnostic(t, data, report.SeverityError, "INJECT: has nothing to modify")
	expectDiagnostic(t, data, report.SeverityError, "REPLACE: has nothing to modify")
	expectDiagnostic(t, data, report.SeverityWarning, "unknown modification FUTURE_MODE:")
}

func TestDecodingIsTolerant(t *testing.T) {
	data := loadTree(t, merge(vanilla, map[string]string{
		"mod/common/goods/zz_mod.txt": `
silver = {
	cost = "expensive"
	category = precious
	local = maybe
	shiny = yes
	stray_value
}
bronze = { category = industrial }
not_a_block = 5
`,
	}), "game", "mod")

	silver, ok := data.Goods.Get("silver")
	if !ok {
		t.Fatal("a definition with bad fields should still load")
	}

	if silver.Cost != 0 || silver.Local || !silver.Tradeable {
		t.Errorf("silver = %+v, want defaults where the values were wrong", silver)
	}

	expectDiagnostic(t, data, report.SeverityWarning, "cost should be a number")
	expectDiagnostic(t, data, report.SeverityWarning, "local should be yes or no")
	expectDiagnostic(t, data, report.SeverityInfo, "category precious is not one of")
	expectDiagnostic(t, data, report.SeverityWarning, "ignored stray_value, which has no key")

	unknown := expectDiagnostic(t, data, report.SeverityInfo, "unknown field shiny")
	if unknown.Line != 6 || !strings.HasSuffix(filepath.ToSlash(unknown.Path), "mod/common/goods/zz_mod.txt") {
		t.Errorf("unknown field reported at %s:%d", unknown.Path, unknown.Line)
	}

	if shiny, ok := silver.Field("shiny"); !ok || shiny.Text != "yes" {
		t.Error("an unknown field should stay readable through the raw script")
	}

	expectDiagnostic(t, data, report.SeverityWarning, "good bronze: has no cost")
	expectDiagnostic(t, data, report.SeverityWarning, "good not_a_block: ignored, because its value 5 is not a block")
}

func TestBrokenFileKeepsTheRest(t *testing.T) {
	data := loadTree(t, merge(vanilla, map[string]string{
		"mod/common/goods/zz_mod.txt": `
tin = { cost = 20 } }
lead = { cost = 25 category = industrial
`,
	}), "game", "mod")

	if tin, ok := data.Goods.Get("tin"); !ok || tin.Cost != 20 {
		t.Errorf("tin = %+v", tin)
	}

	if lead, ok := data.Goods.Get("lead"); !ok || lead.Cost != 25 {
		t.Errorf("lead = %+v", lead)
	}

	expectDiagnostic(t, data, report.SeverityWarning, "closes nothing")
	expectDiagnostic(t, data, report.SeverityWarning, "never closed")
}

func TestModifierScalings(t *testing.T) {
	data := loadTree(t, merge(vanilla, map[string]string{
		"mod/common/production_methods/zz_mod.txt": `
pm_monument = {
	country_modifiers = { throughput_scaled = { country_prestige_add = 25 } }
	state_modifiers = { future_scaled = { state_x_add = 1 } }
	building_modifiers = {
		goods_output_steel_add = 5
		workforce_scaled = { goods_input_iron_add = 10 goods_input_iron_add = 5 }
		level_scaled = { building_employment_laborers_add = 100 building_allowed = yes building_law = law_key }
	}
}
`,
	}), "game", "mod")

	method, _ := data.ProductionMethods.Get("pm_monument")

	if len(method.CountryModifiers.ThroughputScaled) != 1 || method.CountryModifiers.ThroughputScaled[0].Value != 25 {
		t.Errorf("throughput scaled = %+v", method.CountryModifiers)
	}

	if other := method.StateModifiers.Other["future_scaled"]; len(other) != 1 || other[0].Key != "state_x_add" {
		t.Errorf("an unknown scaling should be kept under its name: %+v", method.StateModifiers)
	}

	if all := method.StateModifiers.All(); len(all) != 1 || all[0].Scaling != OtherScaled {
		t.Errorf("All should report an unknown scaling as other: %+v", all)
	}

	expectDiagnostic(t, data, report.SeverityInfo, "unknown scaling future_scaled")
	expectDiagnostic(t, data, report.SeverityWarning, "modifier goods_output_steel_add should be inside a scaling block")

	if inputs := method.Inputs(); len(inputs) != 1 || inputs[0].Amount != 15 {
		t.Errorf("repeated modifiers should be summed, inputs = %+v", inputs)
	}

	if outputs := method.Outputs(); len(outputs) != 1 || outputs[0].Scaling != Unscaled {
		t.Errorf("a stray modifier should be read as unscaled, outputs = %+v", outputs)
	}

	level := method.BuildingModifiers.LevelScaled
	if len(level) != 3 || level[1].Value != 1 || level[2].Text != "law_key" {
		t.Errorf("level scaled = %+v", level)
	}
}

func TestReferencesAreChecked(t *testing.T) {
	data := loadTree(t, merge(vanilla, map[string]string{
		"mod/common/buildings/zz_mod.txt": `
building_mod = {
	building_group = bg_missing
	production_method_groups = { pmg_steelmaking pmg_missing }
}
`,
		"mod/common/production_method_groups/zz_mod.txt": `
INJECT:pmg_steelmaking = { production_methods = { pm_missing } }
`,
		"mod/common/production_methods/zz_mod.txt": `
pm_mod = {
	unlocking_technologies = { no_such_tech }
	building_modifiers = {
		workforce_scaled = { goods_input_unobtainium_add = 5 }
		level_scaled = { building_employment_wizards_add = 10 }
	}
}
`,
	}), "game", "mod")

	expectDiagnostic(t, data, report.SeverityWarning, "building group bg_missing does not exist")
	expectDiagnostic(t, data, report.SeverityWarning, "production method group pmg_missing does not exist")
	expectDiagnostic(t, data, report.SeverityWarning, "technology no_such_tech does not exist")
	expectDiagnostic(t, data, report.SeverityWarning, "goods_input_unobtainium_add")
	expectDiagnostic(t, data, report.SeverityWarning, "building_employment_wizards_add")

	// The dangling method was injected by the mod, so the problem belongs in
	// the mod's file even though the group was defined by the game.
	injected := expectDiagnostic(t, data, report.SeverityWarning, "production method pm_missing does not exist")
	if !strings.HasSuffix(filepath.ToSlash(injected.Path), "mod/common/production_method_groups/zz_mod.txt") {
		t.Errorf("reported in %s, want the mod's file", injected.Path)
	}

	// References that do not resolve are kept, not dropped.
	building, _ := data.Buildings.Get("building_mod")
	if len(building.ProductionMethodGroups) != 2 || building.BuildingGroup != "bg_missing" {
		t.Errorf("building_mod = %+v", building)
	}
}

func TestBuildingGroupInheritance(t *testing.T) {
	data := loadTree(t, merge(vanilla, map[string]string{
		"mod/common/building_groups/zz_mod.txt": `
bg_light = { parent_group = bg_manufacturing economy_of_scale = no }
bg_loop_a = { parent_group = bg_loop_b }
bg_loop_b = { parent_group = bg_loop_a }
`,
	}), "game", "mod")

	if value, ok := ResolveGroup(data, "bg_heavy_industry", func(g *BuildingGroup) database.Opt[bool] { return g.EconomyOfScale }); !ok || !value {
		t.Error("heavy industry should inherit economy of scale from manufacturing")
	}

	if value, ok := ResolveGroup(data, "bg_light", func(g *BuildingGroup) database.Opt[bool] { return g.EconomyOfScale }); !ok || value {
		t.Error("a group's own setting should win over its parent's")
	}

	if value, ok := ResolveGroup(data, "bg_heavy_industry", func(g *BuildingGroup) database.Opt[float64] { return g.CashReservesMax }); !ok || value != 25000 {
		t.Errorf("cash reserves = %v", value)
	}

	if value, ok := ResolveGroup(data, "bg_heavy_industry", func(g *BuildingGroup) database.Opt[string] { return database.Text(g.Lens) }); !ok || value != "heavy_industry" {
		t.Errorf("lens = %q", value)
	}

	if chain := data.GroupChain("bg_loop_a"); len(chain) != 2 {
		t.Errorf("a loop should be walked once, chain = %d groups", len(chain))
	}

	expectDiagnostic(t, data, report.SeverityWarning, "lead back to itself")
}

func TestTechnologyCycles(t *testing.T) {
	data := loadTree(t, merge(vanilla, map[string]string{
		"mod/common/technology/technologies/zz_mod.txt": `
tech_a = { era = era_1 unlocking_technologies = { tech_b } }
tech_b = { era = era_1 unlocking_technologies = { tech_a } }
tech_c = { era = era_9 }
`,
	}), "game", "mod")

	expectDiagnostic(t, data, report.SeverityWarning, "requires itself")
	expectDiagnostic(t, data, report.SeverityWarning, "era era_9 does not exist")
}
