//go:build ignore

// Reads source PNGs without changing pixels; prints atlas crop metadata.
package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

type metadata struct {
	Width      int          `json:"width"`
	Height     int          `json:"height"`
	Frames     [][4]int     `json:"frames"`
	Anchors    [][2]float64 `json:"anchors"`
	SourceSize int          `json:"sourceSize"`
	EdgePixels []int        `json:"edgePixels"`
}

func cuts(im image.Image, count int, horizontal bool) []int {
	size, other := im.Bounds().Dx(), im.Bounds().Dy()
	if !horizontal {
		size, other = other, size
	}
	out := []int{0}
	for i := 1; i < count; i++ {
		center, radius := size*i/count, size/count/8
		best, cost := center, int64(1<<62)
		for pos := center - radius; pos <= center+radius; pos++ {
			var score int64
			for j := 0; j < other; j++ {
				x, y := pos, j
				if !horizontal {
					x, y = j, pos
				}
				_, _, _, a := im.At(x, y).RGBA()
				if a >= 8192 {
					score += int64(a)
				}
			}
			d := pos - center
			if d < 0 {
				d = -d
			}
			score = score*int64(size) + int64(d)
			if score < cost {
				best, cost = pos, score
			}
		}
		out = append(out, best)
	}
	return append(out, size)
}
func read(path string) (metadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return metadata{}, err
	}
	defer f.Close()
	im, err := png.Decode(f)
	if err != nil {
		return metadata{}, err
	}
	name := filepath.Base(path)
	cols, rows := 4, 4
	if strings.HasPrefix(name, "exp-planet-") {
		cols, rows = 2, 2
	} else if strings.HasPrefix(name, "exp-effect-") || strings.HasPrefix(name, "exp-weapon-") {
		rows = 3
		if strings.Contains(name, "weapon-seeker") || strings.Contains(name, "weapon-swarm") {
			rows = 2
		}
	}
	xs, ys := cuts(im, cols, true), cuts(im, rows, false)
	m := metadata{Width: im.Bounds().Dx(), Height: im.Bounds().Dy()}
	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			x, y, w, h := xs[col], ys[row], xs[col+1]-xs[col], ys[row+1]-ys[row]
			minX, minY, maxX, maxY := w, h, 0, 0
			edge := 0
			for yy := 0; yy < h; yy++ {
				for xx := 0; xx < w; xx++ {
					_, _, _, a := im.At(x+xx, y+yy).RGBA()
					if a < 8192 {
						continue
					}
					if xx < minX {
						minX = xx
					}
					if yy < minY {
						minY = yy
					}
					if xx > maxX {
						maxX = xx
					}
					if yy > maxY {
						maxY = yy
					}
					if xx == 0 || yy == 0 || xx == w-1 || yy == h-1 {
						edge++
					}
				}
			}
			if maxX <= minX || maxY <= minY {
				return m, fmt.Errorf("empty frame %s %d", name, len(m.Frames))
			}
			m.Frames = append(m.Frames, [4]int{x, y, w, h})
			m.Anchors = append(m.Anchors, [2]float64{float64(minX+maxX+1) / 2, float64(minY+maxY+1) / 2})
			m.SourceSize = max(m.SourceSize, max(maxX-minX+1, maxY-minY+1))
			m.EdgePixels = append(m.EdgePixels, edge)
		}
	}
	return m, nil
}
func main() {
	files, err := filepath.Glob("ui/img/galaxa/exp-*.png")
	if err != nil {
		panic(err)
	}
	out := map[string]metadata{}
	for _, p := range files {
		m, e := read(p)
		if e != nil {
			panic(e)
		}
		out[filepath.Base(p)] = m
	}
	if err = json.NewEncoder(os.Stdout).Encode(out); err != nil {
		panic(err)
	}
}
