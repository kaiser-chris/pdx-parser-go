package asset

import (
	"math"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// Mesh is a pdxmesh definition: a .mesh file, with the textures and shaders
// its parts are drawn with and the animations and blend shapes it can play.
//
//	pdxmesh = {
//		name = "female_belt_01_mesh"
//		file = "female_belt_01.mesh"
//		blend_shape = { id = "female_belt_01_bs_fat" type = "female_belt_01_bs_fat.mesh" }
//		meshsettings = {
//			name = "female_belt_01Shape"
//			index = 0
//			texture_diffuse = "female_belt_01_diffuse.dds"
//			shader = "portrait_attachment"
//			shader_file = "gfx/FX/jomini/portrait.shader"
//		}
//	}
//
// The Key of the embedded Definition is the mesh's name, which is how
// entities refer to it.
type Mesh struct {
	database.Definition

	// File is the .mesh file as written, relative to the folder of the asset
	// file. Resolve turns it into a path below the game's root.
	File string

	// Scale resizes the whole mesh. It is 1 when the file gives none.
	Scale float64

	// CullDistance is how far away the mesh stops being drawn, when the file
	// says.
	CullDistance database.Opt[float64]

	// Settings say how the parts of the mesh are drawn, one entry for each
	// part the file names, in the order written.
	Settings []MeshSettings

	// Animations are the animations the mesh can play, by the id the states
	// of an entity refer to them by.
	Animations []Animation

	// AdditiveAnimations are animations played on top of the others, such as
	// the proportions of a child's body, by the id an entity attribute refers
	// to them by.
	AdditiveAnimations []Animation

	// BlendShapes are shapes the mesh can be morphed towards, such as a fatter
	// or older version of itself, by the id an entity attribute refers to
	// them by.
	BlendShapes []BlendShape

	// LODs are the levels of detail, by the share of the screen below which
	// each takes over.
	LODs []LOD
}

// Animation is an animation file a mesh can play.
type Animation struct {
	// ID is the name the animation is referred to by.
	ID string

	// File is the .anim file as written, relative to the folder of the asset
	// file.
	File string
}

// BlendShape is a shape a mesh can be morphed towards.
type BlendShape struct {
	// ID is the name the shape is referred to by.
	ID string

	// File is the .mesh file holding the shape, as written, relative to the
	// folder of the asset file.
	File string

	// Data names the vertex data the shape changes, as written, such as
	// "normal|tangent". Europa Universalis 5 writes it; it is empty when the
	// file gives none.
	Data string
}

// LOD is one level of detail of a mesh.
type LOD struct {
	// Index is the level: 1 is the first simplified version of the mesh.
	Index int

	// Percent is the share of the screen, in percent, below which this level
	// is drawn. The files write inf for a level that is always drawn, which
	// is read as positive infinity.
	Percent float64
}

// MeshSettings say how one part of a mesh is drawn. The engine matches them to
// the parts of the .mesh file by Name.
type MeshSettings struct {
	// Name is the part of the mesh these settings are for, as the .mesh file
	// names it.
	Name string

	// Index tells apart settings for the same part. The shipped files number
	// them from 0.
	Index int

	// The three textures every shader takes, as written: a file name, which
	// the engine looks for next to the asset file first.
	Diffuse  string
	Normal   string
	Specular string

	// Textures are further textures for the shader, by the slot each goes in.
	Textures []Texture

	// Shader is the effect within ShaderFile the part is drawn with, and
	// ShaderFile the shader file, below the game's root.
	Shader     string
	ShaderFile string

	// ShadowShader replaces the shader the part casts its shadow with. The
	// shipped files only ever set it to "", for no shadow.
	ShadowShader database.Opt[string]

	// ShadowOnly draws nothing but the part's shadow.
	ShadowOnly bool

	// ShaderDefines switch on variants of the shader.
	ShaderDefines []string

	// Subpass is the pass of the renderer the part is drawn in, such as
	// Decals.
	Subpass string
}

// Texture is a texture for a further slot of a shader.
type Texture struct {
	// Index is the slot.
	Index int

	// File is the texture as written.
	File string

	// SRGB says the texture holds colours rather than data, so the engine
	// converts it from sRGB as it samples it.
	SRGB bool
}

// The keys of a pdxmesh definition and of the blocks inside it.
const (
	keyFile              = "file"
	keyScale             = "scale"
	keyCullDistance      = "cull_distance"
	keyMeshSettings      = "meshsettings"
	keyAnimation         = "animation"
	keyAdditiveAnimation = "additive_animation"
	keyBlendShape        = "blend_shape"
	keyLODPercentages    = "lod_percentages"
	keyLOD               = "lod"
	keyIndex             = "index"
	keyPercent           = "percent"
	keyID                = "id"
	keyType              = "type"
	keyData              = "data"
	keyTextureDiffuse    = "texture_diffuse"
	keyTextureNormal     = "texture_normal"
	keyTextureSpecular   = "texture_specular"
	keyTexture           = "texture"
	keySRGB              = "srgb"
	keyShader            = "shader"
	keyShaderFile        = "shader_file"
	keyShadowShader      = "shadow_shader"
	keyShadowOnly        = "shadow_only"
	keyShaderDefines     = "additional_shader_defines"
	keySubpass           = "subpass"
	infinitePercent      = "inf"
)

// decodeMesh reads one pdxmesh definition.
func decodeMesh(r *database.Reader) *Mesh {
	r.Known(keyName)

	mesh := &Mesh{
		File:         r.Text(keyFile),
		Scale:        r.NumberOr(keyScale, 1),
		CullDistance: r.Number(keyCullDistance),
	}

	if mesh.File == "" {
		r.Missing(keyFile, "the mesh has nothing to draw")
	}

	for _, block := range r.Blocks(keyMeshSettings) {
		mesh.Settings = append(mesh.Settings, decodeMeshSettings(r.Child(block.Index, block.Value, keyMeshSettings)))
	}

	mesh.Animations = animations(r, keyAnimation)
	mesh.AdditiveAnimations = animations(r, keyAdditiveAnimation)

	for _, block := range r.Blocks(keyBlendShape) {
		child := r.Child(block.Index, block.Value, keyBlendShape)
		shape := BlendShape{ID: child.Text(keyID), File: child.Text(keyType), Data: child.Text(keyData)}

		if shape.ID == "" {
			child.Missing(keyID, "the shape cannot be referred to")
		}

		child.Finish()
		mesh.BlendShapes = append(mesh.BlendShapes, shape)
	}

	for _, block := range r.Blocks(keyLODPercentages) {
		mesh.LODs = append(mesh.LODs, lods(r.Child(block.Index, block.Value, keyLODPercentages))...)
	}

	return mesh
}

// decodeMeshSettings reads one meshsettings block, of a mesh or of an entity.
func decodeMeshSettings(r *database.Reader) MeshSettings {
	defer r.Finish()

	settings := MeshSettings{
		Name:          r.Text(keyName),
		Index:         integer(r, keyIndex, 0),
		Diffuse:       r.Text(keyTextureDiffuse),
		Normal:        r.Text(keyTextureNormal),
		Specular:      r.Text(keyTextureSpecular),
		Shader:        r.Text(keyShader),
		ShaderFile:    r.Text(keyShaderFile),
		ShadowOnly:    r.FlagOr(keyShadowOnly, false),
		ShaderDefines: r.Names(keyShaderDefines),
		Subpass:       r.Text(keySubpass),
	}

	// An empty shadow shader means something, no shadow, so it is told apart
	// from one that is not given.
	if r.Has(keyShadowShader) {
		settings.ShadowShader = database.Some(r.Text(keyShadowShader))
	}

	for _, block := range r.Blocks(keyTexture) {
		child := r.Child(block.Index, block.Value, keyTexture)

		settings.Textures = append(settings.Textures, Texture{
			Index: integer(child, keyIndex, 0),
			File:  child.Text(keyFile),
			SRGB:  child.FlagOr(keySRGB, false),
		})

		child.Finish()
	}

	return settings
}

// animations reads every animation block of a key.
func animations(r *database.Reader, key string) []Animation {
	var found []Animation

	for _, block := range r.Blocks(key) {
		child := r.Child(block.Index, block.Value, key)
		animation := Animation{ID: child.Text(keyID), File: child.Text(keyType)}

		if animation.ID == "" {
			child.Missing(keyID, "the animation cannot be referred to")
		}

		child.Finish()
		found = append(found, animation)
	}

	return found
}

// lods reads the levels of detail of a lod_percentages block.
func lods(r *database.Reader) []LOD {
	defer r.Finish()

	var found []LOD

	for _, block := range r.Blocks(keyLOD) {
		child := r.Child(block.Index, block.Value, keyLOD)
		lod := LOD{Index: integer(child, keyIndex, 0)}

		index, ok := child.Last(keyPercent)
		if ok {
			lod.Percent = percent(child, index, child.Fields()[index].Value)
		}

		child.Finish()
		found = append(found, lod)
	}

	return found
}

// percent reads the share of the screen of a level of detail, which is a
// number or inf.
func percent(r *database.Reader, index int, value script.Node) float64 {
	if number, ok := value.Num(); ok {
		return number
	}

	if text, ok := value.Str(); ok && text == infinitePercent {
		return math.Inf(1)
	}

	r.Warn(index, value, "%s should be a number or %s, found %s; ignored", keyPercent, infinitePercent, value.Describe())

	return 0
}
