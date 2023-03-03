package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"
)

const (
	dateFormat     = "20060102150405"
	migrationsPath = "server/store/migrations"
)

var (
	contents = []byte(`-- +migrate Up


-- +migrate Down`)
)

func main() {
	flag.Usage = func() {
		fmt.Println("Usage: migration <migration name>")
	}
	flag.Parse()
	if len(flag.Args()) != 1 {
		flag.Usage()
	}
	name := fmt.Sprintf("%s_%s.sql", time.Now().UTC().Format(dateFormat), flag.Arg(0))
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	gitRoot, err := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		log.Fatal(err)
	}
	root := strings.TrimSpace(string(gitRoot))
	ents, err := os.ReadDir(path.Join(root, migrationsPath))
	if err != nil {
		log.Fatal(err)
	}
	for _, e := range ents {
		if e.IsDir() {
			filename := path.Join(migrationsPath, e.Name(), name)
			fmt.Printf("Wrote empty migration file: %s\n", filename)
			if err := os.WriteFile(filename, contents, 0644); err != nil {
				log.Fatal(err)
			}
		}
	}
}
