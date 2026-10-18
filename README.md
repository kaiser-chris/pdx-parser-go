# pdx-parser-go

Go packages for reading the data of Paradox games and their mods: the script
language of their files, the way the games combine a game with its mods, their
localization, and the economic definitions of Victoria 3.

Everything here is built to keep working on files it has never seen. Parsing
never fails and never panics: broken files, unknown fields and fields of the
wrong kind are read as far as they can be and reported as diagnostics with file,
line and column, so a tool built on these packages keeps working when a game
update or a mod changes the format.

```
go get github.com/kaiser-chris/pdx-parser-go@latest
```

Needs Go 1.26. There are no dependencies outside the standard library.

## Packages

The packages are layered from the timeless to the game specific. Each only
depends on the ones above it, so the script parser can be used on its own and
none of the general packages depend on any particular game.

| Package | What it does | Depends on | Stability |
|---|---|---|---|
| [`script`](script) | Parses the script language of the game files into a tree, and puts a definition into a file's text where it leaves the rest as it was. | nothing | Follows the language, which has not changed in years. |
| [`report`](report) | Diagnostics: what a loader skipped, guessed at or found wrong. | nothing | Stable. |
| [`folders`](folders) | Works out which files the game reads from the game and its mods, and in what order. | `script`, `report` | Follows the engine. |
| [`database`](database) | Reads database folders such as `common/goods`: applies `INJECT:`, `REPLACE:` and the other modes, tracks where every part came from, and decodes definitions into your own types with a tolerant field reader. Also reads defines. | `script`, `folders`, `report` | Follows the engine. |
| [`localization`](localization) | Parses and loads localization files, with `replace` folders. | `folders`, `report` | Follows the engine. |
| [`asset`](asset) | Reads the 3D asset definitions in the `.asset` files anywhere below `gfx`: the `pdxmesh`, `entity` and `skeletal_animation_set` definitions, the references between them, and the files they name. | `script`, `folders`, `database`, `report` | Follows the engine. |
| [`genes`](genes) | Reads the genes of the portraits in `common/genes`: morph genes and the attributes they set and the decals they lay, accessory genes, colour genes and age presets. Also the DNA of the characters in `common/dna_data`, the ethnicities of `common/ethnicities` and the DNA they preview as, the portrait accessories of `gfx/portraits/accessories`, the animation sets of `gfx/portraits/portrait_animations` with the portrait modifiers their variants name, and the portrait modifiers of `gfx/portraits/portrait_modifiers`, such as the props an animation holds. Writes a DNA as `common/dna_data` holds one or as the game's persistent DNA, and reads and writes DNA strings. | `script`, `folders`, `database`, `report` | Follows the engine. |
| [`portrait`](portrait) | Reads how the engine stages a portrait: the environments of `gfx/portraits/environments`, their cubemaps and lights, the cameras of `gfx/portraits/cameras`, and the tonemap and colour settings of `gfx/portraits/jomini_environment.txt`. | `script`, `folders`, `database`, `report` | Follows the engine. |
| [`victoria3`](victoria3) | The economy of Victoria 3: goods, buildings, production methods, pops, companies, technologies and eras. Also its coats of arms and named colours. | all of the above | **Follows the game.** Written against 1.13. |

`victoria3` is the one package expected to date. When the game adds or renames
fields it keeps working, reporting what it does not know, but its types need
updating to cover them. Everything it does can be done for any other kind of
definition, or any other game, with `database`.

## Examples

Every package has runnable examples in its documentation. In short:

### Parsing a file

```go
document := script.Parse(`steel = { cost = 50 possible = { level >= 5 } }`)

steel, _ := document.Get("steel")
cost, _ := steel.Get("cost")
fmt.Println(cost.Number) // 50

for _, warning := range document.Warnings {
	fmt.Println(warning) // nothing here; a broken file would say what was skipped
}
```

### Reading a game with its mods

```go
set := folders.Open([]folders.Source{
	{Name: "Victoria 3", Path: `C:\Steam\steamapps\common\Victoria 3\game`},
	{Path: `C:\mods\community_mod_framework`},
	{Path: `C:\mods\my_mod`},
})

data := victoria3.Load(set)

steel, _ := data.Goods.Get("steel")
fmt.Println(steel.Cost, steel.Origins) // the cost after every mod, and where it came from

bessemer, _ := data.ProductionMethods.Get("pm_bessemer_process")
fmt.Println(bessemer.Inputs(), bessemer.Outputs())

for _, diagnostic := range data.Diagnostics.Filter(report.SeverityWarning) {
	fmt.Println(diagnostic) // warning: path/to/file.txt:12:5: good tin: cost should be a number, ...
}
```

### Decoding a kind of definition yourself

```go
type Law struct {
	database.Definition
	Group string
}

collector := &report.Collector{}
store := database.Collect(set, folders.Folder{Path: "common/laws"}, "law", collector)

laws := database.Decode(store, collector, func(r *database.Reader, _ *database.Definition) *Law {
	r.Known("possible", "ai_will_do") // triggers this decoder does not read
	return &Law{Group: r.Text("group")}
})
```

### Reading the coats of arms

```go
heraldry := victoria3.LoadHeraldry(set)

arms, _ := heraldry.CoatOfArms.Get("TUR")
fmt.Println(arms.Pattern.File) // pattern_solid.tga

color, _ := arms.Colors.Get("color1")
fmt.Println(color.Resolve(heraldry.Colors, arms.Colors)) // the named colour, resolved

for _, layer := range arms.Layers {
	emblem, ok := layer.(*victoria3.ColoredEmblem)
	if ok {
		fmt.Println(emblem.Texture.File, emblem.Placements())
	}
}
```

The coats of arms of Europa Universalis 5 are written the same way and are
read by the same code, layers, mods and all.

### Localization

```go
english := localization.Load(set, "english")
fmt.Println(english.Name("steel")) // Steel, or a mod's text from a replace folder
```

### 3D assets

```go
assets := asset.Load(set)

entity, _ := assets.Entities.Get("female_body_entity")
mesh, _ := assets.MeshOf(entity.Key) // its own mesh, or that of the entity it clones

fmt.Println(asset.Resolve(mesh.Origin(), mesh.File)) // gfx/models/portraits/female_body/female_body.mesh

for _, shape := range mesh.BlendShapes {
	fmt.Println(shape.ID, shape.File)
}
```

The asset files of Europa Universalis 5 and Crusader Kings 3 are read by the
same code, and what they write that Victoria 3 does not is read too: a state's
chance that depends on the state before it, and the animation sets Crusader
Kings 3 shares the animations of its portraits through, which `AnimationsOf`
lists with a mesh's own animations.

### Genes and DNA

```go
loaded := genes.Load(set)

nose, _ := loaded.MorphGenes.Get("gene_nose_size")
template, _ := nose.Template("nose_size")

for _, setting := range template.For(genes.Male).Settings {
	fmt.Println(setting.Attribute, setting.At(0.75)) // the weight of a blend shape of the head entity
}

dna, _ := genes.LoadDNA(set)
lincoln, _ := dna.Get("dna_abraham_lincoln")
value, _ := lincoln.Gene("gene_nose_size")
fmt.Println(value.Dominant.Template, value.Dominant.Strength())

// Written back into its file in place of the definition it was read from,
// leaving the rest of the file, comments and all, as it was.
merged, _, err := script.PutDefinition(file, "dna_abraham_lincoln", "dna_abraham_lincoln",
	genes.WriteDNA("dna_abraham_lincoln", lincoln, "\n"))

// The same genes as a DNA string, and as the game's persistent DNA.
text, _ := loaded.WriteString(lincoln)
again, _ := loaded.ReadString(text)
persistent := genes.WritePersistentDNA("dna_abraham_lincoln", again, "\n")
```

A DNA string, such as `z78g+0cGKAYrmni4AAAAfACI...`, is base64 of four bytes
for each gene: the dominant allele's two values and then the recessive
allele's, a colour gene's two places in its palette or a morph or accessory
gene's template index and strength. The string names no gene. The genes come
in the order the game defines them: colour genes, then morph genes, then
accessory genes. A string can therefore only be read with the genes it was
written with. Once a gene is added, every string written before it is read
wrong from that gene on.

## Robustness

The games read past the odd typo, and so must a tool. What each layer does
with input that does not fit:

- **script** skips a closing brace that closes nothing, closes a block left
  open at the end of the file, drops a key without a value without taking the
  next line's key as its value, ends an unclosed string at its line, keeps an
  undefined variable as text, and skips anything nested more than 1000 levels
  deep, so a corrupt file cannot exhaust the stack. Every one of these is a
  warning with line and column, and the definitions around the damage survive.
- **database** reads a missing field as its default, a field of the wrong kind
  as its default plus a warning, and a field it was never asked about as an
  unknown field that stays readable through `Definition.Raw`. A reference
  between definitions that does not resolve is reported and kept.
- **localization** keeps an entry with a missing colon or an unclosed quote, and
  reports it.
- **asset** skips a definition without a name, which nothing could refer to,
  reads the rest the way **database** does, and reports a reference to a mesh
  or entity that does not exist, and a chain of clones that loops, without
  losing the definition.

The test suites include hostile input written by hand, random input, inputs of
several megabytes shaped to expose quadratic steps, and fuzz targets for every
parser. The shipped economy files of Victoria 3 and all eleven of its languages
load without a single warning. The asset files of Victoria 3, Europa
Universalis 5 and Crusader Kings 3 load without a field left unread and
without a warning beyond the typos and broken references in the files
themselves.

The general packages have also been run over every database folder,
localization and define of Europa Universalis 5 and Crusader Kings 3, with
workshop mods loaded on top. What those games write that Victoria 3 does not is
read as they write it, each case with a test of its own:

- layers of game files (`in_game`, `main_menu`, `loading_screen`) and DLCs in
  `game/dlc` holding game files of their own;
- a byte order mark before `.metadata/metadata.json`;
- variables holding strings, names and lists (`@dlc = "d008"`);
- strings over several lines, started by a quote at the end of a line;
- a keyword before a definition (`scripted_trigger is_ready = { ... }`);
- single-quoted strings, commas between values, an operator as a value
  (`OPERATOR = <=`), negated parameters (`-$AMOUNT$`), and `'`, `&` and `%`
  inside names;
- definitions that are not blocks, such as script values that are plain
  numbers.

What is left are real typos in the shipped files, such as a block missing its
last brace or a `=` missing before a block, which are read the way the games
read them and reported.

## How the game and its mods are combined

`folders` applies the rules the games use:

- Folders are given in load order, the game first.
- A file replaces the file of the same path in every earlier folder.
- A mod's replace paths, from `.metadata/metadata.json` (Victoria 3) or
  `descriptor.mod` (the older games), throw away every earlier file of a
  directory. A replace path covers its directory, not the ones below it.
- The rest is read in order of file name, whichever folder each file came from.
- A game that splits its files into layers, folders such as `in_game` and
  `main_menu` that each hold a `common` and a `localization`, has a folder
  looked for in every layer, so `common/goods` finds `in_game/common/goods`. A
  folder can also be named with its layer to read that layer alone.
- DLCs in the game's `dlc` folder that hold game files are read right after
  the game, each as a folder of its own.

`Set.Find` looks up a single file the same way, such as a mesh an `.asset`
file names: the latest folder that has it wins, and a replace path hides the
folders before it.

`database` then applies the modification modes as the definitions are read:

| Key | Effect |
|---|---|
| `steel` | Defines steel, or overrides an earlier definition completely. |
| `REPLACE:steel` | Replaces the earlier definition; an error if there is none. |
| `TRY_REPLACE:steel` | Replaces the earlier definition, if there is one. |
| `REPLACE_OR_CREATE:steel` | Replaces or defines. |
| `INJECT:steel` | Adds fields to the earlier definition; an error if there is none. A list field is extended, any other field overridden. |
| `TRY_INJECT:steel` | Adds fields, if there is an earlier definition. |
| `INJECT_OR_CREATE:steel` | Adds fields, or defines. |

In localization, a text runs from the first quote to the last quote that is not
part of a trailing comment, so quotes inside it need no escaping. Texts in a
`replace` folder override texts anywhere else. Outside `replace` folders, the
first definition of a key in load order is used, which is the reason `replace`
folders exist; this is the one rule here that was inferred rather than read
from the files, and `localization.Load` documents it.

## Versioning

The module follows [semantic versioning](https://semver.org). Until 1.0 the API
may still change between minor versions. After 1.0, `script`, `report`,
`folders`, `database`, `localization` and `asset` keep their API; `victoria3` may add
fields in minor versions as the game changes, and only removes or renames them
in a major version.

## Development

```
make test        # everything that needs no game installation
make gametest PDX_GAME_DIR="C:/Steam/steamapps/common/Victoria 3/game"
make fuzz        # a minute of fuzzing for each parser
```

`make gametest` also runs the tests against a real installation, and
`PDX_MOD_DIRS` adds mods on top of it, in load order, separated by `;` on
Windows and `:` elsewhere. The tests expect the game's own files to load
without warnings, which makes them the quickest way to find out what a game
update changed.

The example game and mod the documentation uses are in `testdata/`.

## License

MIT, see [LICENSE](LICENSE).
