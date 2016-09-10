package client

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"strings"

	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/ssh"
)

// Key is a private key.
type Key interface{}
type keyfunc func(int) (Key, ssh.PublicKey, error)

var (
	keytypes = map[string]keyfunc{
		"rsa":     generateRSAKey,
		"ecdsa":   generateECDSAKey,
		"ed25519": generateED25519Key,
	}
)

func generateED25519Key(bits int) (Key, ssh.PublicKey, error) {
	p, k, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	pub, err := ssh.NewPublicKey(p)
	if err != nil {
		return nil, nil, err
	}
	return &k, pub, nil
}

func generateRSAKey(bits int) (Key, ssh.PublicKey, error) {
	k, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, nil, err
	}
	pub, err := ssh.NewPublicKey(&k.PublicKey)
	if err != nil {
		return nil, nil, err
	}
	return k, pub, nil
}

func generateECDSAKey(bits int) (Key, ssh.PublicKey, error) {
	var curve elliptic.Curve
	switch bits {
	case 256:
		curve = elliptic.P256()
	case 384:
		curve = elliptic.P384()
	case 521:
		curve = elliptic.P521()
	default:
		return nil, nil, fmt.Errorf("Unsupported key size. Valid sizes are '256', '384', '521'")
	}
	k, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	pub, err := ssh.NewPublicKey(&k.PublicKey)
	if err != nil {
		return nil, nil, err
	}
	return k, pub, nil
}

// GenerateKey generates a ssh key-pair according to the type and size specified.
func GenerateKey(keytype string, bits int) (Key, ssh.PublicKey, error) {
	f, ok := keytypes[keytype]
	if !ok {
		var valid []string
		for k := range keytypes {
			valid = append(valid, k)
		}
		return nil, nil, fmt.Errorf("Unsupported key type %s. Valid choices are %s", keytype, strings.Join(valid, "|"))
	}
	return f(bits)
}
