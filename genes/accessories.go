package genes

import (
	"strings"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// FolderAccessories is where the portrait accessories are, which accessory
// genes pick by name.
var FolderAccessories = folders.Folder{Path: "gfx/portraits/accessories"}

// kindAccessory names a portrait accessory in diagnostics.
const kindAccessory = "portrait accessory"

// Accessory is a portrait accessory: the entities an accessory gene draws
// when it picks it.
//
//	male_coat_european_01 = {
//		set_tags = "shrink_arms,shrink_chest,shrink_belly"
//		entity = { required_tags = "" shared_pose_entity = torso entity = "male_coat_european_01_entity" }
//	}
type Accessory struct {
	database.Definition

	// SetTags are the tags the accessory sets on the portrait, which
	// entities and settings of genes that require them look for.
	SetTags []string

	// Entities are the entities the accessory draws, of which one is drawn:
	// the first whose required tags the portrait has.
	Entities []AccessoryEntity
}

// AccessoryEntity is one entity a portrait accessory can draw.
type AccessoryEntity struct {
	// Entity is the entity of the .asset files.
	Entity string

	// SharedPoseEntity is the entity of the portrait whose pose the entity
	// takes, by the part of the portrait it is, such as head or torso: its
	// mesh is skinned to the same skeleton.
	SharedPoseEntity string

	// Node is the bone the entity is attached to, when it is attached to
	// one rather than posed with an entity, such as a prop in a hand.
	Node string

	// RequiredTags are the tags the portrait must have for this entity to
	// be drawn. None means it is drawn whatever the tags.
	RequiredTags []string
}

// The keys of a portrait accessory.
const (
	keySetTags          = "set_tags"
	keySharedPoseEntity = "shared_pose_entity"
	keyNode             = "node"
)

// LoadAccessories reads the portrait accessories of a set of folders, by
// name.
//
// Loading never fails. Everything worth knowing about the files is in the
// diagnostics.
func LoadAccessories(set *folders.Set) (database.Table[Accessory], report.Diagnostics) {
	collector := &report.Collector{}
	store := database.Collect(set, FolderAccessories, kindAccessory, collector)

	table := database.Decode(store, collector, func(r *database.Reader, _ *database.Definition) *Accessory {
		return decodeAccessory(r)
	})

	return table, collector.Diagnostics
}

// ReadAccessories reads the portrait accessories of one file that has
// already been parsed.
func ReadAccessories(document *script.Document, file folders.File, collector *report.Collector) database.Table[Accessory] {
	store := database.NewStore(kindAccessory)
	store.AddDocument(document, file, collector)

	return database.Decode(store, collector, func(r *database.Reader, _ *database.Definition) *Accessory {
		return decodeAccessory(r)
	})
}

// decodeAccessory reads one portrait accessory.
func decodeAccessory(r *database.Reader) *Accessory {
	accessory := &Accessory{SetTags: tags(r, keySetTags)}

	for _, block := range r.Blocks(keyEntity) {
		child := r.Child(block.Index, block.Value, keyEntity)

		entity := AccessoryEntity{
			Entity:           child.Text(keyEntity),
			SharedPoseEntity: child.Text(keySharedPoseEntity),
			Node:             child.Text(keyNode),
			RequiredTags:     tags(child, keyRequiredTags),
		}

		child.Finish()
		accessory.Entities = append(accessory.Entities, entity)
	}

	return accessory
}

// tags reads tags written as one string, which may name several, separated
// by commas.
func tags(r *database.Reader, key string) []string {
	var found []string

	for _, written := range r.Names(key) {
		for tag := range strings.SplitSeq(written, ",") {
			if tag = strings.TrimSpace(tag); tag != "" {
				found = append(found, tag)
			}
		}
	}

	return found
}

// Has reports whether a portrait with the tags given may draw the entity:
// whether it has every tag the entity requires.
func (e AccessoryEntity) Has(tags []string) bool {
	for _, required := range e.RequiredTags {
		found := false

		for _, tag := range tags {
			found = found || tag == required
		}

		if !found {
			return false
		}
	}

	return true
}

// Choose returns the entity a portrait with the tags given draws of the
// accessory: the first whose required tags it has, as the shipped files are
// written, a hairstyle drawing nothing under a tag that hides the hair, its
// own version under a cap, and itself otherwise. It reports false when no
// entity fits. The entity chosen may name no entity of the .asset files,
// which draws nothing.
func (a *Accessory) Choose(tags []string) (AccessoryEntity, bool) {
	for _, entity := range a.Entities {
		if entity.Has(tags) {
			return entity, true
		}
	}

	return AccessoryEntity{}, false
}
