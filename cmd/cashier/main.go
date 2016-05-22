package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/nsheridan/cashier/lib"
	"github.com/pkg/browser"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

var (
	ca       = flag.String("ca", "http://localhost:10000", "CA server")
	keybits  = flag.Int("bits", 2048, "Key size. Ignored for ed25519 keys")
	validity = flag.Duration("validity", time.Hour*24, "Key validity")
	keytype  = flag.String("key_type", "rsa", "Type of private key to generate - rsa, ecdsa or ed25519")
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

func send(s []byte, token string) (*lib.SignResponse, error) {
	req, err := http.NewRequest("POST", *ca+"/sign", bytes.NewReader(s))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	client := &http.Client{}
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

func sign(pub ssh.PublicKey, token string) (*ssh.Certificate, error) {
	marshaled := ssh.MarshalAuthorizedKey(pub)
	marshaled = marshaled[:len(marshaled)-1]
	s, err := json.Marshal(&lib.SignRequest{
		Key:        string(marshaled),
		ValidUntil: time.Now().Add(*validity),
	})
	if err != nil {
		return nil, err
	}
	resp, err := send(s, token)
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

	fmt.Printf("Your browser has been opened to visit %s\n", *ca)
	if err := browser.OpenURL(*ca); err != nil {
		fmt.Println("Error launching web browser. Go to the link in your web browser")
	}
	fmt.Println("Generating new key pair")
	priv, pub, err := generateKey(*keytype, *keybits)
	if err != nil {
		log.Fatalln("Error generating key pair: ", err)
	}

	fmt.Print("Enter token: ")
	var token string
	fmt.Scanln(&token)

	cert, err := sign(pub, token)
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
