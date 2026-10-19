package portrait

import (
	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// FileAssembly is the file that says how a portrait is put together: the
// entities each age and sex is drawn with, and the joints the head is
// attached to the body by.
//
//	male = {
//		head = "male_head_entity"
//		torso = "male_body_entity"
//	}
//	attach = {
//		what = head
//		where = torso
//		joint_attachment = {
//			parent_joint = "bn_sp_thoracic"
//			child_joint = "head_root"
//		}
//	}
//	root = torso
//
// The games differ in all of it: one draws a boy with the body of a man and
// attaches the head at the chest, another gives a boy a body of his own and
// attaches the head at the base of the neck.
const FileAssembly = "gfx/portraits/portraitsettings.txt"

// kindAssembly is what the assembly goes by in diagnostics.
const kindAssembly = "portrait assembly"

// Assembly is how a portrait is put together.
type Assembly struct {
	// Entities are the entities each age and sex is drawn with, by the key
	// the file writes them under, such as male, boy, adolescent_girl or
	// infant.
	Entities map[string]Entities

	// Attachments say which entity is attached to which, and by which
	// joints, in the order written.
	Attachments []Attachment

	// Root is the entity the others are attached to, such as torso.
	Root string

	// SRGBColorPalettes says the palettes the skin, hair and eyes are
	// coloured from hold sRGB colours, which the engine takes into linear
	// light before it multiplies a diffuse map by one, as Europa Universalis
	// 5's file says; otherwise it multiplies by the colour as it is stored.
	SRGBColorPalettes bool
}

// Entities are the entities one age and sex is drawn with.
type Entities struct {
	Head, Torso string
}

// Attachment attaches one entity to another, such as the head to the torso,
// by the joints of each that are to meet.
type Attachment struct {
	What, Where string

	// Joints are the joints that meet, in the order written. The first
	// places the attached entity; the others are where the two are to stay
	// together as they move.
	Joints []JointAttachment
}

// JointAttachment is a joint of the entity attached to, Parent, and the
// joint of the attached entity that goes where it is, Child.
type JointAttachment struct {
	Parent, Child string
}

// The keys of the assembly.
const (
	keyAttach          = "attach"
	keyWhat            = "what"
	keyWhere           = "where"
	keyJointAttachment = "joint_attachment"
	keyParentJoint     = "parent_joint"
	keyChildJoint      = "child_joint"
	keyRoot            = "root"
	keyHead            = "head"
	keyTorso           = "torso"
	keySRGBPalettes    = "srgb_color_palettes"
)

// assemblyGroups are the keys of the ages and sexes the file gives entities
// for.
var assemblyGroups = []string{"male", "female", "boy", "girl", "adolescent_boy", "adolescent_girl", "infant"}

// assemblyOthers are the keys of the file that are not about putting the
// portrait together, which other parts of a game read: the settings of its
// post effects, and in some games its ambient occlusion, which
// jomini_environment.txt holds in others.
var assemblyOthers = []string{"enable_posteffects", "ssao"}

// For returns the entities an age and sex is drawn with, by its key, such as
// male or girl.
func (a Assembly) For(group string) (Entities, bool) {
	entities, ok := a.Entities[group]

	return entities, ok
}

// Attachment returns the attachment of one entity to another, such as of
// head to torso: the last written, as the game takes a field given twice.
func (a Assembly) Attachment(what, where string) (Attachment, bool) {
	for index := len(a.Attachments) - 1; index >= 0; index-- {
		if a.Attachments[index].What == what && a.Attachments[index].Where == where {
			return a.Attachments[index], true
		}
	}

	return Attachment{}, false
}

// LoadAssembly reads how a portrait is put together from a set of folders:
// the last of them that has the file, as the game reads a single file.
// Folders without it give an assembly of nothing, and say so.
//
// Loading never fails. Everything worth knowing about the file is in the
// diagnostics.
func LoadAssembly(set *folders.Set) (Assembly, report.Diagnostics) {
	collector := &report.Collector{}

	file, ok := set.Find(FileAssembly)
	if !ok {
		collector.Addf(report.SeverityWarning, "", FileAssembly, 0, 0, kindAssembly, "%s is in none of the folders; nothing says how a portrait is put together", FileAssembly)

		return Assembly{}, collector.Diagnostics
	}

	parsed := database.ParseFiles([]folders.File{file}, collector)
	if len(parsed) == 0 {
		return Assembly{}, collector.Diagnostics
	}

	return ReadAssembly(parsed[0].Document, file, collector), collector.Diagnostics
}

// ReadAssembly reads how a portrait is put together from a file that has
// already been parsed.
func ReadAssembly(document *script.Document, file folders.File, collector *report.Collector) Assembly {
	definition := &database.Definition{
		Key:     "portraitsettings",
		Origins: []database.Origin{{Source: file.Source, File: file.Relative, Path: file.Path}},
		Raw:     script.Node{Kind: script.KindBlock, Fields: document.Fields},
	}

	r := database.NewReader(definition, kindAssembly, collector)
	r.Known(assemblyOthers...)

	assembly := Assembly{Entities: map[string]Entities{}, Root: r.Text(keyRoot), SRGBColorPalettes: r.FlagOr(keySRGBPalettes, false)}

	for _, group := range assemblyGroups {
		blocks := r.Blocks(group)
		if len(blocks) == 0 {
			continue
		}

		last := blocks[len(blocks)-1]
		child := r.Child(last.Index, last.Value, group)
		assembly.Entities[group] = Entities{Head: child.Text(keyHead), Torso: child.Text(keyTorso)}
		child.Finish()
	}

	for _, block := range r.Blocks(keyAttach) {
		child := r.Child(block.Index, block.Value, keyAttach)
		attachment := Attachment{What: child.Text(keyWhat), Where: child.Text(keyWhere)}

		for _, joint := range child.Blocks(keyJointAttachment) {
			grandchild := child.Child(joint.Index, joint.Value, keyJointAttachment)
			attachment.Joints = append(attachment.Joints, JointAttachment{
				Parent: grandchild.Text(keyParentJoint),
				Child:  grandchild.Text(keyChildJoint),
			})
			grandchild.Finish()
		}

		child.Finish()
		assembly.Attachments = append(assembly.Attachments, attachment)
	}

	r.Finish()

	return assembly
}
