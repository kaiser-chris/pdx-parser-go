package asset

import (
	"strings"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// link checks the references between definitions. A reference to something
// that does not exist is reported and left as written: the entity is still
// there to be shown, and the name tells a mod author what to look for.
//
// Only the references to definitions are checked. Whether the files a
// definition names exist is a question about the disk, which reading the
// definitions never asks.
func link(assets *Assets, collector *report.Collector) {
	for _, mesh := range assets.Meshes.All() {
		check := newReferenceCheck(&mesh.Definition, kindMesh, collector)

		for _, imported := range mesh.Imports {
			if imported.Type != kindAnimationSet {
				check.note(keyImport, imported.Type, "imports a %s, which is no kind of definition this package reads", imported.Type)

				continue
			}

			check.one(keyImport, imported.Name, assets.AnimationSets.Has, kindAnimationSet)
		}
	}

	for _, set := range assets.AnimationSets.All() {
		newReferenceCheck(&set.Definition, kindAnimationSet, collector).
			one(keyReferenceSkeleton, set.ReferenceSkeleton, assets.Meshes.Has, kindMesh)
	}

	meshes, cycles := followClones(assets)

	for _, entity := range assets.Entities.All() {
		check := newReferenceCheck(&entity.Definition, kindEntity, collector)

		check.one(keyPDXMesh, entity.Mesh, assets.Meshes.Has, kindMesh)
		check.one(keyClone, entity.Clone, assets.Entities.Has, kindEntity)

		for _, attachment := range entity.Attachments {
			check.one(keyAttach, attachment.Entity, assets.Entities.Has, kindEntity)
		}

		for _, group := range entity.Groups {
			for _, choice := range group.Choices {
				for _, attachment := range choice.Attachments {
					check.one(keyGroup, attachment.Entity, assets.Entities.Has, kindEntity)
				}
			}
		}

		for _, state := range entity.States {
			for _, events := range [][]Event{state.StartEvents, state.Events} {
				for _, event := range events {
					check.one(keyState, event.Entity, assets.Entities.Has, kindEntity)
				}
			}
		}

		if mesh := meshes[entity.Key]; mesh != nil {
			checkMeshIDs(check, entity, mesh, assets.AnimationsOf(mesh))
		}
	}

	reportCloneCycles(cycles, collector)
}

// checkMeshIDs checks that what an entity's states play and its attributes
// morph are animations and blend shapes of the mesh it draws. playable are
// the animations the mesh can play, its own and those it imports.
func checkMeshIDs(check *referenceCheck, entity *Entity, mesh *Mesh, playable []MeshAnimation) {
	animations := map[string]bool{}
	for _, animation := range playable {
		animations[animation.ID] = true
	}

	additive := map[string]bool{}
	for _, animation := range mesh.AdditiveAnimations {
		additive[animation.ID] = true
	}

	shapes := map[string]bool{}
	for _, shape := range mesh.BlendShapes {
		shapes[shape.ID] = true
	}

	notOf := func(field, id string, has map[string]bool, what string) {
		if id != "" && !has[id] {
			check.problem(field, id, "%s %s is not one of the %ss of %s %s", what, id, what, kindMesh, mesh.Key)
		}
	}

	for _, state := range entity.States {
		notOf(keyState, state.Animation, animations, keyAnimation)
	}

	for _, attribute := range entity.Attributes {
		notOf(keyAttribute, attribute.BlendShape, shapes, "blend shape")
		notOf(keyAttribute, attribute.AdditiveAnimation, additive, "additive animation")
	}
}

// followClones works out, for every entity at once, the mesh it draws through
// the entities it clones, and finds the clones that end up cloning themselves.
//
// Following each entity's chain on its own would take as long as the chain for
// every entity in it, which a mod with one long chain of clones turns into
// minutes. Here every entity is passed once: a chain is followed until it meets
// an entity already worked out, and what was learned is filled in backwards.
// Each cycle is found once, starting at the entity of it reached first.
func followClones(assets *Assets) (meshes map[string]*Mesh, cycles [][]*Entity) {
	const (
		onPath = 1
		done   = 2
	)

	state := map[string]int{}
	meshes = map[string]*Mesh{}

	for name := range assets.Entities.All() {
		var (
			path []*Entity
			base *Mesh
		)

		for current := name; current != ""; {
			entity, ok := assets.Entities.Get(current)
			if !ok {
				break
			}

			if state[current] == done {
				base = meshes[current]

				break
			}

			if state[current] == onPath {
				start := 0
				for path[start].Key != current {
					start++
				}

				cycles = append(cycles, path[start:])

				break
			}

			state[current] = onPath
			path = append(path, entity)
			current = entity.Clone
		}

		for index := len(path) - 1; index >= 0; index-- {
			entity := path[index]

			if entity.Mesh != "" {
				base, _ = assets.Meshes.Get(entity.Mesh)
			}

			meshes[entity.Key] = base
			state[entity.Key] = done
		}
	}

	return meshes, cycles
}

// reportCloneCycles reports entities that end up cloning themselves, which no
// order of reading can resolve, once for each cycle.
func reportCloneCycles(cycles [][]*Entity, collector *report.Collector) {
	for _, cycle := range cycles {
		names := make([]string, 0, len(cycle)+1)
		for _, entity := range cycle {
			names = append(names, entity.Key)
		}

		names = append(names, cycle[0].Key)

		check := newReferenceCheck(&cycle[0].Definition, kindEntity, collector)
		check.problem(keyClone, cycle[0].Clone, "clones itself through %s", strings.Join(names, " -> "))
	}
}

// CloneChain returns an entity followed by the entity it clones, the one that
// clones and so on. It stops at a name that does not exist or at an entity it
// has already passed, so a cycle cannot hang it, and it is empty for a name
// that is no entity.
func (a *Assets) CloneChain(name string) []*Entity {
	var (
		chain []*Entity
		seen  = map[string]bool{}
	)

	for name != "" && !seen[name] {
		entity, ok := a.Entities.Get(name)
		if !ok {
			break
		}

		seen[name] = true
		chain = append(chain, entity)
		name = entity.Clone
	}

	return chain
}

// MeshOf returns the mesh an entity draws: its own, or when it names none,
// that of the entity it clones, and so on. It reports false when no entity in
// the chain names a mesh, or the one named does not exist.
func (a *Assets) MeshOf(name string) (*Mesh, bool) {
	for _, entity := range a.CloneChain(name) {
		if entity.Mesh != "" {
			return a.Meshes.Get(entity.Mesh)
		}
	}

	return nil, false
}

// referenceCheck reports references of one definition that do not resolve.
type referenceCheck struct {
	definition *database.Definition
	subject    string
	collector  *report.Collector

	// places holds, for each field key a problem was reported in, where the
	// strings inside those fields are first written. It is built the first
	// time a key is asked about, so that a definition with thousands of
	// broken references is walked once rather than once for each.
	places map[string]map[string]place
}

// place is where a string is written in a definition.
type place struct {
	field        int
	line, column int
}

// newReferenceCheck returns a check for the references of a definition of a
// kind.
func newReferenceCheck(definition *database.Definition, kind string, collector *report.Collector) *referenceCheck {
	return &referenceCheck{
		definition: definition,
		subject:    kind + " " + definition.Key,
		collector:  collector,
		places:     map[string]map[string]place{},
	}
}

// one checks a single reference, held somewhere in the field with the given
// key. An empty reference is not checked: a field that is not set refers to
// nothing.
func (c *referenceCheck) one(field, name string, exists func(string) bool, kind string) {
	if name != "" && !exists(name) {
		c.problem(field, name, "%s %s does not exist", kind, name)
	}
}

// problem reports a problem at the value that names it, found in the fields
// with the given key, which may hold it deep inside a block. It is reported at
// the definition when no such value is found.
func (c *referenceCheck) problem(field, name, format string, args ...any) {
	c.report(report.SeverityWarning, field, name, format, args...)
}

// note reports something worth knowing that is not a problem, at the value
// that names it, as problem does.
func (c *referenceCheck) note(field, name, format string, args ...any) {
	c.report(report.SeverityInfo, field, name, format, args...)
}

func (c *referenceCheck) report(severity report.Severity, field, name, format string, args ...any) {
	origin := c.definition.Origin()
	line, column := origin.Line, 0

	if found, ok := c.placesIn(field)[name]; ok {
		origin = c.definition.OriginOf(found.field)
		line, column = found.line, found.column
	}

	c.collector.Addf(severity, origin.Source, origin.Path, line, column, c.subject, format, args...)
}

// placesIn returns where each string inside the fields with the given key is
// first written.
func (c *referenceCheck) placesIn(field string) map[string]place {
	if places, ok := c.places[field]; ok {
		return places
	}

	places := map[string]place{}

	var walk func(index int, node script.Node)

	walk = func(index int, node script.Node) {
		if node.Kind == script.KindString {
			if _, seen := places[node.Text]; !seen {
				places[node.Text] = place{field: index, line: node.Line, column: node.Column}
			}
		}

		for _, inner := range node.Fields {
			walk(index, inner.Value)
		}

		for _, item := range node.Items {
			walk(index, item)
		}
	}

	for index, candidate := range c.definition.Raw.Fields {
		if candidate.Key == field {
			walk(index, candidate.Value)
		}
	}

	c.places[field] = places

	return places
}
