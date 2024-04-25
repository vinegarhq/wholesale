package main

import (
	"os"
	"archive/zip"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"path"
	"sort"
	"strings"
	"syscall/js"
	"golang.org/x/sync/errgroup"
	"time"

	"github.com/apprehensions/rbxbin"
	cs "github.com/apprehensions/rbxweb/clientsettings"
	_ "github.com/klauspost/compress/zip"
)

var (
	guid    string
	binType string
	channel string
)

func init() {

}

type BinaryAssembler struct {
	w *zip.Writer
	d rbxbin.Deployment
	m *rbxbin.Mirror
	c js.Value
}

type htmlLogWriter struct {
	outputHTMLName string
}

type htmlProgressWriter struct {
	bar js.Value
	max int64
	current int64
}

func newHtmlProgressWriter(max int64, id string) *htmlProgressWriter {
	doc := js.Global().Get("document")
	bar := doc.Call("createElement", "progress")
	bar.Set("id", id)
	bar.Set("class", "package_progress")
	bar.Set("value", 0)
	bar.Set("max", max)
	doc.Get("body").Call("appendChild", bar)

	return &htmlProgressWriter{
		bar: bar,
		max: max,
		current: 0,
	}
}

func (hpw *htmlProgressWriter) Write(p []byte) (int, error) {
	n := len(p)
	hpw.current += int64(n)
	go func() {
		if hpw.current == hpw.max {
			hpw.bar.Call("remove")
			hpw = nil
			return
		}
		hpw.bar.Set("value", hpw.current)
	}()
	return n, nil
}

func (h htmlLogWriter) Write(p []byte) (n int, err error) {
	doc := js.Global().Get("document")
	out := js.Global().Get("document").Call("getElementById", h.outputHTMLName)
	node := doc.Call("createTextNode", string(p))
	out.Call("appendChild", node)
	return len(p), nil
}

func main() {
	log.SetOutput(&htmlLogWriter{outputHTMLName: "log"})
	log.SetFlags(0)
	guid := flag.String("guid", "", "Roblox deployment GUID to retrieve")
	channel := flag.String("channel", "", "Roblox deployment channel for the GUID")
	bin := flag.String("type", "WindowsPlayer", "Roblox BinaryType for the GUID")
	flag.Parse()

	if len(os.Args) < 2 {
		log.Fatalf("%s\n%s", "usage: wholesale?guid=guid[&channel=channel][&type=binaryType]",
			"example: wholesale?guid=version-1870963560174427&type=WindowsStudio64")
	}

	var t cs.BinaryType
	switch *bin {
	case "WindowsPlayer":
		t = cs.WindowsPlayer
	case "WindowsStudio64":
		t = cs.WindowsStudio64
	default:
		log.Fatal("Unsupported binary type", binType,
			"must be either WindowsPlayer or WindowsStudio64")
	}

	d := rbxbin.Deployment{
		Type:    t,
		Channel: *channel,
		GUID:    *guid,
	}
	name := fmt.Sprintf("%s-%s-%s.zip", d.Channel, d.Type, d.GUID)

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	cacheFly := rbxbin.Mirrors[1]

	ps, err := cacheFly.GetPackages(d)
	if err != nil {
		log.Fatal(err)
	}

	sort.Slice(ps, func(i, j int) bool {
		return ps[i].ZipSize < ps[j].ZipSize
	})

	ba := BinaryAssembler{
		w: zw,
		d: d,
		m: &cacheFly,
	}

	cur := time.Now()

	if err := ba.Assemble(ps, &cacheFly); err != nil {
		log.Fatal(err)
	}

	if err := zw.Close(); err != nil {
		log.Fatal(err)
	}

	log.Printf("Took %s!", time.Now().Sub(cur))

	// what a mess
	b := buf.Bytes()
	data := js.Global().Get("Uint8Array").New(len(b))
	js.CopyBytesToJS(data, b)
	blob := js.Global().Get("Blob").New([]interface{}{data}, map[string]interface{}{"type": "application/zip"})
	url := js.Global().Get("URL").Call("createObjectURL", blob)
	link := js.Global().Get("document").Call("createElement", "a")
	link.Set("href", url)
	link.Set("download", name)
	button := js.Global().Get("document").Call("createElement", "button")
	button.Set("innerText", "Redownload "+name)
	link.Call("appendChild", button)
	js.Global().Get("document").Get("body").Call("appendChild", link)
}

type BinaryAssembleJob struct {
	Dir string
	Length int64
	Name string
	Data []byte
}

func (ba *BinaryAssembler) Assemble(pkgs []rbxbin.Package, mirror *rbxbin.Mirror) error {
	dirs := rbxbin.BinaryDirectories(ba.d.Type)
	eg := new(errgroup.Group)
	bag := new(errgroup.Group)
	baj := make(chan BinaryAssembleJob, len(pkgs))

	bag.Go(func() error {
		for j := range baj {
			if err := ba.HandleJob(&j); err != nil {
				return err
			}
		}
		return nil
	})

	for _, p := range pkgs {
		eg.Go(func() error {
		if p.Name == "RobloxPlayerLauncher.exe" {
			return nil
		}

		dir, ok := dirs[p.Name]
		if !ok {
			log.Fatalf("unhandled package %s, was the correct binary type set?", p.Name)
		}
		url := ba.m.PackageURL(ba.d, p.Name)
	
		resp, err := http.Get(url)
		if err != nil {
			return err
		}

		log.Println("Downloading", p.Name)

		if resp.ContentLength == 0 {
			return errors.New("ContentLength is unknown")
		}

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
			return fmt.Errorf("read: %w", err)
		}
		resp.Body.Close()

		baj <- BinaryAssembleJob{
			Dir: dir,
			Name: p.Name,
			Length: resp.ContentLength,
			Data: data,
		}
		return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	close(baj)

	if err := bag.Wait(); err != nil {
		return err
	}

	return nil
}

func (ba *BinaryAssembler) HandleJob(baj *BinaryAssembleJob) error {
	zr, err := zip.NewReader(bytes.NewReader(baj.Data), int64(len(baj.Data)))
	if err != nil {
		return fmt.Errorf("zip reader: %w", err)
	}

	log.Println("Extracting", baj.Name)

	for _, f := range zr.File {
		rc, err := f.OpenRaw()
		if err != nil {
			return fmt.Errorf("open: %w", err)
		}

		f.Name = path.Join(baj.Dir, strings.ReplaceAll(f.Name, `\`, "/"))
		if f.FileInfo().IsDir() {
			f.Name += "/"
		}

		r, err := ba.w.CreateRaw(&f.FileHeader)
		if err != nil {
			return fmt.Errorf("create dest: %w", err)
		}

		if _, err := io.Copy(r, rc); err != nil {
			return fmt.Errorf("copy dest: %w", err)
		}
	}

	return nil
}
