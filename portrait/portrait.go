// Package portrait reads how the Jomini engine stages a portrait: the
// environments that light it, in gfx/portraits/environments, the cameras
// that look at it, in gfx/portraits/cameras, and the settings every
// environment shares, gfx/portraits/jomini_environment.txt.
//
// An environment is a cubemap the portrait reflects, and a few lights:
//
//	environment_standard = {
//		cubemap = "gfx/portraits/environments/castle_interior_01.dds"
//		cubemap_intensity = 2.8
//		lights = {
//			0 = {
//				type = "spot_light"
//				color = hsv{ 0.01 0.01 5 }
//				position = cylindrical{ 340 250 -55 }
//				look_at = { 0 0 0 }
//				look_at_node = camera_torso_look_at
//				radius = 2000
//				falloff = 120
//				outer_cone_angle = 50
//				inner_cone_angle = 0
//			}
//		}
//	}
//
// A camera is where the eye stands and what it looks at:
//
//	camera_head = {
//		camera = {
//			position = cylindrical{ 150 3 0 }
//			position_node = camera_torso_look_at
//			look_at = { 0 3 0 }
//			look_at_node = bn_h_head_mid
//			fov = 20
//			camera_near_far = { 50 450 }
//		}
//	}
//
// Places are given in the game's own coordinates, in which a portrait faces
// along -z: a cylindrical position of angle 0 stands in front of it. A node
// is a bone of the portrait's skeleton, such as bn_h_head_mid, which a place
// is measured from where the file names one.
//
// The format belongs to the engine rather than to any one game, so this
// package knows nothing about a particular game. As everywhere in this
// module, reading never fails: problems are diagnostics.
package portrait

import (
	"math"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// Position is a place, as the files write it: cylindrical, as a radius around
// the vertical, a height and an angle in degrees, or cartesian, as x, y and
// z.
type Position struct {
	Cylindrical bool
	Values      [3]float64
}

// Cartesian is the place as x, y and z in the game's coordinates. A
// cylindrical place of angle 0 stands in front of the portrait, which faces
// along -z, and one of a negative angle to its right, which the camera in
// front of it sees on its left.
func (p Position) Cartesian() [3]float64 {
	if !p.Cylindrical {
		return p.Values
	}

	radius, height, angle := p.Values[0], p.Values[1], p.Values[2]*math.Pi/180

	return [3]float64{radius * math.Sin(angle), height, -radius * math.Cos(angle)}
}

// Color is a colour as the files write it, hsv{ } or rgb{ }, as red, green
// and blue. A value above 1 is a light brighter than white.
type Color [3]float64

// The tags a value can be written with.
const (
	tagCylindrical = "cylindrical"
	tagCartesian   = "cartesian"
	tagHSV         = "hsv"
	tagRGB         = "rgb"
)

// position reads a place: cylindrical{ }, cartesian{ }, or a plain block of
// three numbers, which is cartesian.
func position(r *database.Reader, key string) (Position, bool) {
	index, ok := r.Last(key)
	if !ok {
		return Position{}, false
	}

	value := r.Fields()[index].Value
	cylindrical := false

	if value.Kind == script.KindTagged && value.Value != nil {
		switch value.Tag {
		case tagCylindrical:
			cylindrical = true
		case tagCartesian:
		default:
			r.Warn(index, value, "%s should be cylindrical or cartesian, found %s; ignored", key, value.Tag)

			return Position{}, false
		}

		value = *value.Value
	}

	numbers, ok := value.Numbers()
	if !ok || len(numbers) != 3 {
		r.Warn(index, value, "%s should be three numbers, found %s; ignored", key, value.Describe())

		return Position{}, false
	}

	return Position{Cylindrical: cylindrical, Values: [3]float64(numbers)}, true
}

// vector reads a block of three numbers.
func vector(r *database.Reader, key string) [3]float64 {
	index, ok := r.Last(key)
	if !ok {
		return [3]float64{}
	}

	value := r.Fields()[index].Value

	numbers, ok := value.Numbers()
	if !ok || len(numbers) != 3 {
		r.Warn(index, value, "%s should be three numbers, found %s; ignored", key, value.Describe())

		return [3]float64{}
	}

	return [3]float64(numbers)
}

// pair reads a block of two numbers.
func pair(r *database.Reader, key string) ([2]float64, bool) {
	index, ok := r.Last(key)
	if !ok {
		return [2]float64{}, false
	}

	value := r.Fields()[index].Value

	numbers, ok := value.Numbers()
	if !ok || len(numbers) != 2 {
		r.Warn(index, value, "%s should be two numbers, found %s; ignored", key, value.Describe())

		return [2]float64{}, false
	}

	return [2]float64(numbers), true
}

// color reads a colour: hsv{ }, rgb{ }, or a plain block of three numbers,
// which is red, green and blue.
func color(r *database.Reader, key string) (Color, bool) {
	index, ok := r.Last(key)
	if !ok {
		return Color{}, false
	}

	value := r.Fields()[index].Value
	hsv := false

	if value.Kind == script.KindTagged && value.Value != nil {
		switch value.Tag {
		case tagHSV:
			hsv = true
		case tagRGB:
		default:
			r.Warn(index, value, "%s should be hsv or rgb, found %s; ignored", key, value.Tag)

			return Color{}, false
		}

		value = *value.Value
	}

	numbers, ok := value.Numbers()
	if !ok || len(numbers) != 3 {
		r.Warn(index, value, "%s should be three numbers, found %s; ignored", key, value.Describe())

		return Color{}, false
	}

	if hsv {
		return fromHSV(numbers[0], numbers[1], numbers[2]), true
	}

	return Color(numbers), true
}

// fromHSV turns a hue, a saturation and a value into red, green and blue. The
// hue goes round once from 0 to 1, so that 1.01 is the hue of 0.01, and the
// value scales the colour, beyond 1 for a light brighter than white.
func fromHSV(hue, saturation, value float64) Color {
	hue -= math.Floor(hue)
	saturation = min(max(saturation, 0), 1)

	sector := hue * 6
	chroma := saturation
	second := chroma * (1 - math.Abs(math.Mod(sector, 2)-1))

	var red, green, blue float64

	switch int(sector) % 6 {
	case 0:
		red, green = chroma, second
	case 1:
		red, green = second, chroma
	case 2:
		green, blue = chroma, second
	case 3:
		green, blue = second, chroma
	case 4:
		red, blue = second, chroma
	default:
		red, blue = chroma, second
	}

	lightest := 1 - chroma

	return Color{(red + lightest) * value, (green + lightest) * value, (blue + lightest) * value}
}
