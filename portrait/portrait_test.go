package portrait

import (
	"math"
	"os"
	"strings"
	"testing"

	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

var testFile = folders.File{Relative: "gfx/portraits/test.txt", Path: "test.txt", Source: "test"}

// near reports whether two numbers are the same but for rounding.
func near(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

// An environment reads its cubemap and its lights in the order of their
// numbers, with variables substituted, places cylindrical and colours of hsv.
func TestReadEnvironments(t *testing.T) {
	collector := &report.Collector{}
	table := ReadEnvironments(script.Parse(`
@radius = 340
environment_standard = {
	cubemap = "gfx/portraits/environments/castle_interior_01.dds"
	cubemap_intensity = 2.8
	lights = {
		0 = {
			type = "spot_light"
			affected_by_shadow = yes
			color = hsv{ 0 0 5 }
			position = cylindrical{ @radius 250 -90 }
			look_at = { 0 0 0 }
			look_at = { 1 2 3 }
			look_at_node = camera_torso_look_at
			radius = 2000
			falloff = 120
			outer_cone_angle = 50
			inner_cone_angle = 0
		}
		1 = { type = "point_light" color = rgb{ 1 0.5 0 } position = { 1 2 3 } }
	}
	shadow_camera = {
		position = cylindrical { 330 220 225 }
		look_at = { 0 0 0 }
		look_at_node = camera_torso_look_at
		fov = 50
		camera_near_far = { 100 1350 }
	}
}
`), testFile, collector)

	for _, diagnostic := range collector.Diagnostics {
		t.Errorf("unexpected diagnostic: %s", diagnostic)
	}

	environment, ok := table.Get("environment_standard")
	if !ok {
		t.Fatal("environment_standard not read")
	}

	if intensity, _ := environment.CubemapIntensity.Get(); environment.Cubemap != "gfx/portraits/environments/castle_interior_01.dds" || intensity != 2.8 {
		t.Errorf("cubemap %q at %v", environment.Cubemap, intensity)
	}

	if len(environment.Lights) != 2 {
		t.Fatalf("lights = %+v", environment.Lights)
	}

	key := environment.Lights[0]
	if key.Number != 0 || key.Type != LightSpot || !key.AffectedByShadow || key.Color != (Color{5, 5, 5}) || key.Radius != 2000 || key.Falloff != 120 || key.OuterConeAngle != 50 || key.LookAtNode != "camera_torso_look_at" {
		t.Errorf("key light = %+v", key)
	}

	// The last look_at counts, as the game reads a field given twice.
	if key.LookAt != [3]float64{1, 2, 3} {
		t.Errorf("look at %v, want the last one written", key.LookAt)
	}

	// An angle of -90 stands to the portrait's right, on the camera's left,
	// along -x.
	if place := key.Position.Cartesian(); !near(place[0], -340) || place[1] != 250 || !near(place[2], 0) {
		t.Errorf("key light at %v, want -340 250 0", place)
	}

	if fill := environment.Lights[1]; fill.Type != LightPoint || fill.Color != (Color{1, 0.5, 0}) || fill.Position.Cylindrical || fill.Position.Cartesian() != [3]float64{1, 2, 3} {
		t.Errorf("fill light = %+v", fill)
	}

	shadow, ok := environment.ShadowCamera.Get()
	if fov, _ := shadow.FOV.Get(); !ok || fov != 50 || !shadow.Position.Cylindrical {
		t.Errorf("shadow camera = %+v", shadow)
	}
}

// A camera stands at a place measured from a bone and looks at another.
func TestReadCameras(t *testing.T) {
	collector := &report.Collector{}
	table := ReadCameras(script.Parse(`
camera_head = {
	camera = {
		position = cylindrical{ 150 3 0 }
		position_node = camera_torso_look_at
		look_at = { 0 3 0 }
		look_at_node = bn_h_head_mid
		fov = 20
		camera_near_far = { 50 450 }
	}
	unknown = "gfx/portraits/example_portraits/unknown.dds"
}
`), testFile, collector)

	for _, diagnostic := range collector.Diagnostics {
		t.Errorf("unexpected diagnostic: %s", diagnostic)
	}

	head, ok := table.Get("camera_head")
	if !ok {
		t.Fatal("camera_head not read")
	}

	camera := head.Camera

	// An angle of 0 stands in front of the portrait, which faces along -z.
	if place := camera.Position.Cartesian(); !near(place[0], 0) || place[1] != 3 || !near(place[2], -150) {
		t.Errorf("camera at %v, want 0 3 -150", place)
	}

	if fov, _ := camera.FOV.Get(); fov != 20 || camera.PositionNode != "camera_torso_look_at" || camera.LookAtNode != "bn_h_head_mid" || camera.LookAt != [3]float64{0, 3, 0} {
		t.Errorf("camera = %+v", camera)
	}

	if nearFar, ok := camera.NearFar.Get(); !ok || nearFar != [2]float64{50, 450} || head.Unknown == "" {
		t.Errorf("near and far %v, unknown %q", nearFar, head.Unknown)
	}
}

// The shared settings read the tonemap curve and the colour adjustments.
func TestReadSettings(t *testing.T) {
	collector := &report.Collector{}
	settings := ReadSettings(script.Parse(`
saturation_scale = 1.05
value_scale = 1.05
colorbalance = { 0.95 1 1 }
levels_min = hsv{ 0 0 0 }
levels_max = hsv{ 0 0 1 }
shadowmap_kernelscale = 8
shadowmap_fadefactor = 0.66
tonemap_curve={
	shoulder_strength=0.1
	linear_strength=0.5
	linear_angle=0.5
	toe_strength=1
	toe_numerator=1
	toe_denominator=0.4
	linear_white=2
}
ssao = {
	enabled = yes
	samples = 16
	radius = 4.0
	max_radius = 10.0
	depth_distance = 1.0
	max_depth = 10.0
	apply_scale = 1.0
	apply_bias = 0.5
	apply_power = 5.0
	blend_factor = 0.9
	blur_enabled = yes
	blur_width = 2.0
	blur_scale = 1.0
	blur_depth_power = 5.0
	blur_weights = 4
}
`), testFile, collector)

	for _, diagnostic := range collector.Diagnostics {
		t.Errorf("unexpected diagnostic: %s", diagnostic)
	}

	curve, ok := settings.Tonemap.Get()
	if !ok || curve.ShoulderStrength != 0.1 || curve.ToeDenominator != 0.4 || curve.LinearWhite != 2 {
		t.Errorf("tonemap = %+v", curve)
	}

	if balance, _ := settings.ColorBalance.Get(); balance != [3]float64{0.95, 1, 1} {
		t.Errorf("colour balance = %v", balance)
	}

	if most, _ := settings.LevelsMax.Get(); most != (Color{1, 1, 1}) {
		t.Errorf("levels max = %v", most)
	}

	if scale, _ := settings.ValueScale.Get(); scale != 1.05 {
		t.Errorf("value scale = %v", scale)
	}

	if ssao, ok := settings.SSAO.Get(); !ok || !ssao.Enabled || ssao.Samples != 16 || ssao.ApplyPower != 5 || ssao.BlurWeights != 4 {
		t.Errorf("ssao = %+v", ssao)
	}
}

// A hue goes round from red through green and blue back to red, and the
// value scales the colour beyond white.
func TestFromHSV(t *testing.T) {
	for _, test := range []struct {
		hue, saturation, value float64
		want                   Color
	}{
		{0, 1, 1, Color{1, 0, 0}},
		{1.0 / 3, 1, 1, Color{0, 1, 0}},
		{2.0 / 3, 1, 1, Color{0, 0, 1}},
		{1, 1, 1, Color{1, 0, 0}},
		{0, 0, 5, Color{5, 5, 5}},
		{0.5, 0.5, 2, Color{1, 2, 2}},
	} {
		got := fromHSV(test.hue, test.saturation, test.value)
		for channel := range got {
			if !near(got[channel], test.want[channel]) {
				t.Errorf("hsv %v %v %v = %v, want %v", test.hue, test.saturation, test.value, got, test.want)

				break
			}
		}
	}
}

// Values of the wrong kind are reported and left out, and what can be read
// still is.
func TestReadReportsProblems(t *testing.T) {
	collector := &report.Collector{}
	table := ReadEnvironments(script.Parse(`e = { lights = { 0 = { color = hsl{ 1 2 3 } position = cylindrical{ 1 2 } } 1 = 5 } }`), testFile, collector)

	environment, ok := table.Get("e")
	if !ok || len(environment.Lights) != 1 {
		t.Fatalf("environment = %+v", environment)
	}

	if len(collector.Diagnostics) != 4 {
		t.Errorf("diagnostics = %v, want the missing type, the odd colour, the short place and the light that is no block", collector.Diagnostics)
	}
}

// The assembly reads the entities of every age and sex, the attachments with
// their joints in order, and the root, and leaves alone the settings of the
// same file that are not about putting a portrait together.
func TestReadAssembly(t *testing.T) {
	collector := &report.Collector{}
	assembly := ReadAssembly(script.Parse(`
enable_posteffects = yes
male = {
	head = "male_head_entity"
	torso = "male_body_entity"
}
boy = {
	head = "male_head_entity"
	torso = "boy_body_entity"
}
attach = {
	what = head
	where = torso
	joint_attachment = {
		parent_joint = "bn_neck_base"
		child_joint = "head_root"
	}
	joint_attachment = { parent_joint = "bn_l_clavicle" child_joint = "bn_h_l_clavicle" }
}
root = torso
srgb_color_palettes = yes
ssao = { enabled = yes }
`), testFile, collector)

	for _, diagnostic := range collector.Diagnostics {
		t.Errorf("unexpected diagnostic: %s", diagnostic)
	}

	if boy, ok := assembly.For("boy"); !ok || boy != (Entities{Head: "male_head_entity", Torso: "boy_body_entity"}) {
		t.Errorf("a boy is drawn with %+v", boy)
	}

	if _, ok := assembly.For("girl"); ok {
		t.Error("a girl is drawn with entities the file does not give her")
	}

	head, ok := assembly.Attachment("head", "torso")
	want := []JointAttachment{{Parent: "bn_neck_base", Child: "head_root"}, {Parent: "bn_l_clavicle", Child: "bn_h_l_clavicle"}}

	if !ok || len(head.Joints) != len(want) || head.Joints[0] != want[0] || head.Joints[1] != want[1] {
		t.Errorf("the head is attached by %+v, want %+v", head, want)
	}

	if assembly.Root != "torso" {
		t.Errorf("root = %q", assembly.Root)
	}

	if !assembly.SRGBColorPalettes {
		t.Error("the palettes are not read as sRGB, which the file says they are")
	}
}

// An assembly of entities and joints of the wrong kind reports them, and
// keeps what can be read.
func TestReadAssemblyReportsProblems(t *testing.T) {
	collector := &report.Collector{}
	assembly := ReadAssembly(script.Parse(`male = { head = { } torso = t legs = l } female = 5 attach = { what = head joint_attachment = 3 }`), testFile, collector)

	if male, _ := assembly.For("male"); male.Torso != "t" || male.Head != "" {
		t.Errorf("a man is drawn with %+v", male)
	}

	if len(assembly.Attachments) != 1 || assembly.Attachments[0].What != "head" || len(assembly.Attachments[0].Joints) != 0 {
		t.Errorf("attachments = %+v", assembly.Attachments)
	}

	if len(collector.Diagnostics) != 4 {
		t.Errorf("diagnostics = %v, want the head that is a block, the legs, the woman who is no block and the joint that is no block", collector.Diagnostics)
	}
}

func FuzzRead(f *testing.F) {
	f.Add(`e = { cubemap = "c" lights = { 0 = { type = "spot_light" color = hsv{ 1 1 1 } position = cylindrical{ 1 2 3 } } } shadow_camera = { fov = 1 } }`)
	f.Add(`c = { camera = { position = cylindrical{ 1 2 3 } camera_near_far = { 1 2 } } }`)
	f.Add(`tonemap_curve = { shoulder_strength = 1 } colorbalance = { 1 2 3 } levels_min = hsv{ 0 0 0 }`)
	f.Add(`male = { head = h torso = t } attach = { what = head where = torso joint_attachment = { parent_joint = a child_joint = b } } root = torso`)

	f.Fuzz(func(t *testing.T, input string) {
		collector := &report.Collector{}
		document := script.Parse(input)

		for _, environment := range ReadEnvironments(document, testFile, collector).All() {
			for _, light := range environment.Lights {
				light.Position.Cartesian()
			}
		}

		ReadCameras(document, testFile, collector)
		ReadSettings(document, testFile, collector)
		ReadAssembly(document, testFile, collector)
	})
}

// TestLoadInstalled loads the environments, the cameras and the settings of a
// real installation, which read without a warning or a field this package
// does not know.
func TestLoadInstalled(t *testing.T) {
	game := os.Getenv("PDX_GAME_DIR")
	if game == "" {
		t.Skip("set PDX_GAME_DIR to the game folder to run this test")
	}

	set := folders.Open([]folders.Source{{Name: "game", Path: game}})

	environments, diagnostics := LoadEnvironments(set)
	cameras, more := LoadCameras(set)
	settings, yet := LoadSettings(set)

	diagnostics = append(append(diagnostics, more...), yet...)

	for _, diagnostic := range diagnostics {
		if diagnostic.Severity >= report.SeverityWarning || strings.Contains(diagnostic.Message, "unknown field") {
			t.Errorf("%s", diagnostic)
		}
	}

	t.Logf("%d environments, %d cameras", environments.Len(), cameras.Len())

	if environments.Len() == 0 {
		t.Error("the game has no environments")
	}

	// Every environment lights the portrait from a cubemap and lights of its
	// own, and casts shadows from a camera, its own or one it borrows.
	for key, environment := range environments.All() {
		if environment.Cubemap == "" || len(environment.Lights) == 0 || !environment.ShadowCamera.Set {
			t.Errorf("%s = %+v", key, environment)
		}
	}

	if _, ok := cameras.Get("camera_head"); !ok {
		t.Error("the game has no camera_head")
	}

	if _, ok := settings.Tonemap.Get(); !ok {
		t.Error("the settings have no tonemap curve")
	}
}

// TestLoadInstalledAssembly reads how a real installation puts its portraits
// together, without a warning or a field this package does not know.
func TestLoadInstalledAssembly(t *testing.T) {
	game := os.Getenv("PDX_GAME_DIR")
	if game == "" {
		t.Skip("set PDX_GAME_DIR to the game folder to run this test")
	}

	assembly, diagnostics := LoadAssembly(folders.Open([]folders.Source{{Name: "game", Path: game}}))

	for _, diagnostic := range diagnostics {
		if diagnostic.Severity >= report.SeverityWarning || strings.Contains(diagnostic.Message, "unknown field") {
			t.Errorf("%s", diagnostic)
		}
	}

	for _, group := range []string{"male", "female", "boy", "girl"} {
		if entities, ok := assembly.For(group); !ok || entities.Head == "" || entities.Torso == "" {
			t.Errorf("%s is drawn with %+v", group, entities)
		}
	}

	if head, ok := assembly.Attachment("head", assembly.Root); !ok || len(head.Joints) == 0 {
		t.Errorf("the head is attached to the %s by %+v", assembly.Root, head)
	}
}

// An environment that names another for its shadow camera casts its shadows
// from the other's; one that names an environment without one casts none,
// and says so.
func TestBorrowedShadowCamera(t *testing.T) {
	collector := &report.Collector{}
	table := ReadEnvironments(script.Parse(`
full_torso = { shadow_camera = { fov = 10 camera_near_far = { 4 500 } } }
gui_portrait = { shadow_camera = full_torso }
lost = { shadow_camera = nowhere }
odd = { shadow_camera = 5 }
`), testFile, collector)

	gui, _ := table.Get("gui_portrait")
	if camera, ok := gui.ShadowCamera.Get(); !ok || camera.FOV.Or(0) != 10 || gui.ShadowCameraOf != "full_torso" {
		t.Errorf("gui_portrait casts from %+v", gui.ShadowCamera)
	}

	if lost, _ := table.Get("lost"); lost.ShadowCamera.Set {
		t.Errorf("lost casts from %+v", lost.ShadowCamera)
	}

	if len(collector.Diagnostics) != 2 {
		t.Errorf("diagnostics = %v, want the environment that is not there and the number", collector.Diagnostics)
	}
}

// The settings say how the light is exposed and tonemapped, and the bloom,
// and leave alone those of the map that some games keep in the same file.
func TestReadExposureAndBloom(t *testing.T) {
	collector := &report.Collector{}
	settings := ReadSettings(script.Parse(`
sun_intensity = 6
ambient_neg_x = hsv{ 0 0 0.4 }
value_scale = 1.4
bloom_strength = 0.13
bloom_steepness = 1.0
bloom_threshold = 0.0
bloom_enabled = yes
exposure = 1.0
exposure_function = "FixedExposure"
tonemap_function = "Uncharted"
lensflare = { enabled = no }
`), testFile, collector)

	for _, diagnostic := range collector.Diagnostics {
		t.Errorf("unexpected diagnostic: %s", diagnostic)
	}

	if settings.TonemapFunction != "Uncharted" || settings.ExposureFunction != "FixedExposure" || settings.Exposure.Or(0) != 1 {
		t.Errorf("settings = %+v", settings)
	}

	if bloom, ok := settings.Bloom.Get(); !ok || !bloom.Enabled || bloom.Strength != 0.13 || bloom.Steepness != 1 {
		t.Errorf("bloom = %+v", settings.Bloom)
	}
}
