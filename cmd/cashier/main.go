package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"os"
	"os/user"
	"path"
	"time"

	"github.com/cashier-go/cashier/client"
	"github.com/cashier-go/cashier/lib"
	"github.com/pkg/browser"
	"github.com/spf13/pflag"
	"golang.org/x/crypto/ssh/agent"
)

var (
	u, _    = user.Current()
	cfg     = pflag.String("config", path.Join(u.HomeDir, ".cashier.conf"), "Path to config file")
	_       = pflag.String("ca", "http://localhost:10000", "CA server")
	_       = pflag.Int("key_size", 0, "Size of key to generate. Ignored for ed25519 keys. (default 2048 for rsa keys, 256 for ecdsa keys)")
	_       = pflag.Duration("validity", time.Hour*24, "Key lifetime. May be overridden by the CA at signing time")
	_       = pflag.String("key_type", "", "Type of private key to generate - rsa, ecdsa or ed25519. (default \"rsa\")")
	_       = pflag.String("key_file_prefix", "", "Prefix for filename for public key and cert (optional, no default)")
	version = pflag.Bool("version", false, "Print version and exit")
)

func main() {
	pflag.Parse()
	if *version {
		fmt.Printf("%s\n", lib.Version)
		os.Exit(0)
	}
	log.SetFlags(0)
	var err error

	c, err := client.ReadConfig(*cfg)
	if err != nil {
		log.Printf("Configuration error: %v\n", err)
	}
	log.Println("Generating new key pair")
	priv, pub, err := client.GenerateKey(client.KeyType(c.Keytype), client.KeySize(c.Keysize))
	if err != nil {
		log.Fatalln("Error generating key pair: ", err)
	}

	// local server to receive the token
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	srv := startServer(c.CA)
	defer srv.stop(ctx)

	url := fmt.Sprintf("%s?localserver=%s", c.CA, srv.url())

	log.Println("Your browser has been opened to visit", url)
	if err = browser.OpenURL(url); err != nil {
		log.Println("Error launching web browser. Go to the link in your web browser")
	}

	var encodedToken string
	go func() {
		fmt.Println("Enter token, followed by a '.' on a new line: ")
		scanner := bufio.NewScanner(os.Stdin)
		var buffer bytes.Buffer
		for scanner.Scan(); scanner.Text() != "."; scanner.Scan() {
			buffer.WriteString(scanner.Text())
		}
		encodedToken = buffer.String()
		cancel()
	}()

	select {
	case encodedToken = <-srv.token:
		// got a token on the http listener
		log.Println("Token received")
	case <-ctx.Done():
		// got a pasted token
	}

	token, err := base64.StdEncoding.DecodeString(encodedToken)
	if err != nil {
		log.Fatalln("Error decoding token:", err)
	}
	log.Println("Sending keys for signing... ")

	cert, err := client.Sign(pub, string(token), c)
	if err != nil {
		srv.respond(srvError)
		log.Fatalln(err)
	}
	sock, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		srv.respond(srvError)
		log.Fatalf("Error connecting to agent: %v\n", err)
	}
	defer sock.Close()
	a := agent.NewClient(sock)
	if err = client.InstallCert(a, cert, priv, c.CA); err != nil {
		srv.respond(srvError)
		log.Fatalln(err)
	}
	// If we got this far then the creds are installed and ready to use.
	log.Println("Credentials added to agent.")
	srv.respond(srvOK)

	if err := client.SavePublicFiles(c.PublicFilePrefix, cert, pub); err != nil {
		log.Fatalln(err)
	}
	if err := client.SavePrivateFiles(c.PublicFilePrefix, cert, priv); err != nil {
		log.Fatalln(err)
	}
}
