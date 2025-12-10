package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var extraMimes = []string{
	`"image/heic"`,
	`"image/heif"`,
	`"image/avif"`,
	`"image/jxl"`,
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
		fmt.Println("read err", err)
		return
	}
	out, changed := patch(data)
	if changed {
		os.WriteFile(path, out, 0644)
		fmt.Println("patched", path)
	}
}

func patchGzip(path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("read gz err", err)
		return
	}
	r, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		fmt.Println("gz open err", err)
		return
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
	}
}

func patch(data []byte) ([]byte, bool) {
	s := string(data)

	idx := strings.Index(s, `"image/jpeg"`)
	if idx < 0 {
		return data, false
	}

	// find start of the array by scanning backwards for [
	start := strings.LastIndex(s[:idx], "[")
	if start < 0 {
		return data, false
	}

	// find end of the array by scanning forward for ]
	end := strings.Index(s[idx:], "]")
	if end < 0 {
		return data, false
	}
	end = idx + end

	array := s[start : end+1]

	// avoid duplicates
	updated := array
	for _, m := range extraMimes {
		if !strings.Contains(updated, m) {
			updated = updated[:len(updated)-1] + "," + m + "]"
		}
	}

	if updated == array {
		return data, false
	}

	result := s[:start] + updated + s[end+1:]
	return []byte(result), true
}

func fatal(format string, v ...interface{}) {
	fmt.Printf("abort: "+format+"\n", v...)
	os.Exit(1)
}
