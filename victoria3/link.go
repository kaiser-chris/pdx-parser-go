package victoria3

import (
	"strings"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// link checks the references between definitions. A reference to something
// that does not exist is reported and left as written: the tool can still show
// the building whose group is missing, and the key tells a mod author what to
// look for.
//
// References to things the loader does not read, such as laws and power bloc
// principles, are not checked.
func link(data *Data, collector *report.Collector) {
	for _, building := range data.Buildings.All() {
		check := newCheck(&building.Definition, "building", collector)
		check.one("building_group", building.BuildingGroup, data.BuildingGroups.Has, "building group")
		check.all("production_method_groups", building.ProductionMethodGroups, data.ProductionMethodGroups.Has, "production method group")
		check.all("unlocking_technologies", building.UnlockingTechnologies, data.Technologies.Has, "technology")
		check.one("owners", building.Owners, data.PopTypes.Has, "pop type")
		check.one("slaves_role", building.SlavesRole, data.PopTypes.Has, "pop type")
	}

	for _, group := range data.BuildingGroups.All() {
		check := newCheck(&group.Definition, "building group", collector)
		check.one("parent_group", group.Parent, data.BuildingGroups.Has, "building group")
		check.one("default_building", group.DefaultBuilding, data.Buildings.Has, "building")
	}

	checkGroupCycles(data, collector)

	for _, group := range data.ProductionMethodGroups.All() {
		newCheck(&group.Definition, "production method group", collector).
			all("production_methods", group.ProductionMethods, data.ProductionMethods.Has, "production method")
	}

	for _, method := range data.ProductionMethods.All() {
		check := newCheck(&method.Definition, "production method", collector)
		check.all("unlocking_technologies", method.UnlockingTechnologies, data.Technologies.Has, "technology")
		check.all("unlocking_global_technologies", method.UnlockingGlobalTechnologies, data.Technologies.Has, "technology")
		check.all("unlocking_production_methods", method.UnlockingProductionMethods, data.ProductionMethods.Has, "production method")
		check.one("replacement_if_valid", method.ReplacementIfValid, data.ProductionMethods.Has, "production method")
		check.all("required_input_goods", method.RequiredInputGoods, data.Goods.Has, "good")

		for _, flow := range method.Inputs() {
			check.modifier("goods_input_"+flow.Key+"_add", flow.Key, data.Goods.Has, "good")
		}

		for _, flow := range method.Outputs() {
			check.modifier("goods_output_"+flow.Key+"_add", flow.Key, data.Goods.Has, "good")
		}

		for _, flow := range method.Employment() {
			check.modifier("building_employment_"+flow.Key+"_add", flow.Key, data.PopTypes.Has, "pop type")
		}

		for _, ratio := range method.ProfessionRatio {
			check.one("profession_ratio", ratio.Key, data.PopTypes.Has, "pop type")
		}
	}

	for _, technology := range data.Technologies.All() {
		check := newCheck(&technology.Definition, "technology", collector)
		check.one("era", technology.Era, data.Eras.Has, "era")
		check.all("unlocking_technologies", technology.UnlockingTechnologies, data.Technologies.Has, "technology")
	}

	checkTechnologyCycles(data, collector)

	for _, need := range data.PopNeeds.All() {
		check := newCheck(&need.Definition, "pop need", collector)
		check.one("default", need.Default, data.Goods.Has, "good")

		for _, entry := range need.Entries {
			check.one("entry", entry.Goods, data.Goods.Has, "good")
		}
	}

	for _, pack := range data.BuyPackages.All() {
		check := newCheck(&pack.Definition, "buy package", collector)

		for _, need := range pack.Needs {
			check.one("goods", need.Key, data.PopNeeds.Has, "pop need")
		}
	}

	for _, company := range data.Companies.All() {
		check := newCheck(&company.Definition, "company", collector)
		check.all("building_types", company.BuildingTypes, data.Buildings.Has, "building")
		check.all("extension_building_types", company.ExtensionBuildingTypes, data.Buildings.Has, "building")
		check.one("replaces_company", company.ReplacesCompany, data.Companies.Has, "company")
	}
}

// Walk states for the cycle checks.
const (
	unvisited = iota
	visiting
	visited
)

// checkGroupCycles reports building groups whose parents lead back to
// themselves. Each group has one parent, so following parents from every
// group once, and remembering where each walk has been, finds every loop in
// one pass over the groups.
func checkGroupCycles(data *Data, collector *report.Collector) {
	state := map[string]int{}

	for start := range data.BuildingGroups.All() {
		var walk []string

		key := start
		for key != "" && state[key] == unvisited {
			group, ok := data.BuildingGroups.Get(key)
			if !ok {
				break
			}

			state[key] = visiting
			walk = append(walk, key)
			key = group.Parent
		}

		// A walk that runs into a group of its own walk has found a loop.
		// One that runs into a group from an earlier walk has not: that walk
		// already reported whatever lay beyond.
		if key != "" && state[key] == visiting {
			loop := walk[indexOf(walk, key):]
			group, _ := data.BuildingGroups.Get(key)
			newCheck(&group.Definition, "building group", collector).
				problem("parent_group", "its parent groups lead back to itself: %s -> %s", strings.Join(loop, " -> "), key)
		}

		for _, walked := range walk {
			state[walked] = visited
		}
	}
}

// checkTechnologyCycles reports technologies that, through their
// prerequisites, require themselves. Such a technology can never be
// researched, and anything that walks the tree has to stop somewhere.
//
// The walk keeps its own stack rather than recursing, so a prerequisite chain
// of any length cannot overflow the goroutine stack.
func checkTechnologyCycles(data *Data, collector *report.Collector) {
	type frame struct {
		key  string
		next int
	}

	state := map[string]int{}

	for start := range data.Technologies.All() {
		if state[start] != unvisited {
			continue
		}

		stack := []frame{{key: start}}
		position := map[string]int{start: 0}
		state[start] = visiting

		for len(stack) > 0 {
			top := &stack[len(stack)-1]
			technology, _ := data.Technologies.Get(top.key)

			if top.next >= len(technology.UnlockingTechnologies) {
				state[top.key] = visited
				delete(position, top.key)
				stack = stack[:len(stack)-1]

				continue
			}

			prerequisite := technology.UnlockingTechnologies[top.next]
			top.next++

			if !data.Technologies.Has(prerequisite) {
				continue
			}

			switch state[prerequisite] {
			case unvisited:
				state[prerequisite] = visiting
				position[prerequisite] = len(stack)
				stack = append(stack, frame{key: prerequisite})

			case visiting:
				loop := make([]string, 0, len(stack)-position[prerequisite]+1)
				for _, entry := range stack[position[prerequisite]:] {
					loop = append(loop, entry.key)
				}

				loop = append(loop, prerequisite)

				owner, _ := data.Technologies.Get(prerequisite)
				newCheck(&owner.Definition, "technology", collector).
					problem("unlocking_technologies", "requires itself: %s", strings.Join(loop, " -> "))
			}
		}
	}
}

func indexOf(list []string, value string) int {
	for index, item := range list {
		if item == value {
			return index
		}
	}

	return 0
}

// definitionCheck checks the references of one definition.
type definitionCheck struct {
	definition *database.Definition
	subject    string
	collector  *report.Collector

	// holders maps a field key to the names its occurrences mention, each to
	// the position of the first occurrence that mentions it. It is built the
	// first time a field is checked, so that finding where a bad reference
	// was written costs one lookup however many references there are.
	holders map[string]map[string]int
}

func newCheck(definition *database.Definition, kind string, collector *report.Collector) *definitionCheck {
	return &definitionCheck{definition: definition, subject: kind + " " + definition.Key, collector: collector}
}

// one checks a single reference. An empty reference is not checked: an
// optional field that is not set refers to nothing.
func (c *definitionCheck) one(field, key string, exists func(string) bool, kind string) {
	if key != "" && !exists(key) {
		c.problemAt(field, key, "%s %s does not exist", kind, key)
	}
}

func (c *definitionCheck) all(field string, keys []string, exists func(string) bool, kind string) {
	for _, key := range keys {
		c.one(field, key, exists, kind)
	}
}

// modifier checks a name that is part of a modifier's key, such as the good in
// goods_input_iron_add, and reports it at the modifier.
func (c *definitionCheck) modifier(modifierKey, key string, exists func(string) bool, kind string) {
	if !exists(key) {
		c.problemAt("building_modifiers", modifierKey, "%s %s in %s does not exist", kind, key, modifierKey)
	}
}

// problem reports a problem with a field, at its first occurrence.
func (c *definitionCheck) problem(field, format string, args ...any) {
	c.problemAt(field, "", format, args...)
}

// problemAt reports a problem at the occurrence of a field that mentions the
// given name. A field occurs more than once when a mod injects into a
// definition, and the problem belongs in the file of the occurrence that
// caused it, not the first one. Without a name, or when no occurrence mentions
// it, the first occurrence is used.
func (c *definitionCheck) problemAt(field, name, format string, args ...any) {
	holder, found := c.holdersOf(field)[name]
	if !found {
		holder, found = c.holdersOf(field)[""]
	}

	origin := c.definition.Origin()
	line, column := origin.Line, 0

	if found {
		origin = c.definition.OriginOf(holder)
		line, column = c.definition.Raw.Fields[holder].Line, c.definition.Raw.Fields[holder].Column
	}

	c.collector.Addf(report.SeverityWarning, origin.Source, origin.Path, line, column, c.subject, format, args...)
}

func (c *definitionCheck) holdersOf(field string) map[string]int {
	if holders, ok := c.holders[field]; ok {
		return holders
	}

	holders := map[string]int{}

	for index, candidate := range c.definition.Raw.Fields {
		if candidate.Key != field {
			continue
		}

		if _, ok := holders[""]; !ok {
			holders[""] = index
		}

		collectNames(candidate.Value, func(name string) {
			if _, ok := holders[name]; !ok {
				holders[name] = index
			}
		})
	}

	if c.holders == nil {
		c.holders = map[string]map[string]int{}
	}

	c.holders[field] = holders

	return holders
}

// collectNames calls add with every string and every key inside a value.
func collectNames(node script.Node, add func(string)) {
	switch node.Kind {
	case script.KindString:
		add(node.Text)

	case script.KindTagged:
		if node.Value != nil {
			collectNames(*node.Value, add)
		}
	}

	for _, item := range node.Items {
		collectNames(item, add)
	}

	for _, field := range node.Fields {
		add(field.Key)
		collectNames(field.Value, add)
	}
}
