// Package script parses the script language of Paradox games: the format of
// the files in common/, history/, events/ and most other folders of Victoria
// 3, Crusader Kings 3, Europa Universalis, Hearts of Iron and Stellaris.
//
// A document is a sequence of fields, each a key, an operator and a value:
//
//	steel = {
//		cost = 50
//		category = industrial
//		texture = "gfx/interface/icons/goods_icons/steel.dds"
//		possible = { level >= 5 }
//		color = hsv { 0.1 0.2 0.3 }
//	}
//
// A value is a string, quoted or not, a number, a list of values in braces, a
// block of further fields in braces, or a tagged value such as hsv { ... }.
// Keys repeat, and the operator is "=" in data but may be a comparison in
// triggers: ==, !=, <, <=, >, >= and ?=. Variables are defined with
// "@name = 0.25", used as "@name" and computed with "@[ name * 2 ]"; the
// parser resolves them, so the tree holds plain values. A variable can hold
// any value, though only numbers can be computed with.
//
// Parse reads a whole file into a Document:
//
//	document := script.Parse(text)
//	steel, _ := document.Get("steel")
//	cost, _ := steel.Get("cost")
//	fmt.Println(cost.Number) // 50
//
// # Robustness
//
// Parse never fails and never panics. Game files and mod files alike contain
// typos the games read past, and a tool that refuses a whole file over one
// stray brace loses every definition in it. So anything that does not fit the
// grammar is skipped the way the games skip it and listed in the document's
// Warnings, with its line and column:
//
//   - a closing brace that closes nothing is dropped;
//   - a block left open at the end of the file is closed there;
//   - a key without a value is dropped, without taking the next line's key
//     as its value;
//   - a string whose closing quote is missing ends at its line, unless its
//     opening quote ends the line, which starts a string over several lines;
//   - an undefined variable is kept as the text it was written as;
//   - characters that belong to no token are skipped;
//   - braces nested deeper than 1000 levels are skipped, which keeps a corrupt
//     file from exhausting the stack.
//
// The shipped files of Victoria 3's economy parse without a single warning,
// and so do the database folders and events of Europa Universalis 5 and
// Crusader Kings 3 but for their real typos. Their additions are read as they
// are written: single-quoted strings, commas between values, an operator as a
// value (OPERATOR = <=), negated parameters (-$AMOUNT$), and a keyword before a
// definition at the top of a file (scripted_trigger is_ready = { ... }), which
// is kept in the Field's Keyword.
// Dates such as 1836.1.1, C-style float literals such as 0.3f, define
// references such as define:NPops|GRACEFUL_AGING_START, modification prefixes
// such as INJECT:pm_bakery, and values without keys among the fields of a
// block, as in on_actions, are all read as the games write them.
//
// # Editing
//
// Every Field records the byte offsets of its first and last character in the
// input, so a tool can replace one definition in a file without touching the
// formatting and comments around it.
//
// The package knows nothing about any particular game; the folders, database
// and victoria3 packages build on it.
package script

import "fmt"

// Kind is the sort of value a Node holds.
type Kind uint8

const (
	// KindString is a quoted string or a bare identifier. Quoted records which
	// of the two it was.
	KindString Kind = iota + 1

	// KindNumber is a number. Everything is kept as a float64 because the
	// script does not distinguish integers from decimals.
	KindNumber

	// KindList is a brace enclosed sequence of values, such as { 0.5 0.5 }.
	KindList

	// KindBlock is a brace enclosed sequence of fields.
	KindBlock

	// KindTagged is a value preceded by a tag, such as rgb { 1 0 0 }.
	KindTagged
)

func (k Kind) String() string {
	switch k {
	case KindString:
		return "string"
	case KindNumber:
		return "number"
	case KindList:
		return "list"
	case KindBlock:
		return "block"
	case KindTagged:
		return "tagged value"
	}

	return "nothing"
}

// Node is one value in a document. The zero Node is "no value", which is what
// lookups return for a missing field.
type Node struct {
	Kind Kind

	// Text holds the contents of a string and the original spelling of a
	// number.
	Text string

	// Quoted reports whether a string was written in quotes.
	Quoted bool

	// Number holds the value of a number.
	Number float64

	// Items holds the elements of a list. A block can have items too: the
	// games allow values without a key among the fields of a block, as in
	// on_actions = { coup_monthly_events delay = { days = 4 } }. Data files have
	// no use for them, so readers of data should treat items in a block as
	// something to report rather than something to read.
	Items []Node

	// Fields holds the fields of a block.
	Fields []Field

	// Tag and Value hold the two halves of a tagged value.
	Tag   string
	Value *Node

	// Line and Column are where the value started.
	Line   int
	Column int
}

// Operator is how a field relates its key to its value. Data fields always use
// "=", but triggers compare: "level >= 5", "has_variable ?= yes".
type Operator string

const (
	OpEquals       Operator = "="
	OpEqualsStrict Operator = "=="
	OpNotEquals    Operator = "!="
	OpLess         Operator = "<"
	OpLessEqual    Operator = "<="
	OpGreater      Operator = ">"
	OpGreaterEqual Operator = ">="
	OpSafeEquals   Operator = "?="
)

// Field is a key and the value assigned to it.
type Field struct {
	// Keyword is a word written before the key of a definition at the top of
	// a file, as in scripted_trigger is_ready = { ... }, which Europa
	// Universalis 5 and Crusader Kings 3 use to define triggers and effects
	// inside event files. It is empty for an ordinary field.
	Keyword string

	Key      string
	Operator Operator
	Value    Node
	Line     int
	Column   int

	// Start and End are byte offsets into the input the document was parsed
	// from: the first byte of the key and just past the last byte of the
	// value. They are what lets a definition be replaced in its file without
	// touching anything around it.
	Start, End int
}

// Document is a parsed file.
type Document struct {
	Fields []Field

	// Warnings lists what the parser had to overlook to get through the file.
	// Nothing here stops a document from being usable, but the interface can
	// show it so that a mod author can see a typo in their own file.
	Warnings []Warning

	// comments maps a line number to the comment written on it, without the
	// leading #. The games ignore comments, but some carry information that
	// exists nowhere else, such as the dates of the eras.
	comments map[int]string
}

// Warning is something the parser skipped or guessed at rather than failed
// over.
type Warning struct {
	Message string
	Line    int
	Column  int
}

func (w Warning) String() string {
	return fmt.Sprintf("line %d column %d: %s", w.Line, w.Column, w.Message)
}

// Comment returns the comment written on a line, if there is one.
func (d *Document) Comment(line int) (string, bool) {
	text, ok := d.comments[line]

	return text, ok
}

// Get returns the value of the last field with the given key.
//
// The last one wins because that is how the games read their own files: a key
// repeated in the same block overrides the earlier value. Keys that are meant
// to repeat are read with All instead.
func (n Node) Get(key string) (Node, bool) {
	return lastField(n.Fields, key)
}

// All returns the values of every field with the given key, in order.
func (n Node) All(key string) []Node {
	return allFields(n.Fields, key)
}

// Get returns the value of the last top level field with the given key.
func (d *Document) Get(key string) (Node, bool) {
	return lastField(d.Fields, key)
}

// All returns the values of every top level field with the given key.
func (d *Document) All(key string) []Node {
	return allFields(d.Fields, key)
}

func lastField(fields []Field, key string) (Node, bool) {
	for index := len(fields) - 1; index >= 0; index-- {
		if fields[index].Key == key {
			return fields[index].Value, true
		}
	}

	return Node{}, false
}

func allFields(fields []Field, key string) []Node {
	var found []Node

	for _, field := range fields {
		if field.Key == key {
			found = append(found, field.Value)
		}
	}

	return found
}

// Exists reports whether the node holds a value at all.
func (n Node) Exists() bool {
	return n.Kind != 0
}

// Str returns the value of a string node.
func (n Node) Str() (string, bool) {
	if n.Kind != KindString {
		return "", false
	}

	return n.Text, true
}

// Num returns the value of a number node.
func (n Node) Num() (float64, bool) {
	if n.Kind != KindNumber {
		return 0, false
	}

	return n.Number, true
}

// Bool returns the value of a yes or no node.
func (n Node) Bool() (value, ok bool) {
	if n.Kind != KindString || n.Quoted {
		return false, false
	}

	switch n.Text {
	case "yes":
		return true, true
	case "no":
		return false, true
	}

	return false, false
}

// Numbers returns the values of a list that holds nothing but numbers.
func (n Node) Numbers() ([]float64, bool) {
	if n.Kind != KindList {
		return nil, false
	}

	numbers := make([]float64, 0, len(n.Items))
	for _, item := range n.Items {
		if item.Kind != KindNumber {
			return nil, false
		}

		numbers = append(numbers, item.Number)
	}

	return numbers, true
}

// Strings returns the values of a list that holds nothing but strings, such as
// a list of technology keys. An empty block counts as an empty list.
func (n Node) Strings() ([]string, bool) {
	if n.Kind == KindBlock && len(n.Fields) == 0 {
		return nil, true
	}

	if n.Kind != KindList {
		return nil, false
	}

	texts := make([]string, 0, len(n.Items))
	for _, item := range n.Items {
		if item.Kind != KindString {
			return nil, false
		}

		texts = append(texts, item.Text)
	}

	return texts, true
}

// IsBlock reports whether a value can be read as a block of fields. An empty
// pair of braces is as much an empty block as an empty list, and the file has
// no way of saying which it meant, so it counts as both.
func (n Node) IsBlock() bool {
	return n.Kind == KindBlock || n.Kind == KindList && len(n.Items) == 0
}

// Describe names a value for a message: its text when it has one, its kind
// otherwise.
func (n Node) Describe() string {
	switch n.Kind {
	case KindString:
		if n.Quoted {
			return fmt.Sprintf("%q", n.Text)
		}

		return n.Text
	case KindNumber:
		return n.Text
	case KindTagged:
		return n.Tag + " { ... }"
	}

	return "a " + n.Kind.String()
}
