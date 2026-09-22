// Package database reads the definitions of a Paradox game's database folders,
// such as common/goods, the way the Jomini engine combines them.
//
// A database folder holds one kind of definition, each a key and a block:
//
//	steel = { cost = 50 category = industrial }
//
// Files are read in the order the folders package works out, and a later file
// can change an earlier definition by prefixing its key with a modification
// mode: INJECT:steel = { ... } adds to it, REPLACE:steel = { ... } swaps it
// out. A Store applies those modes and remembers where every part of every
// definition came from.
//
// The typed side is left to the caller, who writes one decode function per
// kind of definition using a Reader:
//
//	type Good struct {
//		database.Definition
//		Cost float64
//	}
//
//	store := database.Collect(set, folders.Folder{Path: "common/goods"}, "good", collector)
//	goods := database.Decode(store, collector, func(r *database.Reader, _ *database.Definition) *Good {
//		return &Good{Cost: r.NumberOr("cost", 0)}
//	})
//
// A Reader does not fail. A missing field gives a default, a field of the
// wrong kind gives the default and a warning, and a field nobody asked about
// is reported as unknown and stays readable through Definition.Raw. That is
// what keeps a decoder written against one version of a game useful against
// the next.
//
// The package also reads defines, the constants in common/defines, which
// combine key by key rather than by definition.
package database

import (
	"iter"
	"strings"

	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// Mode is how a definition combines with an existing one of the same key.
type Mode uint8

const (
	// ModeCreate is a plain key. It defines something new, or overrides an
	// earlier definition of the same key completely.
	ModeCreate Mode = iota

	// ModeReplace replaces an existing definition; the key must exist.
	ModeReplace

	// ModeTryReplace replaces an existing definition, and does nothing when
	// there is none.
	ModeTryReplace

	// ModeReplaceOrCreate replaces an existing definition or creates one.
	ModeReplaceOrCreate

	// ModeInject adds fields to an existing definition; the key must exist.
	ModeInject

	// ModeTryInject adds fields to an existing definition, and does nothing
	// when there is none.
	ModeTryInject

	// ModeInjectOrCreate adds fields to an existing definition or creates one.
	ModeInjectOrCreate
)

var modePrefixes = map[string]Mode{
	"REPLACE":           ModeReplace,
	"TRY_REPLACE":       ModeTryReplace,
	"REPLACE_OR_CREATE": ModeReplaceOrCreate,
	"INJECT":            ModeInject,
	"TRY_INJECT":        ModeTryInject,
	"INJECT_OR_CREATE":  ModeInjectOrCreate,
}

// String returns the prefix as written in a file, or CREATE for a plain key.
func (m Mode) String() string {
	for prefix, mode := range modePrefixes {
		if mode == m {
			return prefix
		}
	}

	return "CREATE"
}

// Injects reports whether the mode adds to a definition rather than replacing
// it.
func (m Mode) Injects() bool {
	return m == ModeInject || m == ModeTryInject || m == ModeInjectOrCreate
}

// SplitKey separates a modification prefix from the key it applies to:
// "INJECT:pmg_canning" is ModeInject and "pmg_canning". known is false for a
// prefix that looks like one, capitals before a colon, but is not a mode this
// package knows; the game may have added one since.
func SplitKey(raw string) (mode Mode, key string, known bool) {
	prefix, rest, found := strings.Cut(raw, ":")
	if !found || prefix == "" || rest == "" || strings.ToUpper(prefix) != prefix || strings.ToLower(prefix) == prefix {
		return ModeCreate, raw, true
	}

	mode, known = modePrefixes[prefix]
	if !known {
		return ModeCreate, rest, false
	}

	return mode, rest, true
}

// Origin is a place a definition, or part of one, was read from.
type Origin struct {
	// Source is the name of the game or mod folder.
	Source string

	// File is the file's path below the folder's root, with forward slashes.
	File string

	// Path is the file on disk.
	Path string

	// Line is where the definition starts in the file.
	Line int

	// Mode is how this part combined with what came before it.
	Mode Mode
}

// Definition is what every kind of definition has in common: its key, where
// it came from, and the script it was read from. Typed definitions embed it.
type Definition struct {
	Key string

	// Origins are the places the current definition was assembled from. The
	// first is the definition itself; any after it are injections into it.
	Origins []Origin

	// Overridden are earlier definitions of the same key that this one
	// replaced, in the order they were read.
	Overridden []Origin

	// Raw is the whole definition as script, injections included, so that
	// fields a decoder does not cover are still there to be read.
	Raw script.Node

	// Comment is the comment written on the line the definition starts on.
	// The game ignores it, but it sometimes carries the only human-readable
	// label there is, such as "#1836-1861" after an era of Victoria 3.
	Comment string

	// fieldOrigins holds, for each field of Raw, the index into Origins of the
	// part it came from.
	fieldOrigins []int
}

// Origin returns where the definition was made.
func (d *Definition) Origin() Origin {
	if len(d.Origins) == 0 {
		return Origin{}
	}

	return d.Origins[0]
}

// Field returns the value of a field of the raw script, the last one when the
// key repeats. It is the way to reach fields a decoder does not cover.
func (d *Definition) Field(key string) (script.Node, bool) {
	return d.Raw.Get(key)
}

// OriginOf returns where the field at an index of Raw.Fields came from: the
// definition itself, or the injection that added it.
func (d *Definition) OriginOf(field int) Origin {
	if field < 0 || field >= len(d.fieldOrigins) || d.fieldOrigins[field] >= len(d.Origins) {
		return d.Origin()
	}

	return d.Origins[d.fieldOrigins[field]]
}

func (d *Definition) replace(value script.Node, origin Origin) {
	d.Overridden = append(d.Overridden, d.Origins...)
	d.Origins = []Origin{origin}
	d.Raw = value
	d.fieldOrigins = make([]int, len(value.Fields))
}

// inject appends the fields of value to the definition. A Reader then takes a
// field that may appear once from its last occurrence, and joins the
// occurrences of a field that holds a list.
func (d *Definition) inject(value script.Node, origin Origin) {
	d.Origins = append(d.Origins, origin)
	index := len(d.Origins) - 1

	d.Raw.Fields = append(d.Raw.Fields, value.Fields...)
	d.Raw.Items = append(d.Raw.Items, value.Items...)

	for range value.Fields {
		d.fieldOrigins = append(d.fieldOrigins, index)
	}
}

// define sets the definition a typed definition was decoded from. Every typed
// definition embeds Definition, which makes this method theirs too.
func (d *Definition) define(definition Definition) {
	*d = definition
}

// Store collects the definitions of one kind as files are read, applying the
// modification modes.
type Store struct {
	// Kind names the definitions in diagnostics, such as "production method".
	Kind string

	keys        []string
	definitions map[string]*Definition
}

// NewStore returns an empty store for one kind of definition.
func NewStore(kind string) *Store {
	return &Store{Kind: kind, definitions: map[string]*Definition{}}
}

// Keys returns the keys of the definitions, in the order they were first
// defined.
func (s *Store) Keys() []string {
	return append([]string(nil), s.keys...)
}

// Get returns a definition.
func (s *Store) Get(key string) (*Definition, bool) {
	definition, ok := s.definitions[key]

	return definition, ok
}

// Subject names a definition of the store's kind in a diagnostic.
func (s *Store) Subject(key string) string {
	return s.Kind + " " + key
}

// AddDocument reads every top level field of a parsed file as a definition.
func (s *Store) AddDocument(document *script.Document, file folders.File, collector *report.Collector) {
	for _, field := range document.Fields {
		comment, _ := document.Comment(field.Line)
		s.Add(field, comment, file, collector)
	}
}

// Add reads one top level field of a file as a definition, applying its
// modification mode. comment is the comment on the field's line, if any.
func (s *Store) Add(field script.Field, comment string, file folders.File, collector *report.Collector) {
	mode, key, known := SplitKey(field.Key)

	origin := Origin{Source: file.Source, File: file.Relative, Path: file.Path, Line: field.Line, Mode: mode}

	note := func(severity report.Severity, format string, args ...any) {
		collector.Addf(severity, origin.Source, origin.Path, field.Line, field.Column, s.Subject(key), format, args...)
	}

	if !known {
		prefix, _, _ := strings.Cut(field.Key, ":")
		note(report.SeverityWarning, "unknown modification %s:, read as a plain definition", prefix)
	}

	if field.Operator != script.OpEquals {
		note(report.SeverityWarning, "definition uses %s instead of =", field.Operator)
	}

	// Most definitions are blocks, but not all: a script value can be a plain
	// number, and a scripted effect a list holding nothing but a parameter.
	// The store keeps them as they are; Decode, which reads fields, skips
	// them.
	value := field.Value

	if value.Kind == script.KindList && len(value.Items) == 0 {
		// An empty pair of braces: an empty block.
		value = script.Node{Kind: script.KindBlock, Line: value.Line, Column: value.Column}
	}

	existing, exists := s.definitions[key]

	switch {
	case !exists && (mode == ModeReplace || mode == ModeInject):
		note(report.SeverityError, "%s: has nothing to modify, there is no earlier definition", mode)

	case !exists && (mode == ModeTryReplace || mode == ModeTryInject):
		note(report.SeverityInfo, "%s: skipped, there is no earlier definition", mode)

	case !exists:
		definition := &Definition{Key: key, Comment: comment}
		definition.replace(value, origin)

		s.keys = append(s.keys, key)
		s.definitions[key] = definition

	case mode.Injects() && !existing.Raw.IsBlock():
		note(report.SeverityWarning, "%s: cannot inject into %s, which is not a block; replaced it instead", mode, existing.Raw.Describe())
		existing.replace(value, origin)

	case mode.Injects() && !value.IsBlock():
		note(report.SeverityWarning, "%s: cannot inject %s, which is not a block; replaced it instead", mode, value.Describe())
		existing.replace(value, origin)

	case mode.Injects():
		existing.inject(value, origin)

	default:
		if mode == ModeCreate {
			note(report.SeverityInfo, "overrides the definition in %s from %s", existing.Origin().File, existing.Origin().Source)
		}

		existing.replace(value, origin)
		existing.Comment = comment
	}
}

// Table holds decoded definitions of one kind, in the order the game reads
// them.
type Table[T any] struct {
	keys  []string
	items map[string]*T
}

// Get returns the definition with the given key.
func (t Table[T]) Get(key string) (*T, bool) {
	item, ok := t.items[key]

	return item, ok
}

// Has reports whether a definition with the given key exists.
func (t Table[T]) Has(key string) bool {
	_, ok := t.items[key]

	return ok
}

// Len is the number of definitions.
func (t Table[T]) Len() int {
	return len(t.keys)
}

// Keys returns the keys of all definitions, in order.
func (t Table[T]) Keys() []string {
	return append([]string(nil), t.keys...)
}

// All iterates over the definitions in order.
func (t Table[T]) All() iter.Seq2[string, *T] {
	return func(yield func(string, *T) bool) {
		for _, key := range t.keys {
			if !yield(key, t.items[key]) {
				return
			}
		}
	}
}

// Definable is a pointer to a typed definition, a struct that embeds
// Definition.
type Definable[T any] interface {
	*T
	define(Definition)
}

// Decode turns every definition in a store into its typed form, in order.
//
// decode reads the fields it knows through the Reader; Decode then fills in
// the embedded Definition and reports every field decode did not ask about.
func Decode[T any, P Definable[T]](store *Store, collector *report.Collector, decode func(r *Reader, definition *Definition) P) Table[T] {
	table := Table[T]{items: make(map[string]*T, len(store.keys))}

	for _, key := range store.keys {
		definition := store.definitions[key]

		if !definition.Raw.IsBlock() {
			origin := definition.Origin()
			collector.Addf(report.SeverityWarning, origin.Source, origin.Path, definition.Raw.Line, definition.Raw.Column, store.Subject(key),
				"ignored, because its value %s is not a block", definition.Raw.Describe())

			continue
		}

		table.keys = append(table.keys, key)

		reader := NewReader(definition, store.Kind, collector)

		item := decode(reader, definition)
		item.define(*definition)
		reader.Finish()

		table.items[key] = (*T)(item)
	}

	return table
}
