package asset

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
)

// installedSources returns the game and mods named by PDX_GAME_DIR and
// PDX_MOD_DIRS, or skips the test when there is no game to read.
//
// PDX_GAME_DIR is the game folder, the one holding gfx/ or the layers that
// do, or its parent. PDX_MOD_DIRS optionally lists mod folders in load order,
// separated by the system's path list separator (; on Windows).
func installedSources(t *testing.T) []folders.Source {
	t.Helper()

	game := os.Getenv("PDX_GAME_DIR")
	if game == "" {
		t.Skip("set PDX_GAME_DIR to the game folder to run this test")
	}

	sources := []folders.Source{{Name: "game", Path: game}}

	for _, mod := range filepath.SplitList(os.Getenv("PDX_MOD_DIRS")) {
		if mod != "" {
			sources = append(sources, folders.Source{Path: mod})
		}
	}

	return sources
}

// shippedProblems are the mistakes in the games' own asset files that the
// package rightly reports, by the file and a fragment of the diagnostic. The
// syntax errors of the files, such as a block that is never closed, are
// expected anywhere and are only logged.
var shippedProblems = []struct{ file, fragment string }{
	// Europa Universalis 5 attaches an entity whose definition is commented
	// out.
	{"unit_skeleton.asset", "entity unit_skeleton_entity does not exist"},

	// A level of detail block of Europa Universalis 5 is missing its closing
	// brace, which puts the mesh settings after it inside it.
	{"unit_artillery_absolutism_mortar.asset", "unknown field meshsettings"},
}

// shippedMissingFiles are the files the games' own definitions name that are
// not on disk.
var shippedMissingFiles = []string{
	// Europa Universalis 5's fallback for a mesh that is not found. The engine
	// apparently provides it itself.
	"mesh_not_found_fallback_object.mesh",
}

func shipped(diagnostic report.Diagnostic) bool {
	for _, known := range shippedProblems {
		if filepath.Base(diagnostic.Path) == known.file && strings.Contains(diagnostic.String(), known.fragment) {
			return true
		}
	}

	return false
}

// TestLoadInstalledGame loads a real installation. The game on its own must
// load without a warning beyond the syntax errors of its files and the
// mistakes listed in shippedProblems, and without a field this package does
// not know: every other one is a gap in the package, not in the game.
func TestLoadInstalledGame(t *testing.T) {
	sources := installedSources(t)
	set := folders.Open(sources)
	assets := Load(set)

	t.Logf("sources: %v", assets.Sources)
	t.Logf("meshes %d, entities %d, diagnostics %d", assets.Meshes.Len(), assets.Entities.Len(), len(assets.Diagnostics))

	if assets.Meshes.Len() == 0 || assets.Entities.Len() == 0 {
		t.Fatal("no meshes or entities found")
	}

	vanilla := len(sources) == 1

	for _, diagnostic := range assets.Diagnostics {
		unknown := strings.Contains(diagnostic.Message, "unknown field")
		syntax := diagnostic.Subject == "" && diagnostic.Severity == report.SeverityWarning

		switch {
		case !vanilla || shipped(diagnostic):
			if diagnostic.Severity >= report.SeverityWarning || unknown {
				t.Log(diagnostic)
			}

		case syntax:
			t.Logf("a mistake in the game's files: %s", diagnostic)

		case diagnostic.Severity >= report.SeverityWarning || unknown:
			t.Errorf("the game should load cleanly: %s", diagnostic)
		}
	}

	checkInstalledFiles(t, set, assets)
	spotCheckVictoria3(t, assets)
	spotCheckEuropaUniversalis5(t, assets)
}

// checkInstalledFiles resolves every mesh, animation and blend shape file the
// definitions name and checks that one of the folders has it, which is what
// shows Resolve resolves the way the engine does.
//
// Textures are left out: the engine finds many of them somewhere other than
// next to their asset file, in a way this package does not know yet.
func checkInstalledFiles(t *testing.T, set *folders.Set, assets *Assets) {
	t.Helper()

	exists := func(relative string) bool {
		for _, root := range set.Roots() {
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); err == nil {
				return true
			}
		}

		return false
	}

	checked := 0

	check := func(mesh *Mesh, reference string) {
		if reference == "" || slices.Contains(shippedMissingFiles, reference) {
			return
		}

		checked++

		if resolved := Resolve(mesh.Origin(), reference); !exists(resolved) {
			t.Errorf("pdxmesh %s in %s names %s, which resolves to %s, where none of the folders has a file",
				mesh.Key, mesh.Origin().File, reference, resolved)
		}
	}

	for _, mesh := range assets.Meshes.All() {
		check(mesh, mesh.File)

		for _, animation := range slices.Concat(mesh.Animations, mesh.AdditiveAnimations) {
			check(mesh, animation.File)
		}

		for _, shape := range mesh.BlendShapes {
			check(mesh, shape.File)
		}
	}

	t.Logf("%d files the definitions name are on disk", checked)
}

// spotCheckVictoria3 compares a few definitions of Victoria 3 against values
// read by hand from its files. It does nothing for another game.
func spotCheckVictoria3(t *testing.T, assets *Assets) {
	t.Helper()

	body, ok := assets.Meshes.Get("female_body_mesh")
	if !ok || body.Origin().File != "gfx/models/portraits/female_body/female_body.asset" {
		return
	}

	if body.File != "female_body.mesh" || len(body.BlendShapes) != 12 || len(body.AdditiveAnimations) != 2 ||
		body.BlendShapes[0] != (BlendShape{ID: "female_body_bs_fat", File: "blendshapes/female_body_bs_fat.mesh"}) ||
		body.AdditiveAnimations[0] != (Animation{ID: "body_infant_proportions", File: "female_body_infant_proportions.anim"}) {
		t.Errorf("female_body_mesh = %+v", body)
	}

	entity, ok := assets.Entities.Get("female_body_entity")
	if !ok || entity.Mesh != "female_body_mesh" {
		t.Fatalf("female_body_entity = %+v", entity)
	}

	if data, _ := entity.GameData.Get("portrait_entity_user_data"); !data.IsBlock() {
		t.Errorf("female_body_entity game data = %+v", entity.GameData)
	}

	rider, ok := assets.Entities.Get("military_cavalry_european_hussar_rider_entity")
	want := Attachment{Locator: "r_hand_attach", Entity: "military_arms_saber_entity", ID: "weapon"}

	if !ok || len(rider.Attachments) == 0 || rider.Attachments[0] != want {
		t.Errorf("hussar rider attachments = %+v, want %+v first", rider, want)
	}

	general, ok := assets.Meshes.Get("military_infantry_carlists_general_mesh")
	if !ok || len(general.LODs) == 0 || !math.IsInf(general.LODs[0].Percent, 1) {
		t.Errorf("carlist general levels of detail = %+v, want inf read as infinity", general)
	}
}

// spotCheckEuropaUniversalis5 compares a few definitions of Europa Universalis
// 5 against values read by hand from its files. It does nothing for another
// game.
func spotCheckEuropaUniversalis5(t *testing.T, assets *Assets) {
	t.Helper()

	head, ok := assets.Meshes.Get("female_head_mesh")
	if !ok || head.Origin().File != "in_game/gfx/models/portraits/female_head/female_head.asset" {
		return
	}

	found := false

	for _, shape := range head.BlendShapes {
		if shape.ID == "nbs_chin_length_max" {
			found = shape.Data == "normal|tangent" && shape.File == "additive_animations/nbs_chin_length_max.mesh"
		}
	}

	if !found {
		t.Errorf("female_head_mesh blend shape nbs_chin_length_max is missing or wrong: %+v", head.BlendShapes)
	}

	horse, ok := assets.Entities.Get("horse_skeleton_entity")
	if !ok || len(horse.States) == 0 || len(horse.Locators) == 0 {
		t.Fatalf("horse_skeleton_entity = %+v", horse)
	}

	if horse.Locators[0].Name != "rider_locator" || horse.Locators[0].ParentJoint != "spine_01_J" {
		t.Errorf("horse locator = %+v", horse.Locators[0])
	}

	if idle := horse.States[0]; idle.Name != "idle" || !slices.Equal(idle.Propagation, []Propagation{{State: "idle_horse_riding"}}) {
		t.Errorf("horse idle state = %+v, want its state set on every attachment", idle)
	}

	trireme, ok := assets.Entities.Get("celtic_gfx_trireme")
	if !ok {
		t.Fatal("celtic_gfx_trireme is missing")
	}

	weighed := false

	for _, state := range trireme.States {
		if state.Name == "move" && slices.Contains(state.ChanceFromState, database.Amount{Key: "idle", Amount: 100}) {
			weighed = true
		}
	}

	if !weighed {
		t.Errorf("celtic_gfx_trireme states = %+v, want a move state weighed by the idle state", trireme.States)
	}
}
