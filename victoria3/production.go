package victoria3

import (
	"strings"

	"github.com/kaiser-chris/pdx-parser-go/database"
)

// ProductionMethodGroup is a set of production methods a building picks one
// of, from common/production_method_groups.
type ProductionMethodGroup struct {
	database.Definition

	Texture string

	// ProductionMethods are the choices, in order. The first is the default
	// unless one of them says is_default.
	ProductionMethods []string

	// AISelection is how the AI picks: most_profitable, the default, or
	// most_productive.
	AISelection string

	IsHiddenWhenUnavailable bool
}

func decodeProductionMethodGroup(r *database.Reader) *ProductionMethodGroup {
	group := &ProductionMethodGroup{
		Texture:                 r.Text("texture"),
		ProductionMethods:       r.Names("production_methods"),
		AISelection:             r.Text("ai_selection"),
		IsHiddenWhenUnavailable: r.FlagOr("is_hidden_when_unavailable", false),
	}

	if group.AISelection == "" {
		group.AISelection = "most_profitable"
	}

	if len(group.ProductionMethods) == 0 {
		r.Missing("production_methods", "a building using it has nothing to choose")
	}

	return group
}

// ProductionMethod is one way a building can run, from
// common/production_methods. Its modifiers hold what the economy cares about:
// goods in and out, and jobs.
type ProductionMethod struct {
	database.Definition

	Texture string

	// IsDefault makes this the method its group starts on.
	IsDefault bool

	// BuildingModifiers apply to the building. Goods and jobs are here.
	BuildingModifiers ScaledModifiers

	// StateModifiers and CountryModifiers apply to the building's state and
	// country while the method is active.
	StateModifiers   ScaledModifiers
	CountryModifiers ScaledModifiers

	// TimedModifiers apply for a while after switching to the method.
	TimedModifiers []string

	// RequiredInputGoods must be available to the building to use the method.
	RequiredInputGoods []string

	// Everything that has to be true to use the method. All the unlocking
	// lists must be met; none of the disallowing ones may be.
	UnlockingTechnologies       []string
	UnlockingGlobalTechnologies []string
	UnlockingProductionMethods  []string
	UnlockingLaws               []string
	DisallowingLaws             []string
	UnlockingPrinciples         []string
	UnlockingIdentities         []string
	UnlockingReligions          []string
	DisallowingReligions        []string
	UnlockingGeographicRegions  []string
	UnlockingCompanyCategories  []string

	// ReplacementIfValid is the method this one switches to on its own once
	// that method can be used.
	ReplacementIfValid string

	// ProfessionRatio splits the recruits of military buildings between pop
	// types.
	ProfessionRatio []database.Amount

	IsHiddenWhenUnavailable bool
	LowPopMethod            bool
	PollutionGeneration     float64
	AIValue                 database.Opt[float64]
	AIWeight                database.Opt[float64]
}

func decodeProductionMethod(r *database.Reader) *ProductionMethod {
	method := &ProductionMethod{
		Texture:           r.Text("texture"),
		IsDefault:         r.FlagOr("is_default", false),
		BuildingModifiers: scaledModifiers(r, "building_modifiers"),
		StateModifiers:    scaledModifiers(r, "state_modifiers"),
		CountryModifiers:  scaledModifiers(r, "country_modifiers"),
		TimedModifiers:    r.Names("timed_modifiers"),

		RequiredInputGoods: r.Names("required_input_goods"),

		UnlockingTechnologies:       r.Names("unlocking_technologies"),
		UnlockingGlobalTechnologies: r.Names("unlocking_global_technologies"),
		UnlockingProductionMethods:  r.Names("unlocking_production_methods"),
		UnlockingLaws:               r.Names("unlocking_laws"),
		DisallowingLaws:             r.Names("disallowing_laws"),
		UnlockingPrinciples:         r.Names("unlocking_principles"),
		UnlockingIdentities:         r.Names("unlocking_identity"),
		UnlockingReligions:          r.Names("unlocking_religions"),
		DisallowingReligions:        r.Names("disallowing_religions"),
		UnlockingGeographicRegions:  r.Names("unlocking_geographic_regions"),
		UnlockingCompanyCategories:  r.Names("unlocking_company_categories"),

		ReplacementIfValid: r.Text("replacement_if_valid"),
		ProfessionRatio:    r.NumberMap("profession_ratio"),

		IsHiddenWhenUnavailable: r.FlagOr("is_hidden_when_unavailable", false),
		LowPopMethod:            r.FlagOr("low_pop_method", false),
		PollutionGeneration:     r.NumberOr("pollution_generation", 0),
		AIValue:                 r.Number("ai_value"),
		AIWeight:                r.Number("ai_weight"),
	}

	r.Known("city_lights_color_index")

	return method
}

// Flow is a quantity of a good or a pop type that a modifier names, such as
// the 40 grain of goods_input_grain_add = 40.
type Flow struct {
	// Key is the good or pop type.
	Key    string
	Amount float64

	// Scaling is how the amount grows with the building.
	Scaling Scaling
}

// Inputs are the goods the method consumes, from goods_input_<good>_add
// modifiers on the building.
func (m *ProductionMethod) Inputs() []Flow {
	return flows(m.BuildingModifiers, "goods_input_", "_add")
}

// Outputs are the goods the method produces, from goods_output_<good>_add
// modifiers on the building.
func (m *ProductionMethod) Outputs() []Flow {
	return flows(m.BuildingModifiers, "goods_output_", "_add")
}

// Employment is the jobs the method provides, from
// building_employment_<pop type>_add modifiers.
func (m *ProductionMethod) Employment() []Flow {
	return flows(m.BuildingModifiers, "building_employment_", "_add")
}

// flows picks the modifiers whose key is prefix, a name, then suffix. A name
// that appears more than once with the same scaling is summed, the way the
// game sums modifiers.
func flows(modifiers ScaledModifiers, prefix, suffix string) []Flow {
	type slot struct {
		name    string
		scaling Scaling
	}

	var found []Flow

	positions := map[slot]int{}

	for _, modifier := range modifiers.All() {
		name, ok := strings.CutPrefix(modifier.Key, prefix)
		if !ok {
			continue
		}

		name, ok = strings.CutSuffix(name, suffix)
		if !ok || name == "" || modifier.Text != "" {
			continue
		}

		key := slot{name: name, scaling: modifier.Scaling}
		if position, seen := positions[key]; seen {
			found[position].Amount += modifier.Value

			continue
		}

		positions[key] = len(found)
		found = append(found, Flow{Key: name, Amount: modifier.Value, Scaling: modifier.Scaling})
	}

	return found
}
