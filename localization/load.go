package localization

import (
	"fmt"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
)

// Text is the text of one key, and where it was read from.
type Text struct {
	Key string

	Version    int
	HasVersion bool

	// Text is the text as written, escapes and formatting codes included.
	Text string

	// Source is the name of the configured folder, File the file's path
	// below it and Path the file on disk.
	Source string
	File   string
	Path   string
	Line   int

	// Replace reports whether the text came from a replace folder, which
	// overrides the text of the same key anywhere else.
	Replace bool
}

// Localization is the text of every key in one language, from the game and
// its mods.
type Localization struct {
	Language string

	keys  []string
	texts map[string]Text

	// Diagnostics are the problems found in the files, and the keys defined
	// more than once.
	Diagnostics report.Diagnostics
}

// Get returns the text of a key.
func (l *Localization) Get(key string) (Text, bool) {
	text, ok := l.texts[key]

	return text, ok
}

// Name returns the text of a key, or the key itself when it has none, which
// is what the game shows for a key it cannot find.
func (l *Localization) Name(key string) string {
	if text, ok := l.texts[key]; ok {
		return text.Text
	}

	return key
}

// Len is the number of keys.
func (l *Localization) Len() int {
	return len(l.keys)
}

// Keys returns every key, in the order they were first defined.
func (l *Localization) Keys() []string {
	return append([]string(nil), l.keys...)
}

// Folder is where the game and mods keep their localization. The game reads
// the folders below it too: one per language, and more inside those.
var Folder = folders.Folder{Path: "localization", Recursive: true, Extension: ".yml"}

// Load reads the text of every key in one language, "english" for example,
// from a set of game and mod folders.
//
// A file belongs to a language by its name, which ends in _l_english.yml for
// English; that is how the game picks the files of a language. Files inside a
// folder named replace override the text of the same key anywhere else.
//
// When a key is defined more than once outside replace folders, the first
// definition in load order is used, the game's before any mod's. That is the
// reason the replace folders exist: without them a mod cannot change a text
// the game already has. Among replace folders the last definition wins.
func Load(set *folders.Set, language string) *Localization {
	localization := &Localization{Language: language, texts: map[string]Text{}}

	files, diagnostics := set.Files(Folder)
	localization.Diagnostics = append(localization.Diagnostics, diagnostics...)

	var ordinary, replacing []folders.File

	for _, file := range files {
		if fileLanguage(file.Relative) != language {
			continue
		}

		if inReplaceFolder(file.Relative) {
			replacing = append(replacing, file)
		} else {
			ordinary = append(ordinary, file)
		}
	}

	byLoadOrder(ordinary)
	byLoadOrder(replacing)

	for _, file := range ordinary {
		localization.read(file, false)
	}

	for _, file := range replacing {
		localization.read(file, true)
	}

	return localization
}

// Languages returns the languages the sources have localization files for,
// sorted.
func Languages(set *folders.Set) []string {
	files, _ := set.Files(Folder)

	seen := map[string]bool{}

	for _, file := range files {
		if language := fileLanguage(file.Relative); language != "" {
			seen[language] = true
		}
	}

	languages := make([]string, 0, len(seen))
	for language := range seen {
		languages = append(languages, language)
	}

	sort.Strings(languages)

	return languages
}

// read parses one file and takes its entries.
func (l *Localization) read(file folders.File, replace bool) {
	origin := func(line, column int, key, format string, args ...any) report.Diagnostic {
		subject := ""
		if key != "" {
			subject = "localization key " + key
		}

		return report.Diagnostic{
			Source:  file.Source,
			Path:    file.Path,
			Line:    line,
			Column:  column,
			Subject: subject,
			Message: fmt.Sprintf(format, args...),
		}
	}

	add := func(severity report.Severity, diagnostic report.Diagnostic) {
		diagnostic.Severity = severity
		l.Diagnostics = append(l.Diagnostics, diagnostic)
	}

	data, err := os.ReadFile(file.Path)
	if err != nil {
		add(report.SeverityError, origin(0, 0, "", "could not be read: %v", err))

		return
	}

	parsed := Parse(string(data))

	if name := path.Base(file.Relative); strings.ContainsAny(name, " \t") {
		add(report.SeverityWarning, origin(0, 0, "",
			"file name %q contains a space; it is read as %s, but the game may not read it at all", name, l.Language))
	}

	if !parsed.HasByteOrderMark {
		add(report.SeverityWarning, origin(1, 1, "",
			"has no UTF-8 byte order mark; the game does not read a localization file without one"))
	}

	for _, warning := range parsed.Warnings {
		add(report.SeverityWarning, origin(warning.Line, warning.Column, warning.Key, "%s", warning.Message))
	}

	if len(parsed.Sections) == 0 {
		// A file with nothing but comments, which is how Crusader Kings 3
		// leaves translations it no longer needs, costs nothing.
		if len(parsed.Warnings) == 0 {
			add(report.SeverityInfo, origin(1, 1, "", "holds no keys"))

			return
		}

		add(report.SeverityWarning, origin(1, 1, "", "has no l_%s: header and no keys", l.Language))

		return
	}

	for _, section := range parsed.Sections {
		// A section for another language in a file named for this one
		// belongs to that language; only a missing header is taken as this
		// one, since the name is all there is to go on.
		if section.Language != "" && section.Language != l.Language {
			add(report.SeverityWarning, origin(section.Line, 1, "",
				"is named for l_%s but has an l_%s: header; its keys are not read as %s",
				l.Language, section.Language, l.Language))

			continue
		}

		for _, entry := range section.Entries {
			l.take(Text{
				Key:        entry.Key,
				Version:    entry.Version,
				HasVersion: entry.HasVersion,
				Text:       entry.Text,
				Source:     file.Source,
				File:       file.Relative,
				Path:       file.Path,
				Line:       entry.Line,
				Replace:    replace,
			}, origin)
		}
	}
}

// take records a text, following the rules for keys defined more than once.
func (l *Localization) take(text Text, origin func(line, column int, key, format string, args ...any) report.Diagnostic) {
	existing, exists := l.texts[text.Key]

	switch {
	case !exists:
		l.keys = append(l.keys, text.Key)
		l.texts[text.Key] = text

	case !text.Replace:
		diagnostic := origin(text.Line, 1, text.Key, "is already defined in %s:%d (%s); this definition is not used",
			existing.File, existing.Line, existing.Source)
		diagnostic.Severity = report.SeverityInfo
		l.Diagnostics = append(l.Diagnostics, diagnostic)

	default:
		if existing.Replace {
			diagnostic := origin(text.Line, 1, text.Key, "replaces the text from the replace folder in %s:%d (%s)",
				existing.File, existing.Line, existing.Source)
			diagnostic.Severity = report.SeverityInfo
			l.Diagnostics = append(l.Diagnostics, diagnostic)
		}

		l.texts[text.Key] = text
	}
}

// fileLanguage reads the language from a file's name: english for
// goods_l_english.yml. It is empty for a file named some other way, such as
// languages.yml, which the game does not read as the text of a language.
func fileLanguage(relative string) string {
	name := strings.ToLower(path.Base(relative))

	stem, found := strings.CutSuffix(name, ".yml")
	if !found {
		return ""
	}

	// A stray space before the extension, as in "units_l_english .yml",
	// happens; the file is still meant for that language.
	stem = strings.TrimSpace(stem)

	index := strings.LastIndex(stem, "l_")
	if index < 0 || index > 0 && stem[index-1] != '_' {
		return ""
	}

	return stem[index+2:]
}

// inReplaceFolder reports whether a file is inside a folder named replace, at
// any depth: localization/replace/english and localization/english/replace
// both occur.
func inReplaceFolder(relative string) bool {
	for _, folder := range strings.Split(strings.ToLower(path.Dir(relative)), "/") {
		if folder == "replace" {
			return true
		}
	}

	return false
}

// byLoadOrder sorts files by source in load order, then by name.
func byLoadOrder(files []folders.File) {
	sort.SliceStable(files, func(first, second int) bool {
		if files[first].SourceOrder != files[second].SourceOrder {
			return files[first].SourceOrder < files[second].SourceOrder
		}

		return strings.ToLower(files[first].Relative) < strings.ToLower(files[second].Relative)
	})
}
