package signer

import (
	"crypto/md5"
	"crypto/rand"
	"fmt"
	"log"
	"strings"
	"time"

	"go4.org/wkfs"
	_ "go4.org/wkfs/gcs" // Register "/gcs/" as a wkfs.

	"github.com/nsheridan/cashier/lib"
	"github.com/nsheridan/cashier/server/config"
	"github.com/nsheridan/cashier/server/store"
	"github.com/stripe/krl"
	"golang.org/x/crypto/ssh"
)

// KeySigner does the work of signing a ssh public key with the CA key.
type KeySigner struct {
	ca          ssh.Signer
	validity    time.Duration
	principals  []string
	permissions map[string]string
}

// SignUserKey returns a signed ssh certificate.
func (s *KeySigner) SignUserKey(req *lib.SignRequest, username string) (*ssh.Certificate, error) {
	pubkey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(req.Key))
	if err != nil {
		return nil, err
	}
	expires := time.Now().UTC().Add(s.validity)
	if req.ValidUntil.After(expires) {
		req.ValidUntil = expires
	}
	cert := &ssh.Certificate{
		CertType:    ssh.UserCert,
		Key:         pubkey,
		KeyId:       fmt.Sprintf("%s_%d", username, time.Now().UTC().Unix()),
		ValidBefore: uint64(req.ValidUntil.Unix()),
		ValidAfter:  uint64(time.Now().UTC().Add(-5 * time.Minute).Unix()),
	}
	cert.ValidPrincipals = append(cert.ValidPrincipals, username)
	cert.ValidPrincipals = append(cert.ValidPrincipals, s.principals...)
	cert.Extensions = s.permissions
	if err := cert.SignCert(rand.Reader, s.ca); err != nil {
		return nil, err
	}
	log.Printf("Issued cert id: %s principals: %s fp: %s valid until: %s\n", cert.KeyId, cert.ValidPrincipals, fingerprint(pubkey), time.Unix(int64(cert.ValidBefore), 0).UTC())
	return cert, nil
}

// GenerateRevocationList returns an SSH key revocation list (KRL).
func (s *KeySigner) GenerateRevocationList(certs []*store.CertRecord) ([]byte, error) {
	revoked := &krl.KRLCertificateSection{
		CA: s.ca.PublicKey(),
	}
	ids := krl.KRLCertificateKeyID{}
	for _, c := range certs {
		ids = append(ids, c.KeyID)
	}
	revoked.Sections = append(revoked.Sections, &ids)
	k := &krl.KRL{
		Sections: []krl.KRLSection{revoked},
	}
	return k.Marshal(rand.Reader)
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

// New creates a new KeySigner from the supplied configuration.
func New(conf *config.SSH) (*KeySigner, error) {
	data, err := wkfs.ReadFile(conf.SigningKey)
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
		principals:  conf.AdditionalPrincipals,
		permissions: makeperms(conf.Permissions),
	}, nil
}

func fingerprint(pubkey ssh.PublicKey) string {
	md5String := md5.New()
	md5String.Write(pubkey.Marshal())
	fp := fmt.Sprintf("% x", md5String.Sum(nil))
	return strings.Replace(fp, " ", ":", -1)
}
