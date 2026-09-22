package victoria3

import (
	"github.com/kaiser-chris/pdx-parser-go/database"
)

// Era is a stage of the technology tree, from common/technology/eras.
type Era struct {
	database.Definition

	// Index is the era's place in order, counting from 0. Eras are ordered as
	// they are defined, which in the game is era_1 to era_5.
	Index int

	// TechnologyCost is the research cost of every technology in the era.
	TechnologyCost float64

	// StartDate and EndDate are documented fields that the game itself does
	// not set. They are empty unless a mod sets them.
	StartDate string
	EndDate   string

	Icon string

	// Label is the comment on the era's first line, which is where the game
	// keeps the only statement of what period the era covers: "1836-1861".
	Label string
}

func decodeEra(r *database.Reader, index int, comment string) *Era {
	era := &Era{
		Index:     index,
		StartDate: r.Text("start_date"),
		EndDate:   r.Text("end_date"),
		Icon:      r.Text("icon"),
		Label:     comment,
	}

	cost := r.Number("technology_cost")
	if !cost.Set {
		r.Missing("technology_cost", "its technologies are taken to cost nothing")
	}

	era.TechnologyCost = cost.Value

	// The dates are read as text because a date such as 1836.1.1 is not a
	// number. A mod may still quote them or not; both work.
	return era
}

// Technology is a technology, from common/technology/technologies.
type Technology struct {
	database.Definition

	// Era is the key of the era the technology belongs to.
	Era string

	// Category is production, military or society.
	Category string

	Texture string

	// UnlockingTechnologies must be researched before this one can be.
	UnlockingTechnologies []string

	// Modifiers apply to a country that has researched the technology. Many
	// change the throughput of buildings.
	Modifiers []database.Modifier

	// CanResearch is false for technologies only given by events. Default
	// yes.
	CanResearch bool

	ShouldUpdateMap bool
}

func decodeTechnology(r *database.Reader) *Technology {
	technology := &Technology{
		Era:                   r.Text("era"),
		Category:              r.Text("category"),
		Texture:               r.Text("texture"),
		UnlockingTechnologies: r.Names("unlocking_technologies"),
		Modifiers:             r.Modifiers("modifier"),
		CanResearch:           r.FlagOr("can_research", true),
		ShouldUpdateMap:       r.FlagOr("should_update_map", false),
	}

	if technology.Era == "" {
		r.Missing("era", "it belongs to no era")
	}

	r.Known("on_researched", "ai_weight")

	return technology
}
