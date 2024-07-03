package main

import (
    "embed"
    "io/ioutil"
    "flag"
    "fmt"
    "log"
    "os"
    "path/filepath"

    "github.com/pdfcpu/pdfcpu/pkg/api"
    "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

//go:embed assets/background.pdf
var backgroundPDF embed.FS

func main() {
    // Command line flags for the input and output files

 // Extracting the PDF data from embedded file system
    data, err := backgroundPDF.ReadFile("assets/background.pdf")
    if err != nil {
        log.Fatal(err)
    }

    // Use the data as needed, for example, write to temporary file and use it
    tempFile, err := ioutil.TempFile("", "background-*.pdf")
    if err != nil {
        log.Fatal(err)
    }
    defer tempFile.Close()

    if _, err := tempFile.Write(data); err != nil {
        log.Fatal(err)
    }

    watermarkFile := flag.String("watermark", "", "The PDF file to use as a watermark.")
    outputFile := flag.String("output", "watermarked_output.pdf", "The output PDF file name.")
    flag.Parse()

    // Check if the background and watermark files have been specified
    if *watermarkFile == "" {
        fmt.Println("Error: You must specify both a background and a watermark PDF file.")
        os.Exit(1)
    }

    // Get the absolute path of the current directory
    currentDir, err := os.Getwd()
    if err != nil {
        log.Fatalf("Error getting current directory: %v", err)
    }

    // Prepend the current directory to the file names
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
    err = api.AddWatermarksFile(tempFile.Name(), outputPath, nil, wm, nil)
    if err != nil {
        log.Fatalf("Error applying watermark: %v", err)
    }

    fmt.Printf("Successfully created watermarked PDF: %s\n", outputPath)
}

