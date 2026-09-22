package victoria3

import "github.com/kaiser-chris/pdx-parser-go/database"

// GroupChain returns a building group followed by its parent, its parent's
// parent and so on up to the root. It stops early at a parent that does not
// exist or at one it has already passed, so a loop in the tree cannot hang it.
func (d *Data) GroupChain(key string) []*BuildingGroup {
	var (
		chain []*BuildingGroup
		seen  = map[string]bool{}
	)

	for key != "" && !seen[key] {
		group, ok := d.BuildingGroups.Get(key)
		if !ok {
			break
		}

		seen[key] = true
		chain = append(chain, group)
		key = group.Parent
	}

	return chain
}

// ResolveGroup reads a building group setting the way the game applies it:
// from the group itself if it sets it, otherwise from the nearest ancestor that
// does. It reports false when no group in the chain sets it.
//
//	eos, _ := ResolveGroup(data, "bg_light_industry", func(g *BuildingGroup) database.Opt[bool] { return g.EconomyOfScale })
//
// Text settings such as Lens are resolved through database.Text:
//
//	lens, _ := ResolveGroup(data, "bg_light_industry", func(g *BuildingGroup) database.Opt[string] { return database.Text(g.Lens) })
func ResolveGroup[T any](data *Data, key string, get func(*BuildingGroup) database.Opt[T]) (T, bool) {
	for _, group := range data.GroupChain(key) {
		if value, ok := get(group).Get(); ok {
			return value, true
		}
	}

	var zero T

	return zero, false
}
