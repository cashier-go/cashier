package client

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

// setupFlags installs the same flags as cmd/cashier, which is where the
// config defaults come from.
func setupFlags(t *testing.T, args ...string) {
	t.Helper()
	orig := pflag.CommandLine
	t.Cleanup(func() { pflag.CommandLine = orig })
	fs := pflag.NewFlagSet("cashier", pflag.ContinueOnError)
	fs.String("ca", "http://localhost:10000", "CA server")
	fs.Int("key_size", 0, "Size of key to generate")
	fs.Duration("validity", 24*time.Hour, "Key lifetime")
	fs.String("key_type", "", "Type of private key to generate")
	fs.String("key_file_prefix", "", "Prefix for filename for public key and cert")
	pflag.CommandLine = fs
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}
}

func TestParseConfigDefaults(t *testing.T) {
	a := assert.New(t)
	setupFlags(t)
	c, err := parseConfig(strings.NewReader(""))
	a.NoError(err, "an empty config should yield the defaults")
	a.Equal("http://localhost:10000", c.CA)
	a.Equal("24h0m0s", c.Validity)
	a.Equal("", c.Keytype)
	a.Equal(0, c.Keysize)
	a.True(c.ValidateTLSCertificate)
}

func TestParseConfig(t *testing.T) {
	a := assert.New(t)
	setupFlags(t)
	c, err := parseConfig(strings.NewReader(`
ca = "https://sshca.example.com"  // a comment
key_type = "ed25519" # hash comment
key_size = 384
validity = "4h"
key_file_prefix = "/tmp/keys"
`))
	a.NoError(err)
	a.Equal("https://sshca.example.com", c.CA)
	a.Equal("ed25519", c.Keytype)
	a.Equal(384, c.Keysize)
	a.Equal("4h", c.Validity)
	a.Equal("/tmp/keys", c.PublicFilePrefix)
}

func TestParseConfigDisableTLSValidation(t *testing.T) {
	a := assert.New(t)
	setupFlags(t)
	c, err := parseConfig(strings.NewReader(`validate_tls_certificate = false`))
	a.NoError(err)
	a.False(c.ValidateTLSCertificate)
}

func TestFlagsOverrideConfig(t *testing.T) {
	a := assert.New(t)
	setupFlags(t, "--ca", "https://other.example.com", "--key_size", "521")
	c, err := parseConfig(strings.NewReader(`
ca = "https://file.example.com"
key_size = 384
key_type = "ecdsa"
`))
	a.NoError(err)
	a.Equal("https://other.example.com", c.CA)
	a.Equal(521, c.Keysize)
	// Not passed as a flag, so the config file value should overrule the default.
	a.Equal("ecdsa", c.Keytype)
}

func TestParseConfigMalformed(t *testing.T) {
	setupFlags(t)
	if _, err := parseConfig(strings.NewReader(`ca = "unterminated`)); err == nil {
		t.Error("expected an error parsing a malformed config")
	}
}

func TestReadConfigMissingFile(t *testing.T) {
	a := assert.New(t)
	setupFlags(t)
	c, err := ReadConfig(filepath.Join(t.TempDir(), "no-such-file.conf"))
	a.NoError(err, "a missing config file should not be an error")
	a.Equal("http://localhost:10000", c.CA)
	a.True(c.ValidateTLSCertificate)
}
