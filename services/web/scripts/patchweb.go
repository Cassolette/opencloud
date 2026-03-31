package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

var extraMimes = [][]byte{
	[]byte("image/jxl"),
	[]byte("image/heic"),
	[]byte("image/heif"),
	[]byte("image/avif"),
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
	out, changed, err := patch(data)
	if err != nil {
		fatal("patch err %s", err)
	}
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

	out, changed, err := patch(data)
	if err != nil {
		fatal("patch err %s", err)
	}
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

var mimeArrayRegex = regexp.MustCompile(
	`\[[` + "`" + `"'][a-zA-Z0-9"'\x60/\-+.,\s]*[` + "`" + `"']\]`,
)
var mimeArrayAnchor = []byte("image/jpeg")

func findMimeArray(data []byte) (start int, end int, quoteUsed byte, err error) {
	allMatches := mimeArrayRegex.FindAllIndex(data, -1)

	start, end = -1, -1
	for _, loc := range allMatches {
		chunk := data[loc[0]+1 : loc[1]-1]
		if bytes.Contains(chunk, mimeArrayAnchor) {
			start, end = loc[0]+1, loc[1]-1
			break
		}
	}
	if start < 0 {
		return -1, -1, 0, fmt.Errorf("could not find anchor")
	}

	// determine quote style from first quote char found in array
	quoteUsed = byte('"')
	for _, b := range data[start:end] {
		if b == '"' || b == '\'' || b == '`' {
			quoteUsed = b
			break
		}
	}

	return
}

func patch(data []byte) (patched []byte, changed bool, err error) {
	start, end, quoteUsed, err := findMimeArray(data)
	if err != nil {
		fatal("match err %s", err)
	}

	array := data[start:end]

	var mimesToAdd [][]byte
	for _, m := range extraMimes {
		formatted := append([]byte{quoteUsed}, append(m, quoteUsed)...)
		// strip duplicates
		if !bytes.Contains(array, formatted) {
			mimesToAdd = append(mimesToAdd, formatted)
		}
	}
	if len(mimesToAdd) == 0 {
		return data, false, nil
	}

	sep := []byte{','}
	payloadToInject := append(sep, bytes.Join(mimesToAdd, sep)...)

	// construct [start...beforeEnd] + payload + [end...]
	var buf bytes.Buffer
	buf.Grow(len(data) + len(payloadToInject))
	buf.Write(data[:end])
	buf.Write(payloadToInject)
	buf.Write(data[end:])

	return buf.Bytes(), true, nil
}

func fatal(format string, v ...interface{}) {
	fmt.Printf("abort: "+format+"\n", v...)
	os.Exit(1)
}
