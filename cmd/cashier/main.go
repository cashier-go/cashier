package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/user"
	"path"
	"time"

	"github.com/nsheridan/cashier/lib"
	"github.com/pkg/browser"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

var (
	u, _ = user.Current()
	cfg  = flag.String("config", path.Join(u.HomeDir, ".cashier.cfg"), "Path to config file")
)

func installCert(a agent.Agent, cert *ssh.Certificate, key key) error {
	pubcert := agent.AddedKey{
		PrivateKey:  key,
		Certificate: cert,
		Comment:     cert.KeyId,
	}
	if err := a.Add(pubcert); err != nil {
		return fmt.Errorf("error importing certificate: %s", err)
	}
	return nil
}

func send(s []byte, token, ca string, ValidateTLSCertificate bool) (*lib.SignResponse, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: !ValidateTLSCertificate},
	}
	client := &http.Client{Transport: transport}
	req, err := http.NewRequest("POST", ca+"/sign", bytes.NewReader(s))
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
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	c := &lib.SignResponse{}
	if err := json.Unmarshal(body, c); err != nil {
		return nil, err
	}
	return c, nil
}

func sign(pub ssh.PublicKey, token string, conf *config) (*ssh.Certificate, error) {
	validity, err := time.ParseDuration(conf.Validity)
	if err != nil {
		return nil, err
	}
	marshaled := ssh.MarshalAuthorizedKey(pub)
	marshaled = marshaled[:len(marshaled)-1]
	s, err := json.Marshal(&lib.SignRequest{
		Key:        string(marshaled),
		ValidUntil: time.Now().Add(validity),
	})
	if err != nil {
		return nil, err
	}
	resp, err := send(s, token, conf.CA, conf.ValidateTLSCertificate)
	if err != nil {
		return nil, err
	}
	if resp.Status != "ok" {
		return nil, fmt.Errorf("error: %s", resp.Response)
	}
	k, _, _, _, err := ssh.ParseAuthorizedKey([]byte(resp.Response))
	if err != nil {
		return nil, err
	}
	cert, ok := k.(*ssh.Certificate)
	if !ok {
		return nil, fmt.Errorf("did not receive a certificate from server")
	}
	return cert, nil
}

func main() {
	flag.Parse()
	c, err := readConfig(*cfg)
	if err != nil {
		log.Fatalf("Error parsing config file: %v\n", err)
	}
	fmt.Printf("Your browser has been opened to visit %s\n", c.CA)
	if err := browser.OpenURL(c.CA); err != nil {
		fmt.Println("Error launching web browser. Go to the link in your web browser")
	}
	fmt.Println("Generating new key pair")
	priv, pub, err := generateKey(c.Keytype, c.Keysize)
	if err != nil {
		log.Fatalln("Error generating key pair: ", err)
	}

	fmt.Print("Enter token: ")
	var token string
	fmt.Scanln(&token)

	cert, err := sign(pub, token, c)
	if err != nil {
		log.Fatalln(err)
	}
	sock, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		log.Fatalln("Error connecting to agent: %s", err)
	}
	defer sock.Close()
	a := agent.NewClient(sock)
	if err := installCert(a, cert, priv); err != nil {
		log.Fatalln(err)
	}
	fmt.Println("Certificate added.")
}
