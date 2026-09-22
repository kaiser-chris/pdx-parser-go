package localization_test

import (
	"fmt"

	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/localization"
)

// byteOrderMark starts every localization file of the games.
var byteOrderMark = string(rune(0xFEFF))

func ExampleParse() {
	file := localization.Parse(byteOrderMark + "l_english:\n" +
		` some_loc_key:0 "The localization text"` + "\n" +
		` other_loc_key: "A text with a "special" word."` + "\n" +
		` bold_key: "#b Bold#! text" # a comment, not part of the text` + "\n")

	fmt.Println(file.Language(), file.HasByteOrderMark)

	for _, entry := range file.Entries() {
		fmt.Printf("%s (version %d, %v): %s\n", entry.Key, entry.Version, entry.HasVersion, entry.Text)
	}

	// Output:
	// english true
	// some_loc_key (version 0, true): The localization text
	// other_loc_key (version 0, false): A text with a "special" word.
	// bold_key (version 0, false): #b Bold#! text
}

// A line that does not fit is read as far as it can be, and reported.
func ExampleParse_recovery() {
	file := localization.Parse(byteOrderMark + "l_english:\n" +
		` missing_colon "Text"` + "\n" +
		` unclosed: "Text without its closing quote` + "\n")

	for _, entry := range file.Entries() {
		fmt.Printf("%s: %s\n", entry.Key, entry.Text)
	}

	for _, warning := range file.Warnings {
		fmt.Println(warning)
	}

	// Output:
	// missing_colon: Text
	// unclosed: Text without its closing quote
	// line 2 column 15: missing : after missing_colon
	// line 3 column 12: text of unclosed is never closed with a quote, read to the end of the line
}

// The example mod renames steel from its replace folder and adds texts of its
// own.
func ExampleLoad() {
	set := folders.Open([]folders.Source{
		{Name: "Victoria 3", Path: "../testdata/game"},
		{Path: "../testdata/mod"},
	})

	english := localization.Load(set, "english")

	for _, key := range []string{"steel", "iron", "aluminium", "pm_open_hearth", "unknown_key"} {
		fmt.Printf("%s: %s\n", key, english.Name(key))
	}

	steel, _ := english.Get("steel")
	fmt.Println(steel.Source, steel.Replace, steel.File)

	// Output:
	// steel: Crucible Steel
	// iron: Iron
	// aluminium: Aluminium
	// pm_open_hearth: Open Hearth Process
	// unknown_key: unknown_key
	// Example Mod true localization/english/replace/mod_replace_l_english.yml
}
