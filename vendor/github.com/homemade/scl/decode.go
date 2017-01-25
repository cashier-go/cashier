package scl

import "github.com/hashicorp/hcl"

/*
DecodeFile reads the given input file and decodes it into the structure given by `out`.
*/
func DecodeFile(out interface{}, path string) error {

	parser, err := NewParser(NewDiskSystem())

	if err != nil {
		return err
	}

	if err := parser.Parse(path); err != nil {
		return err
	}

	return hcl.Decode(out, parser.String())
}
