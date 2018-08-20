package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/nsheridan/cashier/lib"
	"github.com/nsheridan/cashier/server"
	"github.com/nsheridan/cashier/server/config"
	"github.com/nsheridan/cashier/server/wkfs/vaultfs"
	"github.com/nsheridan/wkfs/s3"
)

var (
	cfg     = flag.String("config_file", "cashierd.conf", "Path to configuration file.")
	version = flag.Bool("version", false, "Print version and exit")
)

func main() {
	flag.Parse()
	if *version {
		fmt.Printf("%s\n", lib.Version)
		os.Exit(0)
	}
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

	// Start the server
	server.Run(conf)
}
