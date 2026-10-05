package asset

import (
	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// Entity is an entity definition: something the game places in its world or
// its portraits, made of a mesh, the states it animates through, the points
// other entities attach to, and the attributes that morph it.
//
//	entity = {
//		name = "female_belt_01_entity"
//		pdxmesh = "female_belt_01_mesh"
//		attribute = { name = "body_bs_fat" blend_shape = "female_belt_01_bs_fat" }
//		default_state = "idle"
//		state = { name = "idle" animation = "idle_animation" }
//		locator = { name = "pos_1" position = { 0 1.5 0 } }
//		attach = { pos_1 = "flag_entity" }
//		game_data = { portrait_entity_user_data = { ... } }
//	}
//
// The Key of the embedded Definition is the entity's name.
type Entity struct {
	database.Definition

	// Mesh is the name of the pdxmesh the entity draws. An entity that only
	// plays sounds or holds attachments has none.
	Mesh string

	// Clone is the name of an entity this one is a copy of, with whatever it
	// sets itself on top. The definition here is the entity's own fields;
	// those of the entity it clones are not merged in.
	Clone string

	// Scale resizes the entity. It is 1 when the file gives none.
	Scale float64

	// CullRadius is the size the engine assumes the entity has when deciding
	// whether it is on screen, when the file says.
	CullRadius database.Opt[float64]

	// DefaultState is the state the entity starts in.
	DefaultState string

	// GetStateFromParent has an attached entity follow the state of the one it
	// is attached to, when the file says.
	GetStateFromParent database.Opt[bool]

	// Settings override the mesh settings of the entity's mesh.
	Settings []MeshSettings

	// Attributes are the values the game sets on the entity, such as how fat
	// or old a portrait is, and what each one morphs.
	Attributes []Attribute

	// States are what the entity can be doing, in the order written.
	States []State

	// Locators are named points on the entity that other entities and effects
	// attach to.
	Locators []Locator

	// Attachments are the entities attached to the locators, in the order
	// written.
	Attachments []Attachment

	// Groups are sets of attachments the engine picks from at random.
	Groups []Group

	// GameData is what the game itself keeps on the entity, such as the
	// portrait accessory of Victoria 3, as written. Its contents differ from
	// game to game, so reading them is left to a game's own package.
	GameData script.Node
}

// Attribute is a value the game sets on an entity, and what it drives.
type Attribute struct {
	// Name is the attribute, such as body_bs_fat.
	Name string

	// BlendShape is the id of the blend shape of the entity's mesh the value
	// morphs towards, when it drives one.
	BlendShape string

	// AdditiveAnimation is the id of the additive animation of the entity's
	// mesh the value plays, when it drives one.
	AdditiveAnimation string

	// Default is the value before the game sets one, when the file says.
	Default database.Opt[float64]
}

// State is one thing an entity can be doing: an animation, and the events
// that happen while it plays.
type State struct {
	Name string

	// Animation is the id of the animation of the entity's mesh the state
	// plays.
	Animation string

	// AnimationSpeed and AnimationBlendTime change how the animation plays,
	// when the file says.
	AnimationSpeed     database.Opt[float64]
	AnimationBlendTime database.Opt[float64]

	// Looping plays the animation over and over, when the file says.
	Looping database.Opt[bool]

	// NextState is the state the entity goes to when this one ends, and
	// StateTime how long this one lasts, when the file says.
	NextState string
	StateTime database.Opt[float64]

	// Chance weighs the state against others of the same name, when the file
	// says.
	Chance database.Opt[float64]

	// ChanceFromState weighs the state by the state the entity is in when it
	// picks, as in chance = { if_current_state = { "idle" = 100 } default = 0 },
	// which Europa Universalis 5 and Crusader Kings 3 write, and
	// ChanceDefault is the weight in any state not listed, when the file
	// says. Crusader Kings 3 writes this as well as a plain chance.
	ChanceFromState []database.Amount
	ChanceDefault   database.Opt[float64]

	// TimeOffset starts the animation a random time into it, between the two
	// values; a single value is written as both.
	TimeOffset database.Opt[[2]float64]

	// Propagation sets the state of attached entities while this one plays.
	Propagation []Propagation

	// StartEvents happen as the state begins, Events at their time into it.
	StartEvents []Event
	Events      []Event
}

// Propagation sets the state of attached entities.
type Propagation struct {
	// Attachment names the attached entities by the ID of their attachment,
	// the name of the attach block that holds them. It is empty for a state
	// that is set on every attachment, written as propagate_state = "idle".
	Attachment string

	// State is the state they are set to.
	State string
}

// Event is something that happens while a state plays: a particle effect, a
// sound, an entity attached or removed.
type Event struct {
	// Time is when the event happens, in seconds into the state.
	Time float64

	// Node is the locator or bone the event happens at.
	Node string

	// Particle is the particle effect to show, Sound the sound effect to play
	// and Light the light to switch on.
	Particle string
	Sound    string
	Light    string

	// SoundStopsOnStateChange says whether the sound stops when the state
	// does, when the file says. Crusader Kings 3 writes it.
	SoundStopsOnStateChange database.Opt[bool]

	// SoundParameters set parameters of the sound.
	SoundParameters []database.Amount

	// Entity is the name of an entity to attach, AttachmentID the name the
	// attachment goes by, and RemoveAttachment one to take off again.
	Entity           string
	AttachmentID     string
	RemoveAttachment string

	// State is a state to set on the attached entity.
	State string

	// The remaining settings, when the file says.
	TriggerOnce     database.Opt[bool]
	KeepParticle    database.Opt[bool]
	KeepSound       database.Opt[bool]
	KeepEntity      database.Opt[bool]
	UseParentNodes  database.Opt[bool]
	Life            database.Opt[float64]
	SkipForward     database.Opt[float64]
	EntityFadeSpeed database.Opt[float64]
}

// Vec3 is a position or an angle in three dimensions.
type Vec3 struct {
	X, Y, Z float64
}

// Locator is a named point on an entity.
type Locator struct {
	Name string

	// ParentJoint is the joint of the skeleton the locator hangs from, when
	// the file says. Europa Universalis 5 writes it.
	ParentJoint string

	Position Vec3

	// Rotation is in degrees around each axis.
	Rotation Vec3

	// Scale resizes what is attached to the locator. It is 1 when the file
	// gives none.
	Scale float64
}

// Attachment hangs an entity from a locator.
type Attachment struct {
	// Locator is the locator, or a bone of the mesh.
	Locator string

	// Entity is the name of the entity attached.
	Entity string

	// ID is the name the attach block that holds the attachment gives it, as
	// in attach = { name = weapon r_hand = "saber_entity" }, which a state
	// refers to it by to set its state. It is empty when the block has none.
	ID string
}

// Group is a set of attachments the engine picks from at random.
type Group struct {
	Name string

	// Choices are the sets of attachments it picks from.
	Choices []GroupChoice
}

// GroupChoice is one set of attachments of a group.
type GroupChoice struct {
	// Chance weighs the choice against the others of its group.
	Chance float64

	Attachments []Attachment
}

// The keys of an entity definition and of the blocks inside it.
const (
	keyName               = "name"
	keyPDXMesh            = "pdxmesh"
	keyClone              = "clone"
	keyCullRadius         = "cull_radius"
	keyDefaultState       = "default_state"
	keyGetStateFromParent = "get_state_from_parent"
	keyAttribute          = "attribute"
	keyDefault            = "default"
	keyState              = "state"
	keyAnimationSpeed     = "animation_speed"
	keyAnimationBlendTime = "animation_blend_time"
	keyLooping            = "looping"
	keyNextState          = "next_state"
	keyStateTime          = "state_time"
	keyChance             = "chance"
	keyIfCurrentState     = "if_current_state"
	keyStopOnStateChange  = "stop_on_state_change"
	keyTimeOffset         = "time_offset"
	keyPropagateState     = "propagate_state"
	keyStartEvent         = "start_event"
	keyEvent              = "event"
	keyTime               = "time"
	keyNode               = "node"
	keyParticle           = "particle"
	keySound              = "sound"
	keySoundEffect        = "soundeffect"
	keySoundParameter     = "soundparameter"
	keyLight              = "light"
	keyEntity             = "entity"
	keyAttachmentID       = "attachment_id"
	keyRemoveAttachment   = "remove_attachment"
	keyTriggerOnce        = "trigger_once"
	keyKeepParticle       = "keep_particle"
	keyKeepSound          = "keep_sound"
	keyKeepEntity         = "keep_entity"
	keyUseParentNodes     = "use_parent_nodes"
	keyLife               = "life"
	keySkipForward        = "skip_forward"
	keyEntityFadeSpeed    = "entity_fade_speed"
	keyLocator            = "locator"
	keyPosition           = "position"
	keyParentJoint        = "parent_joint"
	keyRotation           = "rotation"
	keyAttach             = "attach"
	keyGroup              = "group"
	keyGameData           = "game_data"

	// keyEntityEditorID is bookkeeping of the engine's entity editor, which
	// Europa Universalis 5 leaves in its events. The game has no use for it.
	keyEntityEditorID = "entity_editor_id"
)

// decodeEntity reads one entity definition.
func decodeEntity(r *database.Reader) *Entity {
	r.Known(keyName)

	entity := &Entity{
		Mesh:               r.Text(keyPDXMesh),
		Clone:              r.Text(keyClone),
		Scale:              r.NumberOr(keyScale, 1),
		CullRadius:         r.Number(keyCullRadius),
		DefaultState:       r.Text(keyDefaultState),
		GetStateFromParent: r.Flag(keyGetStateFromParent),
		GameData:           r.Raw(keyGameData),
	}

	for _, block := range r.Blocks(keyMeshSettings) {
		entity.Settings = append(entity.Settings, decodeMeshSettings(r.Child(block.Index, block.Value, keyMeshSettings)))
	}

	for _, block := range r.Blocks(keyAttribute) {
		child := r.Child(block.Index, block.Value, keyAttribute)

		attribute := Attribute{
			Name:              child.Text(keyName),
			BlendShape:        child.Text(keyBlendShape),
			AdditiveAnimation: child.Text(keyAdditiveAnimation),
			Default:           child.Number(keyDefault),
		}

		if attribute.Name == "" {
			child.Missing(keyName, "the game cannot set the attribute")
		}

		child.Finish()
		entity.Attributes = append(entity.Attributes, attribute)
	}

	for _, block := range r.Blocks(keyState) {
		entity.States = append(entity.States, decodeState(r.Child(block.Index, block.Value, keyState)))
	}

	for _, block := range r.Blocks(keyLocator) {
		entity.Locators = append(entity.Locators, decodeLocator(r.Child(block.Index, block.Value, keyLocator)))
	}

	for _, block := range r.Blocks(keyAttach) {
		child := r.Child(block.Index, block.Value, keyAttach)
		entity.Attachments = append(entity.Attachments, attachments(child)...)
		child.Finish()
	}

	for _, block := range r.Blocks(keyGroup) {
		entity.Groups = append(entity.Groups, decodeGroup(r.Child(block.Index, block.Value, keyGroup)))
	}

	return entity
}

// decodeState reads one state block.
func decodeState(r *database.Reader) State {
	defer r.Finish()

	state := State{
		Name:               r.Text(keyName),
		Animation:          r.Text(keyAnimation),
		AnimationSpeed:     r.Number(keyAnimationSpeed),
		AnimationBlendTime: r.Number(keyAnimationBlendTime),
		Looping:            r.Flag(keyLooping),
		NextState:          r.Text(keyNextState),
		StateTime:          r.Number(keyStateTime),
		TimeOffset:         timeOffset(r),
	}

	if state.Name == "" {
		r.Missing(keyName, "the state cannot be entered")
	}

	chance(r, &state)

	for _, index := range r.Indices(keyPropagateState) {
		state.Propagation = append(state.Propagation, propagation(r, index)...)
	}

	for _, block := range r.Blocks(keyStartEvent) {
		state.StartEvents = append(state.StartEvents, decodeEvent(r.Child(block.Index, block.Value, keyStartEvent)))
	}

	for _, block := range r.Blocks(keyEvent) {
		state.Events = append(state.Events, decodeEvent(r.Child(block.Index, block.Value, keyEvent)))
	}

	return state
}

// timeOffset reads the time offset of a state: a range of two numbers, or a
// single number that is both ends of it.
func timeOffset(r *database.Reader) database.Opt[[2]float64] {
	index, ok := r.Last(keyTimeOffset)
	if !ok {
		return database.Opt[[2]float64]{}
	}

	value := r.Fields()[index].Value

	if number, ok := value.Num(); ok {
		return database.Some([2]float64{number, number})
	}

	if numbers, ok := value.Numbers(); ok && len(numbers) == 2 {
		return database.Some([2]float64{numbers[0], numbers[1]})
	}

	r.Warn(index, value, "%s should be a number or two numbers, found %s; ignored", keyTimeOffset, value.Describe())

	return database.Opt[[2]float64]{}
}

// chance reads the chance of a state. It is written as a number, as a block
// that weighs the state by the state the entity is in, or both, each in a
// field of its own, so every occurrence is read: the last number is the plain
// chance, and the last block the weights.
func chance(r *database.Reader, state *State) {
	for _, index := range r.Indices(keyChance) {
		value := r.Fields()[index].Value

		if number, ok := value.Num(); ok {
			state.Chance = database.Some(number)

			continue
		}

		if !value.IsBlock() {
			r.Warn(index, value, "%s should be a number or a block, found %s; ignored", keyChance, value.Describe())

			continue
		}

		child := r.Child(index, value, keyChance)
		state.ChanceFromState = child.NumberMap(keyIfCurrentState)
		state.ChanceDefault = child.Number(keyDefault)
		child.Finish()
	}
}

// propagation reads a propagate_state field: a block whose keys name the
// attachments and whose values are the states they are set to, or the name
// of a state for every attachment.
func propagation(r *database.Reader, index int) []Propagation {
	value := r.Fields()[index].Value

	if state, ok := value.Str(); ok {
		return []Propagation{{State: state}}
	}

	if !value.IsBlock() {
		r.Warn(index, value, "%s should be the name of a state or a block, found %s; ignored", keyPropagateState, value.Describe())

		return nil
	}

	block := database.Located{Index: index, Value: value}

	var found []Propagation

	for _, field := range block.Value.Fields {
		state, ok := field.Value.Str()
		if !ok {
			r.Warn(block.Index, field.Value, "%s.%s should be the name of a state, found %s; ignored", keyPropagateState, field.Key, field.Value.Describe())

			continue
		}

		found = append(found, Propagation{Attachment: field.Key, State: state})
	}

	for _, item := range block.Value.Items {
		r.Warn(block.Index, item, "ignored %s in %s, which has no key", item.Describe(), keyPropagateState)
	}

	return found
}

// decodeEvent reads one event or start_event block.
func decodeEvent(r *database.Reader) Event {
	defer r.Finish()

	r.Known(keyEntityEditorID)

	event := Event{
		Time:             r.NumberOr(keyTime, 0),
		Node:             r.Text(keyNode),
		Particle:         r.Text(keyParticle),
		Light:            r.Text(keyLight),
		SoundParameters:  r.NumberMap(keySoundParameter),
		Entity:           r.Text(keyEntity),
		AttachmentID:     r.Text(keyAttachmentID),
		RemoveAttachment: r.Text(keyRemoveAttachment),
		State:            r.Text(keyState),
		TriggerOnce:      r.Flag(keyTriggerOnce),
		KeepParticle:     r.Flag(keyKeepParticle),
		KeepSound:        r.Flag(keyKeepSound),
		KeepEntity:       r.Flag(keyKeepEntity),
		UseParentNodes:   r.Flag(keyUseParentNodes),
		Life:             r.Number(keyLife),
		SkipForward:      r.Number(keySkipForward),
		EntityFadeSpeed:  r.Number(keyEntityFadeSpeed),
	}

	for _, block := range r.Blocks(keySound) {
		child := r.Child(block.Index, block.Value, keySound)
		event.Sound = child.Text(keySoundEffect)
		event.SoundStopsOnStateChange = child.Flag(keyStopOnStateChange)
		child.Finish()
	}

	return event
}

// decodeLocator reads one locator block.
func decodeLocator(r *database.Reader) Locator {
	defer r.Finish()

	locator := Locator{
		Name:        r.Text(keyName),
		ParentJoint: r.Text(keyParentJoint),
		Position:    vector(r, keyPosition),
		Rotation:    vector(r, keyRotation),
		Scale:       r.NumberOr(keyScale, 1),
	}

	if locator.Name == "" {
		r.Missing(keyName, "nothing can be attached to the locator")
	}

	return locator
}

// attachments reads an attach block, whose keys are locators and whose values
// are the entities attached to them. A key that repeats attaches several
// entities to the same locator.
//
// Two keys are no locators: name, the id of the block's attachments, and
// chance, the weight of a choice of a group, which the group reads itself; in
// an attach block of an entity it is reported as unknown.
func attachments(r *database.Reader) []Attachment {
	var found []Attachment

	id := r.Text(keyName)

	for index, field := range r.Fields() {
		if field.Key == keyName || field.Key == keyChance {
			continue
		}

		r.Known(field.Key)

		entity, ok := field.Value.Str()
		if !ok {
			r.Warn(index, field.Value, "the entity attached to %s should be a name, found %s; ignored", field.Key, field.Value.Describe())

			continue
		}

		found = append(found, Attachment{Locator: field.Key, Entity: entity, ID: id})
	}

	return found
}

// decodeGroup reads one group block.
func decodeGroup(r *database.Reader) Group {
	defer r.Finish()

	group := Group{Name: r.Text(keyName)}

	for _, block := range r.Blocks(keyAttach) {
		child := r.Child(block.Index, block.Value, keyAttach)

		choice := GroupChoice{Chance: child.NumberOr(keyChance, 1)}
		choice.Attachments = attachments(child)

		child.Finish()
		group.Choices = append(group.Choices, choice)
	}

	return group
}
