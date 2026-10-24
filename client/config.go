package client

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"

	"github.com/hashicorp/hcl"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/pflag"
)

// Config holds the client configuration.
type Config struct {
	CA                     string `hcl:"ca"`
	Keytype                string `hcl:"key_type"`
	Keysize                int    `hcl:"key_size"`
	Validity               string `hcl:"validity"`
	ValidateTLSCertificate bool   `hcl:"validate_tls_certificate"`
	PublicFilePrefix       string `hcl:"key_file_prefix"`
}

// flagValue returns the value of a command-line flag.
// If onlyChanged is set then only flags which were explicitly set on the
// command line are returned, otherwise an unset flag returns its default.
func flagValue(name string, onlyChanged bool) (string, bool) {
	f := pflag.Lookup(name)
	switch {
	case f == nil:
		return "", false
	case f.Changed:
		return f.Value.String(), true
	case onlyChanged:
		return "", false
	}
	return f.DefValue, true
}

// setValue applies command-line flags to the config.
func (c *Config) setValue(onlyChanged bool) {
	for name, set := range map[string]func(string){
		"ca":              func(v string) { c.CA = v },
		"key_type":        func(v string) { c.Keytype = v },
		"validity":        func(v string) { c.Validity = v },
		"key_file_prefix": func(v string) { c.PublicFilePrefix = v },
		"key_size": func(v string) {
			if i, err := strconv.Atoi(v); err == nil {
				c.Keysize = i
			}
		},
	} {
		if v, ok := flagValue(name, onlyChanged); ok {
			set(v)
		}
	}
}

// parseConfig reads the client configuration from r into a Config struct.
// Empty input is valid and yields the defaults.
func parseConfig(r io.Reader) (*Config, error) {
	bs, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	c := &Config{
		ValidateTLSCertificate: true,
	}
	c.setValue(false)
	if err := hcl.Unmarshal(bs, c); err != nil {
		return nil, err
	}
	// Called again so that any flags set on the cli override the configfile
	c.setValue(true)
	p, err := homedir.Expand(c.PublicFilePrefix)
	if err != nil {
		return nil, err
	}
	c.PublicFilePrefix = p
	return c, nil
}

// ReadConfig reads the client configuration from a file into a Config struct.
// A missing config file is not an error - the defaults and any command-line
// flags are used instead.
func ReadConfig(path string) (*Config, error) {
	// A missing file leaves bs empty, which parses as an empty config.
	bs, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("unable to read config file %s: %w", path, err)
	}
	c, err := parseConfig(bytes.NewReader(bs))
	if err != nil {
		return nil, fmt.Errorf("error parsing config file %s: %w", path, err)
	}
	return c, nil
}
