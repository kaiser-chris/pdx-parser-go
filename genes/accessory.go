package genes

import (
	"strconv"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// AccessoryGene picks an accessory of the portrait, such as a hairstyle, by
// weighted chance.
//
//	hairstyles = {
//		inheritable = yes
//		european_hairstyles = {
//			index = 1
//			male = {
//				1 = male_hair_european_01
//				2 = male_hair_european_03
//			}
//			female = { 1 = female_hair_european_02 }
//			boy = male
//			girl = female
//		}
//	}
//
// The accessories are named as the portrait accessories of gfx/portraits
// name them. A DNA picks a template, european_hairstyles here, and its
// strength picks among the accessories by their weights.
type AccessoryGene struct {
	database.Definition

	// Inheritable says whether a child inherits the gene from its parents,
	// when the file says.
	Inheritable database.Opt[bool]

	// Group is the group of genes the gene belongs to, when the file says.
	Group string

	// Templates are what a DNA picks from, in the order written.
	Templates []AccessoryTemplate
}

// Template returns a template of the gene by name.
func (g *AccessoryGene) Template(name string) (*AccessoryTemplate, bool) {
	for index := range g.Templates {
		if g.Templates[index].Name == name {
			return &g.Templates[index], true
		}
	}

	return nil, false
}

// AccessoryTemplate is one of the choices of an accessory gene.
type AccessoryTemplate struct {
	Name string

	// Index is the number the template goes by in a DNA string.
	Index int

	// SetTags are the tags a character the template is picked for is given,
	// such as no_clothes, when the file says. They are written as one string,
	// which may name several, separated by commas.
	SetTags []string

	// Sexes is what the template picks from for each sex, by Sex.
	Sexes [len(sexKeys)]Accessories
}

// For returns what the template picks from for a sex.
func (t *AccessoryTemplate) For(sex Sex) *Accessories {
	if sex < Male || sex > Girl {
		return &Accessories{}
	}

	return &t.Sexes[sex]
}

// Accessories is what a template of an accessory gene picks from for one
// sex.
type Accessories struct {
	// SameAs is the sex this one is written as being the same as, after any
	// chain of them. What follows is then that sex's.
	SameAs string

	// Choices are the accessories, in the order written.
	Choices []Choice
}

// Choice is one accessory a template can pick.
type Choice struct {
	// Weight is the chance of the accessory against the others.
	Weight int

	// Accessory is the name of the portrait accessory. The shipped files
	// write empty for none.
	Accessory string
}

// Pick returns the accessory a strength from 0 to 1 picks: the weights are
// laid end to end, and the strength falls on one of them. That is how the
// engine is taken to pick, which the files do not say for certain. It reports
// false when there is nothing to pick from.
func (a *Accessories) Pick(strength float64) (Choice, bool) {
	total := 0

	for _, choice := range a.Choices {
		total += max(choice.Weight, 0)
	}

	if total == 0 {
		return Choice{}, false
	}

	point := min(max(strength, 0), 1) * float64(total)
	reached := 0

	for _, choice := range a.Choices {
		reached += max(choice.Weight, 0)
		if point < float64(reached) {
			return choice, true
		}
	}

	return a.Choices[len(a.Choices)-1], true
}

// The keys of an accessory gene.
const keyInheritable = "inheritable"

// decodeAccessoryGene reads one accessory gene.
func decodeAccessoryGene(r *database.Reader) *AccessoryGene {
	gene := &AccessoryGene{Inheritable: r.Flag(keyInheritable), Group: r.Text(keyGroup)}

	templates(r, accessoryGeneFields, func(index int, field script.Field) {
		child := r.Child(index, field.Value, "template "+field.Key)
		template := AccessoryTemplate{Name: field.Key, Index: integer(child, keyIndex, 0), SetTags: tags(child, keySetTags)}

		if !child.Has(keyIndex) {
			child.Missing(keyIndex, "it is taken as 0")
		}

		choices, sameAs := sexes(child, decodeAccessories)
		for sex := range choices {
			template.Sexes[sex] = choices[sex]
			template.Sexes[sex].SameAs = sameAs[sex]
		}

		child.Finish()
		gene.Templates = append(gene.Templates, template)
	})

	return gene
}

// decodeAccessories reads the weighted accessories of one sex, written as
// weight = accessory.
func decodeAccessories(r *database.Reader) Accessories {
	accessories := Accessories{}

	for index, field := range r.Fields() {
		weight, err := strconv.Atoi(field.Key)
		if err != nil {
			// Left for Finish to report as unknown.
			continue
		}

		r.Known(field.Key)

		// An accessory is a name, but a name of digits alone is read as a
		// number, whose spelling is the name.
		name, ok := field.Value.Str()
		if !ok && field.Value.Kind == script.KindNumber {
			name, ok = field.Value.Text, true
		}

		if !ok {
			r.Warn(index, field.Value, "the accessory of weight %d should be a name, found %s; left out", weight, field.Value.Describe())

			continue
		}

		accessories.Choices = append(accessories.Choices, Choice{Weight: weight, Accessory: name})
	}

	return accessories
}
