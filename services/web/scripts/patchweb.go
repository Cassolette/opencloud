package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var extraMimes = [][]byte{
	[]byte(`"image/jxl"`),
	[]byte(`"image/heic"`),
	[]byte(`"image/heif"`),
	[]byte(`"image/avif"`),
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: patchweb.go <dir_with_js_files>")
		return
	}
	root := os.Args[1]

	pattern := filepath.Join(root, "web-app-preview-*.mjs")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		fatal("Error globbing files: %v", err)
	}
	if len(matches) == 0 {
		fatal("No files found matching: %s", pattern)
	}
	fmt.Println(">> Extra mimetypes to be patched in: " + string(bytes.Join(extraMimes, []byte(" "))))
	for _, f := range matches {
		fmt.Println(">> Patching: " + f)
		patchFile(f)
		gzf := f + ".gz"

		_, err := os.Stat(gzf)
		if err != nil {
			fmt.Println("Couldn't find gunzip equivalent. Skipping: " + gzf)
			continue
		}
		fmt.Println(">> Patching GZ: " + gzf)
		patchGzip(gzf)
	}
}

func patchFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fatal("read err %s", err)
	}
	out, changed := patch(data)
	if changed {
		os.WriteFile(path, out, 0644)
		fmt.Println("patched", path)
	} else {
		fmt.Println("already patched. skipped.")
	}
}

func patchGzip(path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		fatal("read gz err %s", err)
	}
	r, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		fatal("read gz err %s", err)
	}
	data, _ := io.ReadAll(r)
	r.Close()

	out, changed := patch(data)
	if changed {
		var buf bytes.Buffer
		w := gzip.NewWriter(&buf)
		w.Write(out)
		w.Close()
		os.WriteFile(path, buf.Bytes(), 0644)
		fmt.Println("patched", path)
	} else {
		fmt.Println("already patched. skipped.")
	}
}

func patch(data []byte) ([]byte, bool) {
	anchor := []byte(`"image/jpeg"`)
	idx := bytes.Index(data, anchor)
	if idx < 0 {
		return data, false
	}

	// find start of the array by scanning backwards for [
	start := bytes.LastIndexByte(data[:idx], '[')
	if start < 0 {
		return data, false
	}

	// find end of the array by scanning forward for ]
	endRel := bytes.IndexByte(data[idx:], ']')
	if endRel < 0 {
		return data, false
	}
	end := idx + endRel

	array := data[start : end+1]

	var mimesToAdd [][]byte
	for _, m := range extraMimes {
		// strip duplicates
		if !bytes.Contains(array, m) {
			mimesToAdd = append(mimesToAdd, m)
		}
	}
	if len(mimesToAdd) == 0 {
		return data, false
	}

	sep := []byte{','}
	payloadToInject := append(sep, bytes.Join(mimesToAdd, sep)...)

	// construct [start...beforeEnd] + payload + [end...]
	var buf bytes.Buffer
	buf.Grow(len(data) + len(payloadToInject))
	buf.Write(data[:end])
	buf.Write(payloadToInject)
	buf.Write(data[end:])

	return buf.Bytes(), true
}

func fatal(format string, v ...interface{}) {
	fmt.Printf("abort: "+format+"\n", v...)
	os.Exit(1)
}
