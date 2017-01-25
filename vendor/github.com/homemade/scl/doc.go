/*
Package scl is an implementation of a parser for the Sepia Configuration
Language.

SCL is a simple, declarative, self-documenting, semi-functional language that
extends HCL (as in https://github.com/hashicorp/hcl) in the same way that Sass
extends CSS.  What that means is, any properly formatted HCL is valid SCL. If
you really enjoy HCL, you can keep using it exclusively: under the hood, SCL
‘compiles’ to HCL.  The difference is that now you can explicitly include
files, use ‘mixins’ to quickly inject boilerplate code, and use properly
scoped, natural variables.  The language is designed to accompany Sepia (and,
specifically, Sepia plugins) but it's a general purpose language, and can be
used for pretty much any configurational purpose.

Full documenation for the language itself, including a language specification,
tutorials and examples, is available at https://github.com/homemade/scl/wiki.
*/
package scl

/*
MixinDoc documents a mixin from a particular SCL file. Since mixins can be nested, it
also includes a tree of all child mixins.
*/
type MixinDoc struct {
	Name      string
	File      string
	Line      int
	Reference string
	Signature string
	Docs      string
	Children  MixinDocs
}

/*
MixinDocs is a slice of MixinDocs, for convenience.
*/
type MixinDocs []MixinDoc
