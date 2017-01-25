package scl

import (
	"fmt"
	"unicode"
)

type variable struct {
	name  string
	value string
}

type mixin struct {
	declaration *scannerLine
	arguments   []variable
	defaults    []string
}

type scope struct {
	parent      *scope
	branch      *scannerLine
	branchScope *scope
	variables   map[string]*variable
	mixins      map[string]*mixin
}

func newScope() *scope {
	return &scope{
		variables: make(map[string]*variable),
		mixins:    make(map[string]*mixin),
	}
}

func (s *scope) setArgumentVariable(name, value string) {
	s.variables[name] = &variable{name, value}
}

func (s *scope) setVariable(name, value string) {

	v, ok := s.variables[name]

	if !ok || v == nil {
		s.variables[name] = &variable{name, value}
	} else {
		s.variables[name].value = value
	}
}

func (s *scope) variable(name string) string {

	value, ok := s.variables[name]

	if !ok || value == nil {
		return ""
	}

	return s.variables[name].value
}

func (s *scope) setMixin(name string, declaration *scannerLine, argumentTokens []token, defaults []string) {

	mixin := &mixin{
		declaration: declaration,
		defaults:    defaults,
	}

	for _, t := range argumentTokens {
		mixin.arguments = append(mixin.arguments, variable{name: t.content})
	}

	s.mixins[name] = mixin
}

func (s *scope) removeMixin(name string) {
	delete(s.mixins, name)
}

func (s *scope) mixin(name string) (*mixin, error) {

	m, ok := s.mixins[name]

	if !ok {
		return nil, fmt.Errorf("Mixin %s not declared in this scope", name)
	}

	return m, nil
}

func (s *scope) interpolateLiteral(literal string) (outp string, err error) {

	isVariableChar := func(c rune) bool {
		return unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_'
	}

	unknownVariable := func(name []byte) {
		err = fmt.Errorf("Unknown variable '$%s'", name)
	}

	unfinishedVariable := func(name []byte) {
		err = fmt.Errorf("Expecting closing right brace in variable ${%s}", name)
	}

	result := func() (result []byte) {

		var (
			backSlash    = '\\'
			dollar       = '$'
			leftBrace    = '{'
			rightBrace   = '}'
			backtick     = '`'
			slashEscaped = false

			variableStarted        = false
			variableIsBraceEscaped = false
			variable               = []byte{}
			literalStarted         = false
		)

		for _, c := range []byte(literal) {

			if literalStarted {

				if rune(c) == backtick {
					literalStarted = false
					continue
				}

				result = append(result, c)
				continue
			}

			if variableStarted {

				if len(variable) == 0 {

					// If the first character is a dollar, then this
					// is a $$var escape
					if rune(c) == dollar {
						variableStarted = false
						variableIsBraceEscaped = false

						// Write out two dollars – one for the skipped var
						// signifier, and the current one
						result = append(result, byte(dollar))
						continue
					}

					// If the first character is a curl brace,
					// it's the start of a ${var} syntax
					if !variableIsBraceEscaped {
						if rune(c) == leftBrace {
							variableIsBraceEscaped = true
							continue
						} else {
							variableIsBraceEscaped = false
						}
					}
				}

				// If this is a valid variable character,
				// add it to the variable building
				if isVariableChar(rune(c)) {
					variable = append(variable, c)
					continue
				}

				// If the variable is zero length, then it's a dollar literal
				if len(variable) == 0 {
					variableStarted = false
					variableIsBraceEscaped = false
					result = append(result, byte(dollar), c)
					continue
				}

				// Brace-escaped variables must end with a closing brace
				if variableIsBraceEscaped {
					if rune(c) != rightBrace {
						unfinishedVariable(variable)
						return
					}
				}

				writeOutput := !variableIsBraceEscaped

				// The variable has ended
				variableStarted = false
				variableIsBraceEscaped = false

				// The variable is complete; look up its value
				if replacement := s.variable(string(variable)); replacement != "" {
					result = append(result, []byte(replacement)...)

					if writeOutput {
						result = append(result, c)
					}

					continue
				}

				unknownVariable(variable)
				return
			}

			if slashEscaped {
				result = append(result, c)
				slashEscaped = false
				continue
			}

			switch rune(c) {
			case backSlash:
				slashEscaped = true
				continue

			case dollar:
				variableStarted, variable = true, []byte{}
				continue

			case backtick:
				literalStarted = true
				continue
			}

			result = append(result, c)

			slashEscaped = false
		}

		if literalStarted {
			err = fmt.Errorf("Unterminated backtick literal")
			return
		}

		// If the last character is a slash, add it
		if slashEscaped {
			result = append(result, byte(backSlash))
		}

		// The string ended mid-variable, so add it if possible
		if variableStarted {

			if variableIsBraceEscaped {
				unfinishedVariable(variable)
				return
			} else if replacement := s.variable(string(variable)); replacement != "" {
				result = append(result, []byte(replacement)...)
			} else {
				unknownVariable(variable)
				return
			}
		}

		return
	}()

	outp = string(result)

	return
}

func (s *scope) clone() *scope {

	s2 := newScope()
	s2.parent = s
	s2.branch = s.branch
	s2.branchScope = s.branchScope

	for k, v := range s.variables {
		s2.variables[k] = v
	}

	for k, v := range s.mixins {
		s2.mixins[k] = v
	}

	return s2
}
