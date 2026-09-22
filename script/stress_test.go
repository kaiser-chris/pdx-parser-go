package script

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"
)

// checkInvariants asserts what must hold for every document, whatever the
// input: spans lie inside the input and in order, positions are real, and
// every warning points at a line that exists.
func checkInvariants(t *testing.T, input string, document *Document) {
	t.Helper()

	lines := strings.Count(input, "\n") + 1

	var walk func(fields []Field, items []Node, low, high int, depth int)

	checkNode := func(node Node, depth int) {
		if node.Kind == 0 {
			t.Errorf("a value without a kind made it into the tree: %+v", node)
		}

		if node.Line < 1 || node.Line > lines {
			t.Errorf("value at line %d of a %d line input", node.Line, lines)
		}

		if node.Kind == KindTagged && node.Value == nil {
			t.Errorf("tagged value %q has no value", node.Tag)
		}
	}

	walk = func(fields []Field, items []Node, low, high int, depth int) {
		if depth > maxDepth+1 {
			t.Fatalf("tree is %d levels deep, more than the limit of %d", depth, maxDepth)
		}

		previousEnd := low

		for _, field := range fields {
			if field.Start < previousEnd || field.End < field.Start || field.End > high {
				t.Errorf("field %q spans [%d,%d), outside [%d,%d) or overlapping the field before it", field.Key, field.Start, field.End, previousEnd, high)
			}

			if field.Line < 1 || field.Line > lines {
				t.Errorf("field %q at line %d of a %d line input", field.Key, field.Line, lines)
			}

			if field.Operator == "" {
				t.Errorf("field %q has no operator", field.Key)
			}

			checkNode(field.Value, depth)
			walk(field.Value.Fields, field.Value.Items, field.Start, field.End, depth+1)

			if field.Value.Kind == KindTagged && field.Value.Value != nil {
				walk(field.Value.Value.Fields, field.Value.Value.Items, field.Start, field.End, depth+1)
			}

			previousEnd = field.End
		}

		for _, item := range items {
			checkNode(item, depth)
			walk(item.Fields, item.Items, low, high, depth+1)
		}
	}

	walk(document.Fields, nil, 0, len(input), 0)

	for _, warning := range document.Warnings {
		if warning.Line < 1 || warning.Line > lines {
			t.Errorf("warning at line %d of a %d line input: %s", warning.Line, lines, warning.Message)
		}

		if warning.Message == "" {
			t.Error("a warning without a message")
		}
	}
}

// parseWithin parses input and fails if it takes longer than limit, which is
// what an accidental quadratic step or an endless loop looks like.
func parseWithin(t *testing.T, input string, limit time.Duration) *Document {
	t.Helper()

	done := make(chan *Document, 1)

	go func() { done <- Parse(input) }()

	select {
	case document := <-done:
		return document
	case <-time.After(limit):
		t.Fatalf("parsing %d bytes took longer than %s", len(input), limit)

		return nil
	}
}

// TestParseHostileInputs feeds the parser input that is broken in unusual
// ways. None of it may crash or hang the parser, every result must satisfy the
// invariants, and where something is recoverable it must be recovered.
func TestParseHostileInputs(t *testing.T) {
	cases := []struct {
		name  string
		input string

		// keys, when set, are the top level keys that must survive.
		keys []string

		// clean inputs must not produce any warning.
		clean bool
	}{
		{name: "empty", input: "", clean: true},
		{name: "only whitespace", input: " \t\r\n\n  ", clean: true},
		{name: "only a byte order mark", input: byteOrderMark, clean: true},
		{name: "two byte order marks", input: byteOrderMark + byteOrderMark + "a = 1", keys: []string{"a"}},
		{name: "only comments", input: "# one\n#two\n   # three", clean: true},
		{name: "comment without newline at end", input: "a = 1 # trailing", keys: []string{"a"}, clean: true},
		{name: "only braces", input: "{{{}}}}}{{", keys: nil},
		{name: "only closing braces", input: strings.Repeat("}", 1000), keys: nil},
		{name: "only opening braces", input: strings.Repeat("{", 1000), keys: nil},
		{name: "only operators", input: "= == != < <= > >= ?= ! ?", keys: nil},
		{name: "only equals", input: strings.Repeat("=", 500), keys: nil},
		{name: "key at end of file", input: "a = 1\nlonely", keys: []string{"a"}},
		{name: "key and operator at end of file", input: "a = 1\nb >=", keys: []string{"a"}},
		{name: "operator chain", input: "a = = = 1\nb = 2", keys: []string{"b"}},
		// An operator alone after an operator is a value, as Crusader Kings 3
		// passes them to scripted triggers.
		{name: "value is an operator", input: "a = >\nb = 2", keys: []string{"a", "b"}, clean: true},
		{name: "operators as values in a row", input: "a = > < b = 2\nc = <=", keys: []string{"b", "c"}},
		{name: "operator value at end of file", input: "a = ?=", keys: []string{"a"}, clean: true},
		{name: "single quotes never closed", input: "a = 'x\nb = '\nc = ''", keys: []string{"a", "c"}},
		{name: "commas everywhere", input: ",,a = { 1,,2 },b = c,", keys: []string{"a", "b"}, clean: true},
		{name: "number keys", input: "1 = a\n2.5 = b\n-3 = c", keys: []string{"1", "2.5", "-3"}, clean: true},
		{name: "quoted keys", input: `"a b" = 1 "" = 2`, keys: []string{"a b", ""}, clean: true},
		{name: "keys that look like dates", input: "1836.1.1 = { a = 1 }", keys: []string{"1836.1.1"}, clean: true},
		{name: "lone signs and dots", input: "a = -\nb = +\nc = .\nd = ..\ne = +.\nf = 1", keys: []string{"d", "e", "f"}},
		{name: "odd numbers", input: "a = 1.2.3\nb = 007\nc = -0\nd = 1e5\ne = 1f\nf = .5f", keys: []string{"a", "b", "c", "d", "e", "f"}, clean: true},
		{name: "backslash at end of file", input: `a = "abc\`, keys: []string{"a"}},
		{name: "quote at end of file", input: `a = "`, keys: []string{"a"}},
		{name: "escaped quote", input: `a = "say \"hi\"" b = 1`, keys: []string{"a", "b"}, clean: true},
		{name: "every line an unclosed string", input: strings.Repeat("a = \"x\n", 200) + "b = 1", keys: append(repeatKey("a", 200), "b")},
		{name: "sigil alone", input: "@ = 1\na = @\nb = 2", keys: []string{"b"}},
		{name: "expression never closed", input: "a = @[ 1 + 2\nb = 3", keys: []string{"a", "b"}},
		{name: "empty expression", input: "a = @[]\nb = 1", keys: []string{"a", "b"}},
		{name: "expression garbage", input: "a = @[ ) ( * / ]\nb = @[ 1 / 0 ]\nc = @[ (((1 ]", keys: []string{"a", "b", "c"}},
		{name: "variable used before definition", input: "a = @x\n@x = 1\nb = @x", keys: []string{"a", "b"}},
		{name: "variable defined twice", input: "@x = 1\n@x = 2\na = @x", keys: []string{"a"}, clean: true},
		{name: "variable defined as block", input: "@x = { 1 }\na = 1", keys: []string{"a"}},
		{name: "tag without value at end", input: "a = rgb", keys: []string{"a"}, clean: true},
		// The second tag starts a block without "=", which reads as a key.
		{name: "tag chain", input: "a = rgb hsv { 1 2 3 }\nb = 1", keys: []string{"a", "hsv", "b"}},
		{name: "tagged string then key", input: `a = list "x" = 1`, keys: []string{"a", "x"}, clean: true},
		{name: "nul bytes", input: "a = 1\x00\x00\nb\x00 = 2\nc = 3", keys: []string{"a", "b", "c"}},
		{name: "control characters", input: "a = 1\x01\x02\x7f\nb = 2", keys: []string{"a", "b"}},
		{name: "invalid utf-8", input: "a\xff\xfe = 1\nb = \xc3\xc3\nc = 2", keys: []string{"a\xff\xfe", "b", "c"}, clean: true},
		{name: "utf-16 byte order mark", input: "\xff\xfea = 1", keys: nil},
		{name: "windows and old mac line endings", input: "a = 1\r\nb = 2\rc = 3\n", keys: []string{"a", "b", "c"}, clean: true},
		{name: "tabs, form feeds and vertical tabs", input: "a\t=\t1\fb = 2\vc = 3", keys: []string{"a", "b", "c"}, clean: true},
		{name: "byte order mark between fields", input: "a = 1\n" + byteOrderMark + "b = 2", keys: []string{"a", "b"}, clean: true},
		{name: "semicolons everywhere", input: ";;a = 1;;b = { c = 2; };;", keys: []string{"a", "b"}, clean: true},
		{name: "commas between values", input: "a = { 1, 2, 3 }", keys: []string{"a"}},
		{name: "square brackets", input: "a = [1 2]\nb = 1", keys: []string{"a", "b"}},
		{name: "mixed block with nested groups", input: "a = { x { y = 1 } z = 2 { 3 } }", keys: []string{"a"}},
		{name: "block closed twice then reopened", input: "a = { b = 1 } } c = { d = 2 }", keys: []string{"a", "c"}},
		{name: "unclosed definition swallows the rest", input: "a = { b = 1\nc = { d = 2 }", keys: []string{"a"}},
		// A key cannot start with a colon; the colon is skipped.
		{name: "prefix keys", input: "INJECT: = 1\n:x = 2\nREPLACE:a:b = { }", keys: []string{"INJECT:", "x", "REPLACE:a:b"}},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			document := parseWithin(t, test.input, 5*time.Second)
			checkInvariants(t, test.input, document)

			if test.clean && len(document.Warnings) > 0 {
				t.Errorf("expected no warnings, got %v", document.Warnings)
			}

			if test.keys != nil || test.clean {
				var keys []string
				for _, field := range document.Fields {
					keys = append(keys, field.Key)
				}

				if strings.Join(keys, "|") != strings.Join(test.keys, "|") {
					t.Errorf("keys = %q, want %q", keys, test.keys)
				}
			}

			// Parsing is a pure function of its input.
			if again := Parse(test.input); !reflect.DeepEqual(document, again) {
				t.Error("parsing the same input twice gave different results")
			}
		})
	}
}

func repeatKey(key string, count int) []string {
	keys := make([]string, count)
	for index := range keys {
		keys[index] = key
	}

	return keys
}

// TestParseDeepNesting makes sure nesting cannot exhaust the stack. A Go
// stack overflow cannot be recovered from, so one malicious or corrupt file
// would otherwise take the whole tool down.
func TestParseDeepNesting(t *testing.T) {
	const depth = 200_000

	cases := map[string]string{
		"closed blocks":   strings.Repeat("a = { ", depth) + strings.Repeat("} ", depth) + "after = 1",
		"closed lists":    "a = " + strings.Repeat("{ ", depth) + strings.Repeat("} ", depth) + "after = 1",
		"unclosed blocks": strings.Repeat("a = { ", depth),
		"unclosed lists":  "a = " + strings.Repeat("{ ", depth),
		"tag chains":      "a = " + strings.Repeat("rgb ", depth) + "{ 1 }\nafter = 1",
		"mixed":           strings.Repeat("a = { x ", depth) + strings.Repeat("} ", depth) + "after = 1",
	}

	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			document := parseWithin(t, input, 20*time.Second)
			checkInvariants(t, input, document)

			if strings.Contains(input, "after = 1") && strings.Contains(name, "closed") && !strings.Contains(name, "unclosed") {
				if _, ok := document.Get("after"); !ok {
					t.Error("the field after the deep nesting was lost")
				}
			}

			found := false
			for _, warning := range document.Warnings {
				if strings.Contains(warning.Message, "nested too deeply") {
					found = true
				}
			}

			if !found && name != "tag chains" {
				t.Error("expected a warning about the nesting")
			}
		})
	}
}

// TestParseDeepExpressions does the same for expressions, which have their own
// recursive reader.
func TestParseDeepExpressions(t *testing.T) {
	const depth = 200_000

	for name, input := range map[string]string{
		"brackets": "a = @[ " + strings.Repeat("(", depth) + "1" + strings.Repeat(")", depth) + " ]\nb = 1",
		"signs":    "a = @[ " + strings.Repeat("-", depth) + "1 ]\nb = 1",
	} {
		t.Run(name, func(t *testing.T) {
			document := parseWithin(t, input, 20*time.Second)

			if _, ok := document.Get("b"); !ok {
				t.Error("the field after the expression was lost")
			}
		})
	}
}

// TestParseAtScale parses inputs far larger than any game file, shaped to hit
// every step that could turn quadratic: many fields, long lists, many
// warnings, many variables, long tokens.
func TestParseAtScale(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}

	var definitions strings.Builder
	for index := range 50_000 {
		fmt.Fprintf(&definitions, "pm_%d = { texture = \"t.dds\" building_modifiers = { workforce_scaled = { goods_input_iron_add = %d } } unlocking_technologies = { a b c } }\n", index, index)
	}

	var variables strings.Builder
	for index := range 20_000 {
		fmt.Fprintf(&variables, "@v%d = %d\nk%d = @v%d\n", index, index, index, index)
	}

	cases := map[string]string{
		"many definitions":    definitions.String(),
		"one huge list":       "a = { " + strings.Repeat("name ", 500_000) + "}",
		"one huge block":      "a = { " + strings.Repeat("k = 1 ", 300_000) + "}",
		"many warnings":       strings.Repeat("} ", 300_000),
		"many stray values":   "a = { " + strings.Repeat("x ", 300_000) + "y = 1 }",
		"many variables":      variables.String(),
		"long identifier":     "a = " + strings.Repeat("x", 5_000_000),
		"long string":         "a = \"" + strings.Repeat("x", 5_000_000) + "\"",
		"long comment":        "#" + strings.Repeat("x", 5_000_000) + "\na = 1",
		"many unclosed lines": strings.Repeat("a = \"x\n", 200_000),
		"many empty groups":   "a = { " + strings.Repeat("{ } ", 300_000) + "}",
	}

	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			start := time.Now()
			document := parseWithin(t, input, 30*time.Second)
			t.Logf("%d bytes in %s, %d fields, %d warnings", len(input), time.Since(start).Round(time.Millisecond), len(document.Fields), len(document.Warnings))
			checkInvariants(t, input, document)
		})
	}
}

// TestParseRandomBytes throws deterministic random input at the parser, both
// raw bytes and random sequences of the language's own tokens, which reach far
// more of the parser than bytes do.
func TestParseRandomBytes(t *testing.T) {
	random := rand.New(rand.NewSource(1836))

	fragments := []string{
		"a", "key", "INJECT:x", "yes", "1", "-2.5", "1836.1.1", "\"s\"", "\"", "{", "}", "=", ">=", "<", "?=", "!=",
		"@v", "@[ 1 + v ]", "@[", "rgb", "#c\n", "\n", " ", ";", ",", "\\", "\x00", "\xff", "é", "$P$", "a|b",
	}

	for round := range 2000 {
		var input strings.Builder

		length := random.Intn(200)

		for range length {
			if round%2 == 0 {
				input.WriteByte(byte(random.Intn(256)))
			} else {
				input.WriteString(fragments[random.Intn(len(fragments))])
				input.WriteByte(' ')
			}
		}

		text := input.String()
		document := parseWithin(t, text, 5*time.Second)
		checkInvariants(t, text, document)

		if t.Failed() {
			t.Fatalf("failed on input %q", text)
		}
	}
}

// FuzzParse checks the invariants on inputs the fuzzer makes up. Run it with
//
//	go test ./script -run '^$' -fuzz FuzzParse -fuzztime 60s
//
// Without -fuzz it runs the seed corpus as a normal test.
func FuzzParse(f *testing.F) {
	for _, seed := range []string{
		"",
		"a = 1",
		"a = { b = { c = 1 } }",
		"a = { 1 2 3 }",
		"a = rgb { 1 0 0 }",
		"@x = 1 a = @x b = @[ x * 2 ]",
		"possible = { level >= 5 has_x ?= yes }",
		"INJECT:pmg_x = { production_methods = { pm_a } }",
		"a = { x y = 1 z }",
		"a = { b = 1 } } c = {",
		"a = \"unclosed\nb = 2",
		"NTooltip = { X = 0.3f; }",
		"date = 1836.1.1 k = define:NPops|X",
		byteOrderMark + "a = 1\r\n",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		document := Parse(input)
		checkInvariants(t, input, document)
	})
}

// BenchmarkParse measures a file shaped like the game's production methods.
func BenchmarkParse(b *testing.B) {
	var input strings.Builder
	for index := range 1000 {
		fmt.Fprintf(&input, `pm_%d = {
	texture = "gfx/interface/icons/production_method_icons/bakeries.dds"
	unlocking_technologies = { manufacturies }
	building_modifiers = {
		workforce_scaled = {
			goods_input_grain_add = 40		# x20 =  800
			goods_output_groceries_add = 45	# x30 = 1350
		}
		level_scaled = {
			building_employment_shopkeepers_add = 500
			building_employment_laborers_add = 4500
		}
	}
}
`, index)
	}

	text := input.String()
	b.SetBytes(int64(len(text)))

	for b.Loop() {
		Parse(text)
	}
}

// TestParseSignsBeforeNames guards the lexing of a sign before a name, which
// once produced an empty token for "+" and never got past it.
func TestParseSignsBeforeNames(t *testing.T) {
	for _, input := range []string{"a = +$X$", "a = -$X$", "a = +b", "+", "-", "+$", "a = { +x -y }"} {
		document := parseWithin(t, input, 5*time.Second)
		checkInvariants(t, input, document)
	}
}
