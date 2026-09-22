package localization

import (
	"strings"
	"testing"
)

// entry parses a file holding one key after a header, and returns the entry.
func entry(t *testing.T, line string) (Entry, []Warning) {
	t.Helper()

	file := Parse(byteOrderMark + "l_english:\n" + line + "\n")

	entries := file.Entries()
	if len(entries) != 1 {
		t.Fatalf("%q: got %d entries, want 1 (warnings %v)", line, len(entries), file.Warnings)
	}

	return entries[0], file.Warnings
}

func TestParseTexts(t *testing.T) {
	cases := []struct {
		name, line string
		key, text  string
		version    int
		hasVersion bool
	}{
		{"with a version", ` some_loc_key:0 "The localization text"`, "some_loc_key", "The localization text", 0, true},
		{"without a version", ` some_loc_key: "The localization text"`, "some_loc_key", "The localization text", 0, false},
		{"a higher version", ` steel:12 "Steel"`, "steel", "Steel", 12, true},
		{"no space before the text", ` steel:0"Steel"`, "steel", "Steel", 0, true},
		{"quotes inside the text", ` some_loc_key: "A text with a "special" word."`, "some_loc_key", `A text with a "special" word.`, 0, false},
		{"text that starts and ends with a quote", ` quote: ""Hello," she said."`, "quote", `"Hello," she said.`, 0, false},
		{"empty text", ` empty: ""`, "empty", "", 0, false},
		{"escapes are kept", ` escaped: "Line one\nLine \"two\""`, "escaped", `Line one\nLine \"two\"`, 0, false},
		{"a trailing comment", ` steel:0 "Steel" # the good`, "steel", "Steel", 0, true},
		{"a trailing comment right after the quote", ` steel:0 "Steel"#the good`, "steel", "Steel", 0, true},
		{
			"a trailing comment with a quote in it",
			` culture_brazil.3.f: ""#italic The placid shores of the Ipiranga heard#!"" #–National anthem of Brazil"`,
			"culture_brazil.3.f", `"#italic The placid shores of the Ipiranga heard#!"`, 0, false,
		},
		{
			"formatting right after the opening quote",
			` WARGOAL_IG_TYPE_NOT_IN_GOVERNMENT:1 " #v $IG_TYPE$#! is not in the government of [COUNTRY.GetName]"`,
			"WARGOAL_IG_TYPE_NOT_IN_GOVERNMENT", ` #v $IG_TYPE$#! is not in the government of [COUNTRY.GetName]`, 1, true,
		},
		{
			"formatting after an escaped quote",
			` sikh_empire.1.f: "\"On the shoulders of 100 men.\" #!" # Sohan Lal Suri`,
			"sikh_empire.1.f", `\"On the shoulders of 100 men.\" #!`, 0, false,
		},
		{"formatting codes", ` bold: "#b Bold#! and #italic italic#!"`, "bold", "#b Bold#! and #italic italic#!", 0, false},
		{"keys with dots, capitals and dashes", ` culture_brazil.3.F-x: "Text"`, "culture_brazil.3.F-x", "Text", 0, false},
		{"indented with a tab", "\tsteel: \"Steel\"", "steel", "Steel", 0, false},
		{"not indented", `steel: "Steel"`, "steel", "Steel", 0, false},
		{"non-ASCII text", ` steel: "Сталь 鋼"`, "steel", "Сталь 鋼", 0, false},
		{"a key that looks like a header", ` l_english:1 "English"`, "l_english", "English", 1, true},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, warnings := entry(t, test.line)

			if got.Key != test.key || got.Text != test.text || got.Version != test.version || got.HasVersion != test.hasVersion {
				t.Errorf("got %+v, want key %q text %q version %d (%v)", got, test.key, test.text, test.version, test.hasVersion)
			}

			if len(warnings) > 0 {
				t.Errorf("unexpected warnings: %v", warnings)
			}

			if got.Line != 2 {
				t.Errorf("line = %d, want 2", got.Line)
			}
		})
	}
}

func TestParseRecovers(t *testing.T) {
	cases := []struct {
		name, line string
		key, text  string
		warning    string
	}{
		{"missing colon", ` L75_ADJ "Shemasakan"`, "L75_ADJ", "Shemasakan", "missing :"},
		{"text never closed", ` airship_tt:0 "Has surveyed a site for Airship Moorings`, "airship_tt", "Has surveyed a site for Airship Moorings", "never closed"},
		{"only an opening quote", ` broken: "`, "broken", "", "never closed"},
		{"text after the closing quote", ` steel: "Steel" extra`, "steel", "Steel", "ignored \"extra\""},
		{"text before the opening quote", ` steel: x "Steel"`, "steel", "Steel", "ignored \"x \""},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, warnings := entry(t, test.line)

			if got.Key != test.key || got.Text != test.text {
				t.Errorf("got %+v, want key %q text %q", got, test.key, test.text)
			}

			if !mentions(warnings, test.warning) {
				t.Errorf("no warning mentions %q: %v", test.warning, warnings)
			}
		})
	}
}

func TestParseSkipsLinesWithoutAText(t *testing.T) {
	file := Parse(byteOrderMark + strings.Join([]string{
		"l_english:",
		" no_text:",
		" no_text_either:0",
		" no_quotes: plain words",
		" just some words",
		` ": "no key"`,
		` kept: "Kept"`,
	}, "\n"))

	entries := file.Entries()
	if len(entries) != 1 || entries[0].Key != "kept" {
		t.Errorf("entries = %+v, want only kept", entries)
	}

	for _, fragment := range []string{"no_text has no text", "no_text_either has no text", "no_quotes has no text in quotes", "not a key and a text", "without a key"} {
		if !mentions(file.Warnings, fragment) {
			t.Errorf("no warning mentions %q: %v", fragment, file.Warnings)
		}
	}
}

func TestParseHeaders(t *testing.T) {
	file := Parse(byteOrderMark + "# a comment first\n\nl_english:  #TODO: reconcile this\n a: \"A\"\n\nl_german:\n a: \"Ä\"\n b: \"B\"\n")

	if !file.HasByteOrderMark {
		t.Error("the byte order mark was not noticed")
	}

	if len(file.Sections) != 2 {
		t.Fatalf("sections = %+v", file.Sections)
	}

	if file.Sections[0].Language != "english" || len(file.Sections[0].Entries) != 1 {
		t.Errorf("first section = %+v", file.Sections[0])
	}

	if file.Sections[1].Language != "german" || len(file.Sections[1].Entries) != 2 || file.Sections[1].Line != 6 {
		t.Errorf("second section = %+v", file.Sections[1])
	}

	if file.Language() != "english" {
		t.Errorf("language = %q", file.Language())
	}

	if len(file.Warnings) > 0 {
		t.Errorf("unexpected warnings: %v", file.Warnings)
	}
}

func TestParseWithoutHeader(t *testing.T) {
	file := Parse(" a: \"A\"\n")

	if file.HasByteOrderMark {
		t.Error("there is no byte order mark")
	}

	if len(file.Entries()) != 1 || !mentions(file.Warnings, "before any language header") {
		t.Errorf("entries %+v, warnings %v", file.Entries(), file.Warnings)
	}
}

func TestParseLineEndings(t *testing.T) {
	file := Parse(byteOrderMark + "l_english:\r\n a: \"A\"\r\n b:0 \"B\" # note\r\n")

	entries := file.Entries()
	if len(entries) != 2 || entries[0].Text != "A" || entries[1].Text != "B" {
		t.Errorf("entries = %+v", entries)
	}

	if len(file.Warnings) > 0 {
		t.Errorf("unexpected warnings: %v", file.Warnings)
	}
}

func TestFileLanguage(t *testing.T) {
	cases := map[string]string{
		"localization/english/goods_l_english.yml":           "english",
		"localization/english/replace/x_L_English.yml":       "english",
		"localization/simp_chinese/goods_l_simp_chinese.yml": "simp_chinese",
		"localization/l_german.yml":                          "german",
		"localization/english/units_l_english .yml":          "english",
		"localization/languages.yml":                         "",
		"localization/english/goods_english.yml":             "",
		"localization/english/goods_l_english.txt":           "",
	}

	for relative, want := range cases {
		if got := fileLanguage(relative); got != want {
			t.Errorf("fileLanguage(%q) = %q, want %q", relative, got, want)
		}
	}
}

func TestInReplaceFolder(t *testing.T) {
	cases := map[string]bool{
		"localization/replace/english/a_l_english.yml":    true,
		"localization/english/replace/a_l_english.yml":    true,
		"localization/english/Replace/a_l_english.yml":    true,
		"localization/english/a_l_english.yml":            false,
		"localization/english/replacements_l_english.yml": false,
	}

	for relative, want := range cases {
		if got := inReplaceFolder(relative); got != want {
			t.Errorf("inReplaceFolder(%q) = %v, want %v", relative, got, want)
		}
	}
}

// FuzzParse checks that nothing the fuzzer makes up can crash the parser, and
// that every entry and warning points at a line that exists. Run it with
//
//	go test ./localization -run '^$' -fuzz FuzzParse -fuzztime 60s
func FuzzParse(f *testing.F) {
	for _, seed := range []string{
		"l_english:\n a:0 \"A\"",
		byteOrderMark + "l_english:\n a: \"A \"b\" c\" # d \"e\"",
		"l_english:\n a: \" #v x#!\" # c",
		"l_english:\n a: \"\\\" #!\" # c",
		"a \"b\"",
		"l_x:\nl_y:\n:",
		"\"\"\"\n#\"\n :0",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		file := Parse(input)
		lines := strings.Count(strings.TrimPrefix(input, byteOrderMark), "\n") + 1

		for _, entry := range file.Entries() {
			if entry.Line < 1 || entry.Line > lines || entry.Key == "" {
				t.Errorf("entry %+v in a %d line input", entry, lines)
			}

			if strings.ContainsAny(entry.Text, "\n") {
				t.Errorf("text %q spans lines", entry.Text)
			}
		}

		for _, warning := range file.Warnings {
			if warning.Line < 1 || warning.Line > lines || warning.Message == "" {
				t.Errorf("warning %+v in a %d line input", warning, lines)
			}
		}
	})
}

func mentions(warnings []Warning, fragment string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning.Message, fragment) {
			return true
		}
	}

	return false
}

// TestParseLineBrokenInsideItsText covers a text a mod broke over two lines by
// accident, found in a mod for Europa Universalis 5. The second line is no
// entry, and its warning names no key, since the first word of a sentence is
// none, and quotes only the start of what can be a very long line.
func TestParseLineBrokenInsideItsText(t *testing.T) {
	long := strings.Repeat("and it goes on ", 100)
	file := Parse(byteOrderMark + "l_english:\n update_text: \"Hello, this update adds\ns $game_concept_culture$ and more " + long + "\"\n after: \"After\"\n")

	var found *Warning

	for index, warning := range file.Warnings {
		if strings.Contains(warning.Message, "not a key and a text") {
			found = &file.Warnings[index]
		}
	}

	if found == nil {
		t.Fatalf("no warning about the second line: %v", file.Warnings)
	}

	if found.Key != "" {
		t.Errorf("the warning names the key %q", found.Key)
	}

	if len(found.Message) > 200 {
		t.Errorf("the warning is %d characters long", len(found.Message))
	}

	entries := file.Entries()
	if len(entries) != 2 || entries[1].Key != "after" {
		t.Errorf("entries = %+v, want update_text and after", entries)
	}
}
