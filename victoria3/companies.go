package victoria3

import (
	"github.com/kaiser-chris/pdx-parser-go/database"
)

// Company is a company type, from common/company_types.
type Company struct {
	database.Definition

	Icon       string
	Background string

	// Category groups companies, such as aristocrat_owned. Some production
	// methods are unlocked by the category of a company owning the building.
	Category string

	FlavoredCompany bool

	UsesDynamicNaming       bool
	DynamicCompanyTypeNames []string

	// BuildingTypes are the buildings the company runs.
	BuildingTypes []string

	// ExtensionBuildingTypes can be added through additional charters.
	ExtensionBuildingTypes []string

	PossiblePrestigeGoods []string
	PreferredHeadquarters []string
	UnlockingPrinciples   []string

	// ReplacesCompany is the company this one takes the place of.
	ReplacesCompany string

	// ProsperityModifier applies to the owning country while the company is
	// prosperous. Throughput modifiers here are part of the economy.
	ProsperityModifier []database.Modifier
}

func decodeCompany(r *database.Reader) *Company {
	company := &Company{
		Icon:                    r.Text("icon"),
		Background:              r.Text("background"),
		Category:                r.Text("category"),
		FlavoredCompany:         r.FlagOr("flavored_company", false),
		UsesDynamicNaming:       r.FlagOr("uses_dynamic_naming", false),
		DynamicCompanyTypeNames: r.Names("dynamic_company_type_names"),
		BuildingTypes:           r.Names("building_types"),
		ExtensionBuildingTypes:  r.Names("extension_building_types"),
		PossiblePrestigeGoods:   r.Names("possible_prestige_goods"),
		PreferredHeadquarters:   r.Names("preferred_headquarters"),
		UnlockingPrinciples:     r.Names("unlocking_principles"),
		ReplacesCompany:         r.Text("replaces_company"),
		ProsperityModifier:      r.Modifiers("prosperity_modifier"),
	}

	if len(company.BuildingTypes) == 0 {
		r.Missing("building_types", "it runs no buildings")
	}

	r.Known(
		"potential", "attainable", "possible", "can_establish_in", "prestige_goods_trigger",
		"ai_will_do", "ai_weight", "ai_construction_targets",
	)

	return company
}
