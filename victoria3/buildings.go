package victoria3

import (
	"github.com/kaiser-chris/pdx-parser-go/database"
)

// BuildingGroup is a category of buildings with shared rules, from
// common/building_groups. Groups form a tree through Parent, and most settings
// are inherited from the nearest ancestor that sets them, which is why they
// are Opt: use ResolveGroup to read them the way the game applies them.
type BuildingGroup struct {
	database.Definition

	// Parent is the group this one belongs to, empty for a root group.
	Parent string

	// Category is urban, rural or development, and is usually set on root
	// groups only.
	Category string

	// DefaultBuilding is built unless a state names a different building of
	// the group.
	DefaultBuilding string

	// Lens is where buildings of the group are sorted in the interface.
	Lens string

	// LandUsage is urban or rural: which state resource the buildings use.
	LandUsage string

	AlwaysPossible database.Opt[bool]

	// EconomyOfScale gives non-subsistence buildings in the group a
	// throughput bonus for every level above one.
	EconomyOfScale database.Opt[bool]

	IsSubsistence        database.Opt[bool]
	IsGovernmentFunded   database.Opt[bool]
	IsMilitary           database.Opt[bool]
	CappedByResources    database.Opt[bool]
	DiscoverableResource database.Opt[bool]
	DepletableResource   database.Opt[bool]
	AutoPlaceBuildings   database.Opt[bool]
	CanUseSlaves         database.Opt[bool]
	PaysTaxes            database.Opt[bool]
	Subsidized           database.Opt[bool]
	HiresUnemployedOnly  database.Opt[bool]
	HasTradeRevenue      database.Opt[bool]
	OwnsOtherBuildings   database.Opt[bool]
	AlwaysSelfOwning     database.Opt[bool]

	CompanyHeadquarter         database.Opt[bool]
	RegionalCompanyHeadquarter database.Opt[bool]

	// CashReservesMax is the most money per level a building can keep.
	CashReservesMax database.Opt[float64]

	// Urbanization is added to the state per level.
	Urbanization database.Opt[float64]

	InfrastructureUsagePerLevel database.Opt[float64]

	MinHiringRate         database.Opt[float64]
	MaxHiringRate         database.Opt[float64]
	ProportionalityLimit  database.Opt[float64]
	MinProductivityToHire database.Opt[float64]

	EconomyOfScaleAIFactor    database.Opt[float64]
	ForeignInvestmentAIFactor database.Opt[float64]
}

func decodeBuildingGroup(r *database.Reader) *BuildingGroup {
	group := &BuildingGroup{
		Parent:          r.Text("parent_group"),
		Category:        r.Text("category"),
		DefaultBuilding: r.Text("default_building"),
		Lens:            r.Text("lens"),
		LandUsage:       r.Text("land_usage"),

		AlwaysPossible:       r.Flag("always_possible"),
		EconomyOfScale:       r.Flag("economy_of_scale"),
		IsSubsistence:        r.Flag("is_subsistence"),
		IsGovernmentFunded:   r.Flag("is_government_funded"),
		IsMilitary:           r.Flag("is_military"),
		CappedByResources:    r.Flag("capped_by_resources"),
		DiscoverableResource: r.Flag("discoverable_resource"),
		DepletableResource:   r.Flag("depletable_resource"),
		AutoPlaceBuildings:   r.Flag("auto_place_buildings"),
		CanUseSlaves:         r.Flag("can_use_slaves"),
		PaysTaxes:            r.Flag("pays_taxes"),
		Subsidized:           r.Flag("subsidized"),
		HiresUnemployedOnly:  r.Flag("hires_unemployed_only"),
		HasTradeRevenue:      r.Flag("has_trade_revenue"),
		OwnsOtherBuildings:   r.Flag("owns_other_buildings"),
		AlwaysSelfOwning:     r.Flag("always_self_owning"),

		CompanyHeadquarter:         r.Flag("company_headquarter"),
		RegionalCompanyHeadquarter: r.Flag("regional_company_headquarter"),

		CashReservesMax:             r.Number("cash_reserves_max"),
		Urbanization:                r.Number("urbanization"),
		InfrastructureUsagePerLevel: r.Number("infrastructure_usage_per_level"),
		MinHiringRate:               r.Number("min_hiring_rate"),
		MaxHiringRate:               r.Number("max_hiring_rate"),
		ProportionalityLimit:        r.Number("proportionality_limit"),
		MinProductivityToHire:       r.Number("min_productivity_to_hire"),
		EconomyOfScaleAIFactor:      r.Number("economy_of_scale_ai_factor"),
		ForeignInvestmentAIFactor:   r.Number("foreign_investment_ai_factor"),
	}

	r.Known(
		"should_auto_expand", "inheritable_construction", "stateregion_max_level", "is_shown_in_outliner",
		"construction_efficiency_modifier", "self_investment_chance_modifier", "fired_pops_become_radical",
		"ignores_productivity_when_hiring", "builds_ships",
	)

	return group
}

// Building is a type of building, from common/buildings.
type Building struct {
	database.Definition

	// BuildingGroup is the group whose rules the building follows.
	BuildingGroup string

	Icon       string
	Background string

	// ProductionMethodGroups are the groups of production methods the
	// building chooses from, one method from each.
	ProductionMethodGroups []string

	// UnlockingTechnologies must all be researched to build it.
	UnlockingTechnologies []string

	// RequiredConstruction names the script value holding the construction
	// points it costs, such as construction_cost_high.
	RequiredConstruction database.ScriptValue

	// Buildable, Expandable and Downsizeable default to yes.
	Buildable    bool
	Expandable   bool
	Downsizeable bool

	// Unique buildings have one level in the whole world and are never built
	// by private investors.
	Unique bool

	HasMaxLevel               bool
	IgnoreStateregionMaxLevel bool
	Port                      bool
	Naval                     bool
	EnableAirConnection       bool
	RecruitsCombatUnits       bool
	RecruitsSailors           bool
	Statue                    bool
	CompanyHeadquarter        bool
	CanSwitchOwner            database.Opt[bool]
	OwnershipType             string
	Owners                    string
	SlavesRole                string
	CityType                  string
	Lens                      string
	Strait                    string
	Canal                     string
	CategoryBuildingType      string
	Aliases                   []string
	EconomicContribution      float64
	MinRaiseToHire            database.Opt[float64]
	AINationalizationDesire   database.Opt[float64]
	AIValue                   database.ScriptValue
	ConstructionPoints        database.Opt[float64]
}

func decodeBuilding(r *database.Reader) *Building {
	building := &Building{
		BuildingGroup:          r.Text("building_group"),
		Icon:                   r.Text("icon"),
		Background:             r.Text("background"),
		ProductionMethodGroups: r.Names("production_method_groups"),
		UnlockingTechnologies:  r.Names("unlocking_technologies"),
		RequiredConstruction:   r.ScriptValue("required_construction"),

		Buildable:    r.FlagOr("buildable", true),
		Expandable:   r.FlagOr("expandable", true),
		Downsizeable: r.FlagOr("downsizeable", true),
		Unique:       r.FlagOr("unique", false),

		HasMaxLevel:               r.FlagOr("has_max_level", false),
		IgnoreStateregionMaxLevel: r.FlagOr("ignore_stateregion_max_level", false),
		Port:                      r.FlagOr("port", false),
		Naval:                     r.FlagOr("naval", false),
		EnableAirConnection:       r.FlagOr("enable_air_connection", false),
		RecruitsCombatUnits:       r.FlagOr("recruits_combat_units", false),
		RecruitsSailors:           r.FlagOr("recruits_sailors", false),
		Statue:                    r.FlagOr("statue", false),
		CompanyHeadquarter:        r.FlagOr("company_headquarter", false),
		CanSwitchOwner:            r.Flag("can_switch_owner"),
		OwnershipType:             r.Text("ownership_type"),
		Owners:                    r.Text("owners"),
		SlavesRole:                r.Text("slaves_role"),
		CityType:                  r.Text("city_type"),
		Lens:                      r.Text("lens"),
		Strait:                    r.Text("strait"),
		Canal:                     r.Text("canal"),
		CategoryBuildingType:      r.Text("category_building_type"),
		Aliases:                   r.Names("aliases"),
		EconomicContribution:      r.NumberOr("economic_contribution", 1),
		MinRaiseToHire:            r.Number("min_raise_to_hire"),
		AINationalizationDesire:   r.Number("ai_nationalization_desire"),
		AIValue:                   r.ScriptValue("ai_value"),
		ConstructionPoints:        r.Number("construction_points"),
	}

	// The documentation calls the list production_methods, but the game's
	// files call it production_method_groups. Read both.
	if r.Has("production_methods") {
		building.ProductionMethodGroups = appendNew(building.ProductionMethodGroups, r.Names("production_methods")...)
		r.NoteDefinition("production_methods read as production_method_groups")
	}

	if building.BuildingGroup == "" {
		r.Missing("building_group", "it follows no group's rules")
	}

	r.Known(
		// Triggers, read by the game while playing.
		"potential", "possible", "can_build", "can_build_private", "can_build_government",
		"can_have_country_monopoly", "should_auto_expand", "investment_scores", "construction_modifier",
		"cannot_switch_owner",
		// Presentation.
		"city_gfx_interactions", "meshes", "entity_not_constructed", "entity_under_construction",
		"entity_constructed", "locator", "terrain_manipulator", "levels_per_mesh",
		"residence_points_per_level", "centerpiece_mesh_weight", "generates_residences",
		"ai_privatization_desire",
	)

	return building
}

// appendNew appends the values that are not in list yet.
func appendNew(list []string, values ...string) []string {
	for _, value := range values {
		if !contains(list, value) {
			list = append(list, value)
		}
	}

	return list
}
