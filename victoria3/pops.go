package victoria3

import (
	"strconv"
	"strings"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// PopType is a profession, from common/pop_types.
type PopType struct {
	database.Definition

	Texture string

	// Color is the profession's colour as written, such as hsv{ 0.02 0.38 0.37 }.
	Color script.Node

	// WageWeight scales the wage of the profession relative to others.
	WageWeight float64

	// DependentWage is the income of each dependent per year.
	DependentWage float64

	StartQualityOfLife float64

	// PaidPrivateWage pops get wages in buildings the government does not
	// fund. Default no.
	PaidPrivateWage bool

	// Unemployment reports whether the pop can be unemployed.
	Unemployment bool

	// UnemploymentWealth is the wealth at or below which unemployed pops
	// switch to this profession.
	UnemploymentWealth database.Opt[float64]

	// WorkingAdultRatio is the share of the profession in the workforce.
	WorkingAdultRatio database.Opt[float64]

	LiteracyTarget  database.Opt[float64]
	EducationAccess database.Opt[float64]

	// ConsumptionMult scales how much the pop buys. Default 1.
	ConsumptionMult float64

	CanAlwaysHire                    bool
	IgnoresEmploymentProportionality bool
	PaysLandTax                      bool
	Military                         bool
	SubsistenceIncome                bool
	IsSlave                          bool
	SolChangeImpact                  database.Opt[float64]

	PoliticalEngagementBase           database.Opt[float64]
	PoliticalEngagementLiteracyFactor database.Opt[float64]
}

func decodePopType(r *database.Reader) *PopType {
	pop := &PopType{
		Texture:            r.Text("texture"),
		Color:              r.Raw("color"),
		WageWeight:         r.NumberOr("wage_weight", 0),
		DependentWage:      r.NumberOr("dependent_wage", 0),
		StartQualityOfLife: r.NumberOr("start_quality_of_life", 0),
		PaidPrivateWage:    r.FlagOr("paid_private_wage", false),
		Unemployment:       r.FlagOr("unemployment", false),
		UnemploymentWealth: r.Number("unemployment_wealth"),
		WorkingAdultRatio:  r.Number("working_adult_ratio"),
		LiteracyTarget:     r.Number("literacy_target"),
		EducationAccess:    r.Number("education_access"),
		ConsumptionMult:    r.NumberOr("consumption_mult", 1),

		CanAlwaysHire:                    r.FlagOr("can_always_hire", false),
		IgnoresEmploymentProportionality: r.FlagOr("ignores_employment_proportionality", false),
		PaysLandTax:                      r.FlagOr("pays_land_tax", false),
		Military:                         r.FlagOr("military", false),
		SubsistenceIncome:                r.FlagOr("subsistence_income", false),
		IsSlave:                          r.FlagOr("is_slave", false),
		SolChangeImpact:                  r.Number("sol_change_impact"),

		PoliticalEngagementBase:           r.Number("political_engagement_base"),
		PoliticalEngagementLiteracyFactor: r.Number("political_engagement_literacy_factor"),
	}

	if !r.Has("wage_weight") {
		r.Missing("wage_weight", "its wages are taken as 0")
	}

	r.Known(
		"political_engagement_mult", "qualifications", "qualifications_growth_desc",
		"portrait_age", "portrait_pose", "portrait_is_female",
	)

	return pop
}

// PopNeed is a need pops buy goods for, such as basic food, from
// common/pop_needs. Its entries are the goods that can meet it.
type PopNeed struct {
	database.Definition

	// Default is the good pops buy when nothing else is available.
	Default string

	Entries []PopNeedEntry

	ObsessionDemandMin          database.Opt[float64]
	ObsessionDemandMult         database.Opt[float64]
	PrestigeGoodsDemandIncrease database.Opt[float64]
}

// PopNeedEntry is one good that can meet a need.
type PopNeedEntry struct {
	Goods string

	// Weight is the base preference for the good, adjusted in the game by its
	// share of the market's sell orders.
	Weight database.Opt[float64]

	// MaxSupplyShare caps the share of supply that still raises the weight.
	MaxSupplyShare database.Opt[float64]

	// MinSupplyShare, above 0, keeps at least this fraction of the base
	// weight whatever the supply.
	MinSupplyShare database.Opt[float64]

	Line int
}

func decodePopNeed(r *database.Reader) *PopNeed {
	need := &PopNeed{
		Default:                     r.Text("default"),
		ObsessionDemandMin:          r.Number("obsession_demand_min"),
		ObsessionDemandMult:         r.Number("obsession_demand_mult"),
		PrestigeGoodsDemandIncrease: r.Number("prestige_goods_demand_increase"),
	}

	for _, block := range r.Blocks("entry") {
		entry := r.Child(block.Index, block.Value, "entry")

		decoded := PopNeedEntry{
			Goods:          entry.Text("goods"),
			Weight:         entry.Number("weight"),
			MaxSupplyShare: entry.Number("max_supply_share"),
			MinSupplyShare: entry.Number("min_supply_share"),
			Line:           block.Value.Line,
		}

		entry.Finish()

		if decoded.Goods == "" {
			entry.Missing("goods", "the entry is ignored")

			continue
		}

		need.Entries = append(need.Entries, decoded)
	}

	if len(need.Entries) == 0 {
		r.Missing("entry", "no good can meet this need")
	}

	return need
}

// BuyPackage is what pops of one wealth level spend on each need, from
// common/buy_packages.
type BuyPackage struct {
	database.Definition

	// Wealth is the wealth level, read from the key: wealth_12 is 12. It is 0
	// when the key does not follow that pattern.
	Wealth int

	PoliticalStrength float64

	// Needs are the pop needs and how much of the package goes to each.
	Needs []database.Amount
}

func decodeBuyPackage(r *database.Reader, key string) *BuyPackage {
	pack := &BuyPackage{
		PoliticalStrength: r.NumberOr("political_strength", 0),
		Needs:             r.NumberMap("goods"),
	}

	if level, ok := strings.CutPrefix(key, "wealth_"); ok {
		if wealth, err := strconv.Atoi(level); err == nil {
			pack.Wealth = wealth
		}
	}

	if pack.Wealth == 0 {
		r.NoteDefinition("key does not follow wealth_<level>, so its wealth level is unknown")
	}

	return pack
}
