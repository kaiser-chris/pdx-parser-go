package asset

import (
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// testFile is where read places the input, so that Resolve has a folder to
// resolve against.
var testFile = folders.File{Relative: "gfx/models/test/test.asset", Path: "test.asset", Source: "test"}

// read reads one file of input.
func read(t *testing.T, input string) (*Assets, report.Diagnostics) {
	t.Helper()

	collector := &report.Collector{}
	document := script.Parse(input)

	for _, warning := range document.Warnings {
		collector.Addf(report.SeverityWarning, testFile.Source, testFile.Path, warning.Line, warning.Column, "", "%s", warning.Message)
	}

	return Read(document, testFile, collector), collector.Diagnostics
}

func mentions(diagnostics report.Diagnostics, severity report.Severity, fragment string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == severity && strings.Contains(diagnostic.String(), fragment) {
			return true
		}
	}

	return false
}

func expect(t *testing.T, diagnostics report.Diagnostics, severity report.Severity, fragment string) {
	t.Helper()

	if !mentions(diagnostics, severity, fragment) {
		var lines []string
		for _, diagnostic := range diagnostics {
			lines = append(lines, "  "+diagnostic.String())
		}

		t.Errorf("no %s mentioning %q; got:\n%s", severity, fragment, strings.Join(lines, "\n"))
	}
}

// expectClean fails on any warning or error.
func expectClean(t *testing.T, diagnostics report.Diagnostics) {
	t.Helper()

	for _, diagnostic := range diagnostics.Filter(report.SeverityWarning) {
		t.Errorf("unexpected %s", diagnostic)
	}
}

func TestMesh(t *testing.T) {
	assets, diagnostics := read(t, `
pdxmesh = {
	name = "body_mesh"
	file = "body.mesh"
	scale = 1.5
	cull_distance = 5000

	animation = { id = "idle" type = "../animation/body_idle.anim" }
	additive_animation = { id = "height" type = "body_height.anim" }
	blend_shape = { id = "body_bs_fat" type = "blendshapes/body_bs_fat.mesh" data = "normal|tangent" }

	lod_percentages = {
		lod = { index = 1 percent = 12.5 }
		lod = { index = 2 percent = inf }
	}

	meshsettings = {
		name = "bodyShape"
		index = 0
		texture_diffuse = "body_diffuse.dds"
		texture_normal = "body_normal.dds"
		texture_specular = "body_properties.dds"
		texture = { index = 3 file = "body_mask.dds" srgb = yes }
		shader = "portrait_skin"
		shader_file = "gfx/FX/jomini/portrait.shader"
		shadow_shader = ""
		additional_shader_defines = { "PDX_MESH_BLENDSHAPES" "SKIN" }
		subpass = "Decals"
	}
	meshsettings = { name = "bodyShape" index = 1 shadow_only = yes }
}
`)

	expectClean(t, diagnostics)

	mesh, ok := assets.Meshes.Get("body_mesh")
	if !ok {
		t.Fatal("body_mesh is missing")
	}

	if mesh.File != "body.mesh" || mesh.Scale != 1.5 || mesh.CullDistance != database.Some(5000.0) {
		t.Errorf("mesh = %+v", mesh)
	}

	if !slices.Equal(mesh.Animations, []Animation{{ID: "idle", File: "../animation/body_idle.anim"}}) ||
		!slices.Equal(mesh.AdditiveAnimations, []Animation{{ID: "height", File: "body_height.anim"}}) {
		t.Errorf("animations = %+v, additive = %+v", mesh.Animations, mesh.AdditiveAnimations)
	}

	if !slices.Equal(mesh.BlendShapes, []BlendShape{{ID: "body_bs_fat", File: "blendshapes/body_bs_fat.mesh", Data: "normal|tangent"}}) {
		t.Errorf("blend shapes = %+v", mesh.BlendShapes)
	}

	if len(mesh.LODs) != 2 || mesh.LODs[0] != (LOD{Index: 1, Percent: 12.5}) || mesh.LODs[1].Index != 2 || !math.IsInf(mesh.LODs[1].Percent, 1) {
		t.Errorf("levels of detail = %+v, want inf read as infinity", mesh.LODs)
	}

	if len(mesh.Settings) != 2 {
		t.Fatalf("settings = %+v", mesh.Settings)
	}

	settings := mesh.Settings[0]
	if settings.Name != "bodyShape" || settings.Index != 0 || settings.Diffuse != "body_diffuse.dds" ||
		settings.Normal != "body_normal.dds" || settings.Specular != "body_properties.dds" ||
		settings.Shader != "portrait_skin" || settings.ShaderFile != "gfx/FX/jomini/portrait.shader" ||
		settings.ShadowShader != database.Some("") || settings.ShadowOnly || settings.Subpass != "Decals" ||
		!slices.Equal(settings.ShaderDefines, []string{"PDX_MESH_BLENDSHAPES", "SKIN"}) ||
		!slices.Equal(settings.Textures, []Texture{{Index: 3, File: "body_mask.dds", SRGB: true}}) {
		t.Errorf("settings = %+v", settings)
	}

	// The second part sets no shadow shader, which is not the same as setting
	// an empty one.
	if second := mesh.Settings[1]; second.Index != 1 || !second.ShadowOnly || second.ShadowShader.Set {
		t.Errorf("second settings = %+v", second)
	}
}

func TestMeshDefaults(t *testing.T) {
	assets, diagnostics := read(t, `pdxmesh = { name = "bare_mesh" file = "bare.mesh" }`)

	expectClean(t, diagnostics)

	mesh, _ := assets.Meshes.Get("bare_mesh")
	if mesh.Scale != 1 || mesh.CullDistance.Set || mesh.Settings != nil || mesh.LODs != nil {
		t.Errorf("mesh = %+v, want the defaults", mesh)
	}
}

func TestEntity(t *testing.T) {
	assets, diagnostics := read(t, `
pdxmesh = {
	name = "unit_mesh"
	file = "unit.mesh"
	animation = { id = "idle_animation" type = "unit_idle.anim" }
	animation = { id = "move_animation" type = "unit_move.anim" }
	blend_shape = { id = "unit_bs_fat" type = "unit_bs_fat.mesh" }
	additive_animation = { id = "unit_height" type = "unit_height.anim" }
}

entity = {
	name = "unit_entity"
	pdxmesh = "unit_mesh"
	scale = 1.2
	cull_radius = 40
	default_state = "idle"
	get_state_from_parent = no

	meshsettings = { name = "unitShape" texture_diffuse = "unit_red_diffuse.dds" }

	attribute = { name = "body_bs_fat" blend_shape = "unit_bs_fat" }
	attribute = { name = "body_height" additive_animation = "unit_height" default = 0.5 }

	locator = { name = "hand" parent_joint = "arm_J" position = { 0 1 0.5 } rotation = { 0 90 0 } scale = 0.8 }
	locator = { name = pos_2 position = { 1 2 3 } }

	attach = { name = weapon hand = "sword_entity" hand = sheath_entity }
	attach = { pos_2 = "flag_entity" }

	group = {
		name = "random_layout"
		attach = { chance = 10 pos_2 = "flag_entity" }
		attach = { hand = "sword_entity" }
	}

	state = {
		name = "idle"
		animation = "idle_animation"
		animation_speed = 0.6
		animation_blend_time = 0.2
		looping = yes
		chance = 2
		time_offset = { 0 5 }
		propagate_state = { weapon = "idle" }
		start_event = { light = "torch_light" node = "hand" trigger_once = yes }
		event = {
			time = 1.5
			node = "hand"
			particle = "dust_effect"
			sound = { soundeffect = "event:/SFX/step" }
			soundparameter = { intensity = 2 }
			entity = "flag_entity"
			attachment_id = "carried_flag"
			keep_particle = yes
			keep_sound = no
			keep_entity = yes
			use_parent_nodes = yes
			life = 2
			skip_forward = 5
			entity_fade_speed = 4
		}
	}

	state = {
		name = "move"
		animation = "move_animation"
		next_state = "idle"
		state_time = 6
		time_offset = 0.5
		chance = { if_current_state = { "idle" = 100 "move" = 0 } }
		propagate_state = "march"
		event = { remove_attachment = "carried_flag" state = "fold" }
	}

	game_data = { portrait_entity_user_data = { portrait_decal = { body_part = torso } } }
}

entity = { name = "sword_entity" }
entity = { name = "sheath_entity" }
entity = { name = "flag_entity" }
`)

	expectClean(t, diagnostics)

	entity, ok := assets.Entities.Get("unit_entity")
	if !ok {
		t.Fatal("unit_entity is missing")
	}

	if entity.Mesh != "unit_mesh" || entity.Scale != 1.2 || entity.CullRadius != database.Some(40.0) ||
		entity.DefaultState != "idle" || entity.GetStateFromParent != database.Some(false) {
		t.Errorf("entity = %+v", entity)
	}

	if len(entity.Settings) != 1 || entity.Settings[0].Diffuse != "unit_red_diffuse.dds" {
		t.Errorf("settings = %+v", entity.Settings)
	}

	wantAttributes := []Attribute{
		{Name: "body_bs_fat", BlendShape: "unit_bs_fat"},
		{Name: "body_height", AdditiveAnimation: "unit_height", Default: database.Some(0.5)},
	}
	if !slices.Equal(entity.Attributes, wantAttributes) {
		t.Errorf("attributes = %+v", entity.Attributes)
	}

	wantLocators := []Locator{
		{Name: "hand", ParentJoint: "arm_J", Position: Vec3{0, 1, 0.5}, Rotation: Vec3{0, 90, 0}, Scale: 0.8},
		{Name: "pos_2", Position: Vec3{1, 2, 3}, Scale: 1},
	}
	if !slices.Equal(entity.Locators, wantLocators) {
		t.Errorf("locators = %+v", entity.Locators)
	}

	// The name of an attach block is the id of its attachments, not a
	// locator, and a locator given twice holds two entities.
	wantAttachments := []Attachment{
		{Locator: "hand", Entity: "sword_entity", ID: "weapon"},
		{Locator: "hand", Entity: "sheath_entity", ID: "weapon"},
		{Locator: "pos_2", Entity: "flag_entity"},
	}
	if !slices.Equal(entity.Attachments, wantAttachments) {
		t.Errorf("attachments = %+v", entity.Attachments)
	}

	if len(entity.Groups) != 1 || entity.Groups[0].Name != "random_layout" || len(entity.Groups[0].Choices) != 2 {
		t.Fatalf("groups = %+v", entity.Groups)
	}

	choices := entity.Groups[0].Choices
	if choices[0].Chance != 10 || !slices.Equal(choices[0].Attachments, []Attachment{{Locator: "pos_2", Entity: "flag_entity"}}) || choices[1].Chance != 1 {
		t.Errorf("choices = %+v, want chance read apart from the locators and 1 by default", choices)
	}

	if len(entity.States) != 2 {
		t.Fatalf("states = %+v", entity.States)
	}

	idle := entity.States[0]
	if idle.Name != "idle" || idle.Animation != "idle_animation" || idle.AnimationSpeed != database.Some(0.6) ||
		idle.AnimationBlendTime != database.Some(0.2) || idle.Looping != database.Some(true) ||
		idle.Chance != database.Some(2.0) || idle.ChanceFromState != nil ||
		idle.TimeOffset != database.Some([2]float64{0, 5}) ||
		!slices.Equal(idle.Propagation, []Propagation{{Attachment: "weapon", State: "idle"}}) {
		t.Errorf("idle = %+v", idle)
	}

	if len(idle.StartEvents) != 1 || idle.StartEvents[0].Light != "torch_light" || idle.StartEvents[0].TriggerOnce != database.Some(true) {
		t.Errorf("start events = %+v", idle.StartEvents)
	}

	if len(idle.Events) != 1 {
		t.Fatalf("events = %+v", idle.Events)
	}

	event := idle.Events[0]
	if event.Time != 1.5 || event.Node != "hand" || event.Particle != "dust_effect" || event.Sound != "event:/SFX/step" ||
		!slices.Equal(event.SoundParameters, []database.Amount{{Key: "intensity", Amount: 2}}) ||
		event.Entity != "flag_entity" || event.AttachmentID != "carried_flag" ||
		event.KeepParticle != database.Some(true) || event.KeepSound != database.Some(false) ||
		event.KeepEntity != database.Some(true) || event.UseParentNodes != database.Some(true) ||
		event.Life != database.Some(2.0) || event.SkipForward != database.Some(5.0) || event.EntityFadeSpeed != database.Some(4.0) {
		t.Errorf("event = %+v", event)
	}

	move := entity.States[1]
	if move.NextState != "idle" || move.StateTime != database.Some(6.0) || move.TimeOffset != database.Some([2]float64{0.5, 0.5}) ||
		move.Chance.Set || !slices.Equal(move.ChanceFromState, []database.Amount{{Key: "idle", Amount: 100}, {Key: "move", Amount: 0}}) ||
		!slices.Equal(move.Propagation, []Propagation{{State: "march"}}) ||
		len(move.Events) != 1 || move.Events[0].RemoveAttachment != "carried_flag" || move.Events[0].State != "fold" {
		t.Errorf("move = %+v", move)
	}

	if decal, _ := entity.GameData.Get("portrait_entity_user_data"); !decal.IsBlock() {
		t.Errorf("game data = %+v, want it kept as written", entity.GameData)
	}
}

// Every reference that does not resolve is reported where it is written, and
// kept as written.
func TestReferences(t *testing.T) {
	assets, diagnostics := read(t, `
pdxmesh = {
	name = "unit_mesh"
	file = "unit.mesh"
	animation = { id = "idle_animation" type = "unit_idle.anim" }
	blend_shape = { id = "unit_bs_fat" type = "unit_bs_fat.mesh" }
}

entity = {
	name = "unit_entity"
	pdxmesh = "unit_mesh"
	attach = { hand = "missing_sword_entity" }
	group = { name = "layout" attach = { pos = "missing_crate_entity" } }
	state = { name = "idle" animation = "missing_animation" event = { entity = "missing_flag_entity" } }
	attribute = { name = "body_bs_old" blend_shape = "missing_shape" }
	attribute = { name = "body_height" additive_animation = "missing_additive" }
}

entity = { name = "ghost_entity" pdxmesh = "missing_mesh" clone = "missing_entity" }
`)

	expect(t, diagnostics, report.SeverityWarning, "test.asset:12:20: entity unit_entity: entity missing_sword_entity does not exist")
	expect(t, diagnostics, report.SeverityWarning, "entity unit_entity: entity missing_crate_entity does not exist")
	expect(t, diagnostics, report.SeverityWarning, "entity unit_entity: entity missing_flag_entity does not exist")
	expect(t, diagnostics, report.SeverityWarning, "animation missing_animation is not one of the animations of pdxmesh unit_mesh")
	expect(t, diagnostics, report.SeverityWarning, "blend shape missing_shape is not one of the blend shapes of pdxmesh unit_mesh")
	expect(t, diagnostics, report.SeverityWarning, "additive animation missing_additive is not one of the additive animations of pdxmesh unit_mesh")
	expect(t, diagnostics, report.SeverityWarning, "test.asset:19:44: entity ghost_entity: pdxmesh missing_mesh does not exist")
	expect(t, diagnostics, report.SeverityWarning, "entity ghost_entity: entity missing_entity does not exist")

	if ghost, _ := assets.Entities.Get("ghost_entity"); ghost.Mesh != "missing_mesh" || ghost.Clone != "missing_entity" {
		t.Errorf("ghost = %+v, want the references kept as written", ghost)
	}

	if _, ok := assets.MeshOf("ghost_entity"); ok {
		t.Error("an entity whose mesh does not exist has a mesh")
	}
}

// A clone draws the mesh of the entity it clones, and is checked against it.
func TestClones(t *testing.T) {
	assets, diagnostics := read(t, `
pdxmesh = { name = "base_mesh" file = "base.mesh" animation = { id = "idle_animation" type = "idle.anim" } }
pdxmesh = { name = "variant_mesh" file = "variant.mesh" }

entity = { name = "base_entity" pdxmesh = "base_mesh" state = { name = "idle" animation = "idle_animation" } }
entity = { name = "copy_entity" clone = "base_entity" state = { name = "wave" animation = "idle_animation" } }
entity = { name = "copy_of_copy_entity" clone = "copy_entity" }
entity = { name = "variant_entity" clone = "base_entity" pdxmesh = "variant_mesh" }
entity = { name = "silent_entity" }
`)

	expectClean(t, diagnostics)

	chain := assets.CloneChain("copy_of_copy_entity")
	if len(chain) != 3 || chain[0].Key != "copy_of_copy_entity" || chain[2].Key != "base_entity" {
		t.Errorf("clone chain = %d entities", len(chain))
	}

	for name, want := range map[string]string{
		"copy_of_copy_entity": "base_mesh",
		"variant_entity":      "variant_mesh",
	} {
		if mesh, ok := assets.MeshOf(name); !ok || mesh.Key != want {
			t.Errorf("mesh of %s = %v, want %s", name, mesh, want)
		}
	}

	if _, ok := assets.MeshOf("silent_entity"); ok {
		t.Error("an entity without a mesh has one")
	}

	if chain := assets.CloneChain("nothing"); len(chain) != 0 {
		t.Errorf("clone chain of a name that is no entity = %v", chain)
	}
}

func TestCloneCycles(t *testing.T) {
	assets, diagnostics := read(t, `
entity = { name = "a" clone = "b" }
entity = { name = "b" clone = "c" }
entity = { name = "c" clone = "a" }
entity = { name = "self" clone = "self" }
entity = { name = "into_cycle" clone = "a" }
`)

	expect(t, diagnostics, report.SeverityWarning, "entity a: clones itself through a -> b -> c -> a")
	expect(t, diagnostics, report.SeverityWarning, "entity self: clones itself through self -> self")

	cycles := 0
	for _, diagnostic := range diagnostics {
		if strings.Contains(diagnostic.Message, "clones itself") {
			cycles++
		}
	}

	if cycles != 2 {
		t.Errorf("%d cycles reported, want each of the 2 once", cycles)
	}

	if chain := assets.CloneChain("into_cycle"); len(chain) != 4 {
		t.Errorf("clone chain into a cycle has %d entities, want it to stop after a, b and c", len(chain))
	}
}

// Definitions without a usable name cannot be referred to, so they are
// skipped, and everything that is not a mesh or an entity is left alone.
func TestDefinitionsThatCannotBeRead(t *testing.T) {
	assets, diagnostics := read(t, `
pdxmesh = { file = "nameless.mesh" }
pdxmesh = { name = { a b } file = "odd.mesh" }
pdxmesh = { name = "" file = "empty.mesh" }
entity = "not_a_block"
entity = { name = "first" name = "second" }
arrowType = { name = "move_arrow" }
pdxmesh = { name = "fine_mesh" file = "fine.mesh" }
`)

	expect(t, diagnostics, report.SeverityWarning, "test.asset:2:1: pdxmesh has no name; ignored")
	expect(t, diagnostics, report.SeverityWarning, "the name of a pdxmesh should be a name or a string, found a list; ignored")
	expect(t, diagnostics, report.SeverityWarning, `the name of a pdxmesh should be a name or a string, found ""; ignored`)
	expect(t, diagnostics, report.SeverityWarning, `entity should be a block, found "not_a_block"; ignored`)
	expect(t, diagnostics, report.SeverityWarning, "entity second is named 2 times; the last name counts")
	expect(t, diagnostics, report.SeverityInfo, "arrowType is no kind of definition this package reads; not read")

	if keys := assets.Meshes.Keys(); !slices.Equal(keys, []string{"fine_mesh"}) {
		t.Errorf("meshes = %v", keys)
	}

	if keys := assets.Entities.Keys(); !slices.Equal(keys, []string{"second"}) {
		t.Errorf("entities = %v", keys)
	}
}

// Fields of the wrong kind are read as their defaults, with a warning, and
// unknown fields are reported and kept.
func TestTolerance(t *testing.T) {
	assets, diagnostics := read(t, `
pdxmesh = {
	name = "odd_mesh"
	scale = big
	lod_percentages = { lod = { index = 1.5 percent = lots } lod = { index = { 1 } percent = 3 } }
	meshsettings = { name = "oddShape" index = 1e12 future_setting = yes }
	blend_shape = { type = "nameless.mesh" }
	shiny = yes
}

entity = {
	name = "odd_entity"
	locator = { name = "short" position = { 1 2 } rotation = straight }
	locator = { position = { 1 2 3 4 } }
	attach = { pos = { nested } chance = 3 }
	state = { animation = "x" time_offset = { 1 2 3 } chance = maybe propagate_state = 7 }
	state = { name = "s" propagate_state = { weapon = { x } } chance = { if_current_state = { idle = high } }
		event = { entity_editor_id = "1" time = soon } }
	attribute = { blend_shape = "nothing" }
}
`)

	for _, fragment := range []string{
		"pdxmesh odd_mesh: has no file; the mesh has nothing to draw",
		"scale should be a number, found big; ignored",
		"index should be a whole number, found 1.5; read as 1",
		"percent should be a number or inf, found lots; ignored",
		"index should be a whole number, found a list; ignored",
		"index should be a whole number, found 1e12; ignored",
		"pdxmesh odd_mesh, blend_shape: has no id",
		"position should be three numbers, found 2",
		"rotation should be three numbers, found straight; ignored",
		"position should be three numbers, found 4",
		"entity odd_entity, locator: has no name",
		"the entity attached to pos should be a name, found a list; ignored",
		"entity odd_entity, state: has no name",
		"time_offset should be a number or two numbers, found a list; ignored",
		"chance should be a number or a block, found maybe; ignored",
		"propagate_state should be the name of a state or a block, found 7; ignored",
		"propagate_state.weapon should be the name of a state, found a list; ignored",
		"if_current_state.idle should be a number, found high; ignored",
		"time should be a number, found soon; ignored",
		"entity odd_entity, attribute: has no name",
	} {
		expect(t, diagnostics, report.SeverityWarning, fragment)
	}

	expect(t, diagnostics, report.SeverityInfo, "pdxmesh odd_mesh: unknown field shiny, kept but not interpreted")
	expect(t, diagnostics, report.SeverityInfo, "pdxmesh odd_mesh, meshsettings: unknown field future_setting")
	expect(t, diagnostics, report.SeverityInfo, "entity odd_entity, attach: unknown field chance")

	if mentions(diagnostics, report.SeverityInfo, "entity_editor_id") {
		t.Error("entity_editor_id, which the game has no use for, is reported as unknown")
	}

	mesh, _ := assets.Meshes.Get("odd_mesh")
	if mesh.Scale != 1 || mesh.LODs[0].Index != 1 || mesh.LODs[0].Percent != 0 || mesh.Settings[0].Index != 0 {
		t.Errorf("mesh = %+v", mesh)
	}

	if shiny, ok := mesh.Field("shiny"); !ok || shiny.Text != "yes" {
		t.Error("the unknown field is not kept")
	}

	entity, _ := assets.Entities.Get("odd_entity")
	if entity.Locators[0].Position != (Vec3{1, 2, 0}) || entity.Locators[1].Position != (Vec3{1, 2, 3}) {
		t.Errorf("locators = %+v, want lists of the wrong length read as far as they go", entity.Locators)
	}
}

// A name defined again in a later file overrides the earlier definition
// completely, the way the database folders work.
func TestLaterDefinitionsOverride(t *testing.T) {
	root := t.TempDir()

	write := func(name, content string) {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("game/gfx/models/a/a.asset", `pdxmesh = { name = "shared_mesh" file = "game.mesh" scale = 2 }`)
	write("game/gfx/deep/down/below/b.asset", `entity = { name = "deep_entity" pdxmesh = "shared_mesh" }`)
	write("game/gfx/models/a/not_an_asset.txt", `pdxmesh = { name = "ignored_mesh" file = "x.mesh" }`)
	write("game/common/stray.asset", `pdxmesh = { name = "outside_mesh" file = "x.mesh" }`)
	write("mod/gfx/models/mod/z.asset", `pdxmesh = { name = "shared_mesh" file = "mod.mesh" }`)

	assets := Load(folders.Open([]folders.Source{
		{Name: "game", Path: filepath.Join(root, "game")},
		{Name: "mod", Path: filepath.Join(root, "mod")},
	}))

	expectClean(t, assets.Diagnostics)
	expect(t, assets.Diagnostics, report.SeverityInfo, "pdxmesh shared_mesh: overrides the definition in gfx/models/a/a.asset from game")

	if keys := assets.Meshes.Keys(); !slices.Equal(keys, []string{"shared_mesh"}) {
		t.Errorf("meshes = %v, want only the ones in .asset files below gfx", keys)
	}

	mesh, _ := assets.Meshes.Get("shared_mesh")
	if mesh.File != "mod.mesh" || mesh.Scale != 1 || mesh.Origin().Source != "mod" || len(mesh.Overridden) != 1 {
		t.Errorf("mesh = %+v, want the mod's definition alone", mesh)
	}

	if !assets.Entities.Has("deep_entity") {
		t.Error("an asset file deep below gfx was not read")
	}

	if !slices.Equal(assets.Sources, []string{"game", "mod"}) {
		t.Errorf("sources = %v", assets.Sources)
	}
}

func TestResolve(t *testing.T) {
	origin := database.Origin{File: "gfx/models/units/infantry/infantry.asset"}

	for reference, want := range map[string]string{
		"infantry.mesh":                       "gfx/models/units/infantry/infantry.mesh",
		"blendshapes/fat.mesh":                "gfx/models/units/infantry/blendshapes/fat.mesh",
		"../animation/walk.anim":              "gfx/models/units/animation/walk.anim",
		`..\animation\walk.anim`:              "gfx/models/units/animation/walk.anim",
		"../../generic/generic.mesh":          "gfx/models/generic/generic.mesh",
		"./here.dds":                          "gfx/models/units/infantry/here.dds",
		"../../../../../../outside/file.mesh": "../../outside/file.mesh",
		"":                                    "",
	} {
		if got := Resolve(origin, reference); got != want {
			t.Errorf("Resolve(%q) = %q, want %q", reference, got, want)
		}
	}

	// A file of a game with layers resolves within its layer.
	layered := database.Origin{File: "in_game/gfx/models/ship/ship.asset"}
	if got := Resolve(layered, "ship.mesh"); got != "in_game/gfx/models/ship/ship.mesh" {
		t.Errorf("Resolve in a layer = %q", got)
	}
}

// Crusader Kings 3 shares animations between meshes through animation sets,
// which a mesh imports. The files of a set's animations are relative to the
// set, wherever the mesh importing it is.
func TestAnimationSets(t *testing.T) {
	root := t.TempDir()

	write := func(name, content string) {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("game/gfx/models/portraits/male_body/male_body.asset", `
skeletal_animation_set = {
	name = "common_body"
	reference_skeleton = "male_body_mesh"
	animation = { id = "body_wave" type = "male_body_wave.anim" }
	animation = { id = "body_bow" type = "male_body_bow.anim" }
}

pdxmesh = {
	name = "male_body_mesh"
	file = "male_body.mesh"
	import = { type = skeletal_animation_set name = "common_body" }
}
`)
	write("game/gfx/models/portraits/female_body/female_body.asset", `
pdxmesh = {
	name = "female_body_mesh"
	file = "female_body.mesh"
	streaming = Never
	animation = { id = "body_curtsy" type = "female_body_curtsy.anim" }
	import = { type = skeletal_animation_set name = "common_body" }
	meshsettings = { name = "female_bodyShape" rasterizerstate = "rasterizer_no_culling" }
}

entity = {
	name = "female_body_entity"
	pdxmesh = "female_body_mesh"
	state = { name = "wave" animation = "body_wave" }
	state = { name = "curtsy" animation = "body_curtsy" }
}
`)

	assets := Load(folders.Open([]folders.Source{{Name: "game", Path: filepath.Join(root, "game")}}))

	expectClean(t, assets.Diagnostics)

	set, ok := assets.AnimationSets.Get("common_body")
	if !ok || set.ReferenceSkeleton != "male_body_mesh" || len(set.Animations) != 2 {
		t.Fatalf("common_body = %+v", set)
	}

	body, _ := assets.Meshes.Get("female_body_mesh")
	if !slices.Equal(body.Imports, []Import{{Type: "skeletal_animation_set", Name: "common_body"}}) ||
		body.Streaming != "Never" || body.Settings[0].RasterizerState != "rasterizer_no_culling" {
		t.Errorf("female_body_mesh = %+v", body)
	}

	var played []string

	for _, animation := range assets.AnimationsOf(body) {
		played = append(played, animation.ID+" "+Resolve(animation.Origin, animation.File))
	}

	want := []string{
		"body_curtsy gfx/models/portraits/female_body/female_body_curtsy.anim",
		"body_wave gfx/models/portraits/male_body/male_body_wave.anim",
		"body_bow gfx/models/portraits/male_body/male_body_bow.anim",
	}
	if !slices.Equal(played, want) {
		t.Errorf("animations of the female body = %q, want its own, then the set's next to the set", played)
	}
}

func TestAnimationSetReferences(t *testing.T) {
	assets, diagnostics := read(t, `
skeletal_animation_set = { name = "empty_set" reference_skeleton = "missing_skeleton_mesh" }

pdxmesh = {
	name = "odd_mesh"
	file = "odd.mesh"
	import = { type = skeletal_animation_set name = "missing_set" }
	import = { type = morph_set name = "faces" }
	import = { type = skeletal_animation_set }
}

entity = { name = "odd_entity" pdxmesh = "odd_mesh" state = { name = "s" animation = "from_missing_set" } }
`)

	expect(t, diagnostics, report.SeverityWarning, "test.asset:2:68: skeletal_animation_set empty_set: pdxmesh missing_skeleton_mesh does not exist")
	expect(t, diagnostics, report.SeverityInfo, "skeletal_animation_set empty_set: has no animations")
	expect(t, diagnostics, report.SeverityWarning, "pdxmesh odd_mesh: skeletal_animation_set missing_set does not exist")
	expect(t, diagnostics, report.SeverityInfo, "pdxmesh odd_mesh: imports a morph_set, which is no kind of definition this package reads")
	expect(t, diagnostics, report.SeverityWarning, "pdxmesh odd_mesh, import: has no name; nothing is imported")
	expect(t, diagnostics, report.SeverityWarning, "animation from_missing_set is not one of the animations of pdxmesh odd_mesh")

	mesh, _ := assets.Meshes.Get("odd_mesh")
	if animations := assets.AnimationsOf(mesh); len(animations) != 0 {
		t.Errorf("animations of a mesh whose imports do not resolve = %+v", animations)
	}
}

// Crusader Kings 3 gives a state a plain chance and a chance by the state
// before it, each in a field of its own. Both are read.
func TestStateWithBothChances(t *testing.T) {
	assets, diagnostics := read(t, `
entity = {
	name = "control_entity"
	state = {
		name = "winning"
		chance = 1.0
		chance = { if_current_state = { "even" = 0 "winning" = 100 } default = 5 }
		event = { sound = { soundeffect = "event:/ambience" stop_on_state_change = no } }
	}
}
`)

	expectClean(t, diagnostics)

	entity, _ := assets.Entities.Get("control_entity")
	state := entity.States[0]

	if state.Chance != database.Some(1.0) || state.ChanceDefault != database.Some(5.0) ||
		!slices.Equal(state.ChanceFromState, []database.Amount{{Key: "even", Amount: 0}, {Key: "winning", Amount: 100}}) {
		t.Errorf("state = %+v, want both chances", state)
	}

	if event := state.Events[0]; event.Sound != "event:/ambience" || event.SoundStopsOnStateChange != database.Some(false) {
		t.Errorf("event = %+v", event)
	}
}
