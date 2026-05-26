package main

import (
    "image/png"
    "log"
    "os"

    ico "github.com/Kodeworks/golang-image-ico"
)

func main() {
    in, err := os.Open("logo.png")
    if err != nil {
        log.Fatalf("open logo.png: %v", err)
    }
    defer in.Close()
    img, err := png.Decode(in)
    if err != nil {
        log.Fatalf("decode png: %v", err)
    }
    out, err := os.Create("logo.ico")
    if err != nil {
        log.Fatalf("create logo.ico: %v", err)
    }
    defer out.Close()
    if err := ico.Encode(out, img); err != nil {
        log.Fatalf("encode ico: %v", err)
    }
}
