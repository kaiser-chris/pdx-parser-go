package script

import (
	"fmt"
	"strconv"
	"strings"
)

type tokenKind uint8

const (
	tokenEOF tokenKind = iota
	tokenIdentifier
	tokenString
	tokenNumber
	tokenOpenBrace
	tokenCloseBrace

	// tokenOperator is "=" or one of the comparisons triggers use. Its text is
	// the operator as written.
	tokenOperator

	// tokenVariable is a variable such as @third, either being defined or used.
	tokenVariable

	// tokenExpression is an arithmetic expression such as @[ 1 / 3 ]. Its text
	// is whatever stood between the brackets.
	tokenExpression
)

// byteOrderMark is what the game tools put at the start of most files. It is
// not part of the script and has to be skipped before lexing.
const byteOrderMark = "\xef\xbb\xbf"

type token struct {
	kind   tokenKind
	text   string
	number float64
	line   int
	column int

	// start and end are byte offsets into the original input, byte order mark
	// included: where the token begins and just past where it ends.
	start, end int
}

type lexer struct {
	input  string
	offset int

	// base is how many bytes were cut from the front of the input, so that
	// offsets can be reported into the input as it was handed over.
	base int

	line     int
	column   int
	warnings []Warning
	comments map[int]string
}

func newLexer(input string) *lexer {
	trimmed := strings.TrimPrefix(input, byteOrderMark)

	return &lexer{
		input:    trimmed,
		base:     len(input) - len(trimmed),
		line:     1,
		column:   1,
		comments: map[int]string{},
	}
}

// tokenize reads the whole input up front. Having every token available lets
// the parser look ahead as far as it needs to tell a list from a block, and to
// find its way back into the file after something it could not read.
//
// Lexing cannot fail. Anything that is not a token is skipped with a warning.
func (l *lexer) tokenize() []token {
	var tokens []token

	for {
		next := l.lexToken()

		if next.kind != tokenEOF {
			next.end = l.base + l.offset
		}

		tokens = append(tokens, next)

		if next.kind == tokenEOF {
			return tokens
		}
	}
}

func (l *lexer) lexToken() token {
	for {
		l.skipSpaceAndComments()

		if l.offset >= len(l.input) {
			return token{kind: tokenEOF, line: l.line, column: l.column, start: l.base + l.offset, end: l.base + l.offset}
		}

		begin := l.offset
		start := l.position()
		char := l.input[l.offset]

		var lexed token

		switch {
		case char == '{':
			l.advance()
			lexed = token{kind: tokenOpenBrace, text: "{"}

		case char == '}':
			l.advance()
			lexed = token{kind: tokenCloseBrace, text: "}"}

		case isOperatorStart(char):
			operator, ok := l.lexOperator()
			if !ok {
				l.warn(start, "ignored unexpected character %q", char)

				continue
			}

			lexed = token{kind: tokenOperator, text: operator}

		case char == '"':
			lexed = l.lexString(start)

		case char == '\'':
			var ok bool

			lexed, ok = l.lexSingleQuoted(start)
			if !ok {
				continue
			}

		case char == '@':
			var ok bool

			lexed, ok = l.lexVariable(start)
			if !ok {
				continue
			}

		case isNumberStart(char) && l.looksLikeNumber():
			lexed = l.lexNumber()

		// A sign before a name negates a scripted parameter: -$AMOUNT$.
		case (char == '-' || char == '+') && l.offset+1 < len(l.input) && isIdentifierStart(l.input[l.offset+1]):
			l.advance() // the sign, which lexIdentifier would not take as a start

			lexed = l.lexIdentifier()
			lexed.text = l.input[begin:l.offset]

		case isIdentifierStart(char):
			lexed = l.lexIdentifier()

		default:
			// Characters that belong to no token are skipped rather than
			// treated as a failure. The shipped game files contain the odd
			// typo, and the games read those files anyway.
			l.warn(start, "ignored unexpected character %q", char)
			l.advance()

			continue
		}

		lexed.line = start.line
		lexed.column = start.column
		lexed.start = l.base + begin

		return lexed
	}
}

func (l *lexer) warn(where position, format string, args ...any) {
	l.warnings = append(l.warnings, Warning{
		Message: fmt.Sprintf(format, args...),
		Line:    where.line,
		Column:  where.column,
	})
}

type position struct {
	line   int
	column int
}

func (l *lexer) position() position {
	return position{line: l.line, column: l.column}
}

func (l *lexer) advance() {
	if l.input[l.offset] == '\n' {
		l.line++
		l.column = 1
	} else {
		l.column++
	}

	l.offset++
}

func (l *lexer) skipSpaceAndComments() {
	for l.offset < len(l.input) {
		switch char := l.input[l.offset]; {
		// A semicolon ends a statement in the C-style defines the engine
		// shares between games ("OPEN_DELAYED_TIME = 0.3f;"), and commas
		// separate the values of lists in the defines of Crusader Kings 3
		// ({ "friend", "rival" }). The games treat both as separators, and so
		// does this lexer.
		case char == ' ' || char == '\t' || char == '\r' || char == '\n' || char == ';' || char == ',' || char == '\f' || char == '\v':
			l.advance()

		// A byte order mark is also a zero width space. Editors that save a
		// file twice sometimes leave a second one behind, which would
		// otherwise stick to the first key.
		case strings.HasPrefix(l.input[l.offset:], byteOrderMark):
			l.offset += len(byteOrderMark)
			l.column++

		case char == '#':
			line := l.line
			begin := l.offset + 1

			for l.offset < len(l.input) && l.input[l.offset] != '\n' {
				l.advance()
			}

			l.comments[line] = strings.TrimSpace(l.input[begin:l.offset])

		default:
			return
		}
	}
}

func isOperatorStart(char byte) bool {
	return char == '=' || char == '<' || char == '>' || char == '!' || char == '?'
}

// lexOperator reads "=", "==", "!=", "<", "<=", ">", ">=" or "?=". A "!" or
// "?" on its own is no operator.
func (l *lexer) lexOperator() (string, bool) {
	char := l.input[l.offset]
	followedByEquals := l.offset+1 < len(l.input) && l.input[l.offset+1] == '='

	switch {
	case followedByEquals:
		l.advance()
		l.advance()

		return string(char) + "=", true

	case char == '=' || char == '<' || char == '>':
		l.advance()

		return string(char), true
	}

	l.advance()

	return "", false
}

// lexString reads a quoted string.
//
// A string normally ends at its line, so a quote that is never closed is ended
// at the line break. That costs one value rather than every value after it,
// which is what pairing it with the next quote in the file would cost.
//
// A quote that ends its line is the exception: that is how a string over
// several lines starts, as in the effects Europa Universalis 5 passes around as
// text. Such a string runs to the next quote.
func (l *lexer) lexString(start position) token {
	l.advance() // opening quote

	if l.opensMultilineString() {
		return l.lexMultilineString(start)
	}

	var text strings.Builder

	for l.offset < len(l.input) {
		switch char := l.input[l.offset]; char {
		case '"':
			l.advance()

			return token{kind: tokenString, text: text.String()}

		case '\n', '\r':
			l.warn(start, "string is never closed, ending it at the end of its line")

			return token{kind: tokenString, text: text.String()}

		case backslash:
			l.advance()

			if l.offset < len(l.input) && l.input[l.offset] != '\n' && l.input[l.offset] != '\r' {
				text.WriteByte(l.input[l.offset])
				l.advance()
			}

		default:
			text.WriteByte(char)
			l.advance()
		}
	}

	l.warn(start, "string is never closed, ending it at the end of the file")

	return token{kind: tokenString, text: text.String()}
}

// lexSingleQuoted reads a string in single quotes, as in the defines of
// Crusader Kings 3: POSITIVE = 'P'. It ends at the next single quote on its
// line; without one, the quote is no string but a stray character, skipped
// with a warning.
func (l *lexer) lexSingleQuoted(start position) (token, bool) {
	rest := l.input[l.offset+1:]

	end := strings.IndexAny(rest, "'\n")
	if end < 0 || rest[end] != '\'' {
		l.warn(start, "ignored unexpected character %q", '\'')
		l.advance()

		return token{}, false
	}

	for range end + 2 {
		l.advance()
	}

	return token{kind: tokenString, text: rest[:end]}, true
}

// opensMultilineString reports whether the quote just read is the last thing
// on its line, a comment aside, and is closed somewhere further on.
func (l *lexer) opensMultilineString() bool {
	rest := l.input[l.offset:]

	end := strings.IndexByte(rest, '\n')
	if end < 0 {
		return false
	}

	line := strings.TrimSpace(rest[:end])
	if line != "" && line[0] != '#' {
		return false
	}

	return strings.IndexByte(rest[end:], '"') >= 0
}

// lexMultilineString reads a string over several lines, up to its closing
// quote, which opensMultilineString has made sure exists. The text is kept as
// written, line breaks and indentation included.
func (l *lexer) lexMultilineString(start position) token {
	var text strings.Builder

	for l.offset < len(l.input) {
		char := l.input[l.offset]

		switch char {
		case '"':
			l.advance()

			return token{kind: tokenString, text: text.String()}

		case backslash:
			l.advance()

			if l.offset < len(l.input) {
				text.WriteByte(l.input[l.offset])
				l.advance()
			}

		default:
			text.WriteByte(char)
			l.advance()
		}
	}

	l.warn(start, "string is never closed, ending it at the end of the file")

	return token{kind: tokenString, text: text.String()}
}

// backslash escapes the next character inside a quoted string.
const backslash = 92

// looksLikeNumber decides whether a leading sign starts a number or something
// else. A sign on its own is neither, so it has to be followed by a digit or a
// decimal point.
func (l *lexer) looksLikeNumber() bool {
	if isDigit(l.input[l.offset]) {
		return true
	}

	if l.offset+1 >= len(l.input) {
		return false
	}

	next := l.input[l.offset+1]

	return isDigit(next) || next == '.'
}

// lexNumber reads a number. What starts like a number is not always one: dates
// such as 1836.1.1 and keys such as 1st_company are read as identifiers
// instead, and a C-style float suffix, as in 0.3f, is accepted and dropped.
func (l *lexer) lexNumber() token {
	begin := l.offset

	if l.input[l.offset] == '+' || l.input[l.offset] == '-' {
		l.advance()
	}

	for l.offset < len(l.input) && (isDigit(l.input[l.offset]) || l.input[l.offset] == '.') {
		l.advance()
	}

	numeric := l.input[begin:l.offset]

	if l.offset < len(l.input) && (l.input[l.offset] == 'f' || l.input[l.offset] == 'F') &&
		(l.offset+1 >= len(l.input) || !isIdentifierPart(l.input[l.offset+1])) {
		if value, err := strconv.ParseFloat(numeric, 64); err == nil {
			l.advance()

			return token{kind: tokenNumber, text: l.input[begin:l.offset], number: value}
		}
	}

	if l.offset < len(l.input) && isIdentifierPart(l.input[l.offset]) {
		for l.offset < len(l.input) && isIdentifierPart(l.input[l.offset]) {
			l.advance()
		}

		return token{kind: tokenIdentifier, text: l.input[begin:l.offset]}
	}

	value, err := strconv.ParseFloat(numeric, 64)
	if err != nil {
		return token{kind: tokenIdentifier, text: numeric}
	}

	return token{kind: tokenNumber, text: numeric, number: value}
}

func (l *lexer) lexIdentifier() token {
	begin := l.offset

	for l.offset < len(l.input) && isIdentifierPart(l.input[l.offset]) {
		l.advance()
	}

	return token{kind: tokenIdentifier, text: l.input[begin:l.offset]}
}

// lexVariable reads either a variable name, @third, or an expression, @[ 1/3 ].
func (l *lexer) lexVariable(start position) (token, bool) {
	l.advance() // the sigil

	if l.offset < len(l.input) && l.input[l.offset] == '[' {
		l.advance()

		begin := l.offset
		for l.offset < len(l.input) && l.input[l.offset] != ']' && l.input[l.offset] != '\n' {
			l.advance()
		}

		text := l.input[begin:l.offset]

		if l.offset >= len(l.input) || l.input[l.offset] != ']' {
			l.warn(start, "expression is never closed")

			return token{kind: tokenExpression, text: text}, true
		}

		l.advance() // the closing bracket

		return token{kind: tokenExpression, text: text}, true
	}

	begin := l.offset
	for l.offset < len(l.input) && isIdentifierPart(l.input[l.offset]) {
		l.advance()
	}

	if begin == l.offset {
		l.warn(start, "ignored an @ that names no variable")

		return token{}, false
	}

	return token{kind: tokenVariable, text: l.input[begin:l.offset]}, true
}

func isDigit(char byte) bool {
	return char >= '0' && char <= '9'
}

func isNumberStart(char byte) bool {
	return isDigit(char) || char == '-' || char == '+' || char == '.'
}

// isIdentifierStart accepts any byte of a multi-byte UTF-8 sequence, because
// keys and values are not restricted to ASCII: the game has character keys such
// as mon_general_petar_vukotić and localization keys in Cyrillic.
func isIdentifierStart(char byte) bool {
	return char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char == '_' || char == '$' || char >= 0x80
}

// isIdentifierPart is deliberately generous: unquoted values in these files
// include file names such as pattern_solid.tga, paths such as gfx/flags, scope
// links such as cu:north_german, references to defines such as
// define:NPops|GRACEFUL_AGING_START and scripted parameters such as $TARGET$.
// Database keys carry their modification mode the same way, as in
// INJECT:pmg_canning. Names may contain an apostrophe, as the Europa
// Universalis 5 name lists do in Abul'l-Hassan, but cannot start with one, and
// an ampersand, as a scripted effect of Crusader Kings 3 does in
// set_religious_head_title_to_&_gain_flag_effect, or a percent sign, as in the
// keys and positions of Crusader Kings 3: SUCCESS_% = 90, position = { 29% 4% }.
// A percentage is read as the string it is written as, since what it is a
// percentage of is up to whatever reads it.
func isIdentifierPart(char byte) bool {
	return isIdentifierStart(char) || isDigit(char) ||
		char == '.' || char == '/' || char == '-' || char == ':' || char == '|' || char == '\'' || char == '&' || char == '%'
}

// IsKey reports whether text reads back as a key when written without quotes.
func IsKey(text string) bool {
	if text == "" || !isIdentifierStart(text[0]) {
		return false
	}

	for index := 1; index < len(text); index++ {
		if !isIdentifierPart(text[index]) {
			return false
		}
	}

	return true
}
