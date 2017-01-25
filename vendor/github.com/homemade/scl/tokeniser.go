package scl

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var hashCommentMatcher = regexp.MustCompile(`#.+$`)
var functionMatcher = regexp.MustCompile(`^([a-zA-Z0-9_]+)\s?\((.*)\):?$`)
var shortFunctionMatcher = regexp.MustCompile(`^([a-zA-Z0-9_]+):$`)
var variableMatcher = regexp.MustCompile(`^\$([a-zA-Z_][a-zA-Z0-9_]*)$`)
var assignmentMatcher = regexp.MustCompile(`^\$([a-zA-Z_][a-zA-Z0-9_]*)\s*=\s*((.|\n)+)$`)
var declarationMatcher = regexp.MustCompile(`^\$([a-zA-Z_][a-zA-Z0-9_]*)\s*:=\s*(.+)$`)
var conditionalVariableMatcher = regexp.MustCompile(`^\$([a-zA-Z_0-9]+)\s*\?=\s*(.+)$`)
var docblockStartMatcher = regexp.MustCompile(`^/\*$`)
var docblockEndMatcher = regexp.MustCompile(`^\*\/$`)
var heredocMatcher = regexp.MustCompile(`<<([a-zA-Z]+)\s*$`)

type tokeniser struct {
	accruedComment []string
}

func newTokeniser() *tokeniser {
	return &tokeniser{}
}

func (t *tokeniser) resetComment() {
	t.accruedComment = make([]string, 0)
}

func (t *tokeniser) stripComments(l *scannerLine) string {

	lastQuote := rune(0)
	slash := rune(47)
	slashCount := 0

	result := func() (result []byte) {

		for i, v := range []byte(l.content) {

			c := rune(v)

			switch {
			case c == lastQuote:
				lastQuote = rune(0)
				slashCount = 0

			case unicode.In(c, unicode.Quotation_Mark):
				lastQuote = c
				slashCount = 0

			case c == slash && lastQuote == rune(0):

				slashCount++
				if slashCount == 2 {
					return result[0:(i - 1)]
				}

			default:
				slashCount = 0
			}

			result = append(result, v)
		}

		return
	}()

	return strings.Trim(string(result), " ")
}

func (t *tokeniser) tokenise(l *scannerLine) (tokens []token, err error) {

	// Remove comments
	content := t.stripComments(l)

	// If the string is empty, the entire line was a comment
	if content == "" {
		return []token{
			token{
				kind:    tokenLineComment,
				content: strings.TrimLeft(string(l.content), "/ "),
				line:    l,
			},
		}, nil
	}

	if docblockStartMatcher.MatchString(content) {
		return t.tokeniseCommentStart(l, lineContent(content))
	}

	if docblockEndMatcher.MatchString(content) {
		return t.tokeniseCommentEnd(l, lineContent(content))
	}

	// Mixin declarations start with a @
	if content[0] == '@' {
		return t.tokeniseMixinDeclaration(l, lineContent(content))
	}

	if shortFunctionMatcher.MatchString(content) {
		return t.tokeniseShortFunctionCall(l, lineContent(content))
	}

	if functionMatcher.MatchString(content) {
		return t.tokeniseFunctionCall(l, lineContent(content))
	}

	if assignmentMatcher.MatchString(content) {
		return t.tokeniseVariableAssignment(l, lineContent(content))
	}

	if declarationMatcher.MatchString(content) {
		return t.tokeniseVariableDeclaration(l, lineContent(content))
	}

	if conditionalVariableMatcher.MatchString(content) {
		return t.tokeniseConditionalVariableAssignment(l, lineContent(content))
	}

	// Assume the result is a literal
	return []token{
		token{kind: tokenLiteral, content: content, line: l},
	}, nil
}

func (t *tokeniser) tokeniseCommentStart(l *scannerLine, content lineContent) (tokens []token, err error) {
	tokens = append(tokens, token{kind: tokenCommentStart, line: l})
	return
}

func (t *tokeniser) tokeniseCommentEnd(l *scannerLine, content lineContent) (tokens []token, err error) {
	tokens = append(tokens, token{kind: tokenCommentEnd, line: l})
	return
}

func (t *tokeniser) tokeniseFunction(l *scannerLine, input string) (name string, tokens []token, err error) {

	parts := functionMatcher.FindStringSubmatch(input)

	if len(parts) < 2 {
		return "", tokens, fmt.Errorf("Can't parse function signature")
	}

	name = parts[1]

	if len(parts) == 3 && parts[2] != "" {

		lastQuote := rune(0)
		comma := rune(0x2c)
		leftBracket := rune(0x5b)
		rightBracket := rune(0x5d)

		f := func(c rune) bool {

			switch {
			case c == lastQuote:
				lastQuote = rune(0)
				return false
			case lastQuote != rune(0):
				return false
			case unicode.In(c, unicode.Quotation_Mark):
				lastQuote = c
				return false
			case c == leftBracket:
				lastQuote = rightBracket
				return false
			case c == comma:
				return true
			default:
				return false

			}
		}

		arguments := strings.FieldsFunc(parts[2], f)

		for _, arg := range arguments {

			arg = strings.Trim(arg, " \t")

			if matches := variableMatcher.FindStringSubmatch(arg); len(matches) > 1 {
				tokens = append(tokens, token{kind: tokenVariable, content: matches[1], line: l})
			} else if matches := assignmentMatcher.FindStringSubmatch(arg); len(matches) > 1 {
				tokens = append(tokens, token{kind: tokenVariableAssignment, content: matches[1], line: l})
				tokens = append(tokens, token{kind: tokenLiteral, content: matches[2], line: l})
			} else {
				tokens = append(tokens, token{kind: tokenLiteral, content: arg, line: l})
			}
		}
	}

	return
}

func (t *tokeniser) tokeniseMixinDeclaration(l *scannerLine, content lineContent) (tokens []token, err error) {

	name, fntokens, fnerr := t.tokeniseFunction(l, string(content)[1:])

	if fnerr != nil {
		return tokens, fmt.Errorf("%s: %s", l, fnerr)
	}

	tokens = append(tokens, token{kind: tokenMixinDeclaration, content: name, line: l})
	tokens = append(tokens, fntokens...)

	return
}

func (t *tokeniser) tokeniseFunctionCall(l *scannerLine, content lineContent) (tokens []token, err error) {

	name, fntokens, fnerr := t.tokeniseFunction(l, string(content))

	if fnerr != nil {
		return tokens, fmt.Errorf("%s: %s", l, fnerr)
	}

	tokens = append(tokens, token{kind: tokenFunctionCall, content: name, line: l})
	tokens = append(tokens, fntokens...)

	return
}

func (t *tokeniser) tokeniseShortFunctionCall(l *scannerLine, content lineContent) (tokens []token, err error) {

	parts := shortFunctionMatcher.FindStringSubmatch(string(content))

	if len(parts) > 0 {
		return []token{
			token{kind: tokenFunctionCall, content: parts[1], line: l},
		}, nil
	}

	return tokens, fmt.Errorf("Failed to parse short function call")
}

func (t *tokeniser) tokeniseVariableAssignment(l *scannerLine, content lineContent) (tokens []token, err error) {

	parts := assignmentMatcher.FindStringSubmatch(string(content))

	if len(parts) > 0 {
		return []token{
			token{kind: tokenVariableAssignment, content: parts[1], line: l},
			token{kind: tokenLiteral, content: parts[2], line: l},
		}, nil
	}

	return tokens, fmt.Errorf("Failed to parse variable assignment")
}

func (t *tokeniser) tokeniseVariableDeclaration(l *scannerLine, content lineContent) (tokens []token, err error) {

	parts := declarationMatcher.FindStringSubmatch(string(content))

	if len(parts) > 0 {
		return []token{
			token{kind: tokenVariableDeclaration, content: parts[1], line: l},
			token{kind: tokenLiteral, content: parts[2], line: l},
		}, nil
	}

	return tokens, fmt.Errorf("Failed to parse variable declaration")
}

func (t *tokeniser) tokeniseConditionalVariableAssignment(l *scannerLine, content lineContent) (tokens []token, err error) {

	parts := conditionalVariableMatcher.FindStringSubmatch(string(content))

	if len(parts) > 0 {
		return []token{
			token{kind: tokenConditionalVariableAssignment, content: parts[1], line: l},
			token{kind: tokenLiteral, content: parts[2], line: l},
		}, nil
	}

	return tokens, fmt.Errorf("Failed to parse conditional variable assignment")
}
