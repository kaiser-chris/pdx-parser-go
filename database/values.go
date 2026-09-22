package database

import "github.com/kaiser-chris/pdx-parser-go/script"

// Opt is a value that a file may or may not set.
//
// Fields whose default a game documents are best decoded as plain values
// holding that default. Opt is for the rest: fields whose default is not
// known, and fields whose absence means something, such as a setting a
// definition inherits from its parent unless it sets its own.
type Opt[T any] struct {
	Value T
	Set   bool
}

// Some returns an Opt holding a value.
func Some[T any](value T) Opt[T] {
	return Opt[T]{Value: value, Set: true}
}

// Get returns the value and whether it was set.
func (o Opt[T]) Get() (T, bool) {
	return o.Value, o.Set
}

// Or returns the value when it was set and fallback otherwise.
func (o Opt[T]) Or(fallback T) T {
	if o.Set {
		return o.Value
	}

	return fallback
}

// Text turns a text field into an Opt that is set when the text is not empty.
func Text(value string) Opt[string] {
	return Opt[string]{Value: value, Set: value != ""}
}

// ScriptValue is a field that holds either a number or a script value: a
// named value defined in common/script_values, or a block the game computes
// while it runs, such as an AI weight.
type ScriptValue struct {
	// Number is the value when it is written as a plain number.
	Number float64

	// IsNumber reports whether it was.
	IsNumber bool

	// Name is the key of a named script value, when it refers to one.
	Name string

	// Script is the block, when the game computes it.
	Script script.Node
}

// Set reports whether the field was present at all.
func (v ScriptValue) Set() bool {
	return v.IsNumber || v.Name != "" || v.Script.Exists()
}

// Amount is a quantity of something named, such as a pop need in a buy
// package of Victoria 3.
type Amount struct {
	Key    string
	Amount float64
}

// Modifier is one entry of a modifier block, such as
// building_steel_mill_throughput_add = 0.1.
type Modifier struct {
	Key string

	// Value is the number the modifier adds or multiplies by. A yes or no
	// modifier holds 1 or 0.
	Value float64

	// Text holds the value as written when it is not a number, such as the
	// key of a law. Value is 0 then.
	Text string

	Line int
}
