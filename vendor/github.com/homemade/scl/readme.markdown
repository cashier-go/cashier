[![Build Status](https://travis-ci.org/homemade/scl.svg?branch=master)](https://travis-ci.org/homemade/scl) [![Coverage Status](https://coveralls.io/repos/github/homemade/scl/badge.svg?branch=master)](https://coveralls.io/github/homemade/scl?branch=master) [![GoDoc](https://godoc.org/github.com/homemade/scl?status.svg)](https://godoc.org/github.com/homemade/scl) [![Language reference](https://img.shields.io/badge/language-reference-736caf.svg)](https://github.com/homemade/scl/wiki)

## Sepia Configuration Language

The Sepia Configuration Language is a simple, declarative, semi-functional, self-documenting language that extends HashiCorp's [HCL](https://github.com/hashicorp/hcl) in the same sort of way that Sass extends CSS. The syntax of SCL is concise, intuitive and flexible. Critically, it also validates much of your configuration by design, so it's harder to configure an application that seems like it should work &mdash; but doesn't. 

SCL transpiles to HCL and, like CSS and Sass, any [properly formatted](https://github.com/fatih/hclfmt) HCL is valid SCL. If you have an existing HCL setup, you can transplant it to SCL directly and then start making use of the code organisation, mixins, and properly scoped variables that SCL offers.

In addition to the language itself, there is a useful [command-line tool](https://github.com/homemade/scl/tree/master/cmd/scl) than can compile your .scl files and write the output to the terminal, run gold standard tests against you code, and even fetch libraries of code from public version control systems. 

This readme is concerned with the technical implementation of the Go package and the CLI tool. For a full language specification complete with examples and diagrams, see the [wiki](https://github.com/homemade/scl/wiki). 

## Installation

Assuming you have Go installed, the package and CLI tool can be fetched in the usual way:

```
$ go get -u github.com/homemade/scl/...
```

## Contributions

This is fairly new software that has been tested intensively over a fairly narrow range of functions. Minor bugs are expected! If you have any suggestions or feature requests [please open an issue](https://github.com/homemade/scl/issues/new). Pull requests for bug fixes or uncontroversial improvements are appreciated. 

We're currently working on standard libraries for Terraform and Hugo. If you build an SCL library for anything else, please let us know!

## Using SCL in your application

SCL is built on top of HCL, and the fundamental procedure for using it is the more or less the same: SCL code is decoded into a Go struct, informed by `hcl` tags on the struct's fields. A trivially simple example is as follows:

``` go
myConfigObject := struct {
    SomeVariable int `hcl:"some_variable"`
}{}

if err := scl.DecodeFile(&myConfigObject, "/path/to/a/config/file.scl"); err != nil {
    // handle error
}

// myConfigObject is now populated!
```

There are many more options&mdash;like include paths, predefined variables and documentation generation&mdash;available in the [API](https://godoc.org/github.com/homemade/scl). If you have an existing HCL set up in your application, you can easily swap out your HCL loading function for an SCL loading function to try it out!

## CLI tool

The tool, which is installed with the package, is named `scl`. With it, you can transpile .scl files to stdout, run gold standard tests that compare .scl files to .hcl files, and fetch external libraries from version control.

### Usage

Run `scl` for a command syntax. 

### Examples

Basic example:
```
$ scl run $GOPATH/src/bitbucket.org/homemade/scl/fixtures/valid/basic.scl
/* .../bitbucket.org/homemade/scl/fixtures/valid/basic.scl */
wrapper {
  inner = "yes"
  another = "1" {
    yet_another = "123"
  }
}
```

Adding includes:
```
$ scl run -include $GOPATH/src/bitbucket.org/homemade/scl $GOPATH/src/bitbucket.org/homemade/scl/fixtures/valid/import.scl
/* .../bitbucket.org/homemade/scl/fixtures/valid/import.scl */
wrapper {
  inner = "yes"
  another = "1" {
    yet_another = "123"
  }
}
output = "this is from simpleMixin"
```

Adding params via cli flags:
```
$ scl run -param myVar=1 $GOPATH/src/bitbucket.org/homemade/scl/fixtures/valid/variables.scl
/* .../bitbucket.org/homemade/scl/fixtures/valid/variables.scl */
outer {
  inner = 1
}
```

Adding params via environmental variables:
```
$ myVar=1 scl run $GOPATH/src/bitbucket.org/homemade/scl/fixtures/valid/variables.scl
/* .../bitbucket.org/homemade/scl/fixtures/valid/variables.scl */
outer {
  inner = 1
}
```

Skipping environmental variable slurping:
```
$ myVar=1 scl run -no-env -param myVar=2 $GOPATH/src/bitbucket.org/homemade/scl/fixtures/valid/variables.scl
/* .../src/bitbucket.org/homemade/scl/fixtures/valid/variables.scl */
outer {
  inner = 2
}
```
