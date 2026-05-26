package main

import (
	"bytes"
	"image"
	"image/draw"
	"log"
	"os"

	ico "github.com/Kodeworks/golang-image-ico"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

func main() {
	data, err := os.ReadFile("logo.svg")
	if err != nil {
		log.Fatalf("read logo.svg: %v", err)
	}

	icon, err := oksvg.ReadIconStream(bytes.NewReader(data))
	if err != nil {
		log.Fatalf("parse logo.svg: %v", err)
	}

	const size = 256
	rgba := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(rgba, rgba.Bounds(), image.Transparent, image.Point{}, draw.Src)

	icon.SetTarget(0, 0, float64(size), float64(size))
	scanner := rasterx.NewScannerGV(size, size, rgba, rgba.Bounds())
	raster := rasterx.NewDasher(size, size, scanner)
	icon.Draw(raster, 1.0)

	out, err := os.Create("logo.ico")
	if err != nil {
		log.Fatalf("create logo.ico: %v", err)
	}
	defer out.Close()

	if err := ico.Encode(out, rgba); err != nil {
		log.Fatalf("encode ico: %v", err)
	}
}
