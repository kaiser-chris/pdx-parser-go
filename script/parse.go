package script

import (
	"fmt"
	"sort"
)

// Parse reads a script document.
//
// Parsing cannot fail. What does not fit the grammar is skipped the way the
// games skip it and listed in the document's Warnings: a stray closing brace is
// dropped, a block left open at the end of the file is closed there, a value
// without a key is ignored, and an undefined variable is kept as the text it
// was written as.
func Parse(input string) *Document {
	lexer := newLexer(input)
	tokens := lexer.tokenize()

	parser := &parser{
		tokens:    tokens,
		closers:   matchBraces(tokens),
		variables: map[string]Node{},
	}

	fields, _ := parser.parseFields(nil)

	warnings := append(lexer.warnings, parser.warnings...)
	sort.SliceStable(warnings, func(first, second int) bool {
		if warnings[first].Line != warnings[second].Line {
			return warnings[first].Line < warnings[second].Line
		}

		return warnings[first].Column < warnings[second].Column
	})

	return &Document{Fields: fields, Warnings: warnings, comments: lexer.comments}
}

// unclosed marks an opening brace that is never closed.
const unclosed = -1

// maxDepth is how deeply braces may nest. Game files nest a few dozen levels
// at most; the limit exists because the parser recurses per level, and a Go
// stack overflow is fatal rather than recoverable, so without it one corrupt
// file of opening braces would crash the whole tool. Anything nested deeper is
// skipped with a warning.
const maxDepth = 1000

// matchBraces pairs every opening brace with its closing brace, so that the
// parser can skip a whole group and see how far a group reaches before it
// reads it.
func matchBraces(tokens []token) map[int]int {
	closers := map[int]int{}

	var open []int

	for index, current := range tokens {
		switch current.kind {
		case tokenOpenBrace:
			open = append(open, index)

		case tokenCloseBrace:
			if len(open) > 0 {
				closers[open[len(open)-1]] = index
				open = open[:len(open)-1]
			}
		}
	}

	for _, index := range open {
		closers[index] = unclosed
	}

	return closers
}

type parser struct {
	tokens  []token
	index   int
	closers map[int]int

	// depth is how many braced groups the parser is inside.
	depth int

	// variables collects the @name definitions of the document. The games
	// resolve them while reading, and so does this parser, so a definition has
	// to appear before it is used. One flat table per document is enough:
	// definitions inside a block stay visible to everything that follows.
	//
	// Victoria 3 only defines numbers, but a variable can hold any value:
	// Europa Universalis 5 defines strings and names as well.
	variables map[string]Node

	warnings []Warning
}

func (p *parser) peek() token {
	return p.tokens[p.index]
}

func (p *parser) peekAt(offset int) token {
	index := p.index + offset
	if index >= len(p.tokens) {
		index = len(p.tokens) - 1
	}

	return p.tokens[index]
}

func (p *parser) take() token {
	current := p.tokens[p.index]
	if current.kind != tokenEOF {
		p.index++
	}

	return current
}

// previousEnd is where the last token taken ends.
func (p *parser) previousEnd() int {
	if p.index == 0 {
		return 0
	}

	return p.tokens[p.index-1].end
}

func (p *parser) warn(at token, format string, args ...any) {
	p.warnings = append(p.warnings, Warning{
		Message: fmt.Sprintf(format, args...),
		Line:    at.line,
		Column:  at.column,
	})
}

// skipGroup skips the braced group starting at the current token, all of it,
// or everything to the end of the file when it is never closed.
func (p *parser) skipGroup() {
	closer := p.closers[p.index]
	if closer == unclosed {
		p.index = len(p.tokens) - 1

		return
	}

	p.index = closer + 1
}

// parseFields reads fields up to the closing brace of the block they belong to,
// or to the end of the input when open is nil. Values written without a key
// inside a block are returned as items; at the top of a file they mean nothing
// and are skipped.
func (p *parser) parseFields(open *token) ([]Field, []Node) {
	var (
		fields []Field
		items  []Node
	)

	for {
		current := p.peek()

		switch current.kind {
		case tokenEOF:
			if open != nil {
				p.warn(*open, "block is never closed, closing it at the end of the file")
			}

			return fields, items

		case tokenCloseBrace:
			p.take()

			if open != nil {
				return fields, items
			}

			p.warn(current, "ignored a closing brace that closes nothing")

		case tokenVariable:
			p.parseVariableDefinition()

		case tokenIdentifier, tokenString, tokenNumber:
			switch next := p.peekAt(1); next.kind {
			case tokenOperator:
				if field, ok := p.parseField(); ok {
					fields = append(fields, field)
				}

			case tokenOpenBrace:
				// "key { ... }" is a typo for "key = { ... }" often enough to
				// be worth reading as one.
				p.warn(next, "missing = between %q and its block", current.text)

				key := p.take()
				value := p.parseBraced()
				fields = append(fields, Field{
					Key: key.text, Operator: OpEquals, Value: value,
					Line: key.line, Column: key.column, Start: key.start, End: p.previousEnd(),
				})

			default:
				if open == nil && p.startsKeywordField() {
					if field, ok := p.parseKeywordField(); ok {
						fields = append(fields, field)
					}

					continue
				}

				if open == nil {
					p.warn(current, "ignored %q, which has no = and value after it", current.text)
					p.take()

					continue
				}

				item, ok := p.parseValue()
				if !ok {
					// Cannot happen for the tokens that reach here, but a
					// parser that stops taking tokens never finishes.
					p.warn(current, "ignored unexpected %q", current.text)
					p.take()

					continue
				}

				items = append(items, item)
			}

		case tokenOpenBrace:
			if open == nil {
				p.warn(current, "ignored a block that has no key")
				p.skipGroup()

				continue
			}

			items = append(items, p.parseBraced())

		default:
			p.warn(current, "ignored unexpected %q", current.text)
			p.take()
		}
	}
}

func (p *parser) parseField() (Field, bool) {
	key := p.take()
	operator := p.take()

	value, ok := p.parseValue()
	if !ok {
		p.warn(operator, "%q has no value after its %s", key.text, operator.text)

		return Field{}, false
	}

	return Field{
		Key:      key.text,
		Operator: Operator(operator.text),
		Value:    value,
		Line:     key.line,
		Column:   key.column,
		Start:    key.start,
		End:      p.previousEnd(),
	}, true
}

// startsKeywordField reports whether the current token is a keyword in front
// of the key of a field, as in scripted_trigger is_ready = { ... }: a bare word,
// then a key and an operator, all on one line. It is only asked at the top of
// a file. Inside a block the same shape is a value without a key followed by a
// field, as in on_actions = { coup_monthly_events delay = { days = 4 } }.
func (p *parser) startsKeywordField() bool {
	keyword, key, operator := p.peek(), p.peekAt(1), p.peekAt(2)

	return keyword.kind == tokenIdentifier &&
		(key.kind == tokenIdentifier || key.kind == tokenString || key.kind == tokenNumber) &&
		operator.kind == tokenOperator &&
		keyword.line == key.line && key.line == operator.line
}

// parseKeywordField reads a field with a keyword in front of its key.
func (p *parser) parseKeywordField() (Field, bool) {
	keyword := p.take()

	field, ok := p.parseField()
	if !ok {
		return Field{}, false
	}

	field.Keyword = keyword.text
	field.Line, field.Column, field.Start = keyword.line, keyword.column, keyword.start

	return field, true
}

// parseVariableDefinition records a variable definition. The definition itself
// is not part of the tree: by the time a caller sees the document, every use of
// a variable has been replaced by its value.
func (p *parser) parseVariableDefinition() {
	name := p.take()

	if next := p.peek(); next.kind != tokenOperator {
		p.warn(name, "ignored variable @%s, which is not assigned a value", name.text)

		return
	}

	p.take()

	value, ok := p.parseValue()
	if !ok {
		p.warn(name, "variable @%s has no value", name.text)

		return
	}

	p.variables[name.text] = value
}

// parseValue reads the value after an operator. It reports false, without
// taking anything, when there is no value there to read.
//
// A plain value on a later line than its operator, and followed by an operator
// itself, is the key of the next field: the value that should have come first
// is missing. Taking it as the value would cost that next field too. This is
// the state of a file while a modder is typing in it ("cost =" with the value
// still to come), so it has to lose as little as possible. A value on the same
// line as its operator is kept, so a stray operator after a complete field
// costs nothing but itself.
func (p *parser) parseValue() (Node, bool) {
	current := p.peek()

	if p.index > 0 && p.peekAt(1).kind == tokenOperator {
		operator := p.tokens[p.index-1]

		switch current.kind {
		case tokenString, tokenNumber, tokenIdentifier:
			if operator.kind == tokenOperator && current.line > operator.line {
				return Node{}, false
			}
		}
	}

	switch current.kind {
	case tokenString:
		p.take()

		return Node{Kind: KindString, Text: current.text, Quoted: true, Line: current.line, Column: current.column}, true

	case tokenNumber:
		p.take()

		return Node{Kind: KindNumber, Text: current.text, Number: current.number, Line: current.line, Column: current.column}, true

	case tokenVariable:
		p.take()

		value, defined := p.variables[current.text]
		if !defined {
			p.warn(current, "variable @%s is not defined before it is used", current.text)

			return Node{Kind: KindString, Text: "@" + current.text, Line: current.line, Column: current.column}, true
		}

		// A number keeps the variable's name as its spelling, the way a number
		// keeps how it was written. Anything else is its value, as if it had
		// been written out where the variable is used.
		if value.Kind == KindNumber {
			value.Text = "@" + current.text
		}

		value.Line, value.Column = current.line, current.column

		return value, true

	case tokenExpression:
		p.take()

		value, err := evaluate(current.text, p.variables)
		if err != nil {
			p.warn(current, "%s", err)

			return Node{Kind: KindString, Text: "@[" + current.text + "]", Line: current.line, Column: current.column}, true
		}

		return Node{Kind: KindNumber, Text: "@[" + current.text + "]", Number: value, Line: current.line, Column: current.column}, true

	case tokenIdentifier:
		// An identifier can tag the value that follows it: a colour written
		// rgb { 1 0 0 }, or a random pick written list "normal_colors".
		//
		// Only a brace, or a quoted string that is not itself the key of the
		// next field, counts as the tagged value. A bare identifier following
		// another bare identifier is two separate values, which is what keeps
		// lists of plain names from collapsing into one.
		next := p.peekAt(1)
		if next.kind == tokenOpenBrace || next.kind == tokenString && p.peekAt(2).kind != tokenOperator {
			p.take()

			tagged, ok := p.parseValue()
			if !ok {
				return Node{Kind: KindString, Text: current.text, Line: current.line, Column: current.column}, true
			}

			return Node{Kind: KindTagged, Tag: current.text, Value: &tagged, Line: current.line, Column: current.column}, true
		}

		p.take()

		return Node{Kind: KindString, Text: current.text, Line: current.line, Column: current.column}, true

	case tokenOpenBrace:
		return p.parseBraced(), true

	case tokenOperator:
		// An operator can be a value itself, passed to a scripted trigger
		// that compares with it: OPERATOR = <=, as in Crusader Kings 3. It
		// counts as one only on the line of the operator before it, and with
		// no further value after it on that line, so that a doubled operator,
		// a = = 1, is still a typo.
		if p.index == 0 || p.tokens[p.index-1].kind != tokenOperator || p.tokens[p.index-1].line != current.line {
			return Node{}, false
		}

		switch next := p.peekAt(1); {
		case next.kind == tokenEOF, next.kind == tokenCloseBrace, next.line != current.line:
			p.take()

			return Node{Kind: KindString, Text: current.text, Line: current.line, Column: current.column}, true
		}
	}

	return Node{}, false
}

// parseBraced reads a braced group, which is either a list of values or a block
// of fields. A group with an operator anywhere at its own level is a block,
// and values in it that have no key become its items. Anything else is a list.
func (p *parser) parseBraced() Node {
	if p.depth >= maxDepth {
		open := p.peek()
		p.warn(open, "nested too deeply, more than %d levels; skipped this group", maxDepth)
		p.skipGroup()

		return Node{Kind: KindList, Line: open.line, Column: open.column}
	}

	p.depth++
	defer func() { p.depth-- }()

	open := p.take()

	if p.groupHasOperator() {
		fields, items := p.parseFields(&open)

		return Node{Kind: KindBlock, Fields: fields, Items: items, Line: open.line, Column: open.column}
	}

	var items []Node

	for {
		current := p.peek()

		switch current.kind {
		case tokenCloseBrace:
			p.take()

			return Node{Kind: KindList, Items: items, Line: open.line, Column: open.column}

		case tokenEOF:
			p.warn(open, "list is never closed, closing it at the end of the file")

			return Node{Kind: KindList, Items: items, Line: open.line, Column: open.column}
		}

		item, ok := p.parseValue()
		if !ok {
			p.warn(current, "ignored unexpected %q in a list", current.text)
			p.take()

			continue
		}

		items = append(items, item)
	}
}

// groupHasOperator looks through the group that was just opened, without
// descending into nested groups, for an operator.
func (p *parser) groupHasOperator() bool {
	for index := p.index; index < len(p.tokens); {
		switch p.tokens[index].kind {
		case tokenOperator:
			return true

		case tokenCloseBrace, tokenEOF:
			return false

		case tokenOpenBrace:
			closer := p.closers[index]
			if closer == unclosed {
				return false
			}

			index = closer + 1

		default:
			index++
		}
	}

	return false
}
