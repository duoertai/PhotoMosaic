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
	newImage := image.NewNRGBA(image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Max.X, bounds.Max.Y))
	db := cloneTilesDB()

	sourcePoint := image.Point{
		X:0,
		Y:0,
	}

	var tile image.Image
	for y := bounds.Min.Y; y < bounds.Max.Y; y += tileSize {
		for x := bounds.Min.X; x < bounds.Max.X; x += tileSize {
			r, g, b, _ := original.At(x, y).RGBA()
			color := [3]float64{
				float64(r),
				float64(g),
				float64(b),
			}

			nearest := getNearestTile(color, db)
			file ,err := os.Open(nearest)
			if err == nil {
				img, _, err := image.Decode(file)
				if err == nil {
					t := resize(img, tileSize)
					tile = t.SubImage(t.Bounds())
					tileBounds := image.Rect(x, y, x + tileSize, y + tileSize)
					draw.Draw(newImage, tileBounds, tile, sourcePoint, draw.Src)
				} else {
					fmt.Println("error:", err, nearest)
				}

			} else {
				fmt.Println("error: cannot open file", nearest)
			}
			file.Close()
		}
	}

	buffer1 := new(bytes.Buffer)
	jpeg.Encode(buffer1, original, nil)
	originalStr := base64.StdEncoding.EncodeToString(buffer1.Bytes())

	buffer2 := new(bytes.Buffer)
	jpeg.Encode(buffer2, newImage, nil)
	mosaic := base64.StdEncoding.EncodeToString(buffer2.Bytes())

	t1 := time.Now()
	images := map[string]string{
		"original": originalStr,
		"mosaic":   mosaic,
		"duration": fmt.Sprintf("%v ", t1.Sub(t0)),
	}
	t, _ := template.ParseFiles("results.html")
	t.Execute(w, images)
}
