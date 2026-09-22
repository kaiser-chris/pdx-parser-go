package localization

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kaiser-chris/pdx-parser-go/folders"
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

func loadTree(t *testing.T, files map[string]string, language string) *Localization {
	t.Helper()

	root := t.TempDir()
	writeTree(t, root, files)

	return Load(folders.Open([]folders.Source{
		{Name: "game", Path: filepath.Join(root, "game")},
		{Name: "mod", Path: filepath.Join(root, "mod")},
		{Name: "later", Path: filepath.Join(root, "later")},
	}), language)
}

func findDiagnostic(localization *Localization, severity report.Severity, fragment string) bool {
	for _, diagnostic := range localization.Diagnostics {
		if diagnostic.Severity == severity && strings.Contains(diagnostic.String(), fragment) {
			return true
		}
	}

	return false
}

func dump(diagnostics report.Diagnostics) string {
	var lines []string
	for _, diagnostic := range diagnostics {
		lines = append(lines, "  "+diagnostic.String())
	}

	return strings.Join(lines, "\n")
}

func TestLoad(t *testing.T) {
	localization := loadTree(t, map[string]string{
		"game/localization/english/goods_l_english.yml": byteOrderMark +
			"l_english:\n steel:0 \"Steel\"\n iron:0 \"Iron\"\n coal:0 \"Coal\"\n",
		"game/localization/english/sub/more_l_english.yml": byteOrderMark + "l_english:\n tools:0 \"Tools\"\n",
		"game/localization/german/goods_l_german.yml":      byteOrderMark + "l_german:\n steel:0 \"Stahl\"\n",
		"game/localization/languages.yml":                  byteOrderMark + "l_english:\n l_english:1 \"English\"\n",

		// Outside a replace folder a mod cannot change a text the game has.
		"mod/localization/english/aaa_mod_l_english.yml": byteOrderMark +
			"l_english:\n steel:0 \"Mod Steel\"\n gold:0 \"Gold\"\n",

		// Inside one it can.
		"mod/localization/english/replace/iron_l_english.yml": byteOrderMark + "l_english:\n iron:0 \"Pig Iron\"\n",
		"later/localization/replace/english/iron_l_english.yml": byteOrderMark +
			"l_english:\n iron:1 \"Wrought Iron\"\n",
	}, "english")

	want := map[string]string{
		"steel": "Steel",
		"iron":  "Wrought Iron",
		"coal":  "Coal",
		"tools": "Tools",
		"gold":  "Gold",
	}

	for key, text := range want {
		if got := localization.Name(key); got != text {
			t.Errorf("%s = %q, want %q", key, got, text)
		}
	}

	if localization.Len() != len(want) {
		t.Errorf("keys = %v, want only the english keys and not languages.yml", localization.Keys())
	}

	if got := localization.Name("missing_key"); got != "missing_key" {
		t.Errorf("a missing key should show as itself, got %q", got)
	}

	iron, _ := localization.Get("iron")
	if !iron.Replace || iron.Source != "later" || iron.Version != 1 || !iron.HasVersion ||
		iron.File != "localization/replace/english/iron_l_english.yml" || iron.Line != 2 {
		t.Errorf("iron = %+v", iron)
	}

	steel, _ := localization.Get("steel")
	if steel.Source != "game" {
		t.Errorf("steel comes from %s, want the game's definition to win outside replace folders", steel.Source)
	}

	for _, fragment := range []string{
		"localization key steel: is already defined in localization/english/goods_l_english.yml:2 (game)",
		"localization key iron: replaces the text from the replace folder",
	} {
		if !findDiagnostic(localization, report.SeverityInfo, fragment) {
			t.Errorf("no info mentioning %q:\n%s", fragment, dump(localization.Diagnostics))
		}
	}

	if count := localization.Diagnostics.Count(report.SeverityWarning); count > 0 {
		t.Errorf("unexpected warnings:\n%s", dump(localization.Diagnostics))
	}
}

func TestLoadReportsProblems(t *testing.T) {
	localization := loadTree(t, map[string]string{
		"game/localization/english/a_l_english.yml":  "l_english:\n no_bom:0 \"No BOM\"\n",
		"game/localization/english/b_l_english.yml":  byteOrderMark + "l_german:\n wrong:0 \"Falsch\"\n",
		"game/localization/english/c_l_english.yml":  byteOrderMark + " headless:0 \"Headless\"\n",
		"game/localization/english/d_l_english.yml":  byteOrderMark + "l_english:\n broken \"x\"\n open:0 \"never closed\n",
		"game/localization/english/e_l_english.yml":  byteOrderMark,
		"game/localization/english/f_l_english .yml": byteOrderMark + "l_english:\n spaced:0 \"Spaced\"\n",
	}, "english")

	for _, key := range []string{"no_bom", "headless", "broken", "open", "spaced"} {
		if _, ok := localization.Get(key); !ok {
			t.Errorf("%s should be read despite its problem", key)
		}
	}

	if _, ok := localization.Get("wrong"); ok {
		t.Error("keys under an l_german header are not english")
	}

	for _, fragment := range []string{
		"a_l_english.yml:1:1: has no UTF-8 byte order mark",
		"b_l_english.yml:1:1: is named for l_english but has an l_german: header",
		"c_l_english.yml:1:1: localization key headless: key comes before any language header",
		"d_l_english.yml:2:8: localization key broken: missing : after broken",
		"d_l_english.yml:3:9: localization key open: text of open is never closed",
		`file name "f_l_english .yml" contains a space`,
	} {
		if !findDiagnostic(localization, report.SeverityWarning, fragment) {
			t.Errorf("no warning mentioning %q:\n%s", fragment, dump(localization.Diagnostics))
		}
	}
}

// TestLoadFilesWithoutKeys covers files that hold no keys at all, empty or
// with every line commented out, as Crusader Kings 3 leaves translations it
// no longer needs. They cost nothing, so they are noted rather than warned
// about; a file with lines that are not comments and still no header is a
// problem.
func TestLoadFilesWithoutKeys(t *testing.T) {
	localization := loadTree(t, map[string]string{
		"game/localization/english/empty_l_english.yml":    byteOrderMark,
		"game/localization/english/comments_l_english.yml": byteOrderMark + "# === [LocEditor:RedundantFile] File contains no active keys\n\n#l_english:\n# old_key:0 \"Old\"\n",
		"game/localization/english/garbage_l_english.yml":  byteOrderMark + "just some words\n",
	}, "english")

	for _, file := range []string{"empty_l_english.yml:1:1: holds no keys", "comments_l_english.yml:1:1: holds no keys"} {
		if !findDiagnostic(localization, report.SeverityInfo, file) {
			t.Errorf("no note mentioning %q:\n%s", file, dump(localization.Diagnostics))
		}
	}

	if !findDiagnostic(localization, report.SeverityWarning, "garbage_l_english.yml:1:1: has no l_english: header and no keys") {
		t.Errorf("a file of unreadable lines should be warned about:\n%s", dump(localization.Diagnostics))
	}
}

func TestLanguages(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"game/localization/english/a_l_english.yml":          byteOrderMark + "l_english:\n",
		"game/localization/german/a_l_german.yml":            byteOrderMark + "l_german:\n",
		"game/localization/languages.yml":                    byteOrderMark + "l_english:\n",
		"mod/localization/simp_chinese/b_l_simp_chinese.yml": byteOrderMark + "l_simp_chinese:\n",
	})

	languages := Languages(folders.Open([]folders.Source{
		{Path: filepath.Join(root, "game")},
		{Path: filepath.Join(root, "mod")},
	}))

	if !slices.Equal(languages, []string{"english", "german", "simp_chinese"}) {
		t.Errorf("languages = %v", languages)
	}
}

// TestLoadInstalledGame reads every language of a real installation. The
// game's own files must load without a single warning, and a few texts are
// checked against the files by hand.
func TestLoadInstalledGame(t *testing.T) {
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

	set := folders.Open(sources)
	languages := Languages(set)
	if len(languages) < 2 {
		t.Fatalf("languages = %v", languages)
	}

	for _, language := range languages {
		localization := Load(set, language)

		warnings := localization.Diagnostics.Filter(report.SeverityWarning)
		t.Logf("%s: %d keys, %d warnings, %d infos", language, localization.Len(), len(warnings),
			len(localization.Diagnostics)-len(warnings))

		if len(sources) == 1 {
			for _, diagnostic := range warnings {
				t.Errorf("vanilla should load cleanly: %s", diagnostic)
			}
		}
	}

	english := Load(set, "english")

	checks := map[string]string{
		"steel":      "Steel",
		"small_arms": "Small Arms",
	}

	for key, want := range checks {
		if got := english.Name(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}

	// A trailing comment holding a quote stays out of the text.
	if anthem := english.Name("culture_brazil.3.f"); !strings.HasSuffix(anthem, `#!"`) || strings.Contains(anthem, "National anthem") {
		t.Errorf("culture_brazil.3.f ends in %q", anthem[max(0, len(anthem)-40):])
	}

	// Formatting right after the opening quote belongs to the text.
	if text := english.Name("WARGOAL_IG_TYPE_NOT_IN_GOVERNMENT"); !strings.HasPrefix(text, " #v ") {
		t.Errorf("WARGOAL_IG_TYPE_NOT_IN_GOVERNMENT = %q", text)
	}
}
