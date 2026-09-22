package victoria3

import (
	"os"
	"sort"
	"strings"
	"testing"

	"path/filepath"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
)

// installedSources returns the game and mods named by PDX_GAME_DIR and
// PDX_MOD_DIRS, or skips the test when there is no game to read.
//
// PDX_GAME_DIR is the game folder, the one holding common/, or its parent.
// PDX_MOD_DIRS optionally lists mod folders in load order, separated by the
// system's path list separator (; on Windows).
func installedSources(t *testing.T) []folders.Source {
	t.Helper()

	game := os.Getenv("PDX_GAME_DIR")
	if game == "" {
		t.Skip("set PDX_GAME_DIR to the game folder to run this test")
	}

	sources := []folders.Source{{Name: "Victoria 3", Path: game}}

	for _, mod := range filepath.SplitList(os.Getenv("PDX_MOD_DIRS")) {
		if mod != "" {
			sources = append(sources, folders.Source{Path: mod})
		}
	}

	return sources
}

// TestLoadInstalledGame loads a real installation and checks it against what
// the game is known to contain. Vanilla on its own must load without a single
// warning: every warning there is a gap in the loader, not in the game.
func TestLoadInstalledGame(t *testing.T) {
	sources := installedSources(t)
	data := Load(folders.Open(sources))

	logDiagnostics(t, data)

	t.Logf("sources: %v", data.Sources)
	t.Logf("goods %d, building groups %d, buildings %d, PM groups %d, PMs %d, pop types %d, pop needs %d, buy packages %d, companies %d, technologies %d, eras %d, defines %d",
		data.Goods.Len(), data.BuildingGroups.Len(), data.Buildings.Len(), data.ProductionMethodGroups.Len(),
		data.ProductionMethods.Len(), data.PopTypes.Len(), data.PopNeeds.Len(), data.BuyPackages.Len(),
		data.Companies.Len(), data.Technologies.Len(), data.Eras.Len(), data.Defines.Len())

	if len(sources) == 1 {
		for _, diagnostic := range data.Diagnostics.Filter(report.SeverityWarning) {
			t.Errorf("vanilla should load cleanly: %s", diagnostic)
		}
	}

	// Spot checks against values read by hand from the game's files.
	good, ok := data.Goods.Get("steel")
	if !ok || good.Cost != 50 || good.Category != "industrial" || !good.Tradeable {
		t.Errorf("steel = %+v", good)
	}

	if electricity, _ := data.Goods.Get("electricity"); electricity == nil || !electricity.Local {
		t.Error("electricity should be a local good")
	}

	method, ok := data.ProductionMethods.Get("pm_open_hearth_process")
	if !ok {
		t.Fatal("pm_open_hearth_process is missing")
	}

	want := map[string]float64{"iron": 90, "coal": 30}
	for _, input := range method.Inputs() {
		if want[input.Key] != input.Amount || input.Scaling != WorkforceScaled {
			t.Errorf("open hearth input %+v", input)
		}
	}

	if outputs := method.Outputs(); len(outputs) != 1 || outputs[0].Key != "steel" || outputs[0].Amount != 120 {
		t.Errorf("open hearth outputs = %+v", outputs)
	}

	if len(method.Employment()) != 4 {
		t.Errorf("open hearth employment = %+v", method.Employment())
	}

	if strings.Join(method.UnlockingTechnologies, ",") != "open_hearth_process" {
		t.Errorf("open hearth techs = %v", method.UnlockingTechnologies)
	}

	if era, ok := data.Eras.Get("era_2"); !ok || era.TechnologyCost != 10000 || era.Index != 1 || era.Label != "1836-1861" {
		t.Errorf("era_2 = %+v", era)
	}

	if technology, ok := data.Technologies.Get("shaft_mining"); !ok || technology.Era != "era_1" ||
		strings.Join(technology.UnlockingTechnologies, ",") != "enclosure,manufacturies" {
		t.Errorf("shaft_mining = %+v", technology)
	}

	if pack, ok := data.BuyPackages.Get("wealth_1"); !ok || pack.Wealth != 1 || len(pack.Needs) != 4 {
		t.Errorf("wealth_1 = %+v", pack)
	}

	if value, ok := data.Defines.Number("NEconomy", "PRICE_RANGE"); !ok || value != 0.75 {
		t.Errorf("NEconomy.PRICE_RANGE = %v, %v", value, ok)
	}

	if eos, ok := ResolveGroup(data, "bg_light_industry", func(g *BuildingGroup) database.Opt[bool] { return g.EconomyOfScale }); !ok || !eos {
		t.Error("light industry should inherit economy of scale from manufacturing")
	}
}

func logDiagnostics(t *testing.T, data *Data) {
	t.Helper()

	counts := map[string]int{}

	for _, diagnostic := range data.Diagnostics {
		// Group by severity and message shape, so a thousand unknown fields
		// read as one line each per field rather than a thousand lines.
		message := diagnostic.Message
		if cut := strings.Index(message, ","); cut > 0 && diagnostic.Severity == report.SeverityInfo {
			message = message[:cut]
		}

		counts[diagnostic.Severity.String()+": "+message]++
	}

	lines := make([]string, 0, len(counts))
	for line, count := range counts {
		lines = append(lines, line+" ("+itoa(count)+")")
	}

	sort.Strings(lines)

	for _, line := range lines {
		t.Log(line)
	}
}

func itoa(value int) string {
	var digits []byte

	if value == 0 {
		return "0"
	}

	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}

	return string(digits)
}
