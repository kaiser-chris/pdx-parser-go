package asset

import (
	"math"

	"github.com/kaiser-chris/pdx-parser-go/database"
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

// vector reads three numbers, such as a position. A list of the wrong length
// is read as far as it goes, with a warning, and the missing values are 0.
func vector(r *database.Reader, key string) Vec3 {
	index, ok := r.Last(key)
	if !ok {
		return Vec3{}
	}

	value := r.Fields()[index].Value

	numbers, ok := value.Numbers()
	if !ok {
		r.Warn(index, value, "%s should be three numbers, found %s; ignored", key, value.Describe())

		return Vec3{}
	}

	if len(numbers) != 3 {
		r.Warn(index, value, "%s should be three numbers, found %d", key, len(numbers))
	}

	var components [3]float64
	copy(components[:], numbers)

	return Vec3{X: components[0], Y: components[1], Z: components[2]}
}
