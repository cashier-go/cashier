package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	parsedConfig = &Config{
		Server: &Server{
			UseTLS:       true,
			TLSKey:       "server.key",
			TLSCert:      "server.crt",
			Addr:         "127.0.0.1",
			Port:         443,
			User:         "nobody",
			CookieSecret: "supersecret",
			CSRFSecret:   "supersecret",
			HTTPLogFile:  "cashierd.log",
			Database:     Database{"type": "mysql", "username": "user", "password": "passwd", "address": "localhost:3306"},
		},
		Auth: &Auth{
			OauthClientID:     "client_id",
			OauthClientSecret: "secret",
			OauthCallbackURL:  "https://sshca.example.com/auth/callback",
			Provider:          "google",
			ProviderOpts:      map[string]string{"domain": "example.com"},
			UsersWhitelist:    []string{"a_user"},
		},
		SSH: &SSH{
			SigningKey:           "signing_key",
			AdditionalPrincipals: []string{"ec2-user", "ubuntu"},
			MaxAge:               "720h",
			Permissions:          []string{"permit-pty", "permit-X11-forwarding", "permit-port-forwarding", "permit-user-rc"},
		},
		AWS: &AWS{
			Region:    "us-east-1",
			AccessKey: "abcdef",
			SecretKey: "omg123",
		},
		Vault: &Vault{
			Address: "https://vault:8200",
			Token:   "abc-def-456-789",
		},
	}
)

func TestConfigParser(t *testing.T) {
	c, err := ReadConfig("testdata/test.config")
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, parsedConfig, c)
}

func TestConfigVerify(t *testing.T) {
	_, err := ReadConfig("testdata/empty.config")
	assert.Contains(t, err.Error(), "missing ssh config section", "missing server config section", "missing auth config section")
}

func TestDatastoreConversion(t *testing.T) {
	tests := []struct {
		in  string
		out Database
	}{
		{
			"mysql:user:passwd:localhost:3306", Database{"type": "mysql", "username": "user", "password": "passwd", "address": "localhost:3306"},
		},
		{
			"mongo:::host1,host2", Database{"type": "mongo", "username": "", "password": "", "address": "host1,host2"},
		},
		{
			"mem", Database{"type": "mem"},
		},
		{
			"sqlite:/data/certs.db", Database{"type": "sqlite", "filename": "/data/certs.db"},
		},
	}

	for _, tc := range tests {
		config := &Config{
			Server: &Server{
				Datastore: tc.in,
			},
		}
		convertDatastoreConfig(config)
		assert.EqualValues(t, config.Server.Database, tc.out)
	}
}
