package script

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// economyFolders are the folders under common/ the tool reads. The installed
// game must parse without a single warning there: a warning on vanilla means
// the grammar is missing something the game really writes.
var economyFolders = []string{
	"goods", "buildings", "building_groups", "production_methods", "production_method_groups",
	"pop_types", "pop_needs", "buy_packages", "company_types", "technology", "defines",
}

// TestParseInstalledGameFiles runs the parser over a real installation.
//
// The unit tests cover the grammar as it is meant to be; this one covers the
// grammar as the game actually writes it. It is skipped unless PDX_GAME_DIR
// points at the game folder (the one holding common/) or its parent.
func TestParseInstalledGameFiles(t *testing.T) {
	common := findCommon(t)

	for _, folder := range economyFolders {
		files := 0

		err := filepath.WalkDir(filepath.Join(common, folder), func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".txt") {
				return err
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			files++

			document := Parse(string(data))
			for _, warning := range document.Warnings {
				t.Errorf("%s: %s", path, warning)
			}

			if len(document.Fields) == 0 {
				t.Errorf("%s: no fields read", path)
			}

			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", folder, err)
		}

		if files == 0 {
			t.Errorf("no files found in %s", folder)
		}
	}
}

// TestParseWholeCommonFolder parses every file under common/, including the
// many folders the tool does not use, and reports what it had to skip. Those
// folders hold trigger and effect script that stretches the grammar further
// than data does, which makes them a good test of the recovery. The test only
// fails when a file yields nothing at all.
func TestParseWholeCommonFolder(t *testing.T) {
	common := findCommon(t)

	var (
		files    int
		warnings = map[string]int{}
	)

	err := filepath.WalkDir(common, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".txt") {
			return err
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		files++

		document := Parse(string(data))
		if len(document.Warnings) > 0 {
			relative, _ := filepath.Rel(common, path)
			warnings[relative] = len(document.Warnings)
		}

		if len(document.Fields) == 0 && len(strings.TrimSpace(strings.TrimPrefix(string(data), byteOrderMark))) > 0 && !onlyComments(string(data)) {
			t.Errorf("%s: nothing could be read", path)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	paths := make([]string, 0, len(warnings))
	for path := range warnings {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		t.Logf("%s: %d warnings", path, warnings[path])
	}

	t.Logf("parsed %d files, %d with warnings", files, len(warnings))
}

func onlyComments(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, byteOrderMark))
		if line != "" && !strings.HasPrefix(line, "#") {
			return false
		}
	}

	return true
}

func findCommon(t *testing.T) string {
	t.Helper()

	root := os.Getenv("PDX_GAME_DIR")
	if root == "" {
		t.Skip("set PDX_GAME_DIR to the game folder to run this test")
	}

	for _, candidate := range []string{root, filepath.Join(root, "common"), filepath.Join(root, "game", "common")} {
		if filepath.Base(candidate) == "common" {
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				return candidate
			}
		}
	}

	t.Fatalf("no common folder found in %s", root)

	return ""
}
