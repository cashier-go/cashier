package config

import (
	"bytes"
	"testing"
	"time"

	"github.com/nsheridan/cashier/testdata"
	"github.com/stretchr/testify/assert"
)

func TestServerConfig(t *testing.T) {
	a := assert.New(t)
	c, err := ReadConfig(bytes.NewBuffer(testdata.ServerConfig))
	if err != nil {
		t.Fatal(err)
	}
	server := c.Server
	a.IsType(server, Server{})
	a.True(server.UseTLS)
	a.Equal(server.TLSKey, "server.key")
	a.Equal(server.TLSCert, "server.crt")
	a.Equal(server.Port, 443)
	a.Equal(server.CookieSecret, "supersecret")
}

func TestAuthConfig(t *testing.T) {
	a := assert.New(t)
	c, err := ReadConfig(bytes.NewBuffer(testdata.AuthConfig))
	if err != nil {
		t.Fatal(err)
	}
	auth := c.Auth
	a.IsType(auth, Auth{})
	a.Equal(auth.Provider, "google")
	a.Equal(auth.ProviderOpts, map[string]string{"domain": "example.com"})
	a.Equal(auth.OauthClientID, "client_id")
	a.Equal(auth.OauthClientSecret, "secret")
	a.Equal(auth.OauthCallbackURL, "https://sshca.example.com/auth/callback")
	a.Equal(auth.JWTSigningKey, "supersecret")
}

func TestSSHConfig(t *testing.T) {
	a := assert.New(t)
	c, err := ReadConfig(bytes.NewBuffer(testdata.SSHConfig))
	if err != nil {
		t.Fatal(err)
	}
	ssh := c.SSH
	a.IsType(ssh, SSH{})
	a.Equal(ssh.SigningKey, "signing_key")
	a.Equal(ssh.AdditionalPrincipals, []string{"ec2-user", "ubuntu"})
	a.Equal(ssh.Permissions, []string{"permit-pty", "permit-X11-forwarding", "permit-port-forwarding", "permit-user-rc"})
	a.Equal(ssh.MaxAge, "720h")
	d, err := time.ParseDuration(ssh.MaxAge)
	if err != nil {
		t.Fatal(err)
	}
	a.Equal(d.Hours(), float64(720))
}
