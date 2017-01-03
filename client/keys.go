package client

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"fmt"

	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/ssh"
)

// Key is a private key.
type Key crypto.Signer

// KeyOptions allows specifying desired key type and size.
type KeyOptions struct {
	// Type of key to generate: "rsa", "ecdsa", "ed25519".
	// Default is "rsa".
	Type string

	// Size of key to generate.
	// RSA keys must be a minimum of 1024 bits. The default is 2048 bits.
	// ECDSA keys must be one of the following: 256, 384 or 521 bits. The default is 256 bits.
	// ED25519 keys are of fixed length and this field is ignored.
	Size int
}

func generateED25519Key() (Key, error) {
	_, k, err := ed25519.GenerateKey(rand.Reader)
	return &k, err
}

func generateRSAKey(size int) (Key, error) {
	if size == 0 {
		size = 2048
	}
	return rsa.GenerateKey(rand.Reader, size)
}

func generateECDSAKey(size int) (Key, error) {
	if size == 0 {
		size = 256
	}
	var curve elliptic.Curve
	switch size {
	case 256:
		curve = elliptic.P256()
	case 384:
		curve = elliptic.P384()
	case 521:
		curve = elliptic.P521()
	default:
		return nil, fmt.Errorf("Unsupported key size: %d. Valid sizes are '256', '384', '521'", size)
	}
	return ecdsa.GenerateKey(curve, rand.Reader)
}

// GenerateKey generates a ssh key-pair according to the type and size specified.
func GenerateKey(options ...KeyOptions) (Key, ssh.PublicKey, error) {
	var privkey Key
	var pubkey ssh.PublicKey
	var err error
	var opts KeyOptions
	if len(options) > 0 {
		opts = options[len(options)-1]
	}
	switch opts.Type {
	case "rsa":
		privkey, err = generateRSAKey(opts.Size)
	case "ecdsa":
		privkey, err = generateECDSAKey(opts.Size)
	case "ed25519":
		privkey, err = generateED25519Key()
	default:
		privkey, err = generateRSAKey(opts.Size)
	}
	if err != nil {
		return nil, nil, err
	}
	pubkey, err = ssh.NewPublicKey(privkey.Public())
	return privkey, pubkey, err
}
