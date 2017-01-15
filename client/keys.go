package client

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"fmt"

	"github.com/pkg/errors"

	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/ssh"
)

// Key is a private key.
type Key crypto.Signer

// Options for key generation.
// Defaults will generate a 2048 bit RSA key.
type options struct {
	keytype string
	size    int
}

var defaultOptions = options{
	keytype: "rsa",
	size:    0, // Different key types have different default sizes.
}

// A KeyOption is used to generate keys of different types and sizes.
type KeyOption func(*options)

// KeyType sets the type of key to generate.
// Valid types are: "rsa", "ecdsa", "ed25519".
// Default is "rsa"
func KeyType(keyType string) KeyOption {
	return func(o *options) {
		o.keytype = keyType
	}
}

// KeySize sets the size of the key in bits.
// RSA keys must be a minimum of 1024 bits. The default is 2048 bits.
// ECDSA keys must be one of 256, 384, or 521 bits. The default is 256 bits.
// Ed25519 keys are of a fixed size. This option is ignored.
func KeySize(size int) KeyOption {
	return func(o *options) {
		o.size = size
	}
}

func generateED25519Key() (Key, error) {
	_, k, err := ed25519.GenerateKey(rand.Reader)
	return &k, err
}

func generateRSAKey(size int) (Key, error) {
	return rsa.GenerateKey(rand.Reader, size)
}

func generateECDSAKey(size int) (Key, error) {
	var curve elliptic.Curve
	switch size {
	case 256:
		curve = elliptic.P256()
	case 384:
		curve = elliptic.P384()
	case 521:
		curve = elliptic.P521()
	default:
		return nil, fmt.Errorf("Unsupported ECDSA key size: %d. Valid sizes are '256', '384', '521'", size)
	}
	return ecdsa.GenerateKey(curve, rand.Reader)
}

// GenerateKey generates a ssh key-pair according to the type and size specified.
func GenerateKey(options ...func(*options)) (Key, ssh.PublicKey, error) {
	var privkey Key
	var pubkey ssh.PublicKey
	var err error

	config := defaultOptions
	for _, o := range options {
		o(&config)
	}

	switch config.keytype {
	case "rsa":
		if config.size == 0 {
			config.size = 2048
		}
		privkey, err = generateRSAKey(config.size)
	case "ecdsa":
		if config.size == 0 {
			config.size = 256
		}
		privkey, err = generateECDSAKey(config.size)
	case "ed25519":
		privkey, err = generateED25519Key()
	default:
		privkey, err = generateRSAKey(config.size)
	}
	if err != nil {
		return nil, nil, errors.Wrapf(err, "unable to generate %s key-pair", config.keytype)
	}
	pubkey, err = ssh.NewPublicKey(privkey.Public())
	return privkey, pubkey, errors.Wrap(err, "error parsing public key")
}
