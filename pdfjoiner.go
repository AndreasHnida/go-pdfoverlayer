package main

import (
    "flag"
    "fmt"
    "log"
    "os"
    "path/filepath"

    "github.com/pdfcpu/pdfcpu/pkg/api"
    "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

func main() {
    // Command line flags for the input and output files
    backgroundFile := flag.String("background", "", "The PDF file to be watermarked.")
    watermarkFile := flag.String("watermark", "", "The PDF file to use as a watermark.")
    outputFile := flag.String("output", "watermarked_output.pdf", "The output PDF file name.")
    flag.Parse()

    // Check if the background and watermark files have been specified
    if *backgroundFile == "" || *watermarkFile == "" {
        fmt.Println("Error: You must specify both a background and a watermark PDF file.")
        os.Exit(1)
    }

    // Get the absolute path of the current directory
    currentDir, err := os.Getwd()
    if err != nil {
        log.Fatalf("Error getting current directory: %v", err)
    }

    // Prepend the current directory to the file names
    backgroundPath := filepath.Join(currentDir, *backgroundFile)
    watermarkPath := filepath.Join(currentDir, *watermarkFile)
    outputPath := filepath.Join(currentDir, *outputFile)

    // Create a watermark configuration using a PDF
    wmConf := "scalefactor:1.0, opacity:1" // Scale factor can be adjusted
    onTop := true
    wm, err := api.PDFWatermark(watermarkPath, wmConf, onTop, false, types.POINTS)
    if err != nil {
        log.Fatalf("Error creating PDF watermark: %v", err)
    }

    // Apply the watermark to the background PDF
    err = api.AddWatermarksFile(backgroundPath, outputPath, nil, wm, nil)
    if err != nil {
        log.Fatalf("Error applying watermark: %v", err)
    }

    fmt.Printf("Successfully created watermarked PDF: %s\n", outputPath)
}

