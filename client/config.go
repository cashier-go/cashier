package client

import (
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config holds the client configuration.
type Config struct {
	CA                     string `mapstructure:"ca"`
	Keytype                string `mapstructure:"key_type"`
	Keysize                int    `mapstructure:"key_size"`
	Validity               string `mapstructure:"validity"`
	ValidateTLSCertificate bool   `mapstructure:"validate_tls_certificate"`
	PublicFilePrefix       string `mapstructure:"public_file_prefix"`
}

func setDefaults() {
	viper.BindPFlag("ca", pflag.Lookup("ca"))
	viper.BindPFlag("key_type", pflag.Lookup("key_type"))
	viper.BindPFlag("key_size", pflag.Lookup("key_size"))
	viper.BindPFlag("validity", pflag.Lookup("validity"))
	viper.BindPFlag("public_file_prefix", pflag.Lookup("public_file_prefix"))
	viper.SetDefault("validateTLSCertificate", true)
}

// ReadConfig reads the client configuration from a file into a Config struct.
func ReadConfig(path string) (*Config, error) {
	setDefaults()
	viper.SetConfigFile(path)
	viper.SetConfigType("hcl")
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}
	c := &Config{}
	if err := viper.Unmarshal(c); err != nil {
		return nil, err
	}
	p, err := homedir.Expand(c.PublicFilePrefix)
	if err != nil {
		return nil, err
	}
	c.PublicFilePrefix = p
	return c, nil
}
