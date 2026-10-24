package genes

import (
	"math"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// Point is one point of a curve: at X, the curve is Y.
type Point struct {
	X, Y float64
}

// Curve is a function given by points, which is linear between them and
// flat beyond the first and the last:
//
//	curve = { { 0.0 0.0 } { 0.5 0.0 } { 1.0 1.0 } }
//
// The points are kept in the order written, which is not always the order of
// X: Victoria 3 writes the age curve of its greying hair out of order.
type Curve []Point

// At returns the value of the curve at x. A curve without points is 0
// everywhere.
//
// The points are walked in the order written: the value is that of the line
// from the point before the first point at or past x to that point, the
// first point's before it and the last point's past them all. For points in
// order of X that is the curve drawn through them, and two points at one X
// make a step. Victoria 3's greying hair is written { -1 -1 } { 0.7 0 }
// { 0.6 1.0 }, which taken in order of X would grey the young and not the
// old; walked as written it greys from 0.7 on, as its other ageing colours
// do, which is taken to be how the engine reads it.
func (c Curve) At(x float64) float64 {
	if len(c) == 0 {
		return 0
	}

	if x <= c[0].X {
		return c[0].Y
	}

	for index := 1; index < len(c); index++ {
		from, to := c[index-1], c[index]
		if x > to.X {
			continue
		}

		if to.X <= from.X {
			return to.Y
		}

		return from.Y + (to.Y-from.Y)*(x-from.X)/(to.X-from.X)
	}

	return c[len(c)-1].Y
}

// Range is the values a strength from 0 to 1 is spread over, linearly:
//
//	value = { min = -0.5 max = 0.5 }
type Range struct {
	Min, Max float64
}

// At returns the value of the range at a strength from 0 to 1.
func (r Range) At(strength float64) float64 {
	return r.Min + (r.Max-r.Min)*strength
}

// Age is how a gene changes with the age of a character: by the curve of an
// age preset it names, or by a curve of its own.
//
//	age = age_preset_child_features
//	age = { mode = multiply curve = { { 0.0 0.0 } { 0.25 1.0 } } }
type Age struct {
	// Preset is the age preset named, or empty for an age given in place.
	Preset string

	// Mode and Curve are the age given in place, as an age preset gives
	// them.
	Mode  string
	Curve Curve
}

// The modes an age combines with the value of a gene by.
const (
	// ModeMultiply multiplies the value by the curve at the age.
	ModeMultiply = "multiply"

	// ModeAdd adds the curve at the age to the value.
	ModeAdd = "add"
)

// The keys used across the kinds of gene.
const (
	keyIndex = "index"
	keyMode  = "mode"
	keyCurve = "curve"
	keyAge   = "age"
)

// integer reads a number that counts something, such as an index. A fraction
// is read rounded down, with a warning.
func integer(r *database.Reader, key string, fallback int) int {
	index, ok := r.Last(key)
	if !ok {
		return fallback
	}

	value := r.Fields()[index].Value

	number, ok := value.Num()
	if !ok || math.IsInf(number, 0) || math.IsNaN(number) || math.Abs(number) > math.MaxInt32 {
		r.Warn(index, value, "%s should be a whole number, found %s; ignored", key, value.Describe())

		return fallback
	}

	if number != math.Trunc(number) {
		r.Warn(index, value, "%s should be a whole number, found %s; read as %d", key, value.Describe(), int(number))
	}

	return int(number)
}

// curve reads a curve. A point that is not two numbers is left out, with a
// warning.
func curve(r *database.Reader, key string) Curve {
	index, ok := r.Last(key)
	if !ok {
		return nil
	}

	value := r.Fields()[index].Value

	if value.IsBlock() && len(value.Fields) == 0 {
		return nil
	}

	if value.Kind != script.KindList {
		r.Warn(index, value, "%s should be a list of points, found %s; ignored", key, value.Describe())

		return nil
	}

	points := make(Curve, 0, len(value.Items))

	for _, item := range value.Items {
		numbers, ok := item.Numbers()
		if !ok || len(numbers) != 2 {
			r.Warn(index, item, "a point of %s should be two numbers, found %s; left out", key, item.Describe())

			continue
		}

		points = append(points, Point{X: numbers[0], Y: numbers[1]})
	}

	return points
}

// age reads an age, which names a preset or gives a curve of its own.
func age(r *database.Reader, key string) Age {
	index, ok := r.Last(key)
	if !ok {
		return Age{}
	}

	value := r.Fields()[index].Value

	if name, ok := value.Str(); ok {
		return Age{Preset: name}
	}

	if !value.IsBlock() {
		r.Warn(index, value, "%s should name an age preset or be a block, found %s; ignored", key, value.Describe())

		return Age{}
	}

	child := r.Child(index, value, key)
	defer child.Finish()

	return Age{Mode: mode(child), Curve: curve(child, keyCurve)}
}

// mode reads how an age combines with a value.
func mode(r *database.Reader) string {
	mode := r.Text(keyMode)

	switch mode {
	case "", ModeMultiply, ModeAdd:
	default:
		index, _ := r.Last(keyMode)
		r.Note(index, script.Node{}, "mode %s is not one this package knows of, %s or %s; kept as written", mode, ModeMultiply, ModeAdd)
	}

	return mode
}
