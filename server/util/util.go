package util

import "golang.org/x/crypto/ssh"

// GetPublicKey marshals a ssh certificate to a string.
func GetPublicKey(cert *ssh.Certificate) string {
	marshaled := ssh.MarshalAuthorizedKey(cert)
	// Strip trailing newline
	return string(marshaled[:len(marshaled)-1])
}
