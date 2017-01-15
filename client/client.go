package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/nsheridan/cashier/lib"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// InstallCert adds the private key and signed certificate to the ssh agent.
func InstallCert(a agent.Agent, cert *ssh.Certificate, key Key) error {
	t := time.Unix(int64(cert.ValidBefore), 0)
	lifetime := t.Sub(time.Now()).Seconds()
	comment := fmt.Sprintf("%s [Expires %s]", cert.KeyId, t)
	pubcert := agent.AddedKey{
		PrivateKey:   key,
		Certificate:  cert,
		Comment:      comment,
		LifetimeSecs: uint32(lifetime),
	}
	if err := a.Add(pubcert); err != nil {
		return errors.Wrap(err, "unable to add cert to ssh agent")
	}
	privkey := agent.AddedKey{
		PrivateKey:   key,
		Comment:      comment,
		LifetimeSecs: uint32(lifetime),
	}
	if err := a.Add(privkey); err != nil {
		return errors.Wrap(err, "unable to add private key to ssh agent")
	}
	return nil
}

// send the signing request to the CA.
func send(s []byte, token, ca string, ValidateTLSCertificate bool) (*lib.SignResponse, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: !ValidateTLSCertificate},
	}
	client := &http.Client{Transport: transport}
	u, err := url.Parse(ca)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse CA url")
	}
	u.Path = path.Join(u.Path, "/sign")
	req, err := http.NewRequest("POST", u.String(), bytes.NewReader(s))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Bad response from server: %s", resp.Status)
	}
	defer resp.Body.Close()
	c := &lib.SignResponse{}
	if err := json.NewDecoder(resp.Body).Decode(c); err != nil {
		return nil, errors.Wrap(err, "unable to decode server response")
	}
	return c, nil
}

// Sign sends the public key to the CA to be signed.
func Sign(pub ssh.PublicKey, token string, conf *Config) (*ssh.Certificate, error) {
	validity, err := time.ParseDuration(conf.Validity)
	if err != nil {
		return nil, err
	}
	s, err := json.Marshal(&lib.SignRequest{
		Key:        lib.GetPublicKey(pub),
		ValidUntil: time.Now().Add(validity),
	})
	if err != nil {
		return nil, errors.Wrap(err, "unable to create sign request")
	}
	resp, err := send(s, token, conf.CA, conf.ValidateTLSCertificate)
	if err != nil {
		return nil, errors.Wrap(err, "error sending request to CA")
	}
	if resp.Status != "ok" {
		return nil, fmt.Errorf("bad response from CA: %s", resp.Response)
	}
	k, _, _, _, err := ssh.ParseAuthorizedKey([]byte(resp.Response))
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse response")
	}
	cert, ok := k.(*ssh.Certificate)
	if !ok {
		return nil, fmt.Errorf("did not receive a valid certificate from server")
	}
	return cert, nil
}
