package lib

import "golang.org/x/crypto/ssh"

// GetPublicKey marshals a ssh certificate to a string.
func GetPublicKey(pub ssh.PublicKey) string {
	marshaled := ssh.MarshalAuthorizedKey(pub)
	// Strip trailing newline
	return string(marshaled[:len(marshaled)-1])
}
