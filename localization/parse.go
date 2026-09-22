// Package localization reads the localization files of Paradox games such as
// Victoria 3 and their mods: the text the game shows for a key, in each
// language.
//
// The files look like YAML but are not. A file starts with a header naming
// its language, followed by one key per line:
//
//	l_english:
//	 steel:0 "Steel"
//	 steel_desc: "Made from iron and coal."
//
// The number after the colon is a version; it is optional, and most keys do
// without it. The text runs from the first quote on the line to the last
// quote that is not part of a comment, and is kept exactly as written, quotes
// inside it included:
//
//	some_key: "A text with a "special" word."
//
// reads as A text with a "special" word. Escapes such as \n and \" and the
// game's formatting codes such as #bold ...#! are left in place for whatever
// shows the text to interpret.
//
// Parsing never fails. A line that does not fit is reported as a Warning and
// the entries around it are kept.
package localization

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// byteOrderMark is what every localization file of the game starts with. The
// game needs it to read a file as UTF-8.
const byteOrderMark = "\xef\xbb\xbf"

// Entry is one key and its text.
type Entry struct {
	Key string

	// Version is the number written after the colon, as in steel:0. It only
	// means something when HasVersion is set.
	Version    int
	HasVersion bool

	// Text is everything between the quotes, as written.
	Text string

	Line int
}

// Section is the entries under one language header.
type Section struct {
	// Language is the name in the header, "english" for l_english. It is
	// empty for entries written before any header.
	Language string

	Line    int
	Entries []Entry
}

// Warning is something the parser skipped or read on a best guess.
type Warning struct {
	Message string
	Line    int
	Column  int

	// Key is the key the problem is with, when it has one.
	Key string
}

func (w Warning) String() string {
	return fmt.Sprintf("line %d column %d: %s", w.Line, w.Column, w.Message)
}

// File is a parsed localization file.
type File struct {
	// Sections are the file's language headers and the entries under each,
	// in order. Almost every file has one; languages.yml has one per
	// language.
	Sections []Section

	// HasByteOrderMark reports whether the file starts with one. The game
	// does not read a localization file without it.
	HasByteOrderMark bool

	Warnings []Warning
}

// Language is the language of the file's first section.
func (f *File) Language() string {
	for _, section := range f.Sections {
		if section.Language != "" {
			return section.Language
		}
	}

	return ""
}

// Entries returns every entry of the file, whatever its section.
func (f *File) Entries() []Entry {
	var entries []Entry

	for _, section := range f.Sections {
		entries = append(entries, section.Entries...)
	}

	return entries
}

// Parse reads a localization file.
func Parse(input string) *File {
	file := &File{HasByteOrderMark: strings.HasPrefix(input, byteOrderMark)}
	input = strings.TrimPrefix(input, byteOrderMark)

	var current *Section

	for index, line := range strings.Split(input, "\n") {
		number := index + 1
		line = strings.TrimRight(line, "\r")

		parsed := parseLine(line, number)
		file.Warnings = append(file.Warnings, parsed.warnings...)

		switch {
		case parsed.header != "":
			file.Sections = append(file.Sections, Section{Language: parsed.header, Line: number})
			current = &file.Sections[len(file.Sections)-1]

		case parsed.entry != nil:
			if current == nil {
				file.Warnings = append(file.Warnings, Warning{
					Message: "key comes before any language header such as l_english:",
					Line:    number, Column: 1, Key: parsed.entry.Key,
				})

				file.Sections = append(file.Sections, Section{Line: number})
				current = &file.Sections[len(file.Sections)-1]
			}

			current.Entries = append(current.Entries, *parsed.entry)
		}
	}

	return file
}

// parsedLine is what one line holds: a header, an entry, or nothing.
type parsedLine struct {
	header   string
	entry    *Entry
	warnings []Warning
}

func parseLine(line string, number int) parsedLine {
	var result parsedLine

	warn := func(column int, key, format string, args ...any) {
		result.warnings = append(result.warnings, Warning{
			Message: fmt.Sprintf(format, args...),
			Line:    number,
			Column:  column + 1,
			Key:     key,
		})
	}

	start := skipSpace(line, 0)
	if start == len(line) || line[start] == '#' {
		return result
	}

	// The key runs up to the colon. Anything that cannot be in a key ends it
	// early, which is how a missing colon is noticed.
	end := start
	for end < len(line) && !isKeyEnd(line[end]) {
		end++
	}

	key := line[start:end]
	if key == "" {
		warn(start, "", "ignored a line without a key")

		return result
	}

	position := end

	if position < len(line) && line[position] == ':' {
		position++

		digits := position
		for position < len(line) && line[position] >= '0' && line[position] <= '9' {
			position++
		}

		entry := &Entry{Key: key, Line: number}

		if position > digits {
			version, err := strconv.Atoi(line[digits:position])
			if err == nil {
				entry.Version, entry.HasVersion = version, true
			}
		}

		position = skipSpace(line, position)

		switch {
		case position == len(line) || line[position] == '#':
			// Nothing after the colon: a language header, or a key with no
			// text.
			if !entry.HasVersion && strings.HasPrefix(key, "l_") && len(key) > 2 {
				result.header = key[2:]

				return result
			}

			warn(end, key, "%s has no text", key)

			return result

		case line[position] != '"':
			quote := strings.IndexByte(line[position:], '"')
			if quote < 0 {
				warn(position, key, "%s has no text in quotes; ignored", key)

				return result
			}

			warn(position, key, "ignored %q before the text of %s", line[position:position+quote], key)
			position += quote
		}

		entry.Text = readText(line, position, key, warn)
		result.entry = entry

		return result
	}

	// No colon. A key followed by text is still readable.
	quote := skipSpace(line, position)
	if quote < len(line) && line[quote] == '"' {
		warn(end, key, "missing : after %s", key)

		entry := &Entry{Key: key, Line: number}
		entry.Text = readText(line, quote, key, warn)
		result.entry = entry

		return result
	}

	// What was read as a key is only the first word of the line, often of a
	// text broken over two lines by accident, so the warning names no key.
	warn(start, "", "ignored a line that is not a key and a text: %q", shorten(strings.TrimSpace(line)))

	return result
}

// shortened is how much of a line a warning quotes.
const shortened = 80

// shorten cuts a line down to what a warning quotes, at a character boundary.
func shorten(line string) string {
	if len(line) <= shortened {
		return line
	}

	end := shortened
	for end > 0 && !utf8.RuneStart(line[end]) {
		end--
	}

	return line[:end] + "..."
}

// readText reads the text that starts with the quote at open.
//
// The text ends at the last quote on the line, unless a comment follows it: a
// # after whitespace after a quote that is neither the opening quote nor
// escaped with a backslash. Then it ends at the quote before the comment,
// which keeps quotes in the comment out of the text, as in
//
//	key: "#italic A line of an anthem#!" #– Anthem of Brazil"
//
// A # right after the opening quote, or after an escaped quote, is one of the
// game's formatting codes and part of the text.
func readText(line string, open int, key string, warn func(column int, key, format string, args ...any)) string {
	closing := -1

	for index := open + 1; index < len(line); index++ {
		switch line[index] {
		case '"':
			closing = index

		case '#':
			before := index - 1
			for before > open && isSpace(line[before]) {
				before--
			}

			if before < index-1 && before > open && line[before] == '"' && !escaped(line, before) {
				closing = before
				index = len(line)
			}
		}
	}

	if closing < 0 {
		warn(open, key, "text of %s is never closed with a quote, read to the end of the line", key)

		return strings.TrimRight(line[open+1:], " \t")
	}

	tail := strings.TrimLeft(line[closing+1:], " \t")
	if tail != "" && tail[0] != '#' {
		warn(closing+1, key, "ignored %q after the text of %s", tail, key)
	}

	return line[open+1 : closing]
}

// escaped reports whether the character at index is escaped by an odd number
// of backslashes before it.
func escaped(line string, index int) bool {
	count := 0
	for before := index - 1; before >= 0 && line[before] == '\\'; before-- {
		count++
	}

	return count%2 == 1
}

func skipSpace(line string, position int) int {
	for position < len(line) && isSpace(line[position]) {
		position++
	}

	return position
}

func isSpace(char byte) bool {
	return char == ' ' || char == '\t'
}

// isKeyEnd reports whether a character cannot be part of a key.
func isKeyEnd(char byte) bool {
	return char == ':' || char == '"' || char == '#' || isSpace(char)
}
