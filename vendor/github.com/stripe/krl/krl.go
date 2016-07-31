// Package krl provides functionality for reading and writing SSH Key Revocation
// Lists (KRLs).
//
// References:
// 	https://raw.githubusercontent.com/openssh/openssh-portable/master/PROTOCOL.krl
package krl

import (
	"bytes"
	"crypto/sha1"
	"io"
	"math/big"
	"sort"
	"time"

	"golang.org/x/crypto/ssh"
)

// KRL, or Key Revocation List, is a list of revoked keys, certificates, and
// identities, possibly signed by some authority. The zero value of KRL is
// appropriate for use, and represents an empty list.
type KRL struct {
	// Version is a number that increases every time the KRL is modified.
	// When marshaling a KRL, if Version is zero GeneratedDate will be used
	// instead.
	Version uint64
	// GeneratedDate is the Unix timestamp the KRL was generated at. When
	// marshaling a KRL, if GeneratedDate is zero the current Unix timestamp
	// will be used instead.
	GeneratedDate uint64
	// Comment is an optional comment for the KRL.
	Comment string
	// Sections is a list of public key and certificate selectors that this
	// KRL applies to.
	Sections []KRLSection
	// SigningKeys is set by ParseKRL and Marshal to the list of Signers
	// that signed (or which claimed to sign) the KRL in the order they
	// appeared (i.e., innermost-first).
	SigningKeys []ssh.PublicKey
}

/*
KRLSection describes a section of a KRL, which selects certain certificates and
keys for revocation. The concrete types KRLCertificateSection,
KRLExplicitKeySection, and KRLFingerprintSection satisfy this interface, and
correspond to the three types of KRL sections currently defined.
*/
type KRLSection interface {
	isRevoked(key ssh.PublicKey) bool
	marshal() []byte
}

/*
KRLCertificateSection revokes SSH certificates by certificate authority and
either serial numbers or key ids.
*/
type KRLCertificateSection struct {
	// CA is the certificate authority whose keys are being revoked by this
	// section. If CA is nil, this section applies to keys signed by any
	// certificate authority.
	CA ssh.PublicKey
	// Sections is a list of certificate selectors.
	Sections []KRLCertificateSubsection
}

func (k *KRLCertificateSection) isRevoked(key ssh.PublicKey) bool {
	cert, ok := key.(*ssh.Certificate)
	if !ok {
		return false
	}

	if k.CA != nil {
		sk := cert.SignatureKey.Marshal()
		ca := k.CA.Marshal()
		if !bytes.Equal(sk, ca) {
			return false
		}
	}

	for _, section := range k.Sections {
		if section.isRevoked(cert) {
			return true
		}
	}
	return false
}

func (k *KRLCertificateSection) marshal() []byte {
	var buf bytes.Buffer
	var ca []byte
	if k.CA != nil {
		ca = k.CA.Marshal()
	}
	buf.Write(ssh.Marshal(krlCertificateSectionHeader{CAKey: ca}))
	headerLen := buf.Len()
	for _, section := range k.Sections {
		buf.Write(section.marshal())
	}
	// All subsections were empty; we should be empty too.
	if buf.Len() == headerLen {
		return nil
	}
	return ssh.Marshal(krlSection{
		SectionType: 1,
		SectionData: buf.Bytes(),
	})
}

/*
KRLCertificateSubsection describes a subsection of a KRL certificate selection,
and selects certain certificates for revocation. The concrete types
KRLCertificateSerialList, KRLCertificateSerialRange, KRLCertificateSerialBitmap,
and KRLCertificateSerialBitmap satisfy this interface, and correspond to the
four subsections currently defined.
*/
type KRLCertificateSubsection interface {
	isRevoked(cert *ssh.Certificate) bool
	marshal() []byte
}

// KRLCertificateSerialList revokes certificates by listing their serial
// numbers.
type KRLCertificateSerialList []uint64

func (k *KRLCertificateSerialList) isRevoked(cert *ssh.Certificate) bool {
	for _, serial := range *k {
		if serial == cert.Serial {
			return true
		}
	}
	return false
}

func (k *KRLCertificateSerialList) marshal() []byte {
	if len(*k) == 0 {
		return nil
	}

	var buf bytes.Buffer
	for _, serial := range *k {
		buf.Write(ssh.Marshal(krlSerialList{
			RevokedCertSerial: serial,
		}))
	}
	return ssh.Marshal(krlCertificateSection{
		CertSectionType: krlSectionCertSerialList,
		CertSectionData: buf.Bytes(),
	})
}

// KRLCertificateSerialRange revokes all certificates with serial numbers in the
// range between Min and Max, inclusive.
type KRLCertificateSerialRange struct {
	Min, Max uint64
}

func (k *KRLCertificateSerialRange) isRevoked(cert *ssh.Certificate) bool {
	return k.Min <= cert.Serial && cert.Serial <= k.Max
}

func (k *KRLCertificateSerialRange) marshal() []byte {
	return ssh.Marshal(krlCertificateSection{
		CertSectionType: krlSectionCertSerialRange,
		CertSectionData: ssh.Marshal(krlSerialRange{
			SerialMin: k.Min,
			SerialMax: k.Max,
		}),
	})
}

// KRLCertificateSerialBitmap revokes certificates densely using a bitmap. If
// bit N of the bitmap is set, the certificate with serial Offset + N is
// revoked.
type KRLCertificateSerialBitmap struct {
	Offset uint64
	Bitmap *big.Int
}

func (k *KRLCertificateSerialBitmap) isRevoked(cert *ssh.Certificate) bool {
	if cert.Serial < k.Offset {
		return false
	}
	if cert.Serial-k.Offset > krlMaxBitmapSize {
		return false
	}
	return k.Bitmap.Bit(int(cert.Serial-k.Offset)) == 1
}

func (k *KRLCertificateSerialBitmap) marshal() []byte {
	return ssh.Marshal(krlCertificateSection{
		CertSectionType: krlSectionCertSerialBitmap,
		CertSectionData: ssh.Marshal(krlSerialBitmap{
			SerialOffset:      k.Offset,
			RevokedKeysBitmap: k.Bitmap,
		}),
	})
}

// KRLCertificateKeyID revokes certificates by listing key ids. This may be
// useful in revoking all certificates associated with a particular identity,
// for instance hosts or users.
type KRLCertificateKeyID []string

func (k *KRLCertificateKeyID) isRevoked(cert *ssh.Certificate) bool {
	for _, id := range *k {
		if id == cert.KeyId {
			return true
		}
	}
	return false
}

func (k *KRLCertificateKeyID) marshal() []byte {
	if len(*k) == 0 {
		return nil
	}

	var buf bytes.Buffer
	for _, id := range *k {
		buf.Write(ssh.Marshal(krlKeyID{
			KeyID: id,
		}))
	}
	return ssh.Marshal(krlCertificateSection{
		CertSectionType: krlSectionCertKeyId,
		CertSectionData: buf.Bytes(),
	})
}

// ssh.PublicKey objects might be certificates, which have a different wire
// format.
func marshalPubkey(key ssh.PublicKey) []byte {
	switch v := key.(type) {
	case *ssh.Certificate:
		return marshalPubkey(v.Key)
	default:
		return key.Marshal()
	}
}

// KRLExplicitKeySection revokes keys by explicitly listing them.
type KRLExplicitKeySection []ssh.PublicKey

func (k *KRLExplicitKeySection) isRevoked(key ssh.PublicKey) bool {
	kbuf := marshalPubkey(key)
	for _, key := range *k {
		if bytes.Equal(kbuf, marshalPubkey(key)) {
			return true
		}
	}
	return false
}

func (k *KRLExplicitKeySection) marshal() []byte {
	if len(*k) == 0 {
		return nil
	}

	var buf bytes.Buffer
	for _, key := range *k {
		buf.Write(ssh.Marshal(krlExplicitKey{
			PublicKeyBlob: marshalPubkey(key),
		}))
	}
	return ssh.Marshal(krlSection{
		SectionType: 2,
		SectionData: buf.Bytes(),
	})
}

// KRLFingerprintSection revokes keys by their SHA1 fingerprints. It is
// semantically equivalent to--but is more space efficient than--
// KRLExplicitKeySection.
type KRLFingerprintSection [][sha1.Size]byte

func (k *KRLFingerprintSection) isRevoked(key ssh.PublicKey) bool {
	sha := sha1.Sum(marshalPubkey(key))
	for _, hash := range *k {
		if hash == sha {
			return true
		}
	}
	return false
}

type bigEndian [][sha1.Size]byte

func (b bigEndian) Len() int {
	return len(b)
}
func (b bigEndian) Less(i, j int) bool {
	return bytes.Compare(b[i][:], b[j][:]) == -1
}
func (b bigEndian) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (k *KRLFingerprintSection) marshal() []byte {
	if len(*k) == 0 {
		return nil
	}

	// For some reason SSH insists that keys revoked by fingerprint must be
	// sorted as if they were big-endian integers (i.e., lexicographically).
	be := make(bigEndian, len(*k))
	for i, hash := range *k {
		be[i] = hash
	}
	sort.Sort(be)

	var buf bytes.Buffer
	for _, hash := range be {
		buf.Write(ssh.Marshal(krlFingerprintSHA1{
			PublicKeyHash: hash[:],
		}))
	}
	return ssh.Marshal(krlSection{
		SectionType: 3,
		SectionData: buf.Bytes(),
	})
}

// Marshal serializes the KRL and optionally signs it with one or more authority
// keys.
func (k *KRL) Marshal(rand io.Reader, keys ...ssh.Signer) ([]byte, error) {
	if k.GeneratedDate == 0 {
		k.GeneratedDate = uint64(time.Now().Unix())
	}
	if k.Version == 0 {
		k.Version = k.GeneratedDate
	}
	k.SigningKeys = nil

	var buf bytes.Buffer
	buf.Write(ssh.Marshal(krlHeader{
		KRLMagic:         krlMagic,
		KRLFormatVersion: 1,
		KRLVersion:       k.Version,
		GeneratedDate:    k.GeneratedDate,
		Comment:          k.Comment,
	}))

	for _, section := range k.Sections {
		buf.Write(section.marshal())
	}

	for _, key := range keys {
		buf.Write(ssh.Marshal(krlSignatureHeader{
			SignatureKey: key.PublicKey().Marshal(),
		}))
		sig, err := key.Sign(rand, buf.Bytes())
		if err != nil {
			return nil, err
		}
		buf.Write(ssh.Marshal(krlSignature{
			Signature: ssh.Marshal(sig),
		}))
		k.SigningKeys = append(k.SigningKeys, key.PublicKey())
	}

	return buf.Bytes(), nil
}

// IsRevoked returns true if the given key has been revoked by this KRL.
func (k *KRL) IsRevoked(key ssh.PublicKey) bool {
	for _, section := range k.Sections {
		if section.isRevoked(key) {
			return true
		}
	}
	return false
}
