package database

import (
	"sort"

	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// DefinesFolder is where the games keep their defines, including the jomini/
// folder below it.
var DefinesFolder = folders.Folder{Path: "common/defines", Recursive: true}

// Defines are the constants of a game's simulation, from common/defines,
// grouped by namespace: NEconomy.PRICE_RANGE, NPops.POP_SIZE_PACKAGE.
//
// Unlike database definitions, defines combine key by key. A mod that writes
// NEconomy = { PRICE_RANGE = 0.5 } changes that one value and leaves every
// other define of the namespace alone, and the games open the same namespace
// more than once themselves.
type Defines struct {
	namespaces map[string]map[string]Define
}

// Define is one constant.
type Define struct {
	Value  script.Node
	Origin Origin
}

// LoadDefines reads every define of a set of folders.
func LoadDefines(set *folders.Set, collector *report.Collector) Defines {
	files, diagnostics := set.Files(DefinesFolder)
	collector.AddAll(diagnostics)

	var defines Defines

	for _, file := range ParseFiles(files, collector) {
		defines.AddDocument(file.Document, file.File, collector)
	}

	return defines
}

// Get returns a define as written.
func (d Defines) Get(namespace, key string) (Define, bool) {
	define, ok := d.namespaces[namespace][key]

	return define, ok
}

// Number returns a define that holds a number.
func (d Defines) Number(namespace, key string) (float64, bool) {
	define, ok := d.Get(namespace, key)
	if !ok {
		return 0, false
	}

	return define.Value.Num()
}

// NumberOr returns a numeric define, or fallback when it is missing or is not
// a number.
func (d Defines) NumberOr(namespace, key string, fallback float64) float64 {
	if number, ok := d.Number(namespace, key); ok {
		return number
	}

	return fallback
}

// Namespaces returns the names of all namespaces, sorted.
func (d Defines) Namespaces() []string {
	names := make([]string, 0, len(d.namespaces))
	for name := range d.namespaces {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

// Keys returns the keys of a namespace, sorted.
func (d Defines) Keys(namespace string) []string {
	keys := make([]string, 0, len(d.namespaces[namespace]))
	for key := range d.namespaces[namespace] {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}

// Len is the number of defines across all namespaces.
func (d Defines) Len() int {
	total := 0
	for _, namespace := range d.namespaces {
		total += len(namespace)
	}

	return total
}

// AddDocument takes the defines of a parsed file. Later values override
// earlier ones key by key.
func (d *Defines) AddDocument(document *script.Document, file folders.File, collector *report.Collector) {
	if d.namespaces == nil {
		d.namespaces = map[string]map[string]Define{}
	}

	for _, namespace := range document.Fields {
		origin := Origin{Source: file.Source, File: file.Relative, Path: file.Path, Line: namespace.Line}
		subject := "define namespace " + namespace.Key

		if namespace.Value.Kind != script.KindBlock {
			if !namespace.Value.IsBlock() {
				collector.Addf(report.SeverityWarning, file.Source, file.Path, namespace.Line, namespace.Column, subject,
					"ignored, because its value %s is not a block", namespace.Value.Describe())
			}

			continue
		}

		values := d.namespaces[namespace.Key]
		if values == nil {
			values = map[string]Define{}
			d.namespaces[namespace.Key] = values
		}

		for _, field := range namespace.Value.Fields {
			fieldOrigin := origin
			fieldOrigin.Line = field.Line
			values[field.Key] = Define{Value: field.Value, Origin: fieldOrigin}
		}

		for _, item := range namespace.Value.Items {
			collector.Addf(report.SeverityWarning, file.Source, file.Path, item.Line, item.Column, subject,
				"ignored %s, which has no key", item.Describe())
		}
	}
}
