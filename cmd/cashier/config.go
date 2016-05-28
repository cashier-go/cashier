package main

import (
	"github.com/spf13/viper"
)

type config struct {
	CA                     string `mapstructure:"ca"`
	Keytype                string `mapstructure:"key_type"`
	Keysize                int    `mapstructure:"key_size"`
	Validity               string `mapstructure:"validity"`
	ValidateTLSCertificate bool   `mapstructure:"validate_tls_certificate"`
}

func setDefaults() {
	viper.SetDefault("ca", "http://localhost:10000")
	viper.SetDefault("key_type", "rsa")
	viper.SetDefault("key_size", 2048)
	viper.SetDefault("validity", "24h")
	viper.SetDefault("validateTLSCertificate", true)
}

func readConfig(path string) (*config, error) {
	setDefaults()
	viper.SetConfigFile(path)
	viper.SetConfigType("hcl")
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}
	c := &config{}
	if err := viper.Unmarshal(c); err != nil {
		return nil, err
	}
	return c, nil
}
