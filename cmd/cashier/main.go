package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/user"
	"path"
	"time"

	"github.com/nsheridan/cashier/client"
	"github.com/pkg/browser"
	"github.com/spf13/pflag"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

var (
	u, _             = user.Current()
	cfg              = pflag.String("config", path.Join(u.HomeDir, ".cashier.conf"), "Path to config file")
	ca               = pflag.String("ca", "http://localhost:10000", "CA server")
	keysize          = pflag.Int("key_size", 0, "Size of key to generate. Ignored for ed25519 keys. (default 2048 for rsa keys, 256 for ecdsa keys)")
	validity         = pflag.Duration("validity", time.Hour*24, "Key lifetime. May be overridden by the CA at signing time")
	keytype          = pflag.String("key_type", "", "Type of private key to generate - rsa, ecdsa or ed25519. (default \"rsa\")")
	publicFilePrefix = pflag.String("key_file_prefix", "", "Prefix for filename for public key and cert (optional, no default)")
	useGRPC          = pflag.Bool("use_grpc", false, "Use grpc (experimental)")
)

func main() {
	pflag.Parse()
	log.SetPrefix("cashier: ")
	log.SetFlags(0)
	var err error

	c, err := client.ReadConfig(*cfg)
	if err != nil {
		log.Printf("Error parsing config file: %v\n", err)
	}
	fmt.Printf("Your browser has been opened to visit %s\n", c.CA)
	if err := browser.OpenURL(c.CA); err != nil {
		fmt.Println("Error launching web browser. Go to the link in your web browser")
	}
	fmt.Println("Generating new key pair")
	priv, pub, err := client.GenerateKey(client.KeyType(c.Keytype), client.KeySize(c.Keysize))
	if err != nil {
		log.Fatalln("Error generating key pair: ", err)
	}

	fmt.Print("Enter token: ")
	var token string
	fmt.Scanln(&token)

	var cert *ssh.Certificate
	if *useGRPC {
		cert, err = client.RPCSign(pub, token, c)
	} else {
		cert, err = client.Sign(pub, token, c)
	}
	if err != nil {
		log.Fatalln(err)
	}
	sock, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		log.Fatalf("Error connecting to agent: %v\n", err)
	}
	defer sock.Close()
	a := agent.NewClient(sock)
	if err := client.InstallCert(a, cert, priv); err != nil {
		log.Fatalln(err)
	}
	if err := client.SavePublicFiles(c.PublicFilePrefix, cert, pub); err != nil {
		log.Fatalln(err)
	}
	if err := client.SavePrivateFiles(c.PublicFilePrefix, cert, priv); err != nil {
		log.Fatalln(err)
	}
	fmt.Println("Credentials added.")
}
