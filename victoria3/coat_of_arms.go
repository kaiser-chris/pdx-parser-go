package victoria3

import (
	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// The keys a coat of arms file uses.
const (
	keyTemplate       = "template"
	keyPattern        = "pattern"
	keyColoredEmblem  = "colored_emblem"
	keyTexturedEmblem = "textured_emblem"
	keySub            = "sub"
	keyTexture        = "texture"
	keyParent         = "parent"
	keyInstance       = "instance"
	keyMask           = "mask"
	keyPosition       = "position"
	keyScale          = "scale"
	keyRotation       = "rotation"
	keyOffset         = "offset"
	keyDepth          = "depth"
	keyColors         = "colors"
)

// The canvas a coat of arms is composed on. Positions and scales in the files
// are fractions of it, and the pattern textures are of this size.
const (
	CanvasWidth  = 768
	CanvasHeight = 512
)

// CoatOfArms is one coat of arms, from common/coat_of_arms/coat_of_arms: a
// pattern with a stack of layers drawn over it.
//
//	TUR = {
//		pattern = "pattern_solid.tga"
//		color1 = "turkish_red"
//		colored_emblem = {
//			texture = "ce_crescent.dds"
//			color1 = "white"
//			instance = { position = { 0.45 0.5 } scale = { 0.6 0.6 } }
//		}
//	}
type CoatOfArms struct {
	database.Definition

	// Pattern is the background texture. A coat of arms that only draws
	// emblems has none.
	Pattern Texture

	// Colors are the slots the pattern is recoloured with, and the slots a
	// layer can refer back to.
	Colors Colors

	// Layers are what is drawn over the pattern, in the order they are drawn.
	Layers []Layer

	// Template reports whether the definition is one of the building blocks
	// in the template block of a file, which the game's random coats of arms
	// are put together from, rather than a coat of arms of its own.
	Template bool
}

// Texture names the picture a pattern or an emblem draws: a file in
// gfx/coat_of_arms, or a list the game picks a file from at random, which the
// randomly generated coats of arms use.
type Texture struct {
	// File is the file name, as the file writes it.
	File string

	// List is the name of a list in common/coat_of_arms/template_lists, when
	// the picture is written as list "pattern_anarchy".
	List string
}

// Set reports whether the texture names anything at all.
func (t Texture) Set() bool {
	return t.File != "" || t.List != ""
}

// Describe returns a short label for the texture.
func (t Texture) Describe() string {
	if t.List != "" {
		return "list " + t.List
	}

	return t.File
}

// Layer is one entry in the stack of layers drawn over a pattern: a
// *ColoredEmblem, a *TexturedEmblem or a *SubCoatOfArms.
type Layer interface {
	// Placements returns where the layer is drawn. A layer whose file gives
	// no instance is drawn once, centred at full size, which is what this
	// returns for it.
	Placements() []Instance

	layer()
}

// ColoredEmblem is an emblem whose texture is recoloured through colour
// slots. Its texture is a mask: what is red in it takes the emblem's first
// colour, what is green the second, and what is blue the third.
type ColoredEmblem struct {
	Texture Texture

	// Colors are the emblem's own slots. A slot may refer back to a slot of
	// the coat of arms the emblem belongs to.
	Colors Colors

	// Masks restricts the emblem to the parts of the pattern matching the
	// pattern's own colours: 1 is the part in the pattern's first colour.
	// The shipped files name one; nothing stops a file from naming more.
	Masks []int

	// Instances are the placements the file gives. Placements fills in the
	// default for a layer that has none.
	Instances []Instance
}

// TexturedEmblem is an emblem drawn as it is, in the colours of its own
// texture.
type TexturedEmblem struct {
	Texture Texture

	// Masks restricts the emblem to the parts of the pattern matching the
	// pattern's own colours, as it does for a coloured emblem.
	Masks []int

	// Instances are the placements the file gives.
	Instances []Instance
}

// SubCoatOfArms draws another coat of arms as a layer of this one, which is
// how quartered arms are put together.
type SubCoatOfArms struct {
	// Parent is the key of the coat of arms to draw.
	Parent string

	// Colors are handed to the parent in place of its own slots, so that the
	// same coat of arms can be drawn in the colours of the one it is part of.
	Colors Colors

	// Instances are the placements the file gives. A sub coat of arms is
	// placed by its offset rather than by its centre.
	Instances []Instance
}

func (*ColoredEmblem) layer()  {}
func (*TexturedEmblem) layer() {}
func (*SubCoatOfArms) layer()  {}

func (e *ColoredEmblem) Placements() []Instance  { return placements(e.Instances) }
func (e *TexturedEmblem) Placements() []Instance { return placements(e.Instances) }
func (s *SubCoatOfArms) Placements() []Instance  { return placements(s.Instances) }

// placements returns the instances of a layer, or the one default placement
// when it has none, which is how the games read a layer without an instance.
func placements(instances []Instance) []Instance {
	if len(instances) == 0 {
		return []Instance{DefaultInstance()}
	}

	return instances
}

// Vec2 is a position, an offset or a scale, in fractions of the canvas.
type Vec2 struct {
	X, Y float64
}

// Instance is one placement of a layer.
type Instance struct {
	// Position is the centre of the drawing, half the canvas across and half
	// of it down by default.
	Position Vec2

	// Scale is the size, as a fraction of the canvas. A negative component
	// mirrors the drawing along that axis.
	Scale Vec2

	// Rotation is in degrees, clockwise.
	Rotation float64

	// Offset moves a sub coat of arms, which is placed by its corner rather
	// than by its centre. Emblems do not use it.
	Offset Vec2

	// Depth decides the order of the layers when the files give one; the
	// shipped files of Victoria 3 and Europa Universalis 5 rely on the order
	// they are written in instead.
	Depth float64
}

// DefaultInstance is where a layer is drawn when its file gives no instance:
// centred, at the size of the whole canvas.
func DefaultInstance() Instance {
	return Instance{Position: Vec2{X: 0.5, Y: 0.5}, Scale: Vec2{X: 1, Y: 1}}
}

// decodeCoatOfArms reads one coat of arms.
//
// The fields are walked in the order they are written, because that is the
// order the layers are drawn in, and a switch over the key is what keeps the
// three kinds of layer in one stack.
func decodeCoatOfArms(r *database.Reader) *CoatOfArms {
	arms := &CoatOfArms{}

	r.Known(keyPattern, keyColoredEmblem, keyTexturedEmblem, keySub)
	r.Known(ColorSlots...)

	for index, field := range r.Fields() {
		switch {
		case field.Key == keyPattern:
			arms.Pattern = texture(r, index, keyPattern, field.Value)

		case IsColorSlot(field.Key):
			if color, ok := colorOf(r, index, field.Key, field.Value); ok {
				arms.Colors.Set(color)
			}

		case field.Key == keyColoredEmblem:
			arms.Layers = append(arms.Layers, decodeColoredEmblem(r, index, field.Value))

		case field.Key == keyTexturedEmblem:
			arms.Layers = append(arms.Layers, decodeTexturedEmblem(r, index, field.Value))

		case field.Key == keySub:
			arms.Layers = append(arms.Layers, decodeSubCoatOfArms(r, index, field.Value))
		}
	}

	if !arms.Pattern.Set() && len(arms.Layers) == 0 {
		// Not a mistake worth a warning: the games ship definitions whose
		// contents are commented out, as placeholders.
		r.NoteDefinition("draws nothing: it has neither a pattern nor a layer")
	}

	return arms
}

func decodeColoredEmblem(parent *database.Reader, index int, node script.Node) *ColoredEmblem {
	r := parent.Child(index, node, "colored emblem")
	defer r.Finish()

	emblem := &ColoredEmblem{Masks: masks(r, keyMask)}

	r.Known(keyTexture, keyInstance)
	r.Known(ColorSlots...)

	for index, field := range r.Fields() {
		switch {
		case field.Key == keyTexture:
			emblem.Texture = texture(r, index, keyTexture, field.Value)

		case IsColorSlot(field.Key):
			if color, ok := colorOf(r, index, field.Key, field.Value); ok {
				emblem.Colors.Set(color)
			}

		case field.Key == keyInstance:
			emblem.Instances = append(emblem.Instances, decodeInstance(r, index, field.Value))
		}
	}

	if !emblem.Texture.Set() {
		r.Missing(keyTexture, "the emblem draws nothing")
	}

	return emblem
}

func decodeTexturedEmblem(parent *database.Reader, index int, node script.Node) *TexturedEmblem {
	r := parent.Child(index, node, "textured emblem")
	defer r.Finish()

	emblem := &TexturedEmblem{Masks: masks(r, keyMask)}

	r.Known(keyTexture, keyInstance)

	for index, field := range r.Fields() {
		switch field.Key {
		case keyTexture:
			emblem.Texture = texture(r, index, keyTexture, field.Value)

		case keyInstance:
			emblem.Instances = append(emblem.Instances, decodeInstance(r, index, field.Value))
		}
	}

	if !emblem.Texture.Set() {
		r.Missing(keyTexture, "the emblem draws nothing")
	}

	return emblem
}

func decodeSubCoatOfArms(parent *database.Reader, index int, node script.Node) *SubCoatOfArms {
	r := parent.Child(index, node, "sub coat of arms")
	defer r.Finish()

	sub := &SubCoatOfArms{Parent: r.Text(keyParent)}

	r.Known(keyInstance)
	r.Known(ColorSlots...)

	for index, field := range r.Fields() {
		switch {
		case IsColorSlot(field.Key):
			if color, ok := colorOf(r, index, field.Key, field.Value); ok {
				sub.Colors.Set(color)
			}

		case field.Key == keyInstance:
			sub.Instances = append(sub.Instances, decodeInstance(r, index, field.Value))
		}
	}

	if sub.Parent == "" {
		r.Missing(keyParent, "there is no coat of arms to draw")
	}

	return sub
}

func decodeInstance(parent *database.Reader, index int, node script.Node) Instance {
	r := parent.Child(index, node, "instance")
	defer r.Finish()

	instance := DefaultInstance()

	instance.Position = vector(r, keyPosition, instance.Position)
	instance.Scale = vector(r, keyScale, instance.Scale)
	instance.Offset = vector(r, keyOffset, instance.Offset)
	instance.Rotation = r.NumberOr(keyRotation, 0)
	instance.Depth = r.NumberOr(keyDepth, 0)

	return instance
}

// texture reads a picture: a file name, or list "name" for a random pick.
func texture(r *database.Reader, index int, key string, value script.Node) Texture {
	if name, ok := value.Str(); ok {
		return Texture{File: name}
	}

	if value.Kind == script.KindTagged && value.Tag == tagList && value.Value != nil {
		if name, ok := value.Value.Str(); ok {
			return Texture{List: name}
		}
	}

	r.Warn(index, value, "%s should be a file name or a list, found %s; ignored", key, value.Describe())

	return Texture{}
}

// colorOf reads one colour slot.
func colorOf(r *database.Reader, index int, slot string, value script.Node) (Color, bool) {
	decoded, ok := DecodeColor(value)
	if !ok {
		r.Warn(index, value, "%s should be a colour, found %s; ignored", slot, value.Describe())

		return Color{}, false
	}

	return Color{Slot: slot, Value: decoded}, true
}

// vector reads a position, an offset or a scale, such as { 0.5 0.5 }.
//
// A single value sets the horizontal component and leaves the vertical one at
// its default. That is not a mistake in the files: the hoist stripes of Sulu
// are written scale = { 0.07 } and are meant to come out as full height bars
// a fourteenth of the flag wide.
func vector(r *database.Reader, key string, fallback Vec2) Vec2 {
	index, ok := r.Last(key)
	if !ok {
		return fallback
	}

	value := r.Fields()[index].Value

	numbers, ok := value.Numbers()
	if !ok || len(numbers) == 0 {
		r.Warn(index, value, "%s should be a pair of numbers, found %s; ignored", key, value.Describe())

		return fallback
	}

	if len(numbers) > 2 {
		r.Warn(index, value, "%s has %d values; only the first two are used", key, len(numbers))
	}

	vector := fallback
	vector.X = numbers[0]

	if len(numbers) > 1 {
		vector.Y = numbers[1]
	}

	return vector
}

// masks reads the mask of an emblem, which is written as a list of the
// pattern colours the emblem is restricted to.
func masks(r *database.Reader, key string) []int {
	var found []int

	for _, block := range r.Indices(key) {
		value := r.Fields()[block].Value

		numbers, ok := value.Numbers()
		if !ok {
			r.Warn(block, value, "%s should be a list of numbers, found %s; ignored", key, value.Describe())

			continue
		}

		for _, number := range numbers {
			found = append(found, int(number))
		}
	}

	return found
}
