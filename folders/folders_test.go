package folders

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kaiser-chris/pdx-parser-go/report"
)

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

// listing writes files as "source:relative", in order.
func listing(files []File) []string {
	lines := make([]string, 0, len(files))
	for _, file := range files {
		lines = append(lines, file.Source+":"+file.Relative)
	}

	return lines
}

func mentions(diagnostics report.Diagnostics, fragment string) bool {
	for _, diagnostic := range diagnostics {
		if strings.Contains(diagnostic.String(), fragment) {
			return true
		}
	}

	return false
}

func TestOpenAcceptsTheFoldersUsersPick(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"Victoria 3/game/common/goods/00_goods.txt":      "iron = { }",
		"translation/localization/german/a_l_german.yml": "l_german:",
	})

	for _, picked := range []string{"Victoria 3", "Victoria 3/game", "Victoria 3/game/common", "translation", "translation/localization"} {
		set := Open([]Source{{Name: "picked", Path: filepath.Join(root, filepath.FromSlash(picked))}})

		if len(set.Roots()) != 1 || len(set.Diagnostics) != 0 {
			t.Errorf("%s: roots %v, diagnostics %v", picked, set.Roots(), set.Diagnostics)
		}
	}

	set := Open([]Source{{Path: filepath.Join(root, "nowhere")}})

	if len(set.Names()) != 0 || !mentions(set.Diagnostics, "error: ") || !mentions(set.Diagnostics, "no game or mod files found") {
		t.Errorf("an unusable folder should be left out and reported: %v %v", set.Names(), set.Diagnostics)
	}
}

func TestNamesComeFromMetadata(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"game/common/goods/a.txt":          "",
		"new/.metadata/metadata.json":      `{ "name": "New Style" }`,
		"new/common/goods/b.txt":           "",
		"old/descriptor.mod":               "name = \"Old Style\"\nversion = \"1.0\"\n",
		"old/common/goods/c.txt":           "",
		"broken/.metadata/metadata.json":   "{ not json",
		"broken/common/goods/d.txt":        "",
		"explicit/.metadata/metadata.json": `{ "name": "Ignored" }`,
		"explicit/common/goods/e.txt":      "",
	})

	set := Open([]Source{
		{Path: filepath.Join(root, "game")},
		{Path: filepath.Join(root, "new")},
		{Path: filepath.Join(root, "old")},
		{Path: filepath.Join(root, "broken")},
		{Name: "Given", Path: filepath.Join(root, "explicit")},
	})

	want := []string{"game", "New Style", "Old Style", "broken", "Given"}
	if !slices.Equal(set.Names(), want) {
		t.Errorf("names = %v, want %v", set.Names(), want)
	}
}

func TestFilesAreLayered(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"game/common/goods/00_goods.txt":   "",
		"game/common/goods/10_more.txt":    "",
		"game/common/goods/notes.md":       "",
		"game/common/goods/sub/deeper.txt": "",
		"mod/common/goods/00_goods.txt":    "",
		"mod/common/goods/05_mod.TXT":      "",
		"mod/common/goods/zz_last.txt":     "",
		"later/common/goods/00_GOODS.txt":  "",
	})

	set := Open([]Source{
		{Name: "game", Path: filepath.Join(root, "game")},
		{Name: "mod", Path: filepath.Join(root, "mod")},
		{Name: "later", Path: filepath.Join(root, "later")},
	})

	files, diagnostics := set.Files(Folder{Path: "common/goods"})

	// The same name in any case is the same file, the last folder's copy is
	// read, and everything is read in order of name.
	want := []string{
		"later:common/goods/00_GOODS.txt",
		"mod:common/goods/05_mod.TXT",
		"game:common/goods/10_more.txt",
		"mod:common/goods/zz_last.txt",
	}

	if got := listing(files); !slices.Equal(got, want) {
		t.Errorf("files =\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}

	if files[0].SourceOrder != 2 || files[2].SourceOrder != 0 {
		t.Errorf("source orders = %d, %d", files[0].SourceOrder, files[2].SourceOrder)
	}

	if !mentions(diagnostics, "replaces the file of the same name from game") ||
		!mentions(diagnostics, "replaces the file of the same name from mod") {
		t.Errorf("overridden files should be noted: %v", diagnostics)
	}

	recursive, _ := set.Files(Folder{Path: "common/goods", Recursive: true})
	if !slices.Contains(listing(recursive), "game:common/goods/sub/deeper.txt") {
		t.Errorf("a recursive listing should include subfolders: %v", listing(recursive))
	}

	markdown, _ := set.Files(Folder{Path: "common/goods", Extension: ".md"})
	if got := listing(markdown); !slices.Equal(got, []string{"game:common/goods/notes.md"}) {
		t.Errorf("markdown = %v", got)
	}

	missing, diagnostics := set.Files(Folder{Path: "common/nothing"})
	if len(missing) != 0 || len(diagnostics) != 0 {
		t.Errorf("a folder no source has is simply empty: %v %v", missing, diagnostics)
	}
}

func TestReplacePaths(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"game/common/company_types/00_companies.txt":  "",
		"game/common/goods/00_goods.txt":              "",
		"game/localization/english/a_l_english.yml":   "",
		"game/localization/german/a_l_german.yml":     "",
		"game/localization/german/sub/b_l_german.yml": "",
		"json/.metadata/metadata.json":                `{ "game_custom_data": { "replace_paths": [ "common/company_types", "localization\\German\\" ] } }`,
		"json/common/company_types/mine.txt":          "",
		"json/localization/german/mine_l_german.yml":  "",
		"descriptor/descriptor.mod":                   "name = \"Descriptor\"\nreplace_path = \"common/goods\"\n",
		"descriptor/common/goods/mine.txt":            "",
	})

	set := Open([]Source{
		{Name: "game", Path: filepath.Join(root, "game")},
		{Name: "json", Path: filepath.Join(root, "json")},
		{Path: filepath.Join(root, "descriptor")},
	})

	companies, diagnostics := set.Files(Folder{Path: "common/company_types"})
	if got := listing(companies); !slices.Equal(got, []string{"json:common/company_types/mine.txt"}) {
		t.Errorf("companies = %v", got)
	}

	if !mentions(diagnostics, "replaces every file of the folders before it in common/company_types") {
		t.Errorf("the replace path should be noted: %v", diagnostics)
	}

	goods, _ := set.Files(Folder{Path: "common/goods"})
	if got := listing(goods); !slices.Equal(got, []string{"Descriptor:common/goods/mine.txt"}) {
		t.Errorf("a replace_path in descriptor.mod should apply: %v", got)
	}

	// A replace path covers its directory, not the ones below it, and is
	// matched whatever its case and slashes.
	localization, _ := set.Files(Folder{Path: "localization", Recursive: true, Extension: ".yml"})

	want := []string{
		"game:localization/english/a_l_english.yml",
		"json:localization/german/mine_l_german.yml",
		"game:localization/german/sub/b_l_german.yml",
	}

	if got := listing(localization); !slices.Equal(got, want) {
		t.Errorf("localization =\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"common/goods":     "common/goods",
		"Common\\Goods\\":  "common/goods",
		"/common//goods/":  "common/goods",
		"./common/../gfx/": "gfx",
	}

	for input, want := range cases {
		if got := Normalize(input); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", input, got, want)
		}
	}
}

// TestLayers covers games that split their files into layers, each with its
// own common, localization and so on, as Europa Universalis 5 does with
// in_game, main_menu and loading_screen, and that ship their DLCs in the same
// shape below game/dlc.
func TestLayers(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"Game/game/in_game/common/goods/00_goods.txt":                      "",
		"Game/game/in_game/common/goods/20_more.txt":                       "",
		"Game/game/main_menu/common/goods/10_menu.txt":                     "",
		"Game/game/loading_screen/common/defines/00_defines.txt":           "",
		"Game/game/main_menu/localization/english/a_l_english.yml":         "",
		"Game/game/in_game/localization/english/b_l_english.yml":           "",
		"Game/game/dlc/D001_first/in_game/common/goods/05_dlc.txt":         "",
		"Game/game/dlc/D001_first/D001_first.dlc.json":                     "{}",
		"Game/game/dlc/D002_music/music/track.ogg":                         "",
		"Game/game/dlc/D000_shared/main_menu/localization/x_l_english.yml": "",
		"Mod/.metadata/metadata.json":                                      `{ "name": "Mod", "game_custom_data": { "replace_paths": [ "in_game/common/goods" ] } }`,
		"Mod/in_game/common/goods/zz_mod.txt":                              "",
		"Other/in_game/common/goods/20_more.txt":                           "",
	})

	set := Open([]Source{
		{Name: "game", Path: filepath.Join(root, "Game")},
		{Name: "other", Path: filepath.Join(root, "Other")},
	})

	if len(set.Diagnostics) != 0 {
		t.Fatalf("diagnostics: %v", set.Diagnostics)
	}

	// A DLC with game files is read after the game it belongs to; one with only
	// music is not a root at all.
	if want := []string{"game", "game/D000_shared", "game/D001_first", "other"}; !slices.Equal(set.Names(), want) {
		t.Errorf("names = %v, want %v", set.Names(), want)
	}

	if want := []string{"in_game", "loading_screen", "main_menu"}; !slices.Equal(set.Layers(), want) {
		t.Errorf("layers = %v, want %v", set.Layers(), want)
	}

	// A folder is looked for in every layer, and the files of all of them are
	// read in order of name, as if they were one folder. A file replaces the
	// file of the same name in the same layer only.
	goods, diagnostics := set.Files(Folder{Path: "common/goods"})

	want := []string{
		"game:in_game/common/goods/00_goods.txt",
		"game/D001_first:in_game/common/goods/05_dlc.txt",
		"game:main_menu/common/goods/10_menu.txt",
		"other:in_game/common/goods/20_more.txt",
	}

	if got := listing(goods); !slices.Equal(got, want) {
		t.Errorf("goods =\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}

	if !mentions(diagnostics, "replaces the file of the same name from game") {
		t.Errorf("the replaced file should be noted: %v", diagnostics)
	}

	// A folder can also be named with its layer, to read that layer alone.
	menu, _ := set.Files(Folder{Path: "main_menu/common/goods"})
	if got := listing(menu); !slices.Equal(got, []string{"game:main_menu/common/goods/10_menu.txt"}) {
		t.Errorf("main menu goods = %v", got)
	}

	defines, _ := set.Files(Folder{Path: "common/defines", Recursive: true})
	if got := listing(defines); !slices.Equal(got, []string{"game:loading_screen/common/defines/00_defines.txt"}) {
		t.Errorf("defines = %v", got)
	}

	localization, _ := set.Files(Folder{Path: "localization", Recursive: true, Extension: ".yml"})
	if len(localization) != 3 {
		t.Errorf("localization = %v", listing(localization))
	}

	// Replace paths name the layer.
	modded := Open([]Source{{Path: filepath.Join(root, "Game")}, {Path: filepath.Join(root, "Mod")}})

	goods, _ = modded.Files(Folder{Path: "common/goods"})
	if got := listing(goods); !slices.Equal(got, []string{"Game:main_menu/common/goods/10_menu.txt", "Mod:in_game/common/goods/zz_mod.txt"}) {
		t.Errorf("replaced goods = %v", got)
	}
}

func TestMetadataWithByteOrderMark(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"mod/.metadata/metadata.json": "\xef\xbb\xbf" + `{ "name": "Marked", "game_custom_data": { "replace_paths": [ "common/goods" ] } }`,
		"mod/common/goods/a.txt":      "",
	})

	set := Open([]Source{{Path: filepath.Join(root, "mod")}})

	if !slices.Equal(set.Names(), []string{"Marked"}) {
		t.Errorf("names = %v", set.Names())
	}
}

// TestFlatDLC covers a DLC shaped like the game folder of Victoria 3, with
// common and localization directly inside it. The DLCs of Victoria 3 hold only
// music today, but nothing stops one from holding script.
func TestFlatDLC(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"Victoria 3/game/common/goods/00_goods.txt":                  "",
		"Victoria 3/game/dlc/dlc001_music/music/track.ogg":           "",
		"Victoria 3/game/dlc/dlc002_goods/common/goods/00_goods.txt": "",
		"Victoria 3/game/dlc/dlc002_goods/common/goods/50_dlc.txt":   "",
		"mod/common/goods/99_mod.txt":                                "",
	})

	set := Open([]Source{
		{Name: "game", Path: filepath.Join(root, "Victoria 3")},
		{Name: "mod", Path: filepath.Join(root, "mod")},
	})

	if want := []string{"game", "game/dlc002_goods", "mod"}; !slices.Equal(set.Names(), want) {
		t.Errorf("names = %v, want %v", set.Names(), want)
	}

	if len(set.Layers()) != 0 {
		t.Errorf("layers = %v", set.Layers())
	}

	goods, _ := set.Files(Folder{Path: "common/goods"})

	want := []string{"game/dlc002_goods:common/goods/00_goods.txt", "game/dlc002_goods:common/goods/50_dlc.txt", "mod:common/goods/99_mod.txt"}
	if got := listing(goods); !slices.Equal(got, want) {
		t.Errorf("goods = %v, want %v", got, want)
	}
}

func TestFind(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"game/gfx/models/body/body.mesh":        "game",
		"game/gfx/models/body/body_diffuse.dds": "game",
		"game/gfx/models/hat/hat.mesh":          "game",
		"game/gfx/models/coat/coat.mesh":        "game",
		"mod/.metadata/metadata.json":           `{ "name": "Mod", "game_custom_data": { "replace_paths": [ "gfx/models/hat" ] } }`,
		"mod/gfx/models/body/body.mesh":         "mod",
		"mod/gfx/models/coat/lining.dds":        "mod",
		"layered/in_game/gfx/models/ship.mesh":  "layered",
	})

	set := Open([]Source{
		{Name: "game", Path: filepath.Join(root, "game")},
		{Path: filepath.Join(root, "mod")},
	})

	cases := map[string]struct {
		source string
		found  bool
	}{
		// The mod replaces the file of the game.
		"gfx/models/body/body.mesh": {"Mod", true},
		// A file only the game has is the game's.
		"gfx/models/body/body_diffuse.dds": {"game", true},
		// Whatever the slashes, and wherever the path wanders on the way.
		`gfx\models\body\..\body\body.mesh`: {"Mod", true},
		// The mod replaces the whole directory, so the game's file is gone.
		"gfx/models/hat/hat.mesh": {"", false},
		// Only the directory itself, not its neighbours.
		"gfx/models/coat/coat.mesh":  {"game", true},
		"gfx/models/coat/lining.dds": {"Mod", true},
		// Nobody has it.
		"gfx/models/nothing.mesh": {"", false},
		// A directory is no file.
		"gfx/models/body": {"", false},
		"":                {"", false},
		// A path cannot climb out of the folders.
		"../../game/gfx/models/body/body.mesh": {"", false},
		"../gfx/models/body/body.mesh":         {"Mod", true},
	}

	for relative, want := range cases {
		file, found := set.Find(relative)
		if found != want.found || file.Source != want.source {
			t.Errorf("Find(%q) = %+v, %v, want from %q, %v", relative, file, found, want.source, want.found)

			continue
		}

		// Each file holds the name of the folder it is in.
		if found {
			data, err := os.ReadFile(file.Path)
			if err != nil || string(data) != map[string]string{"game": "game", "Mod": "mod"}[file.Source] {
				t.Errorf("Find(%q) points at %s, holding %q", relative, file.Path, data)
			}
		}
	}

	if file, _ := set.Find("gfx/models/body/body.mesh"); file.SourceOrder != 1 || file.Relative != "gfx/models/body/body.mesh" {
		t.Errorf("found file = %+v", file)
	}

	// In a game with layers the path names its layer.
	layered := Open([]Source{{Name: "layered", Path: filepath.Join(root, "layered")}})

	file, found := layered.Find("in_game/gfx/models/ship.mesh")
	if !found || file.Layer != "in_game" {
		t.Errorf("Find in a layer = %+v, %v", file, found)
	}

	// A path that names no layer is looked for in each of them.
	file, found = layered.Find("gfx/models/ship.mesh")
	if !found || file.Layer != "in_game" || file.Relative != "in_game/gfx/models/ship.mesh" {
		t.Errorf("Find below the layers = %+v, %v", file, found)
	}

	if _, found := layered.Find("other_layer/gfx/models/ship.mesh"); found {
		t.Error("found a file in a layer the folder does not have")
	}
}
