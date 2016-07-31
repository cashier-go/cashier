package krl

import (
	"crypto/sha1"
	"fmt"

	"golang.org/x/crypto/ssh"
)

// Hundreds of millions, or 32MB of bitmap
const krlMaxBitmapSize = 0x10000000

// KRLSigningErrors is a slice of error messages which correspond one-to-one
// with KRL.SigningKeys.
type KRLSigningErrors []error

func (k KRLSigningErrors) Error() string {
	return fmt.Sprintf("krl: bad signatures: %v", []error(k))
}

func (k KRLSigningErrors) err() error {
	for _, err := range k {
		if err != nil {
			return k
		}
	}
	return nil
}

// ParseKRL parses a KRL. If the KRL was signed by one or more authorities,
// those signatures will be checked, and any verification errors will be
// returned.
func ParseKRL(in []byte) (*KRL, error) {
	orig := in

	var header krlHeader
	if err := ssh.Unmarshal(in, &header); err != nil {
		return nil, fmt.Errorf("krl: while parsing header: %v", err)
	}
	if header.KRLMagic != krlMagic {
		return nil, fmt.Errorf("krl: bad magic value %x", header.KRLMagic)
	}
	if header.KRLFormatVersion != 1 {
		return nil, fmt.Errorf("krl: bad format version %v", header.KRLFormatVersion)
	}
	krl := &KRL{
		Version:       header.KRLVersion,
		GeneratedDate: header.GeneratedDate,
		Comment:       header.Comment,
	}
	in = header.Rest

	for len(in) > 0 && in[0] != 4 { // // KRL_SECTION_SIGNATURE
		var sdata krlSection
		if err := ssh.Unmarshal(in, &sdata); err != nil {
			return nil, fmt.Errorf("krl: malformed section: %v", err)
		}
		in = sdata.Rest

		var err error
		var section KRLSection
		switch sdata.SectionType {
		case 1: // KRL_SECTION_CERTIFICATES
			section, err = parseCertificateSection(sdata.SectionData)
		case 2: // KRL_SECTION_EXPLICIT_KEY
			section, err = parseExplicitKeySection(sdata.SectionData)
		case 3: // KRL_SECTION_FINGERPRINT_SHA1
			section, err = parseFingerprintSection(sdata.SectionData)
		default:
			return nil, fmt.Errorf("krl: unexpected section type %d", sdata.SectionType)
		}
		if err != nil {
			return nil, err
		}
		krl.Sections = append(krl.Sections, section)
	}

	var signingErrors KRLSigningErrors
	for len(in) > 0 {
		var sigHeader krlSignatureHeader
		if err := ssh.Unmarshal(in, &sigHeader); err != nil {
			return nil, fmt.Errorf("krl: malfored signature header: %v", err)
		}
		in = sigHeader.Rest

		key, err := ssh.ParsePublicKey(sigHeader.SignatureKey)
		if err != nil {
			return nil, fmt.Errorf("krl: malformed signing key: %v", err)
		}

		var sig krlSignature
		if err := ssh.Unmarshal(in, &sig); err != nil {
			return nil, fmt.Errorf("krl: malfored signature wrapper: %v", err)
		}
		in = sig.Rest

		sshsig := new(ssh.Signature)
		if err := ssh.Unmarshal(sig.Signature, sshsig); err != nil {
			return nil, fmt.Errorf("krl: malformed signature: %v", err)
		}

		// The entire KRL up until immediately after the signature
		// header is signed.
		data := orig[:len(orig)-len(sigHeader.Rest)]

		krl.SigningKeys = append(krl.SigningKeys, key)
		signingErrors = append(signingErrors, key.Verify(data, sshsig))
	}

	return krl, signingErrors.err()
}

func parseCertificateSection(in []byte) (*KRLCertificateSection, error) {
	var header krlCertificateSectionHeader
	if err := ssh.Unmarshal(in, &header); err != nil {
		return nil, fmt.Errorf("krl: while parsing certificate section header: %v", err)
	}
	ca, err := ssh.ParsePublicKey(header.CAKey)
	if err != nil {
		return nil, fmt.Errorf("krl: while parsing CA key: %v", err)
	}
	k := &KRLCertificateSection{CA: ca}
	in = header.Rest
	for len(in) > 0 {
		var section krlCertificateSection
		if err := ssh.Unmarshal(in, &section); err != nil {
			return nil, fmt.Errorf("krl: malformed certificate section: %v", err)
		}
		in = section.Rest
		var err error
		var subsection KRLCertificateSubsection
		switch section.CertSectionType {
		case krlSectionCertSerialList:
			subsection, err = parseCertSerialList(section.CertSectionData)
		case krlSectionCertSerialRange:
			subsection, err = parseCertSerialRange(section.CertSectionData)
		case krlSectionCertSerialBitmap:
			subsection, err = parseCertSerialBitmap(section.CertSectionData)
		case krlSectionCertKeyId:
			subsection, err = parseCertKeyID(section.CertSectionData)
		default:
			return nil, fmt.Errorf("krl: unexpected cert section type %x", in[0])
		}
		if err != nil {
			return nil, err
		}
		k.Sections = append(k.Sections, subsection)
	}

	return k, nil
}

func parseCertSerialList(in []byte) (*KRLCertificateSerialList, error) {
	s := &KRLCertificateSerialList{}
	for len(in) > 0 {
		var list krlSerialList
		if err := ssh.Unmarshal(in, &list); err != nil {
			return nil, fmt.Errorf("krl: while parsing serial in list: %v", err)
		}
		in = list.Rest
		*s = append(*s, list.RevokedCertSerial)
	}
	return s, nil
}

func parseCertSerialRange(in []byte) (*KRLCertificateSerialRange, error) {
	var s krlSerialRange
	if err := ssh.Unmarshal(in, &s); err != nil {
		return nil, fmt.Errorf("krl: while parsing serial range: %v", err)
	}
	return &KRLCertificateSerialRange{
		Min: s.SerialMin,
		Max: s.SerialMax,
	}, nil
}

func parseCertSerialBitmap(in []byte) (*KRLCertificateSerialBitmap, error) {
	var s krlSerialBitmap
	if err := ssh.Unmarshal(in, &s); err != nil {
		return nil, fmt.Errorf("krl: while parsing serial bitmap: %v", err)
	}
	if bl := s.RevokedKeysBitmap.BitLen(); bl > krlMaxBitmapSize {
		return nil, fmt.Errorf("krl: serial bitmap too wide: %v", bl)
	}
	return &KRLCertificateSerialBitmap{
		Offset: s.SerialOffset,
		Bitmap: s.RevokedKeysBitmap,
	}, nil
}

func parseCertKeyID(in []byte) (*KRLCertificateKeyID, error) {
	s := &KRLCertificateKeyID{}
	for len(in) > 0 {
		var list krlKeyID
		if err := ssh.Unmarshal(in, &list); err != nil {
			return nil, fmt.Errorf("krl: while parsing key id in list: %v", err)
		}
		in = list.Rest
		*s = append(*s, list.KeyID)
	}
	return s, nil
}

func parseExplicitKeySection(in []byte) (*KRLExplicitKeySection, error) {
	s := &KRLExplicitKeySection{}
	for len(in) > 0 {
		var list krlExplicitKey
		if err := ssh.Unmarshal(in, &list); err != nil {
			return nil, fmt.Errorf("krl: while parsing explicit key in list: %v", err)
		}
		in = list.Rest
		key, err := ssh.ParsePublicKey(list.PublicKeyBlob)
		if err != nil {
			return nil, fmt.Errorf("krl: while parsing explicit key: %v", err)
		}
		*s = append(*s, key)
	}
	return s, nil
}

func parseFingerprintSection(in []byte) (*KRLFingerprintSection, error) {
	s := &KRLFingerprintSection{}
	for len(in) > 0 {
		var list krlFingerprintSHA1
		if err := ssh.Unmarshal(in, &list); err != nil {
			return nil, fmt.Errorf("krl: while parsing fingerprint in list: %v", err)
		}
		in = list.Rest
		if len(list.PublicKeyHash) != sha1.Size {
			return nil, fmt.Errorf("krl: key fingerprint wrong length for SHA1: %x", list.PublicKeyHash)
		}
		var sha [sha1.Size]byte
		copy(sha[:], list.PublicKeyHash)
		*s = append(*s, sha)
	}
	return s, nil
}
