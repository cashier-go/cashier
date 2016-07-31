package krl

import "math/big"

// We (unfortunately) make extensive use of x/crypto/ssh.Unmarshal's "rest"
// parameter here. The KRL specification makes extensive use of sections placed
// back-to-back, and there's no other way to get x/crypto/ssh.Unmarshal to emit
// the portion of the input that has not yet been parsed.

const krlMagic = 0x5353484b524c0a00

/*
#define KRL_MAGIC		0x5353484b524c0a00ULL  /* "SSHKRL\n\0" * /
#define KRL_FORMAT_VERSION	1

	uint64	KRL_MAGIC
	uint32	KRL_FORMAT_VERSION
	uint64	krl_version
	uint64	generated_date
	uint64	flags
	string	reserved
	string	comment
*/
type krlHeader struct {
	KRLMagic         uint64
	KRLFormatVersion uint32
	KRLVersion       uint64
	GeneratedDate    uint64
	Flags            uint64
	Reserved         []byte
	Comment          string

	Rest []byte `ssh:"rest"`
}

/*
	byte	section_type
	string	section_data

#define KRL_SECTION_CERTIFICATES		1
#define KRL_SECTION_EXPLICIT_KEY		2
#define KRL_SECTION_FINGERPRINT_SHA1		3
#define KRL_SECTION_SIGNATURE			4
*/
type krlSection struct {
	SectionType byte
	SectionData []byte

	Rest []byte `ssh:"rest"`
}

/*
	string ca_key
	string reserved
*/
type krlCertificateSectionHeader struct {
	CAKey    []byte
	Reserved []byte

	Rest []byte `ssh:"rest"`
}

/*
	byte	cert_section_type
	string	cert_section_data

#define KRL_SECTION_CERT_SERIAL_LIST	0x20
#define KRL_SECTION_CERT_SERIAL_RANGE	0x21
#define KRL_SECTION_CERT_SERIAL_BITMAP	0x22
#define KRL_SECTION_CERT_KEY_ID		0x23
*/
type krlCertificateSection struct {
	CertSectionType byte
	CertSectionData []byte

	Rest []byte `ssh:"rest"`
}

const (
	krlSectionCertSerialList   = 0x20
	krlSectionCertSerialRange  = 0x21
	krlSectionCertSerialBitmap = 0x22
	krlSectionCertKeyId        = 0x23
)

/*
	uint64	revoked_cert_serial
	uint64	...
*/
type krlSerialList struct {
	RevokedCertSerial uint64

	Rest []byte `ssh:"rest"`
}

/*
	uint64	serial_min
	uint64	serial_max
*/
type krlSerialRange struct {
	SerialMin uint64
	SerialMax uint64
}

/*
	uint64	serial_offset
	mpint	revoked_keys_bitmap
*/
type krlSerialBitmap struct {
	SerialOffset      uint64
	RevokedKeysBitmap *big.Int
}

/*
	string	key_id[0]
	...
*/
type krlKeyID struct {
	KeyID string

	Rest []byte `ssh:"rest"`
}

/*
	string	public_key_blob[0]
	....
*/
type krlExplicitKey struct {
	PublicKeyBlob []byte

	Rest []byte `ssh:"rest"`
}

/*
	string	public_key_hash[0]
	....
*/
type krlFingerprintSHA1 struct {
	PublicKeyHash []byte

	Rest []byte `ssh:"rest"`
}

/*
	byte	KRL_SECTION_SIGNATURE
	string	signature_key
	string	signature

We split this struct into two parts: krlSignatureHeader is included in the
signature, and so the inverse of its "Rest" key is the data coverd by the
signature.
*/
type krlSignatureHeader struct {
	SignatureKey []byte `sshtype:"4"`

	Rest []byte `ssh:"rest"`
}

type krlSignature struct {
	Signature []byte

	Rest []byte `ssh:"rest"`
}
