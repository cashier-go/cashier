package main

//go:generate go run static.go

import (
	"context"
	"log"
	"os/exec"
	"strings"

	"github.com/gobuffalo/packr/builder"
)

func main() {
	root, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		log.Fatal(err)
	}
	b := builder.New(context.Background(), strings.TrimSpace(string(root)))
	b.Compress = true
	if err := b.Run(); err != nil {
		log.Fatal(err)
	}
}
