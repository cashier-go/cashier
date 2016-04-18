package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"fmt"

	"golang.org/x/crypto/ssh"
)

const (
	rsaKey   = "rsa"
	ecdsaKey = "ecdsa"
)

type key interface{}

func generateRSAKey(bits int) (*rsa.PrivateKey, ssh.PublicKey, error) {
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

func generateECDSAKey(bits int) (*ecdsa.PrivateKey, ssh.PublicKey, error) {
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

func generateKey(keytype string, bits int) (key, ssh.PublicKey, error) {
	switch keytype {
	case rsaKey:
		return generateRSAKey(bits)
	case ecdsaKey:
		return generateECDSAKey(bits)
	default:
		return nil, nil, fmt.Errorf("Unsupported key type %s. Valid choices are [%s, %s]", keytype, rsaKey, ecdsaKey)
	}
}
