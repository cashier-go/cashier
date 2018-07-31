package main

import (
	"context"
	"log"
	"os"

	"github.com/gobuffalo/packr/builder"
)

func main() {
	b := builder.New(context.Background(), os.Args[1])
	b.Compress = true
	if err := b.Run(); err != nil {
		log.Fatal(err)
	}
}
