package signer

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/nsheridan/cashier/lib"
	"github.com/nsheridan/cashier/server/config"
	"golang.org/x/crypto/ssh"
)

type KeySigner struct {
	ca          ssh.Signer
	validity    time.Duration
	principals  []string
	permissions map[string]string
}

func (s *KeySigner) Sign(req *lib.SignRequest) (string, error) {
	pubkey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(req.Key))
	if err != nil {
		return "", err
	}
	expires := time.Now().Add(s.validity)
	if req.ValidUntil.After(expires) {
		req.ValidUntil = expires
	}
	cert := &ssh.Certificate{
		CertType:    ssh.UserCert,
		Key:         pubkey,
		KeyId:       req.Principal,
		ValidBefore: uint64(req.ValidUntil.Unix()),
		ValidAfter:  uint64(time.Now().Add(-5 * time.Minute).Unix()),
	}
	cert.ValidPrincipals = append(cert.ValidPrincipals, req.Principal)
	cert.ValidPrincipals = append(cert.ValidPrincipals, s.principals...)
	cert.Extensions = s.permissions
	if err := cert.SignCert(rand.Reader, s.ca); err != nil {
		return "", err
	}
	marshaled := ssh.MarshalAuthorizedKey(cert)
	// Remove the trailing newline.
	marshaled = marshaled[:len(marshaled)-1]
	return string(marshaled), nil
}

func makeperms(perms []string) map[string]string {
	if len(perms) > 0 {
		m := make(map[string]string)
		for _, p := range perms {
			m[p] = ""
		}
		return m
	}
	return map[string]string{
		"permit-X11-forwarding":   "",
		"permit-agent-forwarding": "",
		"permit-port-forwarding":  "",
		"permit-pty":              "",
		"permit-user-rc":          "",
	}
}

func NewSigner(conf config.SSH) (*KeySigner, error) {
	data, err := ioutil.ReadFile(conf.SigningKey)
	if err != nil {
		return nil, fmt.Errorf("unable to read CA key %s: %v", conf.SigningKey, err)
	}
	key, err := ssh.ParsePrivateKey(data)
	if err != nil {
		return nil, fmt.Errorf("unable to parse CA key: %v", err)
	}
	validity, err := time.ParseDuration(conf.MaxAge)
	if err != nil {
		return nil, fmt.Errorf("error parsing duration '%s': %v", conf.MaxAge, err)
	}
	return &KeySigner{
		ca:          key,
		validity:    validity,
		principals:  conf.Principals,
		permissions: makeperms(conf.Permissions),
	}, nil
}
