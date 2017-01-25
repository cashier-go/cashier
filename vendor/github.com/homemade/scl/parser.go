package scl

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl"
	hclparser "github.com/hashicorp/hcl/hcl/parser"
)

const (
	builtinMixinBody    = "__body__"
	builtinMixinInclude = "include"
	hclIndentSize       = 2
	noMixinParamValue   = "_"
)

/*
A Parser takes input in the form of filenames, variables values and include
paths, and transforms any SCL into HCL. Generally, a program will only call
Parse() for one file (the configuration file for that project) but it can be
called on any number of files, each of which will add to the Parser's HCL
output.

Variables and includes paths are global for all files parsed; that is, if you
Parse() multiple files, each of them will have access to the same set of
variables and use the same set of include paths. The parser variables are part
of the top-level scope: if a file changes them while it's being parsed, the
next file will have the same variable available with the changed value.
Similarly, if a file declares a new variable or mixin on the root scope, then
the next file will be able to access it. This can become confusing quickly,
so it's usually best to parse only one file and let it explicitly include
and other files at the SCL level.

SCL is an auto-documenting language, and the documentation is obtained using
the Parser's Documentation() function. Only mixins are currently documented.
Unlike the String() function, the documentation returned for Documentation()
only includes the nominated file.
*/
type Parser interface {
	Parse(fileName string) error
	Documentation(fileName string) (MixinDocs, error)
	SetParam(name, value string)
	AddIncludePath(name string)
	String() string
}

type parser struct {
	fs           FileSystem
	rootScope    *scope
	output       []string
	indent       int
	includePaths []string
}

/*
NewParser creates a new, standard Parser given a FileSystem. The most common FileSystem is
the DiskFileSystem, but any will do. The parser opens all files and reads all
includes using the FileSystem provided.
*/
func NewParser(fs FileSystem) (Parser, error) {

	p := &parser{
		fs:        fs,
		rootScope: newScope(),
	}

	return p, nil
}

func (p *parser) SetParam(name, value string) {
	p.rootScope.setVariable(name, value)
}

func (p *parser) AddIncludePath(name string) {
	p.includePaths = append(p.includePaths, name)
}

func (p *parser) String() string {
	return strings.Join(p.output, "\n")
}

func (p *parser) Parse(fileName string) error {

	lines, err := p.scanFile(fileName)

	if err != nil {
		return err
	}

	if err := p.parseTree(lines, newTokeniser(), p.rootScope); err != nil {
		return err
	}

	return nil
}

func (p *parser) Documentation(fileName string) (MixinDocs, error) {

	docs := MixinDocs{}

	lines, err := p.scanFile(fileName)

	if err != nil {
		return docs, err
	}

	if err := p.parseTreeForDocumentation(lines, newTokeniser(), &docs); err != nil {
		return docs, err
	}

	return docs, nil
}

func (p *parser) scanFile(fileName string) (lines scannerTree, err error) {

	f, _, err := p.fs.ReadCloser(fileName)

	if err != nil {
		return lines, fmt.Errorf("Can't read %s: %s", fileName, err)
	}

	defer f.Close()

	lines, err = newScanner(f, fileName).scan()

	if err != nil {
		return lines, fmt.Errorf("Can't scan %s: %s", fileName, err)
	}

	return
}

func (p *parser) isValid(hclString string) error {

	e := hcl.Decode(&struct{}{}, hclString)

	if pe, ok := e.(*hclparser.PosError); ok {
		return pe.Err
	} else if pe != nil {
		return pe
	}

	return nil
}

func (p *parser) indentedValue(literal string) string {
	return fmt.Sprintf("%s%s", strings.Repeat(" ", p.indent*hclIndentSize), literal)
}

func (p *parser) writeLiteralToOutput(scope *scope, literal string, block bool) error {

	literal, err := scope.interpolateLiteral(literal)

	if err != nil {
		return err
	}

	line := p.indentedValue(literal)

	if block {

		if err := p.isValid(line + "{}"); err != nil {
			return err
		}

		line += " {"
		p.indent++

	} else {

		if hashCommentMatcher.MatchString(line) {
			// Comments are passed through directly
		} else if err := p.isValid(line + "{}"); err == nil {
			line = line + "{}"
		} else if err := p.isValid(line); err != nil {
			return err
		}
	}

	p.output = append(p.output, line)

	return nil
}

func (p *parser) endBlock() {
	p.indent--
	p.output = append(p.output, p.indentedValue("}"))
}

func (p *parser) err(branch *scannerLine, e string, args ...interface{}) error {
	return fmt.Errorf("[%s] %s", branch.String(), fmt.Sprintf(e, args...))
}

func (p *parser) parseTree(tree scannerTree, tkn *tokeniser, scope *scope) error {

	for _, branch := range tree {

		tokens, err := tkn.tokenise(branch)

		if err != nil {
			return p.err(branch, err.Error())
		}

		if len(tokens) > 0 {

			token := tokens[0]

			switch token.kind {

			case tokenLiteral:

				if err := p.parseLiteral(branch, tkn, token, scope); err != nil {
					return err
				}

			case tokenVariableAssignment:

				value, err := scope.interpolateLiteral(tokens[1].content)

				if err != nil {
					return err
				}

				scope.setVariable(token.content, value)

			case tokenVariableDeclaration:

				value, err := scope.interpolateLiteral(tokens[1].content)

				if err != nil {
					return err
				}

				scope.setArgumentVariable(token.content, value)

			case tokenConditionalVariableAssignment:

				value, err := scope.interpolateLiteral(tokens[1].content)

				if err != nil {
					return err
				}

				if v := scope.variable(token.content); v == "" {
					scope.setArgumentVariable(token.content, value)
				}

			case tokenMixinDeclaration:
				if err := p.parseMixinDeclaration(branch, tokens, scope); err != nil {
					return err
				}

			case tokenFunctionCall:
				if err := p.parseFunctionCall(branch, tkn, tokens, scope.clone()); err != nil {
					return err
				}

			case tokenCommentStart, tokenCommentEnd, tokenLineComment:
				// Do nothing

			default:
				return p.err(branch, "Unexpected token: %s (%s)", token.kind, branch.content)
			}
		}
	}

	return nil
}

func (p *parser) parseTreeForDocumentation(tree scannerTree, tkn *tokeniser, docs *MixinDocs) error {

	comments := []string{}

	resetComments := func() {
		comments = []string{}
	}

	for _, branch := range tree {

		tokens, err := tkn.tokenise(branch)

		if err != nil {
			return p.err(branch, err.Error())
		}

		if len(tokens) > 0 {

			token := tokens[0]

			switch token.kind {
			case tokenLineComment, tokenCommentEnd:
				// Do nothing

			case tokenCommentStart:
				p.parseBlockComment(branch.children, &comments, branch.line, 0)

			case tokenMixinDeclaration:

				if token.content[0] == '_' {
					resetComments()
					continue
				}

				doc := MixinDoc{
					Name:      token.content,
					File:      branch.file,
					Line:      branch.line,
					Reference: branch.String(),
					Signature: string(branch.content),
					Docs:      strings.Join(comments, "\n"),
				}

				// Clear comments
				resetComments()

				// Store the mixin docs and empty the running comment
				if err := p.parseTreeForDocumentation(branch.children, tkn, &doc.Children); err != nil {
					return err
				}

				*docs = append(*docs, doc)

			default:
				resetComments()
				if err := p.parseTreeForDocumentation(branch.children, tkn, docs); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (p *parser) parseBlockComment(tree scannerTree, comments *[]string, line, indentation int) error {

	for _, branch := range tree {

		// Re-add missing blank lines
		if line == 0 {
			line = branch.line
		} else {
			if line != branch.line-1 {
				*comments = append(*comments, "")
			}
			line = branch.line
		}

		*comments = append(*comments, strings.Repeat(" ", indentation*4)+string(branch.content))

		if err := p.parseBlockComment(branch.children, comments, line, indentation+1); err != nil {
			return nil
		}
	}

	return nil
}

func (p *parser) parseLiteral(branch *scannerLine, tkn *tokeniser, token token, scope *scope) error {

	children := len(branch.children) > 0

	if err := p.writeLiteralToOutput(scope, token.content, children); err != nil {
		return p.err(branch, err.Error())
	}

	if children {

		if err := p.parseTree(branch.children, tkn, scope.clone()); err != nil {
			return err
		}

		p.endBlock()
	}

	return nil
}

func (p *parser) parseMixinDeclaration(branch *scannerLine, tokens []token, scope *scope) error {

	i := 0
	literalExpected := false
	optionalArgStart := false

	var (
		arguments []token
		defaults  []string
		current   token
	)

	// Make sure that only variables are given as arguments
	for _, v := range tokens[1:] {

		switch v.kind {

		case tokenLiteral:
			if !literalExpected {
				return p.err(branch, "Argument declaration %d [%s]: Unexpected literal", i, v.content)
			}

			value := v.content

			// Underscore literals are 'no values' in mixin
			// declarations
			if value == noMixinParamValue {
				value = ""
			}

			arguments = append(arguments, current)
			defaults = append(defaults, value)
			literalExpected = false

		case tokenVariableAssignment:
			optionalArgStart = true
			literalExpected = true
			current = token{
				kind:    tokenVariable,
				content: v.content,
				line:    v.line,
			}
			i++

		case tokenVariable:

			if optionalArgStart {
				return p.err(branch, "Argument declaration %d [%s]: A required argument can't follow an optional argument", i, v.content)
			}

			arguments = append(arguments, v)
			defaults = append(defaults, "")
			i++

		default:
			return p.err(branch, "Argument declaration %d [%s] is not a variable or a variable assignment", i, v.content)
		}
	}

	if literalExpected {
		return p.err(branch, "Expected a literal in mixin signature")
	}

	if a, d := len(arguments), len(defaults); a != d {
		return p.err(branch, "Expected eqaual numbers of arguments and defaults (a:%d,d:%d)", a, d)
	}

	scope.setMixin(tokens[0].content, branch, arguments, defaults)

	return nil
}

func (p *parser) parseFunctionCall(branch *scannerLine, tkn *tokeniser, tokens []token, scope *scope) error {

	// Handle built-ins
	if tokens[0].content == builtinMixinBody {
		return p.parseBodyCall(branch, tkn, scope)
	} else if tokens[0].content == builtinMixinInclude {
		return p.parseIncludeCall(branch, tokens, scope)
	}

	// Make sure the mixin exists in the scope
	mx, err := scope.mixin(tokens[0].content)

	if err != nil {
		return p.err(branch, err.Error())
	}

	args, err := p.extractValuesFromArgTokens(branch, tokens[1:], scope)

	if err != nil {
		return p.err(branch, err.Error())
	}

	// Add in the defaults
	if l := len(args); l < len(mx.defaults) {
		args = append(args, mx.defaults[l:]...)
	}

	// Check the argument counts
	if r, g := len(mx.arguments), len(args); r != g {
		return p.err(branch, "Wrong number of arguments for %s (required %d, got %d)", tokens[0].content, r, g)
	}

	// Set the argument values
	for i := 0; i < len(mx.arguments); i++ {
		scope.setArgumentVariable(mx.arguments[i].name, args[i])
	}

	// Set an anchor branch for the __body__ built-in
	scope.branch = branch
	scope.branchScope = scope.parent

	// Call the function!
	return p.parseTree(mx.declaration.children, tkn, scope)
}

func (p *parser) parseBodyCall(branch *scannerLine, tkn *tokeniser, scope *scope) error {

	if scope.branchScope == nil {
		return p.err(branch, "Unexpected error: No parent scope somehow!")
	}

	if scope.branch == nil {
		return p.err(branch, "Unexpected error: No anchor branch!")
	}

	s := scope.branchScope.clone()
	s.mixins = scope.mixins
	s.variables = scope.variables // FIXME Merge?

	return p.parseTree(scope.branch.children, tkn, s)
}

func (p *parser) includeGlob(name string, branch *scannerLine) error {

	name = strings.TrimSuffix(strings.Trim(name, `"'`), ".scl") + ".scl"

	vendorPath := []string{filepath.Join(filepath.Dir(branch.file), "vendor")}
	vendorPath = append(vendorPath, p.includePaths...)

	var paths []string

	for _, ip := range vendorPath {

		ipaths, err := p.fs.Glob(ip + "/" + name)

		if err != nil {
			return err
		}

		if len(ipaths) > 0 {
			paths = ipaths
			break
		}
	}

	if len(paths) == 0 {

		var err error
		paths, err = p.fs.Glob(name)

		if err != nil {
			return err
		}
	}

	if len(paths) == 0 {
		return fmt.Errorf("Can't read %s: no files found", name)
	}

	for _, path := range paths {
		if err := p.Parse(path); err != nil {
			return fmt.Errorf(err.Error())
		}
	}

	return nil
}

func (p *parser) parseIncludeCall(branch *scannerLine, tokens []token, scope *scope) error {

	args, err := p.extractValuesFromArgTokens(branch, tokens[1:], scope)

	if err != nil {
		return p.err(branch, err.Error())
	}

	for _, v := range args {

		if err := p.includeGlob(v, branch); err != nil {
			return p.err(branch, err.Error())
		}
	}

	return nil
}

func (p *parser) extractValuesFromArgTokens(branch *scannerLine, tokens []token, scope *scope) ([]string, error) {

	var args []string

	for _, v := range tokens {
		switch v.kind {

		case tokenLiteral:

			value, err := scope.interpolateLiteral(v.content)

			if err != nil {
				return args, err
			}

			args = append(args, value)

		case tokenVariable:

			value := scope.variable(v.content)

			if value == "" {
				return args, fmt.Errorf("Variable $%s is not declared in this scope", v.content)
			}

			args = append(args, value)

		default:
			return args, fmt.Errorf("Invalid token type for function argument: %s (%s)", v.kind, branch.content)
		}
	}

	return args, nil
}
