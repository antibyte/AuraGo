//go:build ignore

package main

import (
	"io"
	"net/http"
	"os"
)

func main() {
	resp, err := http.Get("https://cdn.tailwindcss.com")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	f, err := os.Create("ui/tailwind.min.js")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	io.Copy(f, resp.Body)
}
