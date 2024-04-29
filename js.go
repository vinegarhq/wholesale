//go:build js && wasm

package main

import (
	"bytes"
	"io"
	"log"
	"sync"
	"syscall/js"

	"github.com/dustin/go-humanize"
)

type logWriter struct {
	outputHTMLName string
}

type coordinator struct {
	bar     js.Value
	div     js.Value
	max     int
	current int
	m       sync.Mutex
}

type humanResources struct{}

func init() {
	log.SetOutput(&logWriter{outputHTMLName: "log"})
}

func usage() {
	log.Fatalf("%s\n%s", "usage: wholesale?guid=guid[&channel=channel][&type=binaryType]",
		"example: wholesale?guid=version-1870963560174427&type=WindowsStudio64")
}

func (l logWriter) Write(p []byte) (n int, err error) {
	doc := js.Global().Get("document")
	out := js.Global().Get("document").Call("getElementById", l.outputHTMLName)
	node := doc.Call("createTextNode", string(p))
	out.Call("appendChild", node)
	return len(p), nil
}

func (hr *humanResources) NewBar(max int, id string, w io.Writer) io.Writer {
	doc := js.Global().Get("document")

	bar := doc.Call("createElement", "progress")
	bar.Set("id", id)
	bar.Set("value", 0)
	bar.Set("max", uint64(max))

	label := doc.Call("createElement", "label")
	label.Call("appendChild", doc.Call("createTextNode",
		id+" ("+humanize.Bytes(uint64(max))+")"))

	div := doc.Call("createElement", "div")
	div.Set("className", "package")
	div.Call("appendChild", label)
	div.Call("appendChild", bar)

	js.Global().Get("packages").Call("appendChild", div)

	return io.MultiWriter(w, &coordinator{
		div:     div,
		bar:     bar,
		max:     max,
		current: 0,
	})
}

func (hr *humanResources) Stop() error {
	return nil
}

func (c *coordinator) Write(p []byte) (int, error) {
	n := len(p)
	c.current += n
	go func() {
		c.m.Lock()
		defer c.m.Unlock()
		// if c.current == c.max {
		// 	c.div.Call("remove")
		// 	c = nil
		// 	return
		// }
		c.bar.Set("value", c.current)
	}()
	return n, nil
}

func link(buf *bytes.Buffer, name string) {
	b := buf.Bytes()

	data := js.Global().Get("Uint8Array").New(len(b))
	js.CopyBytesToJS(data, b)
	blob := js.Global().Get("Blob").New(
		[]interface{}{data},
		map[string]interface{}{"type": "application/zip"},
	)

	doc := js.Global().Get("document")

	button := doc.Call("createElement", "button")
	button.Set("innerText", "Redownload "+name)

	url := js.Global().Get("URL").Call("createObjectURL", blob)
	link := doc.Call("createElement", "a")
	link.Set("href", url)
	link.Set("download", name)
	link.Call("appendChild", button)

	doc.Get("body").Call("appendChild", doc.Call("createElement", "hr"))
	js.Global().Get("document").Get("body").Call("appendChild", link)
}
