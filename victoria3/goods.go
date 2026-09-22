package victoria3

import (
	"github.com/kaiser-chris/pdx-parser-go/database"
)

// Good is a tradeable commodity, from common/goods.
type Good struct {
	database.Definition

	Texture string

	// Cost is the base price. Market prices move around it within
	// NEconomy.PRICE_RANGE.
	Cost float64

	// Category is staple, industrial, luxury or military. It groups goods in
	// the interface.
	Category string

	// Tradeable goods go onto the world market. Default yes.
	Tradeable bool

	// Local goods never leave the state they are made in, such as electricity
	// and services. Default no.
	Local bool

	// FixedPrice goods never move off their base price, such as gold.
	FixedPrice bool

	// PrestigeFactor is the prestige for the lowest awarding spot on the
	// leaderboard of this good's producers, doubled for every spot above it.
	PrestigeFactor float64

	// TradedQuantity is how many units move per trade. Default
	// NEconomy.GOODS_DEFAULT_TRADE_QUANTITY.
	TradedQuantity float64

	// ConvoyCostMultiplier scales the merchant marine cost of shipping this
	// good as military supply. Default
	// NEconomy.GOODS_DEFAULT_MERCHANT_MARINE_COST_MULTIPLIER.
	ConvoyCostMultiplier float64

	// ObsessionChance is the weight for pops to become obsessed with this
	// good. Zero means they never do.
	ObsessionChance float64

	// ConsumptionTaxCost is the authority cost of taxing this good. Unset
	// means it cannot be taxed.
	ConsumptionTaxCost database.Opt[float64]

	// PopConsumptionCanAddInfrastructure makes pop consumption of the good
	// add infrastructure to the state, like automobiles.
	PopConsumptionCanAddInfrastructure bool
}

// GoodCategories are the categories the game groups goods by.
var GoodCategories = []string{"staple", "industrial", "luxury", "military"}

func decodeGood(r *database.Reader, defines database.Defines) *Good {
	good := &Good{
		Texture:                            r.Text("texture"),
		Category:                           r.Text("category"),
		Tradeable:                          r.FlagOr("tradeable", true),
		Local:                              r.FlagOr("local", false),
		FixedPrice:                         r.FlagOr("fixed_price", false),
		PrestigeFactor:                     r.NumberOr("prestige_factor", 0),
		TradedQuantity:                     r.NumberOr("traded_quantity", defines.NumberOr("NEconomy", "GOODS_DEFAULT_TRADE_QUANTITY", 10)),
		ConvoyCostMultiplier:               r.NumberOr("convoy_cost_multiplier", defines.NumberOr("NEconomy", "GOODS_DEFAULT_MERCHANT_MARINE_COST_MULTIPLIER", 1)),
		ObsessionChance:                    r.NumberOr("obsession_chance", 0),
		ConsumptionTaxCost:                 r.Number("consumption_tax_cost"),
		PopConsumptionCanAddInfrastructure: r.FlagOr("pop_consumption_can_add_infrastructure", false),
	}

	cost := r.Number("cost")
	if !cost.Set {
		r.Missing("cost", "its price is taken as 0")
	}

	good.Cost = cost.Value

	if good.Category != "" && !contains(GoodCategories, good.Category) {
		r.NoteDefinition("category %s is not one of %v", good.Category, GoodCategories)
	}

	return good
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}

	return false
}
