package server

import (
	"net/http"
	"os"
)

// neuteredFileSystem wraps an http.FileSystem to prevent directory listing.
type neuteredFileSystem struct {
	fs http.FileSystem
}

// Open implements the http.FileSystem interface.
// If the requested path is a directory, it returns os.ErrNotExist.
func (nfs neuteredFileSystem) Open(path string) (http.File, error) {
	f, err := nfs.fs.Open(path)
	if err != nil {
		return nil, err
	}

	s, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if s.IsDir() {
		// Close the directory and return an error pretending it doesn't exist
		f.Close()
		return nil, os.ErrNotExist
	}

	return f, nil
}
