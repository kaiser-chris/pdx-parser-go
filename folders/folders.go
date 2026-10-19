// Package folders works out which files a Paradox game reads from a set of
// game and mod folders, and in what order.
//
// A game with mods reads one merged view of all their files. The rules that
// build that view are the same across the games on Paradox's Clausewitz and
// Jomini engines:
//
//   - Folders are given in load order: the game first, then each mod.
//   - A file replaces the file with the same path in every earlier folder,
//     which is how a mod replaces a whole file of the game.
//   - A mod can throw away every earlier file of a directory by listing it as a
//     replace path: under game_custom_data.replace_paths in the
//     .metadata/metadata.json of Victoria 3, or as replace_path in the
//     descriptor.mod of the older games. A replace path covers that directory
//     only, not the ones below it.
//   - Whatever is left is read in order of file name, whichever folder each
//     file came from.
//
// Some games split their files into layers, each a folder with its own
// common, localization and so on: Europa Universalis 5 has in_game, main_menu
// and loading_screen, and ships its DLCs in the same shape below game/dlc.
// Files looks for a folder in every layer, and Open adds the DLCs that hold
// game files as folders of their own, read right after their game.
//
// Open resolves the folders once; Files then lists the files of any folder
// below them, such as common/goods or localization.
package folders

import (
	"bytes"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// Source is one configured folder: the game, or a mod.
type Source struct {
	// Name is how diagnostics and files refer to the folder. When it is
	// empty, the mod's name from its metadata is used, or the folder's name.
	Name string

	// Path is the folder. It may be the root of a mod, the game's install
	// folder or its game folder, or a common or localization folder directly.
	Path string
}

// Folder is a folder below the roots whose files are wanted.
type Folder struct {
	// Path is relative to the roots, with forward slashes, such as
	// "common/goods" or "localization".
	Path string

	// Recursive includes the files in folders below Path as well. The games
	// read localization and defines this way, but most of common/ not.
	Recursive bool

	// Extension picks the files, such as ".yml". It is ".txt" when empty.
	Extension string
}

// File is one file the game reads.
type File struct {
	// Relative is the file's path below its source's root, with forward
	// slashes, as written: common/goods/00_goods.txt, or
	// in_game/common/goods/00_goods.txt for a file in a layer.
	Relative string

	// Layer is the layer the file is in, such as in_game, or empty for a
	// game without layers.
	Layer string

	// Path is where the file is on disk.
	Path string

	// Source is the name of the folder the file came from, and SourceOrder
	// that folder's place in the load order, counting from 0.
	Source      string
	SourceOrder int
}

// RootMarkers are the folders that mark the root of a game or mod: a folder
// holding any of them is a root. A mod that only translates has a
// localization folder and nothing else.
var RootMarkers = []string{"common", "localization", "events", "gfx", "gui", "interface", "history", "map_data"}

// Set is a list of game and mod folders, resolved and in load order.
type Set struct {
	roots []root

	// Diagnostics are the problems with the folders themselves, such as one
	// that holds no game or mod files.
	Diagnostics report.Diagnostics
}

// root is a Source whose root folder has been found.
type root struct {
	Source

	// dir is the folder that holds common/, localization/ and so on, or the
	// layers that do.
	dir string

	// layers are the folders below dir that hold game files of their own,
	// such as in_game, in order of name.
	layers []string

	// replacePaths are the directories, relative to dir and normalized, whose
	// files from earlier folders this one throws away.
	replacePaths map[string]bool
}

// Open resolves the folders, in load order. A folder that cannot be used is
// left out and reported in the set's Diagnostics.
func Open(sources []Source) *Set {
	set := &Set{}
	collector := &report.Collector{}

	for _, source := range sources {
		if found, ok := resolve(source, collector); ok {
			set.roots = append(set.roots, found)
			set.roots = append(set.roots, dlcs(found)...)
		}
	}

	set.Diagnostics = collector.Diagnostics

	return set
}

// dlcFolder is where a game keeps its DLCs, below its root.
const dlcFolder = "dlc"

// dlcs returns the DLCs of a game that hold game files, in order of name, as
// roots of their own named after the game and the DLC's folder. Most DLCs of
// Victoria 3 hold only music, which is no game file.
func dlcs(game root) []root {
	entries, err := os.ReadDir(filepath.Join(game.dir, dlcFolder))
	if err != nil {
		return nil
	}

	var found []root

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dir := filepath.Join(game.dir, dlcFolder, entry.Name())
		layers := findLayers(dir)

		if !holdsMarker(dir) && len(layers) == 0 {
			continue
		}

		found = append(found, root{
			Source:       Source{Name: game.Name + "/" + entry.Name(), Path: dir},
			dir:          dir,
			layers:       layers,
			replacePaths: map[string]bool{},
		})
	}

	return found
}

// Layers returns the layers of the folders, in order of name, or nothing when
// no folder has any.
func (s *Set) Layers() []string {
	layers := map[string]bool{}

	for _, root := range s.roots {
		for _, layer := range root.layers {
			layers[layer] = true
		}
	}

	return sortedKeys(layers)
}

// Names returns the names of the folders that could be used, in load order.
func (s *Set) Names() []string {
	names := make([]string, 0, len(s.roots))
	for _, root := range s.roots {
		names = append(names, root.Name)
	}

	return names
}

// Roots returns the root folders on disk, in load order.
func (s *Set) Roots() []string {
	roots := make([]string, 0, len(s.roots))
	for _, root := range s.roots {
		roots = append(roots, root.dir)
	}

	return roots
}

// Files lists the files of a folder the game reads, in the order it reads
// them. The diagnostics note which files replaced which, and any folder that
// could not be read.
func (s *Set) Files(folder Folder) ([]File, report.Diagnostics) {
	collector := &report.Collector{}
	byName := map[string]File{}

	extension := folder.Extension
	if extension == "" {
		extension = ".txt"
	}

	for order, root := range s.roots {
		replaced := map[string]bool{}

		for name, file := range byName {
			directory := Normalize(path.Dir(file.Relative))
			if root.replacePaths[directory] {
				delete(byName, name)
				replaced[directory] = true
			}
		}

		for _, directory := range sortedKeys(replaced) {
			collector.Add(report.Diagnostic{
				Severity: report.SeverityInfo,
				Source:   root.Name,
				Path:     filepath.Join(root.dir, filepath.FromSlash(directory)),
				Message:  "replaces every file of the folders before it in " + directory,
			})
		}

		// The folder is looked for below the root itself and below each of its
		// layers.
		for _, layer := range append([]string{""}, root.layers...) {
			below := strings.Trim(folder.Path, "/")
			if layer != "" {
				below = layer + "/" + below
			}

			directory := filepath.Join(root.dir, filepath.FromSlash(below))
			if !isDirectory(directory) {
				continue
			}

			for _, relativeToFolder := range listFiles(directory, folder.Recursive, extension, root.Name, collector) {
				relative := below + "/" + relativeToFolder
				name := strings.ToLower(relative)
				onDisk := filepath.Join(directory, filepath.FromSlash(relativeToFolder))

				if previous, ok := byName[name]; ok {
					collector.Add(report.Diagnostic{
						Severity: report.SeverityInfo,
						Source:   root.Name,
						Path:     onDisk,
						Message:  "replaces the file of the same name from " + previous.Source,
					})
				}

				byName[name] = File{Relative: relative, Layer: layer, Path: onDisk, Source: root.Name, SourceOrder: order}
			}
		}
	}

	files := make([]File, 0, len(byName))
	for _, file := range byName {
		files = append(files, file)
	}

	// The files of all layers are read in order of name as if they were one
	// folder; the layer only decides between files of the same name.
	sort.Slice(files, func(first, second int) bool {
		a, b := files[first].withinLayer(), files[second].withinLayer()
		if a != b {
			return a < b
		}

		return strings.ToLower(files[first].Relative) < strings.ToLower(files[second].Relative)
	})

	return files, collector.Diagnostics
}

// Find looks up one file the game reads, by its path below the roots, such as
// gfx/models/portraits/male_body/male_body.mesh. The file of the latest folder
// in load order that has it is the one the game reads, the way a mod replaces
// a file of the game.
//
// A folder that lists the file's directory as a replace path hides the folders
// before it, so a file only they have is not found. In a game with layers the
// path may name the layer it is in, as File.Relative does: in_game/gfx/....
// A path that names none is looked for in each layer, in order of name, as
// the game finds main_menu/gfx/portraits/portraitsettings.txt by
// gfx/portraits/portraitsettings.txt.
//
// A path that climbs above the roots with ../ is read as if it stopped at
// them, so Find never looks outside the folders it was given.
func (s *Set) Find(relative string) (File, bool) {
	relative = strings.Trim(path.Clean("/"+strings.ReplaceAll(relative, "\\", "/")), "/")
	if relative == "" {
		return File{}, false
	}

	directory := Normalize(path.Dir(relative))

	for order := len(s.roots) - 1; order >= 0; order-- {
		root := s.roots[order]

		// A path that names no layer of a folder with layers is looked for in
		// each of them: the game merges its layers, and the files that name
		// another, such as the genes naming a palette, write the path below
		// them.
		candidates := []string{relative}
		if first, _, _ := strings.Cut(relative, "/"); !slices.Contains(root.layers, first) {
			for _, layer := range root.layers {
				candidates = append(candidates, layer+"/"+relative)
			}
		}

		for _, candidate := range candidates {
			onDisk := filepath.Join(root.dir, filepath.FromSlash(candidate))

			if info, err := os.Stat(onDisk); err == nil && !info.IsDir() {
				file := File{Relative: candidate, Path: onDisk, Source: root.Name, SourceOrder: order}

				if layer, _, found := strings.Cut(candidate, "/"); found && slices.Contains(root.layers, layer) {
					file.Layer = layer
				}

				return file, true
			}
		}

		if root.replacePaths[directory] {
			return File{}, false
		}
	}

	return File{}, false
}

// withinLayer is the file's path below its layer, in lower case.
func (f File) withinLayer() string {
	return strings.ToLower(strings.TrimPrefix(f.Relative, f.Layer+"/"))
}

// resolve finds a source's root. It accepts the several folders a user is
// likely to pick: a mod's root, the game's install folder, its game folder, or
// one of the folders that mark a root.
func resolve(source Source, collector *report.Collector) (root, bool) {
	found := root{Source: source, replacePaths: map[string]bool{}}

	candidates := []string{source.Path, filepath.Join(source.Path, "game")}

	for _, marker := range RootMarkers {
		if strings.EqualFold(filepath.Base(source.Path), marker) {
			candidates = append(candidates, filepath.Dir(source.Path))
		}
	}

	// A candidate that holds game files itself wins over one whose layers do:
	// otherwise the install folder of Victoria 3 would pass for a game with
	// one layer, called game.
	for _, candidate := range candidates {
		if holdsMarker(candidate) {
			found.dir = candidate
			found.layers = findLayers(candidate)

			break
		}
	}

	if found.dir == "" {
		for _, candidate := range candidates {
			if layers := findLayers(candidate); len(layers) > 0 {
				found.dir = candidate
				found.layers = layers

				break
			}
		}
	}

	meta := readMetadata(source.Path, found.dir)

	if found.Name == "" {
		found.Name = meta.name
	}

	if found.Name == "" {
		found.Name = filepath.Base(filepath.Clean(source.Path))
	}

	if found.dir == "" {
		collector.Add(report.Diagnostic{
			Severity: report.SeverityError,
			Source:   found.Name,
			Path:     source.Path,
			Message:  "no game or mod files found here, such as a common or localization folder; pick the game folder or the root folder of a mod",
		})

		return found, false
	}

	for _, replaced := range meta.replacePaths {
		found.replacePaths[Normalize(replaced)] = true
	}

	return found, true
}

// holdsMarker reports whether a folder holds any of the RootMarkers.
func holdsMarker(dir string) bool {
	for _, marker := range RootMarkers {
		if isDirectory(filepath.Join(dir, marker)) {
			return true
		}
	}

	return false
}

// findLayers returns the folders directly below dir that hold game files of
// their own, in order of name. The markers themselves are no layers, and
// neither are the DLCs, which are roots of their own.
func findLayers(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var layers []string

	for _, entry := range entries {
		name := entry.Name()

		if !entry.IsDir() || strings.EqualFold(name, dlcFolder) || isMarker(name) {
			continue
		}

		if holdsMarker(filepath.Join(dir, name)) {
			layers = append(layers, name)
		}
	}

	sort.Strings(layers)

	return layers
}

func isMarker(name string) bool {
	for _, marker := range RootMarkers {
		if strings.EqualFold(name, marker) {
			return true
		}
	}

	return false
}

// metadata is what the loader needs from a mod's description of itself.
type metadata struct {
	name         string
	replacePaths []string
}

// readMetadata reads a mod's description of itself: the
// .metadata/metadata.json of Victoria 3, or the descriptor.mod of the older
// games. A missing or broken file is not a problem: the game itself has none,
// and a mod without one still loads.
func readMetadata(folders ...string) metadata {
	for _, folder := range folders {
		if folder == "" {
			continue
		}

		if data, err := os.ReadFile(filepath.Join(folder, ".metadata", "metadata.json")); err == nil {
			var parsed struct {
				Name           string `json:"name"`
				GameCustomData struct {
					ReplacePaths []string `json:"replace_paths"`
				} `json:"game_custom_data"`
			}

			// Editors on Windows like to start the file with a byte order
			// mark, which the JSON decoder refuses.
			data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))

			if json.Unmarshal(data, &parsed) == nil {
				return metadata{name: parsed.Name, replacePaths: parsed.GameCustomData.ReplacePaths}
			}
		}

		if data, err := os.ReadFile(filepath.Join(folder, "descriptor.mod")); err == nil {
			document := script.Parse(string(data))

			var found metadata

			if name, ok := document.Get("name"); ok {
				found.name = name.Text
			}

			for _, replaced := range document.All("replace_path") {
				if text, ok := replaced.Str(); ok {
					found.replacePaths = append(found.replacePaths, text)
				}
			}

			return found
		}
	}

	return metadata{}
}

// listFiles returns the files of a directory with an extension, relative to
// it and with forward slashes.
func listFiles(directory string, recursive bool, extension, source string, collector *report.Collector) []string {
	var files []string

	walk := func(current string, entry os.DirEntry, err error) error {
		if err != nil {
			collector.Add(report.Diagnostic{Severity: report.SeverityWarning, Source: source, Path: current, Message: "could not be read: " + err.Error()})

			return nil
		}

		if entry.IsDir() {
			if current != directory && !recursive {
				return filepath.SkipDir
			}

			return nil
		}

		if !strings.EqualFold(filepath.Ext(current), extension) {
			return nil
		}

		relative, err := filepath.Rel(directory, current)
		if err != nil {
			return nil //nolint:nilerr // a path under the folder always has a relative form
		}

		files = append(files, filepath.ToSlash(relative))

		return nil
	}

	if err := filepath.WalkDir(directory, walk); err != nil {
		collector.Add(report.Diagnostic{Severity: report.SeverityWarning, Source: source, Path: directory, Message: "could not be read: " + err.Error()})
	}

	return files
}

// Normalize turns a folder as written in metadata or on disk into the form
// used to compare folders: forward slashes, lower case, no slashes at either
// end. The games run on file systems that ignore case, so mods and games do not
// always agree on it.
func Normalize(folder string) string {
	folder = strings.ReplaceAll(folder, "\\", "/")
	folder = strings.Trim(path.Clean("/"+folder), "/")

	return strings.ToLower(folder)
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)

	return err == nil && info.IsDir()
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}
