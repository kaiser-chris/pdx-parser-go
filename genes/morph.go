package genes

import (
	"strings"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// MorphGene changes the shape of a portrait and lays decals over its skin.
//
//	gene_eye_size = {
//		eye_size = {
//			index = 0
//			male = {
//				setting = {
//					attribute = "head_bs_eye_size_max"
//					curve = { { 0.0 0.0 } { 0.5 0.0 } { 1.0 1.0 } }
//					age = age_preset_child_features
//				}
//			}
//			female = male
//			boy = male
//			girl = female
//		}
//	}
//
// A DNA picks one of the templates of the gene, eye_size here, and gives it a
// strength. Each setting of the template turns the strength into the value of
// an attribute, which the entities of the portrait say what to do with: the
// weight of a blend shape, or how far to play an additive animation. Two
// settings of opposite curves, as above with head_bs_eye_size_min, let one
// gene morph between two blend shapes, one at either end of its strength.
type MorphGene struct {
	database.Definition

	// Inheritable says whether a child inherits the gene from its parents,
	// when the file says.
	Inheritable database.Opt[bool]

	// Group is the group of genes the gene belongs to, when the file says,
	// such as eyes.
	Group string

	// Templates are what a DNA picks from, in the order written.
	Templates []MorphTemplate
}

// Template returns a template of the gene by name.
func (g *MorphGene) Template(name string) (*MorphTemplate, bool) {
	for index := range g.Templates {
		if g.Templates[index].Name == name {
			return &g.Templates[index], true
		}
	}

	return nil, false
}

// MorphTemplate is one of the choices of a morph gene, which a DNA names.
type MorphTemplate struct {
	Name string

	// Index is the number the template goes by in a DNA string.
	Index int

	// Sexes is what the template does for each sex, by Sex.
	Sexes [len(sexKeys)]Morph
}

// For returns what the template does for a sex.
func (t *MorphTemplate) For(sex Sex) *Morph {
	if sex < Male || sex > Girl {
		return &Morph{}
	}

	return &t.Sexes[sex]
}

// Morph is what a template of a morph gene does for one sex.
type Morph struct {
	// SameAs is the sex this one is written as being the same as, such as
	// male for boy = male, after any chain of them. What follows is then that
	// sex's.
	SameAs string

	// Settings set the attributes of the portrait.
	Settings []Setting

	// Decals are laid over the skin.
	Decals []Decal

	// TextureOverrides replace the textures of the skin, one of them picked
	// by weighted chance.
	TextureOverrides []TextureOverride

	// HSVShifts shift the hue, saturation and value of a colour of the
	// portrait, such as the greying of the hair.
	HSVShifts []HSVShift
}

// SetAttributes adds what the settings set at a strength of the gene from 0
// to 1 to the values of the attributes, by name, before age changes them. Two
// settings of one attribute add up, which is also how this package takes the
// values of an attribute several genes set to combine; the files do not say.
func (m *Morph) SetAttributes(strength float64, attributes map[string]float64) {
	for index := range m.Settings {
		setting := &m.Settings[index]
		if setting.Attribute != "" {
			attributes[setting.Attribute] += setting.At(strength)
		}
	}
}

// Setting turns the strength of a gene into the value of an attribute of the
// portrait.
//
//	setting = { attribute = "head_chin_forward" value = { min = -0.5 max = 0.5 } age = age_preset_child_features }
//	setting = { attribute = "head_bs_cheek_fat_max" curve = { { 0.5 0.0 } { 1.0 1.0 } } }
type Setting struct {
	// Attribute is the attribute of the portrait's entities the setting sets.
	Attribute string

	// Value spreads the strength over a range, when the setting gives one.
	Value database.Opt[Range]

	// Curve maps the strength to the value, when the setting gives one.
	Curve Curve

	// Age changes the value with the age of the character.
	Age Age

	// RequiredTags are the tags an entity must have for the setting to apply
	// to it.
	RequiredTags []string
}

// At returns the value of the attribute at a strength of the gene from 0 to
// 1, before age changes it: by the curve when the setting gives one, by the
// range otherwise. A setting that gives neither is taken to pass the strength
// on as it is, which the shipped files of Victoria 3 never rely on.
func (s *Setting) At(strength float64) float64 {
	if len(s.Curve) > 0 {
		return s.Curve.At(strength)
	}

	if value, ok := s.Value.Get(); ok {
		return value.At(strength)
	}

	return strength
}

// Decal is a texture laid over the skin of a part of the portrait.
type Decal struct {
	// BodyPart is the part the decal is laid over, such as head.
	BodyPart string

	// Textures are the textures of the decal, as paths below the game's root.
	Textures DecalTextures

	// BlendModes are how each texture is laid over the skin's: overlay,
	// replace, hard_light or multiply.
	BlendModes DecalTextures

	// Priority orders the decals; a higher one is laid over a lower one.
	Priority int

	// AlphaCurve maps the strength of the gene to how opaque the decal is.
	// Without one the decal is as opaque as the gene is strong.
	AlphaCurve Curve

	// Age changes how opaque the decal is with the age of the character.
	Age Age

	// ApplyOrder is whether the decal is laid over the skin before the skin
	// colour is blended in, DecalPreSkinColor, or after, DecalPostSkinColor.
	// It is empty where the file does not say, which is before: the decals
	// of the shipped files that say nothing are the wrinkles and stubble of
	// the skin, which the skin colour tints.
	ApplyOrder string
}

// The orders a decal can be laid over the skin in.
const (
	DecalPreSkinColor  = "pre_skin_color"
	DecalPostSkinColor = "post_skin_color"
)

// PostSkinColor reports whether the decal is laid over the skin after the
// skin colour is blended in.
func (d *Decal) PostSkinColor() bool {
	return d.ApplyOrder == DecalPostSkinColor
}

// Alpha returns how opaque the decal is at a strength of the gene from 0 to
// 1, before age changes it: by the alpha curve, or as opaque as the gene is
// strong where there is none, as the shipped files say of it.
func (d *Decal) Alpha(strength float64) float64 {
	if len(d.AlphaCurve) > 0 {
		return d.AlphaCurve.At(strength)
	}

	return strength
}

// DecalTextures holds one value for each texture of a decal.
type DecalTextures struct {
	Diffuse, Normal, Specular, Properties string
}

// TextureOverride replaces the textures of the skin.
type TextureOverride struct {
	// Weight is the chance of this override against the others a DNA has.
	Weight int

	// The textures, as paths below the game's root.
	Diffuse, Normal, Specular, Properties string
}

// HSVShift shifts the hue, saturation and value of a colour of the portrait.
type HSVShift struct {
	// Color is the colour shifted: hair, skin or eye.
	Color string

	// Curve maps the strength of the gene to the shift.
	Curve []HSVPoint

	// Age changes the shift with the age of the character.
	Age Age
}

// HSVPoint is one point of the curve of an HSVShift: at strength X, the
// colour is shifted by Shift, in hue, saturation and value.
type HSVPoint struct {
	X     float64
	Shift [3]float64
}

// At returns the shift at a strength of the gene from 0 to 1: each of hue,
// saturation and value along the curve as a Curve has it, linear between the
// points and flat beyond the first and the last. A shift without points
// shifts nothing.
func (s *HSVShift) At(strength float64) [3]float64 {
	var shift [3]float64

	for channel := range shift {
		curve := make(Curve, len(s.Curve))
		for index, point := range s.Curve {
			curve[index] = Point{X: point.X, Y: point.Shift[channel]}
		}

		shift[channel] = curve.At(strength)
	}

	return shift
}

// The keys of a morph gene and of the blocks inside it.
const (
	keyGroup           = "group"
	keySetting         = "setting"
	keyAttribute       = "attribute"
	keyValue           = "value"
	keyMin             = "min"
	keyMax             = "max"
	keyRequiredTags    = "required_tags"
	keyDecal           = "decal"
	keyBodyPart        = "body_part"
	keyTextures        = "textures"
	keyBlendModes      = "blend_modes"
	keyPriority        = "priority"
	keyAlphaCurve      = "alpha_curve"
	keyApplyOrder      = "decal_apply_order"
	keyTextureOverride = "texture_override"
	keyWeight          = "weight"
	keyDiffuse         = "diffuse"
	keyNormal          = "normal"
	keySpecular        = "specular"
	keyProperties      = "properties"
	suffixHSVShift     = "_hsv_shift_curve"
)

// geneFields are the keys of a gene that are not templates, for each kind of
// gene.
var (
	morphGeneFields     = []string{keyGroup, keyInheritable}
	accessoryGeneFields = []string{keyGroup, keyInheritable}
)

// templates calls read for every template of a gene: every field holding a
// block, other than the gene's own fields. A field of another kind is left
// for Finish to report as unknown.
func templates(r *database.Reader, own []string, read func(index int, field script.Field)) {
	isOwn := map[string]bool{}
	for _, key := range own {
		isOwn[key] = true
	}

	seen := map[string]bool{}

	for index, field := range r.Fields() {
		if isOwn[field.Key] || !field.Value.IsBlock() {
			continue
		}

		r.Known(field.Key)

		if seen[field.Key] {
			r.Warn(index, script.Node{}, "template %s is given more than once; each is kept", field.Key)
		}

		seen[field.Key] = true

		read(index, field)
	}
}

// decodeMorphGene reads one morph gene.
func decodeMorphGene(r *database.Reader) *MorphGene {
	gene := &MorphGene{Inheritable: r.Flag(keyInheritable), Group: r.Text(keyGroup)}

	templates(r, morphGeneFields, func(index int, field script.Field) {
		child := r.Child(index, field.Value, "template "+field.Key)
		template := MorphTemplate{Name: field.Key, Index: integer(child, keyIndex, 0)}

		if !child.Has(keyIndex) {
			child.Missing(keyIndex, "it is taken as 0")
		}

		morphs, sameAs := sexes(child, decodeMorph)
		for sex := range morphs {
			template.Sexes[sex] = morphs[sex]
			template.Sexes[sex].SameAs = sameAs[sex]
		}

		child.Finish()
		gene.Templates = append(gene.Templates, template)
	})

	return gene
}

// decodeMorph reads what a template does for one sex.
func decodeMorph(r *database.Reader) Morph {
	morph := Morph{}

	for _, block := range r.Blocks(keySetting) {
		child := r.Child(block.Index, block.Value, keySetting)
		morph.Settings = append(morph.Settings, decodeSetting(child))
		child.Finish()
	}

	for _, block := range r.Blocks(keyDecal) {
		child := r.Child(block.Index, block.Value, keyDecal)
		morph.Decals = append(morph.Decals, decodeDecal(child))
		child.Finish()
	}

	for _, block := range r.Blocks(keyTextureOverride) {
		child := r.Child(block.Index, block.Value, keyTextureOverride)
		morph.TextureOverrides = append(morph.TextureOverrides, TextureOverride{
			Weight:     integer(child, keyWeight, 0),
			Diffuse:    child.Text(keyDiffuse),
			Normal:     child.Text(keyNormal),
			Specular:   child.Text(keySpecular),
			Properties: child.Text(keyProperties),
		})
		child.Finish()
	}

	seen := map[string]bool{}

	for _, field := range r.Fields() {
		color, ok := strings.CutSuffix(field.Key, suffixHSVShift)
		if !ok || color == "" || seen[field.Key] {
			continue
		}

		seen[field.Key] = true

		for _, block := range r.Blocks(field.Key) {
			child := r.Child(block.Index, block.Value, field.Key)
			morph.HSVShifts = append(morph.HSVShifts, decodeHSVShift(child, color))
			child.Finish()
		}
	}

	return morph
}

// decodeSetting reads one setting.
func decodeSetting(r *database.Reader) Setting {
	setting := Setting{
		Attribute: r.Text(keyAttribute),
		Curve:     curve(r, keyCurve),
		Age:       age(r, keyAge),
	}

	if setting.Attribute == "" {
		r.Missing(keyAttribute, "the setting sets nothing")
	}

	// Europa Universalis 5 writes some values as one number, value = 0.7,
	// which is taken as the value at the full strength of the gene, from
	// nothing at none: the range from 0 to it. The files do not say, but it
	// is what keeps such a setting following its gene, as every other does.
	if index, ok := r.Last(keyValue); ok {
		if number, ok := r.Fields()[index].Value.Num(); ok {
			setting.Value = database.Some(Range{Min: 0, Max: number})
		} else {
			setting.Value = valueRange(r)
		}
	}

	setting.RequiredTags = tags(r, keyRequiredTags)

	return setting
}

// valueRange reads the range a setting spreads the strength over, value = {
// min = -0.5 max = 0.5 }: the last one written that gives both ends.
func valueRange(r *database.Reader) database.Opt[Range] {
	var value database.Opt[Range]

	for _, block := range r.Blocks(keyValue) {
		child := r.Child(block.Index, block.Value, keyValue)

		low, high := child.Number(keyMin), child.Number(keyMax)
		if !low.Set || !high.Set {
			child.Missing("min or max", "the value is ignored")
		} else {
			value = database.Some(Range{Min: low.Value, Max: high.Value})
		}

		child.Finish()
	}

	return value
}

// decodeDecal reads one decal.
func decodeDecal(r *database.Reader) Decal {
	decal := Decal{
		BodyPart:   r.Text(keyBodyPart),
		Priority:   integer(r, keyPriority, 0),
		AlphaCurve: curve(r, keyAlphaCurve),
		Age:        age(r, keyAge),
		ApplyOrder: r.Text(keyApplyOrder),
	}

	switch decal.ApplyOrder {
	case "", DecalPreSkinColor, DecalPostSkinColor:
	default:
		index, _ := r.Last(keyApplyOrder)
		r.Warn(index, script.Node{}, "%s %s is not one this package knows of, %s or %s; laid before the skin colour", keyApplyOrder, decal.ApplyOrder, DecalPreSkinColor, DecalPostSkinColor)
		decal.ApplyOrder = ""
	}

	decal.Textures = decalTextures(r, keyTextures)
	decal.BlendModes = decalTextures(r, keyBlendModes)

	return decal
}

// decalTextures reads a block holding a value for each texture of a decal.
// Several blocks add up, a later one replacing only what it names, as
// Europa Universalis 5 writes a diffuse map and a normal map each in a block
// of its own:
//
//	textures = { diffuse = "...diffuse.dds" }
//	textures = { normal = "...normal.dds" }
func decalTextures(r *database.Reader, key string) DecalTextures {
	var textures DecalTextures

	for _, block := range r.Blocks(key) {
		child := r.Child(block.Index, block.Value, key)

		for _, field := range []struct {
			key    string
			target *string
		}{{keyDiffuse, &textures.Diffuse}, {keyNormal, &textures.Normal}, {keySpecular, &textures.Specular}, {keyProperties, &textures.Properties}} {
			if value := child.Text(field.key); value != "" {
				*field.target = value
			}
		}

		child.Finish()
	}

	return textures
}

// decodeHSVShift reads the shift of one colour.
func decodeHSVShift(r *database.Reader, color string) HSVShift {
	shift := HSVShift{Color: color, Age: age(r, keyAge)}

	index, ok := r.Last(keyCurve)
	if !ok {
		return shift
	}

	value := r.Fields()[index].Value

	if value.Kind != script.KindList {
		if !value.IsBlock() || len(value.Fields) > 0 {
			r.Warn(index, value, "%s should be a list of points, found %s; ignored", keyCurve, value.Describe())
		}

		return shift
	}

	for _, item := range value.Items {
		point, ok := hsvPoint(item)
		if !ok {
			r.Warn(index, item, "a point of %s should be a number and three numbers, such as { 0.0 { 0.0 -0.5 0.0 } }; left out", keyCurve)

			continue
		}

		shift.Curve = append(shift.Curve, point)
	}

	return shift
}

// hsvPoint reads a point such as { 0.0 { 0.0 -0.5 0.0 } }.
func hsvPoint(item script.Node) (HSVPoint, bool) {
	if item.Kind != script.KindList || len(item.Items) != 2 {
		return HSVPoint{}, false
	}

	x, ok := item.Items[0].Num()
	if !ok {
		return HSVPoint{}, false
	}

	shift, ok := item.Items[1].Numbers()
	if !ok || len(shift) != 3 {
		return HSVPoint{}, false
	}

	return HSVPoint{X: x, Shift: [3]float64(shift)}, true
}
