package server

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"

	"github.com/pkg/errors"

	"go4.org/wkfs"
	"golang.org/x/crypto/acme/autocert"

	wkfscache "github.com/nsheridan/autocert-wkfs-cache"
	"github.com/nsheridan/cashier/server/auth"
	"github.com/nsheridan/cashier/server/auth/github"
	"github.com/nsheridan/cashier/server/auth/gitlab"
	"github.com/nsheridan/cashier/server/auth/google"
	"github.com/nsheridan/cashier/server/auth/microsoft"
	"github.com/nsheridan/cashier/server/config"
	"github.com/nsheridan/cashier/server/metrics"
	"github.com/nsheridan/cashier/server/signer"
	"github.com/nsheridan/cashier/server/store"
	"github.com/sid77/drop"
)

var (
	authprovider auth.Provider
	certstore    store.CertStorer
	keysigner    *signer.KeySigner
)

func loadCerts(certFile, keyFile string) (tls.Certificate, error) {
	key, err := wkfs.ReadFile(keyFile)
	if err != nil {
		return tls.Certificate{}, errors.Wrap(err, "error reading TLS private key")
	}
	cert, err := wkfs.ReadFile(certFile)
	if err != nil {
		return tls.Certificate{}, errors.Wrap(err, "error reading TLS certificate")
	}
	return tls.X509KeyPair(cert, key)
}

// Run the HTTP and RPC servers.
func Run(conf *config.Config) {
	var err error
	keysigner, err = signer.New(conf.SSH)
	if err != nil {
		log.Fatal(err)
	}

	laddr := fmt.Sprintf("%s:%d", conf.Server.Addr, conf.Server.Port)
	l, err := net.Listen("tcp", laddr)
	if err != nil {
		log.Fatal(errors.Wrapf(err, "unable to listen on %s:%d", conf.Server.Addr, conf.Server.Port))
	}

	tlsConfig := &tls.Config{}
	if conf.Server.UseTLS {
		if conf.Server.LetsEncryptServername != "" {
			m := autocert.Manager{
				Prompt:     autocert.AcceptTOS,
				Cache:      wkfscache.Cache(conf.Server.LetsEncryptCache),
				HostPolicy: autocert.HostWhitelist(conf.Server.LetsEncryptServername),
			}
			tlsConfig.GetCertificate = m.GetCertificate
		} else {
			if conf.Server.TLSCert == "" || conf.Server.TLSKey == "" {
				log.Fatal("TLS cert or key not specified in config")
			}
			tlsConfig.Certificates = make([]tls.Certificate, 1)
			tlsConfig.Certificates[0], err = loadCerts(conf.Server.TLSCert, conf.Server.TLSKey)
			if err != nil {
				log.Fatal(errors.Wrap(err, "unable to create TLS listener"))
			}
		}
		l = tls.NewListener(l, tlsConfig)
	}

	if conf.Server.User != "" {
		log.Print("Dropping privileges...")
		if err := drop.DropPrivileges(conf.Server.User); err != nil {
			log.Fatal(errors.Wrap(err, "unable to drop privileges"))
		}
	}

	// Unprivileged section
	metrics.Register()

	switch conf.Auth.Provider {
	case "github":
		authprovider, err = github.New(conf.Auth)
	case "gitlab":
		authprovider, err = gitlab.New(conf.Auth)
	case "google":
		authprovider, err = google.New(conf.Auth)
	case "microsoft":
		authprovider, err = microsoft.New(conf.Auth)
	default:
		log.Fatalf("Unknown provider %s\n", conf.Auth.Provider)
	}
	if err != nil {
		log.Fatal(errors.Wrapf(err, "unable to use provider '%s'", conf.Auth.Provider))
	}

	certstore, err = store.New(conf.Server.Database)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Starting server on %s", laddr)
	runHTTPServer(conf.Server, l)
}
