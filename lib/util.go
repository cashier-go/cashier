package lib

import "golang.org/x/crypto/ssh"

// GetPublicKey marshals a ssh certificate to a string.
func GetPublicKey(pub ssh.PublicKey) []byte {
	marshaled := ssh.MarshalAuthorizedKey(pub)
	// Strip trailing newline
	return marshaled[:len(marshaled)-1]
}
