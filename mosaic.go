package main

import (
	"fmt"
	"io/ioutil"
	"image"
	"os"
	"math"
	"image/color"
	"sync"
)

type DB struct {
	mutex *sync.Mutex
	store map[string][3]float64
}

func resize(img image.Image, newWidth int) *image.NRGBA {
	bounds := img.Bounds()
	width := bounds.Dx()
	ratio := width / newWidth
	output := image.NewNRGBA(image.Rect(bounds.Min.X / ratio, bounds.Min.Y / ratio, bounds.Max.X / ratio, bounds.Max.Y / ratio))

	for y, j := bounds.Min.Y, bounds.Min.Y; y < bounds.Max.Y; y, j = y + ratio, j + 1 {
		for x, i := bounds.Min.X, bounds.Min.X; x < bounds.Max.X; x, i = x + ratio, i + 1 {
			r, g, b, a := img.At(x, y).RGBA()
			output.SetNRGBA(i, j, color.NRGBA{uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8)})
		}
	}
	return output
}

func averageColor(img image.Image) [3]float64 {
	bounds := img.Bounds()
	r, g, b := 0.0, 0.0, 0.0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r1, g1, b1, _ := img.At(x, y).RGBA()
			r, g, b = r + float64(r1), g + float64(g1), b + float64(b1)
		}
	}
	totalPixels := float64((bounds.Max.X - bounds.Min.X) * (bounds.Max.Y - bounds.Min.Y))
	fmt.Println(totalPixels)
	return [3]float64{r / totalPixels, g / totalPixels, b / totalPixels}
}

func cloneTilesDB() *DB {
	db := make(map[string][3]float64)
	for k, v := range TILESDB {
		db[k] = v
	}

	tiles := &DB{
		store: db,
		mutex: &sync.Mutex{},
	}
	return tiles
}

func tilesDB() map[string][3]float64 {
	fmt.Println("Start populating tiles db ...")

	db := make(map[string][3]float64)
	files, _ := ioutil.ReadDir("tiles")
	for _, f := range files {
		name := "tiles/" + f.Name()
		file, err := os.Open(name)
		if err == nil {
			img, _, err := image.Decode(file)
			if err == nil {
				db[name] = averageColor(img)
			} else {
				fmt.Println("error in populating tiles db:", err, name)
			}
		} else {
			fmt.Println("cannot open file", name, "when populating tiles db:", err)
		}
		_ = file.Close()
	}
	fmt.Println("Finished populating tiles db.")
	return db
}

func (db *DB) getNearestTile(target [3]float64) string {
	var filename string
	smallest := 1e9

	db.mutex.Lock()
	for k, v := range db.store {
		distance := distance(target, v)
		if distance < smallest {
			filename = k
			smallest = distance
		}
	}
	//delete(tilesDB, filename)
	db.mutex.Unlock()
	return filename
}

func distance(color1 [3]float64, color2 [3]float64) float64 {
	return math.Sqrt(square(color1[0] - color2[0]) + square(color1[1] - color2[1]) + square(color1[2] - color2[2]))
}

func square(n float64) float64 {
	return n * n
}
