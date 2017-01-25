package scl

//go:generate stringer -type=tokenKind -output=token_string.go
type tokenKind int

const (
	tokenLineComment tokenKind = iota
	tokenMixinDeclaration
	tokenVariable
	tokenVariableAssignment
	tokenFunctionCall
	tokenLiteral
	tokenVariableDeclaration
	tokenConditionalVariableAssignment
	tokenCommentStart
	tokenCommentEnd
)

var tokenKindsByString = map[tokenKind]string{
	tokenLineComment:                   "line comment",
	tokenMixinDeclaration:              "mixin declaration",
	tokenVariableAssignment:            "variable assignment",
	tokenVariableDeclaration:           "variable declaration",
	tokenConditionalVariableAssignment: "conditional variable declaration",
	tokenFunctionCall:                  "function call",
	tokenLiteral:                       "literal",
	tokenCommentStart:                  "comment start",
	tokenCommentEnd:                    "comment end",
}

type token struct {
	kind    tokenKind
	content string
	line    *scannerLine
}

func (t token) String() string {
	return tokenKindsByString[t.kind]
}
