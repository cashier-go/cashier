package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

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
		fmt.Println(lib.Version)
		return
	}
	conf, err := config.ReadConfig(*cfg)
	if err != nil {
		log.Fatalln(err)
	}
	if err := run(conf); err != nil {
		log.Fatalln("Forced shutdown: ", err)
	}
}

func run(conf *config.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

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

	gracePeriod, err := time.ParseDuration(conf.Server.ShutdownTimeout)
	if err != nil {
		log.Printf("Unable to parse ShutdownTimeout value %s: %v", conf.Server.ShutdownTimeout, err)
	}

	var s *http.Server
	started := make(chan struct{}, 1)
	go func() {
		s = server.Run(conf)
		close(started)
	}()
	<-started

	// wait for a signal
	<-ctx.Done()
	stop()
	log.Printf("shutting down in %d seconds\n", int64(gracePeriod.Seconds()))

	ctx, cancel := context.WithTimeout(context.Background(), gracePeriod)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		return fmt.Errorf("error when shutting down: %w", err)
	}
	return nil
}
