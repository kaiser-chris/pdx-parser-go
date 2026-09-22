package database

import (
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// Reader reads the fields of one definition, or of one block inside it, into
// typed values. It is where the tolerance for changing file formats lives:
//
//   - A missing field gives the caller's default, or an unset Opt.
//   - A field of the wrong kind gives the same, plus a warning.
//   - A field the reader was never asked about is reported as unknown by
//     Finish, so new fields in a game update show up instead of vanishing,
//     and remains readable through Definition.Raw.
//   - A field that appears more than once, which an INJECT produces, is read
//     the way the game combines it: lists are joined, anything else is taken
//     from the last one.
//
// Every diagnostic lands in the file, line and column of the field it is
// about, even when that field was injected by a different mod than the one
// that made the definition.
type Reader struct {
	node      script.Node
	originOf  func(field int) Origin
	subject   string
	collector *report.Collector
	used      map[string]bool
}

// NewReader returns a reader for a definition. kind names the definition in
// diagnostics, such as "good". Decode makes one for each definition; a caller
// only needs this to read definitions without Decode.
func NewReader(definition *Definition, kind string, collector *report.Collector) *Reader {
	return &Reader{
		node:      definition.Raw,
		originOf:  definition.OriginOf,
		subject:   kind + " " + definition.Key,
		collector: collector,
		used:      map[string]bool{},
	}
}

// Child returns a reader for a block held by the field at index, such as one
// of several entry blocks. what names the block in diagnostics. Call Finish on
// it too, to have its unknown fields reported.
func (r *Reader) Child(index int, block script.Node, what string) *Reader {
	origin := r.originOf(index)

	return &Reader{
		node:      block,
		originOf:  func(int) Origin { return origin },
		subject:   r.subject + ", " + what,
		collector: r.collector,
		used:      map[string]bool{},
	}
}

// Subject names what the reader reads in diagnostics, such as "good steel".
func (r *Reader) Subject() string {
	return r.subject
}

// Fields are the fields of the block being read, in order.
func (r *Reader) Fields() []script.Field {
	return r.node.Fields
}

// Warn reports a problem with a value read from the field at index. The value
// places the diagnostic; a zero value places it at the field.
func (r *Reader) Warn(index int, value script.Node, format string, args ...any) {
	r.at(report.SeverityWarning, index, value, format, args...)
}

// Note reports something worth knowing about the field at index that is not a
// problem, at info severity.
func (r *Reader) Note(index int, value script.Node, format string, args ...any) {
	r.at(report.SeverityInfo, index, value, format, args...)
}

// NoteDefinition reports something worth knowing about the definition as a
// whole, at info severity.
func (r *Reader) NoteDefinition(format string, args ...any) {
	origin := r.originOf(0)
	r.collector.Addf(report.SeverityInfo, origin.Source, origin.Path, r.node.Line, r.node.Column, r.subject, format, args...)
}

func (r *Reader) at(severity report.Severity, index int, value script.Node, format string, args ...any) {
	line, column := value.Line, value.Column
	if line == 0 && index >= 0 && index < len(r.node.Fields) {
		line, column = r.node.Fields[index].Line, r.node.Fields[index].Column
	}

	origin := r.originOf(index)
	r.collector.Addf(severity, origin.Source, origin.Path, line, column, r.subject, format, args...)
}

// Missing reports a field that should be there and is not, and what follows
// from its absence, such as "its price is taken as 0".
func (r *Reader) Missing(key, consequence string) {
	origin := r.originOf(0)
	r.collector.Addf(report.SeverityWarning, origin.Source, origin.Path, r.node.Line, r.node.Column, r.subject,
		"has no %s; %s", key, consequence)
}

// Indices returns the positions of every field with the key, and marks the
// key as known.
func (r *Reader) Indices(key string) []int {
	r.used[key] = true

	var found []int

	for index, field := range r.node.Fields {
		if field.Key == key {
			found = append(found, index)
		}
	}

	return found
}

// Last returns the position of the last field with the key, and marks the key
// as known.
func (r *Reader) Last(key string) (int, bool) {
	found := r.Indices(key)
	if len(found) == 0 {
		return -1, false
	}

	return found[len(found)-1], true
}

// Has reports whether the key is present, without marking it as known.
func (r *Reader) Has(key string) bool {
	for _, field := range r.node.Fields {
		if field.Key == key {
			return true
		}
	}

	return false
}

// Known marks keys as understood without reading them: triggers, effects and
// presentation fields a decoder has no use for. They stay available through
// Definition.Raw, and Finish does not report them.
func (r *Reader) Known(keys ...string) {
	for _, key := range keys {
		r.used[key] = true
	}
}

// Number reads a number.
func (r *Reader) Number(key string) Opt[float64] {
	index, ok := r.Last(key)
	if !ok {
		return Opt[float64]{}
	}

	value := r.node.Fields[index].Value
	if number, ok := value.Num(); ok {
		return Some(number)
	}

	r.Warn(index, value, "%s should be a number, found %s; ignored", key, value.Describe())

	return Opt[float64]{}
}

// NumberOr reads a number, or returns fallback.
func (r *Reader) NumberOr(key string, fallback float64) float64 {
	return r.Number(key).Or(fallback)
}

// Flag reads yes or no.
func (r *Reader) Flag(key string) Opt[bool] {
	index, ok := r.Last(key)
	if !ok {
		return Opt[bool]{}
	}

	value := r.node.Fields[index].Value
	if flag, ok := value.Bool(); ok {
		return Some(flag)
	}

	r.Warn(index, value, "%s should be yes or no, found %s; ignored", key, value.Describe())

	return Opt[bool]{}
}

// FlagOr reads yes or no, or returns fallback.
func (r *Reader) FlagOr(key string, fallback bool) bool {
	return r.Flag(key).Or(fallback)
}

// Text reads a name or a string, quoted or not. It returns "" when the field
// is missing or is not a string.
func (r *Reader) Text(key string) string {
	index, ok := r.Last(key)
	if !ok {
		return ""
	}

	value := r.node.Fields[index].Value
	if text, ok := value.Str(); ok {
		return text
	}

	r.Warn(index, value, "%s should be a name or a string, found %s; ignored", key, value.Describe())

	return ""
}

// Names reads a list of keys. Every occurrence of the field counts, so an
// injection adds to the list rather than replacing it, and a single name
// written without braces counts as a list of one, which is how repeatable
// fields are written. Duplicates are dropped.
func (r *Reader) Names(key string) []string {
	var (
		names []string
		seen  = map[string]bool{}
	)

	add := func(name string) {
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}

	for _, index := range r.Indices(key) {
		value := r.node.Fields[index].Value

		switch {
		case value.Kind == script.KindString:
			add(value.Text)

		case value.Kind == script.KindList:
			for _, item := range value.Items {
				if item.Kind == script.KindString {
					add(item.Text)
				} else {
					r.Warn(index, item, "%s should list names, found %s; ignored", key, item.Describe())
				}
			}

		case value.IsBlock():
			if len(value.Fields) > 0 {
				r.Warn(index, value, "%s should list names, found a block; ignored", key)
			}

			for _, item := range value.Items {
				if item.Kind == script.KindString {
					add(item.Text)
				}
			}

		default:
			r.Warn(index, value, "%s should list names, found %s; ignored", key, value.Describe())
		}
	}

	return names
}

// Located is a value together with the position of the field that holds it.
type Located struct {
	Index int
	Value script.Node
}

// Blocks returns every occurrence of a field that holds a block. An empty
// pair of braces counts as an empty block.
func (r *Reader) Blocks(key string) []Located {
	var found []Located

	for _, index := range r.Indices(key) {
		value := r.node.Fields[index].Value
		if !value.IsBlock() {
			r.Warn(index, value, "%s should be a block, found %s; ignored", key, value.Describe())

			continue
		}

		found = append(found, Located{Index: index, Value: value})
	}

	return found
}

// Raw returns the last occurrence of a field as it was written, and marks it
// as known.
func (r *Reader) Raw(key string) script.Node {
	index, ok := r.Last(key)
	if !ok {
		return script.Node{}
	}

	return r.node.Fields[index].Value
}

// ScriptValue reads a field that holds a number, the name of a script value,
// or a block the game computes.
func (r *Reader) ScriptValue(key string) ScriptValue {
	index, ok := r.Last(key)
	if !ok {
		return ScriptValue{}
	}

	value := r.node.Fields[index].Value

	switch {
	case value.Kind == script.KindNumber:
		return ScriptValue{Number: value.Number, IsNumber: true}

	case value.Kind == script.KindString:
		return ScriptValue{Name: value.Text}

	case value.IsBlock():
		return ScriptValue{Script: value}
	}

	r.Warn(index, value, "%s should be a number, a script value or a block, found %s; ignored", key, value.Describe())

	return ScriptValue{}
}

// NumberMap reads a block of "key = number" fields, keeping their order.
// Every occurrence of the field contributes, and a key given twice keeps its
// last value.
func (r *Reader) NumberMap(key string) []Amount {
	var amounts []Amount

	positions := map[string]int{}

	for _, block := range r.Blocks(key) {
		for _, field := range block.Value.Fields {
			number, ok := field.Value.Num()
			if !ok {
				r.Warn(block.Index, field.Value, "%s.%s should be a number, found %s; ignored", key, field.Key, field.Value.Describe())

				continue
			}

			if position, seen := positions[field.Key]; seen {
				amounts[position].Amount = number

				continue
			}

			positions[field.Key] = len(amounts)
			amounts = append(amounts, Amount{Key: field.Key, Amount: number})
		}

		for _, item := range block.Value.Items {
			r.Warn(block.Index, item, "ignored %s in %s, which has no key", item.Describe(), key)
		}
	}

	return amounts
}

// Modifiers reads a flat block of modifiers, such as the modifier of a
// technology. Every occurrence of the field contributes.
func (r *Reader) Modifiers(key string) []Modifier {
	var modifiers []Modifier

	for _, block := range r.Blocks(key) {
		modifiers = append(modifiers, r.ModifierFields(block.Index, key, block.Value)...)
	}

	return modifiers
}

// ModifierFields reads the fields of a modifier block held by the field at
// index. key names the block in diagnostics.
func (r *Reader) ModifierFields(index int, key string, block script.Node) []Modifier {
	modifiers := make([]Modifier, 0, len(block.Fields))

	for _, field := range block.Fields {
		modifier := Modifier{Key: field.Key, Line: field.Line}

		if number, ok := field.Value.Num(); ok {
			modifier.Value = number
		} else if flag, ok := field.Value.Bool(); ok {
			if flag {
				modifier.Value = 1
			}
		} else if text, ok := field.Value.Str(); ok {
			modifier.Text = text
		} else {
			r.Warn(index, field.Value, "modifier %s in %s should be a value, found %s; ignored", field.Key, key, field.Value.Describe())

			continue
		}

		modifiers = append(modifiers, modifier)
	}

	for _, item := range block.Items {
		r.Warn(index, item, "ignored %s in %s, which has no key", item.Describe(), key)
	}

	return modifiers
}

// Finish reports every field the reader was never asked about, once per key,
// and any value written without a key.
func (r *Reader) Finish() {
	reported := map[string]bool{}

	for index, field := range r.node.Fields {
		if r.used[field.Key] || reported[field.Key] {
			continue
		}

		reported[field.Key] = true

		origin := r.originOf(index)
		r.collector.Addf(report.SeverityInfo, origin.Source, origin.Path, field.Line, field.Column, r.subject,
			"unknown field %s, kept but not interpreted", field.Key)
	}

	for _, item := range r.node.Items {
		r.Warn(-1, item, "ignored %s, which has no key", item.Describe())
	}
}
