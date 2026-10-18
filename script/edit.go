package script

import (
	"errors"
	"fmt"
	"strings"
)

// ErrKeyTaken is returned by PutDefinition when a definition is to take a key
// the file already defines another one under.
var ErrKeyTaken = errors.New("the file already defines something under that key")

// ErrUnreadable is returned by PutDefinition for a file the parser had to read
// past something in.
var ErrUnreadable = errors.New("the file could not be read as it is, so it is left alone")

// PutDefinition puts a definition into the contents of a script file and
// returns the new contents, for a tool that writes what it edits back into the
// files of a game or a mod.
//
// definition is the whole of it, from its key to its closing brace, written
// with "\n" between its lines; it takes the file's own line breaks. It takes
// the place of the top level block defined under replace, where it stands, so
// that everything around it, comments and variables included, stays as it
// was. A file that defines nothing under replace gets the definition at its
// end, a blank line apart from what comes before, and an empty file gets it
// alone, after the byte order mark the games' files start with.
//
// key is the key the definition is written under. When it is not replace, the
// definition renames the one it replaces, and a file that already defines
// something else under key is refused with ErrKeyTaken, since the games would
// read only one of the two. Where a key is defined more than once, the last
// definition is the one meant, as it is the one the games read.
//
// Parsing never fails, but a file it had to read past something in is refused
// with ErrUnreadable: a block left open by a missing brace runs to the end of
// the file, and replacing it would take everything after it along.
func PutDefinition(file, replace, key, definition string) (merged string, replaced bool, err error) {
	lineBreak := "\n"
	if strings.Contains(file, "\r\n") {
		lineBreak = "\r\n"
	}

	// A definition written with "\r\n" already does not get them doubled.
	written := strings.ReplaceAll(strings.ReplaceAll(definition, "\r\n", "\n"), "\n", lineBreak)

	if strings.TrimSpace(strings.TrimPrefix(file, byteOrderMark)) == "" {
		return byteOrderMark + written + lineBreak, false, nil
	}

	document := Parse(file)
	if len(document.Warnings) > 0 {
		return "", false, fmt.Errorf("%w: %s", ErrUnreadable, document.Warnings[0])
	}

	if key != replace && lastDefinition(document, key) >= 0 {
		return "", false, fmt.Errorf("%w: %s", ErrKeyTaken, key)
	}

	if index := lastDefinition(document, replace); index >= 0 {
		field := document.Fields[index]

		return file[:field.Start] + written + file[field.End:], true, nil
	}

	trimmed := strings.TrimRight(file, " \t\r\n")

	return trimmed + lineBreak + lineBreak + written + lineBreak, false, nil
}

// Defines reports whether the contents of a script file define a block at the
// top under a key.
func Defines(file, key string) bool {
	return lastDefinition(Parse(file), key) >= 0
}

// lastDefinition returns the index of the last top level block defined under
// a key, or minus one.
func lastDefinition(document *Document, key string) int {
	found := -1

	for index, field := range document.Fields {
		if field.Key == key && field.Value.IsBlock() {
			found = index
		}
	}

	return found
}
