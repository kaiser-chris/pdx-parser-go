package script

import (
	"math"
	"strings"
	"testing"
)

// parseClean parses input that is expected to be free of problems.
func parseClean(t *testing.T, input string) *Document {
	t.Helper()

	document := Parse(input)
	for _, warning := range document.Warnings {
		t.Errorf("unexpected warning: %s", warning)
	}

	return document
}

func TestParseFieldsAndValues(t *testing.T) {
	document := parseClean(t, `
		# a comment
		texture = "gfx/interface/icons/goods_icons/ammunition.dds"
		category = military
		cost = 50
		offset = -0.5
		fraction = .25
	`)

	texture, ok := document.Get("texture")
	if !ok {
		t.Fatal("texture is missing")
	}
	if text, _ := texture.Str(); text != "gfx/interface/icons/goods_icons/ammunition.dds" {
		t.Errorf("texture = %q", text)
	}
	if !texture.Quoted {
		t.Error("texture should be marked as quoted")
	}

	category, _ := document.Get("category")
	if category.Quoted {
		t.Error("an unquoted identifier should not be marked as quoted")
	}

	for key, want := range map[string]float64{"cost": 50, "offset": -0.5, "fraction": 0.25} {
		value, _ := document.Get(key)
		if number, ok := value.Num(); !ok || number != want {
			t.Errorf("%s = %+v, want %v", key, value, want)
		}
	}
}

func TestParseByteOrderMarkAndLineEndings(t *testing.T) {
	document := parseClean(t, byteOrderMark+"a = 1\r\nb = { c = 2 }\r\n")

	if _, ok := document.Get("a"); !ok {
		t.Error("a leading byte order mark hid the first field")
	}

	b, _ := document.Get("b")
	if c, _ := b.Get("c"); c.Number != 2 {
		t.Errorf("b.c = %+v, want 2", c)
	}
}

func TestParseRepeatedKeys(t *testing.T) {
	document := parseClean(t, `
		entry = { goods = fabric }
		entry = { goods = clothes }
		cost = 10
		cost = 20
	`)

	if entries := document.All("entry"); len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	// A repeated scalar is an override, so the last one is the one that counts.
	cost, _ := document.Get("cost")
	if cost.Number != 20 {
		t.Errorf("cost = %v, want 20", cost.Number)
	}
}

func TestParseListsAndBlocks(t *testing.T) {
	document := parseClean(t, `
		pm = {
			unlocking_technologies = { manufacturies shaft_mining }
			building_modifiers = { workforce_scaled = { goods_input_grain_add = 40 } }
			color = { 12 251 123 }
			empty = {}
		}
	`)

	pm, _ := document.Get("pm")
	if pm.Kind != KindBlock {
		t.Fatalf("pm kind = %v, want a block", pm.Kind)
	}

	techs, _ := pm.Get("unlocking_technologies")
	if names, ok := techs.Strings(); !ok || strings.Join(names, ",") != "manufacturies,shaft_mining" {
		t.Errorf("unlocking_technologies = %v", names)
	}

	modifiers, _ := pm.Get("building_modifiers")
	scaled, _ := modifiers.Get("workforce_scaled")
	if grain, _ := scaled.Get("goods_input_grain_add"); grain.Number != 40 {
		t.Errorf("goods_input_grain_add = %+v", grain)
	}

	color, _ := pm.Get("color")
	if numbers, ok := color.Numbers(); !ok || len(numbers) != 3 {
		t.Errorf("color = %v", numbers)
	}

	empty, _ := pm.Get("empty")
	if !empty.IsBlock() {
		t.Error("an empty group should read as a block")
	}
	if names, ok := empty.Strings(); !ok || len(names) != 0 {
		t.Error("an empty group should read as an empty list")
	}
}

func TestParseComparisonOperators(t *testing.T) {
	document := parseClean(t, `
		possible = {
			level >= 5
			count > 0
			size < 3
			size <= 4
			has_law != yes
			country_rank ?= rank_value:great_power
			value == 1
			is_building_type = building_wheat_farm
		}
	`)

	possible, _ := document.Get("possible")

	want := []Operator{OpGreaterEqual, OpGreater, OpLess, OpLessEqual, OpNotEquals, OpSafeEquals, OpEqualsStrict, OpEquals}
	if len(possible.Fields) != len(want) {
		t.Fatalf("got %d fields, want %d", len(possible.Fields), len(want))
	}

	for index, field := range possible.Fields {
		if field.Operator != want[index] {
			t.Errorf("%s operator = %q, want %q", field.Key, field.Operator, want[index])
		}
	}

	rank, _ := possible.Get("country_rank")
	if rank.Text != "rank_value:great_power" {
		t.Errorf("country_rank = %q, want a scope link kept whole", rank.Text)
	}
}

func TestParseTaggedValue(t *testing.T) {
	document := parseClean(t, `
		color = hsv { 0.5 0.4 0.3 }
		other = rgb { 1 0 0 }
		picked = list "normal_colors"
	`)

	color, _ := document.Get("color")
	if color.Kind != KindTagged || color.Tag != "hsv" {
		t.Fatalf("color = %+v, want a value tagged hsv", color)
	}

	if numbers, ok := color.Value.Numbers(); !ok || numbers[2] != 0.3 {
		t.Errorf("color = %v", numbers)
	}

	picked, _ := document.Get("picked")
	if picked.Tag != "list" || picked.Value.Text != "normal_colors" {
		t.Errorf("picked = %+v", picked)
	}
}

func TestParseQuotedKeyIsNotATag(t *testing.T) {
	// A quoted string that is the key of the next field must not be swallowed
	// as the tagged value of the identifier before it.
	document := parseClean(t, `a = b "c" = d`)

	if len(document.Fields) != 2 {
		t.Fatalf("got %d fields, want 2", len(document.Fields))
	}

	if a, _ := document.Get("a"); a.Kind != KindString || a.Text != "b" {
		t.Errorf("a = %+v, want b", a)
	}
}

func TestParseDatesAndOddNumbers(t *testing.T) {
	document := parseClean(t, `
		start_date = 1836.1.1
		named = 1st_regiment
		delay = 0.3f
		time = 2F
	`)

	date, _ := document.Get("start_date")
	if date.Kind != KindString || date.Text != "1836.1.1" {
		t.Errorf("start_date = %+v, want the string 1836.1.1", date)
	}

	named, _ := document.Get("named")
	if named.Kind != KindString {
		t.Errorf("named = %+v, want a string", named)
	}

	for key, want := range map[string]float64{"delay": 0.3, "time": 2} {
		value, _ := document.Get(key)
		if number, ok := value.Num(); !ok || number != want {
			t.Errorf("%s = %+v, want %v", key, value, want)
		}
	}
}

func TestParseCStyleDefines(t *testing.T) {
	document := parseClean(t, "NTooltip = {\n\tOPEN_DELAYED_TIME = 0.3f;\n\tTENDENCY_BUFFER = 3;\n}\n")

	tooltip, _ := document.Get("NTooltip")
	if value, _ := tooltip.Get("OPEN_DELAYED_TIME"); value.Number != 0.3 {
		t.Errorf("OPEN_DELAYED_TIME = %+v", value)
	}
	if value, _ := tooltip.Get("TENDENCY_BUFFER"); value.Number != 3 {
		t.Errorf("TENDENCY_BUFFER = %+v", value)
	}
}

func TestParseDatabaseModificationKeys(t *testing.T) {
	document := parseClean(t, `INJECT:pmg_canning = { production_methods = { pm_new } }`)

	if _, ok := document.Get("INJECT:pmg_canning"); !ok {
		t.Error("a key with a modification prefix should be read whole")
	}
}

func TestParseVariablesAndExpressions(t *testing.T) {
	document := parseClean(t, `
		@third = @[1/3]
		@offset = 0.25
		@canton_x = @[ ( 333 / 768 ) + 0.001 ]

		a = @third
		b = @[0.5 + offset]
		c = @canton_x
		d = @[ -offset * 2 ]
	`)

	cases := map[string]float64{"a": 1.0 / 3.0, "b": 0.75, "c": (333.0 / 768.0) + 0.001, "d": -0.5}

	for key, want := range cases {
		value, ok := document.Get(key)
		if !ok {
			t.Errorf("%s is missing", key)

			continue
		}

		if number, ok := value.Num(); !ok || math.Abs(number-want) > 1e-9 {
			t.Errorf("%s = %+v, want %v", key, value, want)
		}
	}

	// A definition is an instruction to the parser, not part of the document.
	if len(document.Fields) != 4 {
		t.Errorf("got %d fields, want 4: a variable definition leaked into the document", len(document.Fields))
	}
}

func TestParseComments(t *testing.T) {
	document := parseClean(t, "era_1 = { #Pre-1836\n\tcost = 1 # base\n}\n# trailing")

	if text, ok := document.Comment(1); !ok || text != "Pre-1836" {
		t.Errorf("comment on line 1 = %q, %v", text, ok)
	}

	if text, _ := document.Comment(2); text != "base" {
		t.Errorf("comment on line 2 = %q", text)
	}

	if _, ok := document.Comment(3); ok {
		t.Error("line 3 has no comment")
	}
}

func TestFieldSpans(t *testing.T) {
	input := byteOrderMark + "@a = 1\nfirst = { x = { 1 2 } }\r\nsecond = \"text\" # comment\nthird = rgb { 1 @a 1 }\nfourth >= 5"

	document := parseClean(t, input)

	want := map[string]string{
		"first":  "first = { x = { 1 2 } }",
		"second": `second = "text"`,
		"third":  "third = rgb { 1 @a 1 }",
		"fourth": "fourth >= 5",
	}

	for _, field := range document.Fields {
		if got := input[field.Start:field.End]; got != want[field.Key] {
			t.Errorf("span of %s = %q, want %q", field.Key, got, want[field.Key])
		}
	}

	inner := document.Fields[0].Value.Fields[0]
	if got := input[inner.Start:inner.End]; got != "x = { 1 2 }" {
		t.Errorf("span of a nested field = %q, want %q", got, "x = { 1 2 }")
	}
}

// TestParseRecovers covers the ways a file can be broken. Every case must keep
// the definitions around the damage, and must say what it skipped.
func TestParseRecovers(t *testing.T) {
	cases := []struct {
		name  string
		input string

		// keys are the top level keys that must survive, in order.
		keys []string

		// warning is a fragment of the warning the damage must produce.
		warning string
	}{
		{
			name:    "stray closing brace",
			input:   "a = { x = 1 } }\nb = { y = 2 }",
			keys:    []string{"a", "b"},
			warning: "closes nothing",
		},
		{
			name:    "block never closed",
			input:   "a = { x = 1 }\nb = { y = 2",
			keys:    []string{"a", "b"},
			warning: "never closed",
		},
		{
			name:    "missing value before a closing brace",
			input:   "a = { x = }\nb = { y = 2 }",
			keys:    []string{"a", "b"},
			warning: "has no value",
		},
		{
			name:    "missing value does not swallow the next key",
			input:   "a =\nb = 2\nc = { x = }\nd = 4",
			keys:    []string{"b", "c", "d"},
			warning: "has no value",
		},
		{
			name:    "missing value at the end of the file",
			input:   "a = 1\nb =",
			keys:    []string{"a"},
			warning: "has no value",
		},
		{
			name:    "missing equals before a block",
			input:   "a { x = 1 }\nb = 2",
			keys:    []string{"a", "b"},
			warning: "missing =",
		},
		{
			name:    "value without a key at the top of the file",
			input:   "a = 1\nstray\nb = 3",
			keys:    []string{"a", "b"},
			warning: "no = and value",
		},
		{
			name:    "block without a key",
			input:   "a = 1\n{ x = 1 }\nb = 2",
			keys:    []string{"a", "b"},
			warning: "has no key",
		},
		{
			name:    "unexpected characters",
			input:   "a = 1^\nb = 50~\nc = 3",
			keys:    []string{"a", "b", "c"},
			warning: "unexpected character",
		},
		{
			name:    "string never closed",
			input:   "a = \"gfx/icon.dds\nb = \"fine\"",
			keys:    []string{"a", "b"},
			warning: "never closed",
		},
		{
			name:    "undefined variable",
			input:   "a = @nope\nb = @[ nope + 1 ]\nc = @[ 1 / 0 ]\nd = 4",
			keys:    []string{"a", "b", "c", "d"},
			warning: "not defined",
		},
		{
			name:    "variable with a non-numeric value in an expression",
			input:   "@name = hello\na = @[ name + 1 ]",
			keys:    []string{"a"},
			warning: "not a number",
		},
		{
			name:    "stray operator",
			input:   "a = 1\n= 2\nb = 3",
			keys:    []string{"a", "b"},
			warning: "unexpected",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			document := Parse(test.input)

			var keys []string
			for _, field := range document.Fields {
				keys = append(keys, field.Key)
			}

			if strings.Join(keys, ",") != strings.Join(test.keys, ",") {
				t.Errorf("keys = %v, want %v", keys, test.keys)
			}

			found := false
			for _, warning := range document.Warnings {
				if strings.Contains(warning.Message, test.warning) {
					found = true
				}
			}

			if !found {
				t.Errorf("no warning mentions %q; got %v", test.warning, document.Warnings)
			}
		})
	}
}

func TestParseMixedBlock(t *testing.T) {
	// on_actions mixes values without keys with fields in one block. Both
	// halves have to survive.
	document := parseClean(t, `on_actions = { coup_monthly_events delay = { days = 4 } garibaldi_monthly_events }`)

	block, _ := document.Get("on_actions")
	if block.Kind != KindBlock {
		t.Fatalf("kind = %v, want a block", block.Kind)
	}

	if len(block.Items) != 2 || block.Items[0].Text != "coup_monthly_events" || block.Items[1].Text != "garibaldi_monthly_events" {
		t.Errorf("items = %+v", block.Items)
	}

	if delay, _ := block.Get("delay"); !delay.IsBlock() {
		t.Errorf("delay = %+v", delay)
	}
}

func TestParseNonASCIIIdentifiers(t *testing.T) {
	document := parseClean(t, "mon_general_petar_vukotić = { localization_key = RU_GL_КP_DEFAULT }")

	general, ok := document.Get("mon_general_petar_vukotić")
	if !ok {
		t.Fatal("a key with a non-ASCII letter was not read")
	}

	if key, _ := general.Get("localization_key"); key.Text != "RU_GL_КP_DEFAULT" {
		t.Errorf("localization_key = %q", key.Text)
	}
}

func TestParseRecoveryKeepsValues(t *testing.T) {
	document := Parse("a = { x = 1 y = } z = 2\nb = { c = \"open\nd = 4 }")

	if z, _ := document.Get("z"); z.Number != 2 {
		t.Errorf("z = %+v, want 2 after the field without a value", z)
	}

	b, _ := document.Get("b")
	if d, _ := b.Get("d"); d.Number != 4 {
		t.Errorf("b.d = %+v, want 4 after the unclosed string", d)
	}
}

func TestWarningsReportPosition(t *testing.T) {
	document := Parse("a = 1\nb = }\n")

	if len(document.Warnings) == 0 {
		t.Fatal("expected a warning")
	}

	if document.Warnings[0].Line != 2 {
		t.Errorf("line = %d, want 2", document.Warnings[0].Line)
	}
}

func TestIsKey(t *testing.T) {
	for text, want := range map[string]bool{
		"pm_bakery": true, "INJECT:pm_bakery": true, "_x": true, "cu:north_german": true,
		"": false, "1pm": false, "two words": false, "quote\"": false, "a{": false,
	} {
		if got := IsKey(text); got != want {
			t.Errorf("IsKey(%q) = %v, want %v", text, got, want)
		}
	}
}

// TestParseEuropaUniversalis5Syntax covers syntax the files of Europa
// Universalis 5 use and Victoria 3 does not, found by parsing that game.
func TestParseEuropaUniversalis5Syntax(t *testing.T) {
	t.Run("variables holding strings and lists", func(t *testing.T) {
		document := parseClean(t, "@dlc = \"d008_fate_of_the_phoenix\"\n@mesh = northgerman_noble\n@names = { a b }\n@scale = 2\n@alias = @scale\n"+
			"dlc = @dlc\nmesh = @mesh\nnames = @names\nalias = @alias\ndouble = @[ alias * 2 ]")

		if value, _ := document.Get("dlc"); value.Kind != KindString || value.Text != "d008_fate_of_the_phoenix" || !value.Quoted {
			t.Errorf("dlc = %+v", value)
		}

		if value, _ := document.Get("mesh"); value.Kind != KindString || value.Text != "northgerman_noble" {
			t.Errorf("mesh = %+v", value)
		}

		if value, _ := document.Get("names"); value.Kind != KindList || len(value.Items) != 2 {
			t.Errorf("names = %+v", value)
		}

		if value, _ := document.Get("alias"); value.Number != 2 {
			t.Errorf("alias = %+v", value)
		}

		if value, _ := document.Get("double"); value.Number != 4 {
			t.Errorf("double = %+v", value)
		}
	})

	t.Run("string variable in an expression", func(t *testing.T) {
		document := Parse("@dlc = \"x\"\na = @[ dlc * 2 ]")

		if len(document.Warnings) != 1 || !strings.Contains(document.Warnings[0].Message, "not a number") {
			t.Errorf("warnings = %v", document.Warnings)
		}
	})

	t.Run("apostrophes in names", func(t *testing.T) {
		document := parseClean(t, "names = { Abul'l-Hassan Ra'ana Kabir }")

		value, _ := document.Get("names")
		if len(value.Items) != 3 || value.Items[0].Text != "Abul'l-Hassan" || value.Items[1].Text != "Ra'ana" {
			t.Errorf("names = %+v", value.Items)
		}
	})

	t.Run("strings over several lines", func(t *testing.T) {
		document := parseClean(t, "effect = {\n\trevolution_effect = \"\n\t\tadd_power = severe\n\t\"\n\tother = \"\n\t\tb = { c = d }\n\t\"\n}\nafter = 1")

		effect, _ := document.Get("effect")

		value, _ := effect.Get("revolution_effect")
		if value.Kind != KindString || strings.TrimSpace(value.Text) != "add_power = severe" {
			t.Errorf("revolution_effect = %+v", value)
		}

		if value, _ := effect.Get("other"); !strings.Contains(value.Text, "b = { c = d }") {
			t.Errorf("other = %+v", value)
		}

		if _, ok := document.Get("after"); !ok {
			t.Error("the field after the strings is missing")
		}
	})

	t.Run("a quote left open at the end of a line still ends at the next quote", func(t *testing.T) {
		document := Parse("a = \"\nb = 1\nc = \"x\"")

		if _, ok := document.Get("a"); !ok {
			t.Error("a is missing")
		}
	})

	t.Run("definitions with a keyword before their key", func(t *testing.T) {
		document := parseClean(t, "namespace = flavor\nscripted_trigger is_ready = { always = yes }\nscripted_effect do_it = { x = 1 }\nevent.1 = { }")

		var keys, keywords []string
		for _, field := range document.Fields {
			keys = append(keys, field.Key)
			keywords = append(keywords, field.Keyword)
		}

		if strings.Join(keys, ",") != "namespace,is_ready,do_it,event.1" {
			t.Errorf("keys = %v", keys)
		}

		if strings.Join(keywords, ",") != ",scripted_trigger,scripted_effect," {
			t.Errorf("keywords = %q", keywords)
		}

		// The span covers the keyword too, so that replacing the definition
		// replaces all of it.
		input := "scripted_trigger is_ready = { always = yes }"
		if field := Parse(input).Fields[0]; input[field.Start:field.End] != input {
			t.Errorf("span = %q", input[field.Start:field.End])
		}
	})

	t.Run("items in a block are not keywords", func(t *testing.T) {
		document := parseClean(t, "on_actions = { coup_monthly_events delay = { days = 4 } }")

		block, _ := document.Get("on_actions")
		if len(block.Items) != 1 || len(block.Fields) != 1 || block.Fields[0].Keyword != "" {
			t.Errorf("on_actions = %+v", block)
		}
	})
}

// TestParseCrusaderKings3Syntax covers syntax the files of Crusader Kings 3
// use, found by parsing that game.
func TestParseCrusaderKings3Syntax(t *testing.T) {
	t.Run("commas between values", func(t *testing.T) {
		document := parseClean(t, `RELATIONS = { "friend", "rival", "nemesis" }`+"\nSIZES = { 1, 2.5,3 }")

		if value, _ := document.Get("RELATIONS"); len(value.Items) != 3 || value.Items[2].Text != "nemesis" {
			t.Errorf("RELATIONS = %+v", value.Items)
		}

		if numbers, ok := document.Fields[1].Value.Numbers(); !ok || len(numbers) != 3 || numbers[1] != 2.5 {
			t.Errorf("SIZES = %v", numbers)
		}
	})

	t.Run("an operator as a value", func(t *testing.T) {
		document := parseClean(t, "trigger = {\n\tRELATION = friend\n\tOPERATOR = <=\n\tCOUNT = 0\n}\nlast = >")

		trigger, _ := document.Get("trigger")

		if value, _ := trigger.Get("OPERATOR"); value.Kind != KindString || value.Text != "<=" {
			t.Errorf("OPERATOR = %+v", value)
		}

		if value, _ := trigger.Get("COUNT"); value.Number != 0 || value.Kind != KindNumber {
			t.Errorf("COUNT = %+v", value)
		}

		if value, _ := document.Get("last"); value.Text != ">" {
			t.Errorf("last = %+v", value)
		}
	})

	t.Run("single quotes", func(t *testing.T) {
		document := parseClean(t, "NTextColoring = {\n\tPOSITIVE = 'P'\n\tNEUTRAL = 'V'\n\tTEXT = 'two words'\n}")

		colouring, _ := document.Get("NTextColoring")

		if value, _ := colouring.Get("POSITIVE"); value.Text != "P" || !value.Quoted {
			t.Errorf("POSITIVE = %+v", value)
		}

		if value, _ := colouring.Get("TEXT"); value.Text != "two words" {
			t.Errorf("TEXT = %+v", value)
		}
	})

	t.Run("percentages", func(t *testing.T) {
		document := parseClean(t, "slot = { position = { 29% 4% } }\nroll = { SUCCESS_% = 90 FAILURE_% = 10 }")

		slot, _ := document.Get("slot")
		if position, _ := slot.Get("position"); len(position.Items) != 2 || position.Items[0].Text != "29%" {
			t.Errorf("position = %+v", position)
		}

		roll, _ := document.Get("roll")
		if value, _ := roll.Get("SUCCESS_%"); value.Number != 90 {
			t.Errorf("SUCCESS_%% = %+v", value)
		}
	})

	t.Run("negated parameters", func(t *testing.T) {
		document := parseClean(t, "effect = { AMOUNT = -$AMOUNT$ }")

		effect, _ := document.Get("effect")
		if value, _ := effect.Get("AMOUNT"); value.Text != "-$AMOUNT$" {
			t.Errorf("AMOUNT = %+v", value)
		}
	})

	t.Run("ampersands in names", func(t *testing.T) {
		document := parseClean(t, "scripted_effect set_title_to_&_gain_flag_effect = { }")

		if field := document.Fields[0]; field.Key != "set_title_to_&_gain_flag_effect" || field.Keyword != "scripted_effect" {
			t.Errorf("field = %+v", field)
		}
	})
}
