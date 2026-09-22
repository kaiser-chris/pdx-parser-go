// Package report describes problems found while reading game and mod files.
//
// Nothing in this module stops over a problem in a file. The game files change
// between versions and mods write them by hand, so every loader reads as much
// as it can and hands back a list of Diagnostics saying what it skipped, what
// it guessed at and what looks wrong. A tool can show that list to a mod
// author, filter it by severity, or ignore it.
package report

import (
	"fmt"
	"strings"
)

// Severity is how much a diagnostic matters.
type Severity uint8

const (
	// SeverityInfo is worth knowing but not wrong: a field a loader does not
	// know yet, a definition a mod overrides.
	SeverityInfo Severity = iota

	// SeverityWarning is probably a mistake, and the data around it was read
	// on a best guess: a value of the wrong kind, a reference to something
	// that does not exist, a stray brace.
	SeverityWarning

	// SeverityError is something the game itself treats as an error, such as
	// an INJECT into a definition that does not exist, or a folder that could
	// not be read at all.
	SeverityError
)

func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityError:
		return "error"
	}

	return "unknown"
}

// Diagnostic is one thing a loader noticed.
type Diagnostic struct {
	Severity Severity

	// Source is the name of the game or mod folder the problem is in.
	Source string

	// Path is the file on disk, when the problem is in a file.
	Path string

	// Line and Column locate the problem in the file, counting from 1. They
	// are 0 when the problem is with the file as a whole.
	Line   int
	Column int

	// Subject names what the problem belongs to, such as "production method
	// pm_bakery" or "localization key steel". It is empty for problems with a
	// file as a whole.
	Subject string

	Message string
}

// String writes the diagnostic the way compilers do:
//
//	warning: path/to/file.txt:12:5: good steel: cost should be a number
func (d Diagnostic) String() string {
	var text strings.Builder

	text.WriteString(d.Severity.String())

	if d.Path != "" {
		text.WriteString(": ")
		text.WriteString(d.Path)

		if d.Line > 0 {
			fmt.Fprintf(&text, ":%d", d.Line)

			if d.Column > 0 {
				fmt.Fprintf(&text, ":%d", d.Column)
			}
		}
	} else if d.Source != "" {
		text.WriteString(": ")
		text.WriteString(d.Source)
	}

	if d.Subject != "" {
		text.WriteString(": ")
		text.WriteString(d.Subject)
	}

	text.WriteString(": ")
	text.WriteString(d.Message)

	return text.String()
}

// Diagnostics is a list of diagnostics.
type Diagnostics []Diagnostic

// Count returns how many diagnostics have at least the given severity.
func (d Diagnostics) Count(minimum Severity) int {
	count := 0

	for _, diagnostic := range d {
		if diagnostic.Severity >= minimum {
			count++
		}
	}

	return count
}

// Filter returns the diagnostics with at least the given severity.
func (d Diagnostics) Filter(minimum Severity) Diagnostics {
	var filtered Diagnostics

	for _, diagnostic := range d {
		if diagnostic.Severity >= minimum {
			filtered = append(filtered, diagnostic)
		}
	}

	return filtered
}

// Collector gathers diagnostics while something is being loaded. The zero
// value is ready to use.
type Collector struct {
	Diagnostics Diagnostics
}

// Add records a diagnostic.
func (c *Collector) Add(diagnostic Diagnostic) {
	c.Diagnostics = append(c.Diagnostics, diagnostic)
}

// Addf records a diagnostic built from its parts, with the message formatted
// as by fmt.Sprintf.
func (c *Collector) Addf(severity Severity, source, path string, line, column int, subject, format string, args ...any) {
	c.Add(Diagnostic{
		Severity: severity,
		Source:   source,
		Path:     path,
		Line:     line,
		Column:   column,
		Subject:  subject,
		Message:  fmt.Sprintf(format, args...),
	})
}

// AddAll records a list of diagnostics.
func (c *Collector) AddAll(diagnostics Diagnostics) {
	c.Diagnostics = append(c.Diagnostics, diagnostics...)
}
