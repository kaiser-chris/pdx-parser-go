package report

import "testing"

func TestDiagnosticString(t *testing.T) {
	cases := map[string]Diagnostic{
		"warning: a.txt:3:4: good x: bad": {Severity: SeverityWarning, Path: "a.txt", Line: 3, Column: 4, Subject: "good x", Message: "bad"},
		"info: a.txt:3: good x: bad":      {Severity: SeverityInfo, Path: "a.txt", Line: 3, Subject: "good x", Message: "bad"},
		"error: a.txt: unreadable":        {Severity: SeverityError, Path: "a.txt", Message: "unreadable"},
		"error: My Mod: no folder":        {Severity: SeverityError, Source: "My Mod", Message: "no folder"},
	}

	for want, diagnostic := range cases {
		if got := diagnostic.String(); got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	}
}

func TestDiagnosticsCountAndFilter(t *testing.T) {
	diagnostics := Diagnostics{
		{Severity: SeverityInfo}, {Severity: SeverityWarning}, {Severity: SeverityError}, {Severity: SeverityWarning},
	}

	if diagnostics.Count(SeverityWarning) != 3 || diagnostics.Count(SeverityInfo) != 4 {
		t.Errorf("counts = %d, %d", diagnostics.Count(SeverityWarning), diagnostics.Count(SeverityInfo))
	}

	if errors := diagnostics.Filter(SeverityError); len(errors) != 1 || errors[0].Severity != SeverityError {
		t.Errorf("errors = %+v", errors)
	}
}

func TestCollector(t *testing.T) {
	var collector Collector

	collector.Addf(SeverityWarning, "game", "a.txt", 1, 2, "good x", "cost is %d", 5)
	collector.AddAll(Diagnostics{{Severity: SeverityInfo, Message: "note"}})

	if len(collector.Diagnostics) != 2 || collector.Diagnostics[0].Message != "cost is 5" || collector.Diagnostics[0].Column != 2 {
		t.Errorf("collected = %+v", collector.Diagnostics)
	}
}
