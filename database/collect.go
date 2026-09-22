package database

import (
	"os"
	"runtime"
	"sync"

	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// Parsed is a file together with what was read from it.
type Parsed struct {
	File     folders.File
	Document *script.Document
}

// ParseFiles reads and parses files. The parsing runs in parallel, since files
// do not depend on each other, but the result keeps the order of files, and
// the syntax warnings and read errors are reported in that order too, so the
// outcome does not depend on timing. A file that cannot be read is reported
// and comes back as an empty document.
func ParseFiles(files []folders.File, collector *report.Collector) []Parsed {
	parsed := make([]Parsed, len(files))
	failures := make([]error, len(files))

	var group sync.WaitGroup

	queue := make(chan int)

	for range max(1, min(runtime.GOMAXPROCS(0), len(files))) {
		group.Go(func() {
			for index := range queue {
				parsed[index].File = files[index]

				data, err := os.ReadFile(files[index].Path)
				if err != nil {
					failures[index] = err
					parsed[index].Document = &script.Document{}

					continue
				}

				parsed[index].Document = script.Parse(string(data))
			}
		})
	}

	for index := range files {
		queue <- index
	}

	close(queue)
	group.Wait()

	for index, file := range parsed {
		if failures[index] != nil {
			collector.Addf(report.SeverityError, file.File.Source, file.File.Path, 0, 0, "", "could not be read: %v", failures[index])

			continue
		}

		for _, warning := range file.Document.Warnings {
			collector.Addf(report.SeverityWarning, file.File.Source, file.File.Path, warning.Line, warning.Column, "", "%s", warning.Message)
		}
	}

	return parsed
}

// Collect reads a database folder: it lists the folder's files, parses them
// and merges their definitions in the order the game reads them. kind names
// the definitions in diagnostics, such as "good".
func Collect(set *folders.Set, folder folders.Folder, kind string, collector *report.Collector) *Store {
	files, diagnostics := set.Files(folder)
	collector.AddAll(diagnostics)

	store := NewStore(kind)

	for _, file := range ParseFiles(files, collector) {
		store.AddDocument(file.Document, file.File, collector)
	}

	return store
}
