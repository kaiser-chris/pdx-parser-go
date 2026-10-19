package portrait

import (
	"strconv"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// FolderEnvironments is where the environments of portraits are.
var FolderEnvironments = folders.Folder{Path: "gfx/portraits/environments"}

// kindEnvironment is what an environment goes by in diagnostics.
const kindEnvironment = "portrait environment"

// Environment is how a portrait is lit: the cubemap of its surroundings,
// which it reflects and which lights it softly from every side, and the
// lights that light it.
type Environment struct {
	database.Definition

	// Cubemap is the path of the cubemap below the folders' root, and
	// CubemapIntensity how bright it lights the portrait.
	Cubemap          string
	CubemapIntensity database.Opt[float64]

	// Lights are the lights, in the order of their numbers.
	Lights []Light

	// ShadowCamera is where the shadows are cast from, when the file says.
	// An environment can name another instead, shadow_camera = full_torso,
	// whose shadow camera it casts from: ShadowCameraOf is that name, and
	// the loaders give it the other's camera where the other has one.
	ShadowCamera   database.Opt[Camera]
	ShadowCameraOf string
}

// The types of light.
const (
	LightSpot        = "spot_light"
	LightPoint       = "point_light"
	LightDirectional = "directional_light"
)

// Light is one light of an environment.
type Light struct {
	// Number is the number the file gives the light.
	Number int

	// Type is one of LightSpot, LightPoint and LightDirectional.
	Type string

	// AffectedByShadow says whether the portrait's shadow darkens the light.
	AffectedByShadow bool

	// Color is the light's colour, as bright as the light is.
	Color Color

	// Position is where a spot or point light stands, measured from the
	// game's origin, the portrait's feet. LookAt is the point a spot or
	// directional light points at, measured from LookAtNode, a bone of the
	// portrait, where the file names one.
	Position   Position
	LookAt     [3]float64
	LookAtNode string

	// Radius is how far a spot or point light reaches, and Falloff over how
	// much of that its light fades to nothing at the end.
	Radius, Falloff float64

	// OuterConeAngle and InnerConeAngle are the angles of a spot light's
	// cone, in degrees, from its edge, where its light is gone, to the
	// middle it lights fully.
	OuterConeAngle, InnerConeAngle float64
}

// The keys of an environment and its lights.
const (
	keyCubemap          = "cubemap"
	keyCubemapIntensity = "cubemap_intensity"
	keyLights           = "lights"
	keyShadowCamera     = "shadow_camera"
	keyType             = "type"
	keyAffectedByShadow = "affected_by_shadow"
	keyColor            = "color"
	keyPosition         = "position"
	keyPositionNode     = "position_node"
	keyLookAt           = "look_at"
	keyLookAtNode       = "look_at_node"
	keyRadius           = "radius"
	keyFalloff          = "falloff"
	keyOuterConeAngle   = "outer_cone_angle"
	keyInnerConeAngle   = "inner_cone_angle"
)

// LoadEnvironments reads the environments of a set of folders, by name.
//
// Loading never fails. Everything worth knowing about the files is in the
// diagnostics.
func LoadEnvironments(set *folders.Set) (database.Table[Environment], report.Diagnostics) {
	collector := &report.Collector{}
	store := database.Collect(set, FolderEnvironments, kindEnvironment, collector)

	table := database.Decode(store, collector, func(r *database.Reader, _ *database.Definition) *Environment {
		return decodeEnvironment(r)
	})
	borrowShadowCameras(table, collector)

	return table, collector.Diagnostics
}

// ReadEnvironments reads the environments of one file that has already been
// parsed.
func ReadEnvironments(document *script.Document, file folders.File, collector *report.Collector) database.Table[Environment] {
	store := database.NewStore(kindEnvironment)
	store.AddDocument(document, file, collector)

	table := database.Decode(store, collector, func(r *database.Reader, _ *database.Definition) *Environment {
		return decodeEnvironment(r)
	})
	borrowShadowCameras(table, collector)

	return table
}

// borrowShadowCameras gives each environment that names another for its
// shadow camera the other's. One that names an environment that is not
// there, or that has no shadow camera of its own, is reported and casts no
// shadows.
func borrowShadowCameras(table database.Table[Environment], collector *report.Collector) {
	for key, environment := range table.All() {
		if environment.ShadowCameraOf == "" {
			continue
		}

		other, ok := table.Get(environment.ShadowCameraOf)
		if ok && other.ShadowCamera.Set {
			environment.ShadowCamera = other.ShadowCamera

			continue
		}

		origin := environment.Origin()
		collector.Addf(report.SeverityWarning, origin.Source, origin.Path, 0, 0, kindEnvironment+" "+key, "casts its shadows from those of %s, which has no shadow camera of its own; it casts none", environment.ShadowCameraOf)
	}
}

// decodeEnvironment reads one environment.
func decodeEnvironment(r *database.Reader) *Environment {
	environment := &Environment{
		Cubemap:          r.Text(keyCubemap),
		CubemapIntensity: r.Number(keyCubemapIntensity),
	}

	for _, block := range r.Blocks(keyLights) {
		lights := r.Child(block.Index, block.Value, keyLights)

		for index, field := range block.Value.Fields {
			number, err := strconv.Atoi(field.Key)
			if err != nil {
				// Left for Finish to report as unknown.
				continue
			}

			lights.Known(field.Key)

			if !field.Value.IsBlock() {
				lights.Warn(index, field.Value, "light %d should be a block, found %s; ignored", number, field.Value.Describe())

				continue
			}

			child := lights.Child(index, field.Value, "light "+field.Key)
			environment.Lights = append(environment.Lights, decodeLight(child, number))
			child.Finish()
		}

		lights.Finish()
	}

	if index, ok := r.Last(keyShadowCamera); ok {
		value := r.Fields()[index].Value

		switch name, named := value.Str(); {
		case value.IsBlock():
			child := r.Child(index, value, keyShadowCamera)
			environment.ShadowCamera = database.Some(decodeCamera(child))
			child.Finish()
		case named:
			environment.ShadowCameraOf = name
		default:
			r.Warn(index, value, "%s should be a block or the name of an environment, found %s; ignored", keyShadowCamera, value.Describe())
		}
	}

	return environment
}

// decodeLight reads one light.
func decodeLight(r *database.Reader, number int) Light {
	light := Light{
		Number:           number,
		Type:             r.Text(keyType),
		AffectedByShadow: r.FlagOr(keyAffectedByShadow, false),
		LookAt:           vector(r, keyLookAt),
		LookAtNode:       r.Text(keyLookAtNode),
		Radius:           r.NumberOr(keyRadius, 0),
		Falloff:          r.NumberOr(keyFalloff, 0),
		OuterConeAngle:   r.NumberOr(keyOuterConeAngle, 0),
		InnerConeAngle:   r.NumberOr(keyInnerConeAngle, 0),
	}

	if light.Type == "" {
		r.Missing(keyType, "the light is taken for a spot light")
		light.Type = LightSpot
	}

	light.Color, _ = color(r, keyColor)
	light.Position, _ = position(r, keyPosition)

	return light
}
