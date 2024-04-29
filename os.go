//go:build !js && !wasm

package main

import (
	"bytes"
	"io"
	"log"
	"os"

	"github.com/cheggaaa/pb/v3"
)

func usage() {
	log.Fatal("usage: wholesale -guid=guid [-channel channel] [-type binaryType]")
}

func link(buf *bytes.Buffer, name string) {
	err := os.WriteFile(name, buf.Bytes(), 0o644)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Exported at: %s", name)
}

type humanResources struct {
	*pb.Pool
}

func (hr *humanResources) NewBar(max int, name string, r io.Writer) io.Writer {
	bar := pb.Simple.New(max).Set("prefix", name)
	if hr.Pool != nil {
		hr.Pool.Add(bar)
	} else {
		pool, err := pb.StartPool(bar)
		if err != nil {
			log.Fatal(err)
		}
		hr.Pool = pool
	}
	return bar.NewProxyWriter(r)
}
