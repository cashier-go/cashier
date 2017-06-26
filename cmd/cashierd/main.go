package main

import (
	"flag"
	"log"
	"math/rand"
	"time"

	"github.com/nsheridan/cashier/server"
	"github.com/nsheridan/cashier/server/config"
	"github.com/nsheridan/cashier/server/wkfs/vaultfs"
	"github.com/nsheridan/wkfs/s3"
)

var (
	cfg = flag.String("config_file", "cashierd.conf", "Path to configuration file.")
)

func main() {
	flag.Parse()
	conf, err := config.ReadConfig(*cfg)
	if err != nil {
		log.Fatal(err)
	}

	// Register well-known filesystems.
	if conf.AWS == nil {
		conf.AWS = &config.AWS{}
	}
	s3.Register(&s3.Options{
		Region:    conf.AWS.Region,
		AccessKey: conf.AWS.AccessKey,
		SecretKey: conf.AWS.SecretKey,
	})
	vaultfs.Register(conf.Vault)

	// Ensure that RNG is seeded
	rand.Seed(time.Now().UnixNano())

	// Start the servers
	server.Run(conf)
}
