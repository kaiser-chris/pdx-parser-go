package portrait

import (
	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// FileSettings is the file of the settings every environment shares.
const FileSettings = "gfx/portraits/jomini_environment.txt"

// kindSettings is what the settings go by in diagnostics.
const kindSettings = "portrait settings"

// Settings are what every environment shares: how the light the portrait
// reflects is turned into the colours of the picture.
type Settings struct {
	// SaturationScale and ValueScale scale the saturation and the brightness
	// of the picture, and ColorBalance its red, green and blue.
	SaturationScale, ValueScale database.Opt[float64]
	ColorBalance                database.Opt[[3]float64]

	// LevelsMin and LevelsMax are the darkest and the brightest colour the
	// picture's levels are stretched between.
	LevelsMin, LevelsMax database.Opt[Color]

	// ShadowmapKernelScale and ShadowmapFadeFactor soften the shadows.
	ShadowmapKernelScale, ShadowmapFadeFactor database.Opt[float64]

	// Tonemap is the curve the light is turned into colours by, when the
	// file says.
	Tonemap database.Opt[Tonemap]

	// TonemapFunction is which curve the light is turned into colours by, such
	// as Uncharted for the curve of Tonemap, and ExposureFunction how the
	// light is scaled before, such as FixedExposure for Exposure, when the file
	// says. Where it does not, the engine picks, which no file shows.
	TonemapFunction, ExposureFunction string
	Exposure                          database.Opt[float64]

	// HueOffset turns the hue of the picture, as SaturationScale and
	// ValueScale scale the rest of it.
	HueOffset database.Opt[float64]

	// Bloom is the glow the brightest parts of the picture shed over it, when
	// the file says.
	Bloom database.Opt[Bloom]

	// SSAO is how the portrait is darkened where its surfaces are hidden from
	// the light around them, when the file says.
	SSAO database.Opt[SSAO]
}

// SSAO is the ambient occlusion the engine works out from the depth of the
// picture: how many samples it takes and how far around a pixel, how strongly
// and how sharply the occlusion is laid over the picture, and how it is
// blurred.
type SSAO struct {
	Enabled               bool
	Samples               int
	Radius, MaxRadius     float64
	DepthDistance         float64
	MaxDepth              float64
	ApplyScale, ApplyBias float64
	ApplyPower            float64
	BlendFactor           float64
	BlurEnabled           bool
	BlurWidth, BlurScale  float64
	BlurDepthPower        float64
	BlurWeights           int
}

// Bloom is the glow the engine lays over a picture: a blurred copy of it,
// of the parts brighter than the threshold, the steeper the more sharply,
// blended in by the strength.
type Bloom struct {
	Enabled                        bool
	Strength, Steepness, Threshold float64
}

// Tonemap is the filmic curve of John Hable, which the engine turns the light
// of a portrait into the colours of its picture by: its shoulder, its linear
// part and its toe, and the light it takes for white.
type Tonemap struct {
	ShoulderStrength, LinearStrength, LinearAngle float64
	ToeStrength, ToeNumerator, ToeDenominator     float64
	LinearWhite                                   float64
}

// The keys of the settings.
const (
	keySaturationScale      = "saturation_scale"
	keyValueScale           = "value_scale"
	keyColorBalance         = "colorbalance"
	keyLevelsMin            = "levels_min"
	keyLevelsMax            = "levels_max"
	keyShadowmapKernelScale = "shadowmap_kernelscale"
	keyShadowmapFadeFactor  = "shadowmap_fadefactor"
	keyTonemapCurve         = "tonemap_curve"
	keyShoulderStrength     = "shoulder_strength"
	keyLinearStrength       = "linear_strength"
	keyLinearAngle          = "linear_angle"
	keyToeStrength          = "toe_strength"
	keyToeNumerator         = "toe_numerator"
	keyToeDenominator       = "toe_denominator"
	keyLinearWhite          = "linear_white"
	keyTonemapFunction      = "tonemap_function"
	keyExposureFunction     = "exposure_function"
	keyExposure             = "exposure"
	keyHueOffset            = "hue_offset"
	keyBloomEnabled         = "bloom_enabled"
	keyBloomStrength        = "bloom_strength"
	keyBloomSteepness       = "bloom_steepness"
	keyBloomThreshold       = "bloom_threshold"
)

// LoadSettings reads the settings of a set of folders: the last of them that
// has the file, as the game reads a single file. Folders without it give
// settings of nothing, and say so.
//
// Loading never fails. Everything worth knowing about the file is in the
// diagnostics.
func LoadSettings(set *folders.Set) (Settings, report.Diagnostics) {
	collector := &report.Collector{}

	file, ok := set.Find(FileSettings)
	if !ok {
		collector.Addf(report.SeverityWarning, "", FileSettings, 0, 0, kindSettings, "%s is in none of the folders; the portrait is drawn with no settings", FileSettings)

		return Settings{}, collector.Diagnostics
	}

	parsed := database.ParseFiles([]folders.File{file}, collector)
	if len(parsed) == 0 {
		return Settings{}, collector.Diagnostics
	}

	return ReadSettings(parsed[0].Document, file, collector), collector.Diagnostics
}

// ReadSettings reads the settings of a file that has already been parsed.
func ReadSettings(document *script.Document, file folders.File, collector *report.Collector) Settings {
	definition := &database.Definition{
		Key:     "jomini_environment",
		Origins: []database.Origin{{Source: file.Source, File: file.Relative, Path: file.Path}},
		Raw:     script.Node{Kind: script.KindBlock, Fields: document.Fields},
	}

	r := database.NewReader(definition, kindSettings, collector)
	r.Known(settingsOthers...)

	settings := Settings{
		SaturationScale:      r.Number(keySaturationScale),
		ValueScale:           r.Number(keyValueScale),
		ShadowmapKernelScale: r.Number(keyShadowmapKernelScale),
		ShadowmapFadeFactor:  r.Number(keyShadowmapFadeFactor),
		TonemapFunction:      r.Text(keyTonemapFunction),
		ExposureFunction:     r.Text(keyExposureFunction),
		Exposure:             r.Number(keyExposure),
		HueOffset:            r.Number(keyHueOffset),
	}

	if r.Has(keyBloomEnabled) || r.Has(keyBloomStrength) {
		settings.Bloom = database.Some(Bloom{
			Enabled:   r.FlagOr(keyBloomEnabled, false),
			Strength:  r.NumberOr(keyBloomStrength, 0),
			Steepness: r.NumberOr(keyBloomSteepness, 0),
			Threshold: r.NumberOr(keyBloomThreshold, 0),
		})
	}

	if r.Has(keyColorBalance) {
		settings.ColorBalance = database.Some(vector(r, keyColorBalance))
	}

	if value, ok := color(r, keyLevelsMin); ok {
		settings.LevelsMin = database.Some(value)
	}

	if value, ok := color(r, keyLevelsMax); ok {
		settings.LevelsMax = database.Some(value)
	}

	if blocks := r.Blocks(keyTonemapCurve); len(blocks) > 0 {
		last := blocks[len(blocks)-1]
		child := r.Child(last.Index, last.Value, keyTonemapCurve)

		settings.Tonemap = database.Some(Tonemap{
			ShoulderStrength: child.NumberOr(keyShoulderStrength, 0),
			LinearStrength:   child.NumberOr(keyLinearStrength, 0),
			LinearAngle:      child.NumberOr(keyLinearAngle, 0),
			ToeStrength:      child.NumberOr(keyToeStrength, 0),
			ToeNumerator:     child.NumberOr(keyToeNumerator, 0),
			ToeDenominator:   child.NumberOr(keyToeDenominator, 0),
			LinearWhite:      child.NumberOr(keyLinearWhite, 0),
		})

		child.Finish()
	}

	if blocks := r.Blocks(keySSAO); len(blocks) > 0 {
		last := blocks[len(blocks)-1]
		child := r.Child(last.Index, last.Value, keySSAO)

		settings.SSAO = database.Some(SSAO{
			Enabled:        child.FlagOr(keyEnabled, false),
			Samples:        int(child.NumberOr(keySamples, 0)),
			Radius:         child.NumberOr(keyRadius, 0),
			MaxRadius:      child.NumberOr(keyMaxRadius, 0),
			DepthDistance:  child.NumberOr(keyDepthDistance, 0),
			MaxDepth:       child.NumberOr(keyMaxDepth, 0),
			ApplyScale:     child.NumberOr(keyApplyScale, 0),
			ApplyBias:      child.NumberOr(keyApplyBias, 0),
			ApplyPower:     child.NumberOr(keyApplyPower, 0),
			BlendFactor:    child.NumberOr(keyBlendFactor, 0),
			BlurEnabled:    child.FlagOr(keyBlurEnabled, false),
			BlurWidth:      child.NumberOr(keyBlurWidth, 0),
			BlurScale:      child.NumberOr(keyBlurScale, 0),
			BlurDepthPower: child.NumberOr(keyBlurDepthPower, 0),
			BlurWeights:    int(child.NumberOr(keyBlurWeights, 0)),
		})

		child.Finish()
	}

	r.Finish()

	return settings
}

// The keys of the ambient occlusion settings.
const (
	keySSAO           = "ssao"
	keyEnabled        = "enabled"
	keySamples        = "samples"
	keyMaxRadius      = "max_radius"
	keyDepthDistance  = "depth_distance"
	keyMaxDepth       = "max_depth"
	keyApplyScale     = "apply_scale"
	keyApplyBias      = "apply_bias"
	keyApplyPower     = "apply_power"
	keyBlendFactor    = "blend_factor"
	keyBlurEnabled    = "blur_enabled"
	keyBlurWidth      = "blur_width"
	keyBlurScale      = "blur_scale"
	keyBlurDepthPower = "blur_depth_power"
	keyBlurWeights    = "blur_weights"
)

// settingsOthers are the settings of the file a portrait is not drawn with:
// the sun, the ambient light, the fog and the water of the map, which some
// games keep in the same file, the lens flares, which are off, and how the
// exposure adapts to a picture, which only the map's does.
var settingsOthers = []string{
	"ambient_pos_x", "ambient_neg_x", "ambient_pos_y", "ambient_neg_y", "ambient_pos_z", "ambient_neg_z",
	"shadow_ambient_pos_x", "shadow_ambient_neg_x", "shadow_ambient_pos_y", "shadow_ambient_neg_y", "shadow_ambient_pos_z", "shadow_ambient_neg_z",
	"sun_color", "sun_intensity", "sun_direction", "shadow_direction_offset",
	"cubemap", "cubemap_intensity", "cubemap_y_rotation",
	"fog_color", "fog_begin", "fog_end", "fog_max", "water_sun_direction_offset",
	"hdr_min_adjustment", "hdr_max_adjustment", "hdr_adjustment_speed",
	"tonemap_middlegrey", "tonemap_whiteluminance", "shadowmap_depthbias",
	"lensflare_dirt", "lensflare", "additional_lensflare",
}
