package genes

import (
	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// Sex is who a part of a template is for: a man, a woman, or a child of
// either.
type Sex int

const (
	Male Sex = iota
	Female
	Boy
	Girl
)

// sexKeys are the keys the sexes are written under, in the order of Sex.
var sexKeys = [...]string{"male", "female", "boy", "girl"}

// String returns the key the sex is written under.
func (s Sex) String() string {
	if s < Male || s > Girl {
		return "unknown"
	}

	return sexKeys[s]
}

// Sexes are all four, in order.
var Sexes = [...]Sex{Male, Female, Boy, Girl}

// sexOf returns the sex a key names.
func sexOf(key string) (Sex, bool) {
	for sex, name := range sexKeys {
		if name == key {
			return Sex(sex), true
		}
	}

	return 0, false
}

// sexes reads what a template does for each sex. A sex is a block, which
// decode reads, or the name of another sex it is the same as:
//
//	male = { ... }
//	boy = male
//
// A sex that is the same as another gets what decode read for that one,
// sharing its slices, and sameAs says which, following a chain of them, such as a girl the
// same as a woman who is the same as a man, to its end. A sex that is the
// same as one that is not written, or as itself, is reported and left empty.
func sexes[T any](r *database.Reader, decode func(*database.Reader) T) (values [len(sexKeys)]T, sameAs [len(sexKeys)]string) {
	var (
		written [len(sexKeys)]bool
		aliases [len(sexKeys)]script.Node
		places  [len(sexKeys)]int
	)

	for sex, key := range sexKeys {
		index, ok := r.Last(key)
		if !ok {
			continue
		}

		value := r.Fields()[index].Value
		places[sex] = index

		if name, ok := value.Str(); ok {
			aliases[sex] = value
			sameAs[sex] = name

			continue
		}

		if !value.IsBlock() {
			r.Warn(index, value, "%s should be a block or the name of another sex, found %s; ignored", key, value.Describe())

			continue
		}

		child := r.Child(index, value, key)
		values[sex] = decode(child)
		child.Finish()
		written[sex] = true
	}

	for sex := range sexKeys {
		if sameAs[sex] == "" {
			continue
		}

		// A chain of sexes the same as one another ends at a sex written as a
		// block. It can be no longer than there are sexes without going round
		// in a loop.
		target, ok := sexOf(sameAs[sex])

		for step := 0; ok && !written[target] && sameAs[target] != "" && step < len(sexKeys); step++ {
			target, ok = sexOf(sameAs[target])
		}

		switch {
		case !ok:
			r.Warn(places[sex], aliases[sex], "%s is the same as %s, which is no sex; left empty", sexKeys[sex], aliases[sex].Describe())
			sameAs[sex] = ""

		case !written[target]:
			r.Warn(places[sex], aliases[sex], "%s is the same as %s, which is not given; left empty", sexKeys[sex], aliases[sex].Describe())
			sameAs[sex] = ""

		default:
			values[sex] = values[target]
			sameAs[sex] = sexKeys[target]
		}
	}

	return values, sameAs
}
