package server

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"image"
	"image/draw"
	"image/png"
	"io"
	"math"
	"net/http"
	"os"
	"regexp"
	"strings"

	"aurago/internal/fileutil"
	"aurago/internal/qrcode"
	"aurago/internal/tools"
	xdraw "golang.org/x/image/draw"
)

var findItImageID = regexp.MustCompile(`^[a-f0-9]{32}$`)

func (s *Server) uploadFindItImage(w http.ResponseWriter, r *http.Request, runtime *findItRuntime) {
	release, ok := tools.AcquireQRWork()
	if !ok {
		findItHTTPError(w, 429, "image_busy")
		return
	}
	defer release()
	r.Body = http.MaxBytesReader(w, r.Body, 11<<20)
	if r.ParseMultipartForm(10<<20) != nil {
		findItHTTPError(w, 413, "image_too_large")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		findItHTTPError(w, 400, "image_required")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (10<<20)+1))
	if err != nil || len(data) > 10<<20 {
		findItHTTPError(w, 413, "image_too_large")
		return
	}
	mime := strings.TrimSpace(strings.Split(header.Header.Get("Content-Type"), ";")[0])
	if mime == "" || mime == "application/octet-stream" {
		mime = http.DetectContentType(data)
	}
	if mime != "image/jpeg" && mime != "image/png" && mime != "image/webp" {
		findItHTTPError(w, 415, "image_type")
		return
	}
	normalized, err := qrcode.NormalizeImage(r.Context(), mime, data)
	if err != nil {
		findItHTTPError(w, 422, "invalid_image")
		return
	}
	im, err := png.Decode(bytes.NewReader(normalized.Data))
	if err != nil {
		findItHTTPError(w, 422, "invalid_image")
		return
	}
	bounds := im.Bounds()
	scale := math.Min(1, math.Min(256/float64(bounds.Dx()), 256/float64(bounds.Dy())))
	thumb := image.NewNRGBA(image.Rect(0, 0, max(1, int(float64(bounds.Dx())*scale)), max(1, int(float64(bounds.Dy())*scale))))
	xdraw.ApproxBiLinear.Scale(thumb, thumb.Bounds(), im, bounds, draw.Src, nil)
	var encoded bytes.Buffer
	if png.Encode(&encoded, thumb) != nil {
		findItHTTPError(w, 503, "image_failed")
		return
	}
	var token [16]byte
	if _, err = rand.Read(token[:]); err != nil {
		findItHTTPError(w, 503, "image_failed")
		return
	}
	id := hex.EncodeToString(token[:])
	err = fileutil.PublishContext(r.Context(), func() error {
		root, err := os.OpenRoot(runtime.dataDir)
		if err != nil {
			return err
		}
		defer root.Close()
		if err = root.Mkdir("findit-images", 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
		info, err := root.Lstat("findit-images")
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("invalid image directory")
		}
		images, err := root.OpenRoot("findit-images")
		if err != nil {
			return err
		}
		defer images.Close()
		for name, content := range map[string][]byte{id + ".png": normalized.Data, id + "_thumb.png": encoded.Bytes()} {
			output, err := images.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err != nil {
				return err
			}
			_, writeErr := output.Write(content)
			if writeErr == nil {
				writeErr = output.Sync()
			}
			closeErr := output.Close()
			if writeErr != nil {
				return writeErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
		return nil
	})
	if err != nil {
		findItHTTPError(w, 503, "image_failed")
		return
	}
	findItJSON(w, 201, map[string]string{"id": id, "url": findItAPI + "/images/" + id, "thumbnail_url": findItAPI + "/images/" + id + "/thumbnail"})
}

func serveFindItImage(w http.ResponseWriter, r *http.Request, runtime *findItRuntime, reference string) {
	parts := strings.Split(reference, "/")
	if len(parts) > 2 || !findItImageID.MatchString(parts[0]) || len(parts) == 2 && parts[1] != "thumbnail" {
		findItHTTPError(w, 404, "not_found")
		return
	}
	name := parts[0]
	if len(parts) == 2 {
		name += "_thumb"
	}
	file, info, err := openRegularFileInRoot(runtime.dataDir, "findit-images/"+name+".png")
	if err != nil {
		findItHTTPError(w, 404, "not_found")
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	http.ServeContent(w, r, "image.png", info.ModTime(), file)
}
