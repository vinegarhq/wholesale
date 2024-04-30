package factory

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/apprehensions/rbxbin"
)

var mirror = rbxbin.Mirrors[1] // cachefly

type Progress interface {
	NewBar(int, string, io.Writer) io.Writer
	Stop() error
}

type BinaryAssembler struct {
	d rbxbin.Deployment
	q bool
	p Progress
}

type Package struct {
	Dir    string
	Name   string
	Reader *bytes.Reader
}

func NewBinaryAssembler(d rbxbin.Deployment, q bool, p Progress) *BinaryAssembler {
	return &BinaryAssembler{d: d, q: q, p: p}
}

func (ba *BinaryAssembler) Run() *bytes.Buffer {
	pkgs, err := mirror.GetPackages(ba.d)
	if err != nil {
		log.Fatal(err)
	}

	sort.Slice(pkgs, func(i, j int) bool {
		return pkgs[i].ZipSize < pkgs[j].ZipSize
	})

	cur := time.Now()

	buf := ba.Assemble(pkgs)

	log.Printf("Took %s!", time.Now().Sub(cur))
	return buf
}

func (ba *BinaryAssembler) Assemble(pkgs []rbxbin.Package) *bytes.Buffer {
	// Errors in this function are unrecoverable.
	var eg, hg sync.WaitGroup
	h := make(chan *Package, len(pkgs))
	dirs := rbxbin.BinaryDirectories(ba.d.Type)

	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	// ZIP is designed to be written in sequential, so a handler
	// goroutine with a channel is made to add to the zip concurrently,
	// while downloading is done concurrently.
	go func() {
		hg.Add(1)
		for j := range h {
			err := ba.HandleJob(j, w)
			if err != nil {
				log.Fatal(err)
			}
		}
		hg.Done()
	}()

	eg.Add(len(pkgs))
	for _, p := range pkgs {
		go func() {
			defer eg.Done()
			if p.Name == "RobloxPlayerLauncher.exe" {
				return
			}

			dir, ok := dirs[p.Name]
			if !ok {
				log.Fatalf("unhandled package %s, was the correct binary type set?", p.Name)
			}

			j, err := ba.CreateJob(&p, dir)
			if err != nil {
				log.Fatal(err)
			}

			h <- j
		}()
	}

	eg.Wait()
	close(h)
	hg.Wait()

	if !ba.q {
		if err := ba.p.Stop(); err != nil {
			log.Fatal(err)
		}
	}

	as, err := w.CreateHeader(&zip.FileHeader{
		Name:     "AppSettings.xml",
		Method:   zip.Deflate,
		Modified: time.Now(),
	})
	if err != nil {
		log.Fatal(err)
	}

	if _, err := as.Write([]byte(rbxbin.AppSettings)); err != nil {
		log.Fatal(err)
	}

	if err := w.Close(); err != nil {
		log.Fatal(err)
	}

	return buf
}

func (ba *BinaryAssembler) CreateJob(pkg *rbxbin.Package, dir string) (*Package, error) {
	url := mirror.PackageURL(ba.d, pkg.Name)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	if resp.ContentLength < 0 {
		return nil, errors.New("source ContentLength missing")
	}

	b := new(bytes.Buffer)
	var r io.Writer
	if ba.q {
		r = b
	} else {
		r = ba.p.NewBar(int(resp.ContentLength), pkg.Name, b)
	}

	_, err = io.Copy(r, resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	return &Package{
		Dir:    dir,
		Name:   pkg.Name,
		Reader: bytes.NewReader(b.Bytes()),
	}, nil
}

func (ba *BinaryAssembler) HandleJob(pkg *Package, w *zip.Writer) error {
	zr, err := zip.NewReader(pkg.Reader, int64(pkg.Reader.Len()))
	if err != nil {
		return fmt.Errorf("zip reader: %w", err)
	}

	for _, f := range zr.File {
		rc, err := f.OpenRaw()
		if err != nil {
			return fmt.Errorf("open: %w", err)
		}

		f.Name = path.Join(pkg.Dir, strings.ReplaceAll(f.Name, `\`, "/"))
		if f.FileInfo().IsDir() {
			f.Name += "/"
		}

		r, err := w.CreateRaw(&f.FileHeader)
		if err != nil {
			return fmt.Errorf("create dest: %w", err)
		}

		if _, err := io.Copy(r, rc); err != nil {
			return fmt.Errorf("copy dest: %w", err)
		}
	}

	return nil
}
