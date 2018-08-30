package main

import (
	"net/http"
	"fmt"
	"html/template"
	"strconv"
	"image"
	"os"
	"image/draw"
	"bytes"
	"image/jpeg"
	"encoding/base64"
	"time"
	"sync"
)

var TILESDB map[string][3]float64

func main() {
	mux := http.NewServeMux()
	files := http.FileServer(http.Dir("public"))
	mux.Handle("/static/", http.StripPrefix("/static/", files))
	mux.HandleFunc("/", upload)
	mux.HandleFunc("/mosaic", mosaic)
	server := &http.Server{
		Addr: "0.0.0.0:8080",
		Handler: mux,
	}

	TILESDB = tilesDB()
	fmt.Println("Mosaic server started.")
	server.ListenAndServe()
}

func upload(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles("upload.html")
	t.Execute(w, nil)
}

func mosaic(w http.ResponseWriter, r *http.Request) {
	t0 := time.Now()

	r.ParseMultipartForm(10485760)
	file, _, _ := r.FormFile("image")
	defer file.Close()

	tileSize, _ := strconv.Atoi(r.FormValue("tile_size"))

	original, _, _ := image.Decode(file)
	bounds := original.Bounds()
	db := cloneTilesDB()

	c1 := cut(original, db, tileSize, bounds.Min.X, bounds.Min.Y, bounds.Max.X/2, bounds.Max.Y/2)
	c2 := cut(original, db, tileSize, bounds.Max.X/2, bounds.Min.Y, bounds.Max.X, bounds.Max.Y/2)
	c3 := cut(original, db, tileSize, bounds.Min.X, bounds.Max.Y/2, bounds.Max.X/2, bounds.Max.Y)
	c4 := cut(original, db, tileSize, bounds.Max.X/2, bounds.Max.Y/2, bounds.Max.X, bounds.Max.Y)

	c := combine(bounds, c1, c2, c3, c4)

	buffer1 := new(bytes.Buffer)
	jpeg.Encode(buffer1, original, nil)
	originalStr := base64.StdEncoding.EncodeToString(buffer1.Bytes())

	t1 := time.Now()
	images := map[string]string{
		"original": originalStr,
		"mosaic":   <-c,
		"duration": fmt.Sprintf("%v ", t1.Sub(t0)),
	}
	t, _ := template.ParseFiles("results.html")
	t.Execute(w, images)
}

func cut(original image.Image, db *DB, tileSize int, x1, y1, x2, y2 int) <-chan image.Image {
	channel := make(chan image.Image)
	sourcePoint := image.Point{
		X:0,
		Y:0,
	}

	go func() {
		newImage := image.NewNRGBA(image.Rect(x1, y1, x2, y2))
		for y := y1; y < y2; y = y + tileSize {
			for x := x1; x < x2; x = x + tileSize {
				r, g, b, _ := original.At(x, y).RGBA()
				color := [3]float64{float64(r), float64(g), float64(b)}
				nearest := db.getNearestTile(color)
				file, err := os.Open(nearest)
				if err == nil {
					img, _, err := image.Decode(file)
					if err == nil {
						t := resize(img, tileSize)
						tile := t.SubImage(t.Bounds())
						tileBounds := image.Rect(x, y, x + tileSize, y + tileSize)
						draw.Draw(newImage, tileBounds, tile, sourcePoint, draw.Src)
					} else {
						fmt.Println("error in decoding nearest color file", err, nearest)
					}
				} else {
					fmt.Println("error opening file when creating mosaic:", nearest)
				}
				file.Close()
			}
		}
		channel <- newImage.SubImage(newImage.Rect)
	}()

	return channel
}

func combine(rec image.Rectangle, c1, c2, c3, c4 <-chan image.Image) <-chan string {
	channel := make(chan string)
	go func() {
		var wg sync.WaitGroup
		newImage := image.NewNRGBA(rec)
		copyImg := func(dst draw.Image, r image.Rectangle, src image.Image, sp image.Point) {
			draw.Draw(dst, r, src, sp, draw.Src)
			wg.Done()
		}
		wg.Add(4)
		var s1, s2, s3, s4 image.Image
		var ok1, ok2, ok3, ok4 bool
		for {
			select {
			case s1, ok1 = <- c1:
				go copyImg(newImage, s1.Bounds(), s1, image.Point{rec.Min.X, rec.Min.Y})
			case s2, ok2 = <- c2:
				go copyImg(newImage, s2.Bounds(), s2, image.Point{rec.Max.X / 2, rec.Min.Y})
			case s3, ok3 = <- c3:
				go copyImg(newImage, s3.Bounds(), s3, image.Point{rec.Min.X, rec.Max.Y / 2})
			case s4, ok4 = <- c4:
				go copyImg(newImage, s4.Bounds(), s4, image.Point{rec.Max.X / 2, rec.Max.Y / 2})
			}

			if ok1 && ok2 && ok3 && ok4 {
				break
			}
		}

		wg.Wait()
		buffer2 := new(bytes.Buffer)
		jpeg.Encode(buffer2, newImage, nil)
		channel <- base64.StdEncoding.EncodeToString(buffer2.Bytes())
	}()

	return channel
}
