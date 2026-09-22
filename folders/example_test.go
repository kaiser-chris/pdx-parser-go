package folders_test

import (
	"fmt"

	"github.com/kaiser-chris/pdx-parser-go/folders"
)

// The example mod adds a file of goods to the game's. The game reads both, in
// order of file name, whichever folder each came from.
func ExampleSet_Files() {
	set := folders.Open([]folders.Source{
		{Name: "Victoria 3", Path: "../testdata/game"},
		{Path: "../testdata/mod"},
	})

	fmt.Println(set.Names())

	files, _ := set.Files(folders.Folder{Path: "common/goods"})
	for _, file := range files {
		fmt.Println(file.Source, file.Relative)
	}

	localization, _ := set.Files(folders.Folder{Path: "localization", Recursive: true, Extension: ".yml"})
	for _, file := range localization {
		fmt.Println(file.Source, file.Relative)
	}

	// Output:
	// [Victoria 3 Example Mod]
	// Victoria 3 common/goods/00_goods.txt
	// Example Mod common/goods/zz_mod_goods.txt
	// Victoria 3 localization/english/goods_l_english.yml
	// Example Mod localization/english/mod_l_english.yml
	// Example Mod localization/english/replace/mod_replace_l_english.yml
}

// A folder that holds no game or mod files is left out and reported.
func ExampleOpen() {
	set := folders.Open([]folders.Source{{Name: "Typo", Path: "../testdata/gmae"}})

	fmt.Println(len(set.Names()))
	fmt.Println(set.Diagnostics[0].Severity, set.Diagnostics[0].Source)

	// Output:
	// 0
	// error Typo
}
