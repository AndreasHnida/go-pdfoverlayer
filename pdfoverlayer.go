package main

import (
	"embed"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

//go:embed assets/background.pdf
var backgroundPDF embed.FS
var verbose bool

const (
	DEFAULT_OUTPUT_FILE = "watermarked_output.pdf"
	WATERMARK_CONFIG    = "scalefactor:.85, opacity:1, rotation:0, offset: 0 72"
	BACKGROUND_PDF_PATH = "assets/background.pdf"
	BOTTOM_CROP_POINTS  = 5.0 * 28.3465 // 5cm in points
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func logln(msg string) {
	if verbose {
		fmt.Println(msg)
	}
}

func logf(format string, args ...interface{}) {
	if verbose {
		fmt.Printf(format+"\n", args)
	}
}

func main() {
	verbosePtr := flag.Bool("v", false, "Enable debug output.")
	watermarkFile := flag.String("watermark", "", "The PDF file to use as a watermark.")
	outputFile := flag.String("output", DEFAULT_OUTPUT_FILE, "The output PDF file name.")
	flag.Parse()

	verbose = *verbosePtr

	if err := run(*watermarkFile, *outputFile); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func run(watermarkFile, outputFile string) error {
	if watermarkFile == "" {
		return fmt.Errorf("you must specify a watermark PDF file")
	}

	tempFile, err := createTempFile()
	if err != nil {
		return err
	}
	defer os.Remove(tempFile.Name())

	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting current directory: %v", err)
	}

	watermarkPath := filepath.Join(currentDir, watermarkFile)
	modifiedWatermarkPath := filepath.Join(os.TempDir(), "modified_watermark.pdf")

	logln("Starting content stream modification.")
	if err := modifyContentStream(watermarkPath, modifiedWatermarkPath); err != nil {
		return fmt.Errorf("error modifying watermark PDF: %v", err)
	}
	logln("Content stream modification completed.")

	wm, err := api.PDFWatermark(modifiedWatermarkPath, WATERMARK_CONFIG, true, false, types.POINTS)
	if err != nil {
		return fmt.Errorf("error creating PDF watermark: %v", err)
	}

	if err := api.AddWatermarksFile(tempFile.Name(), outputFile, nil, wm, nil); err != nil {
		return fmt.Errorf("error applying watermark: %v", err)
	}

	logf("Successfully created watermarked PDF: %s\n", outputFile)
	return nil
}

func createTempFile() (*os.File, error) {
	data, err := backgroundPDF.ReadFile(BACKGROUND_PDF_PATH)
	if err != nil {
		return nil, fmt.Errorf("error reading embedded background PDF: %v", err)
	}

	tempFile, err := ioutil.TempFile("", "background-*.pdf")
	if err != nil {
		return nil, fmt.Errorf("error creating temporary file: %v", err)
	}

	if _, err := tempFile.Write(data); err != nil {
		return nil, fmt.Errorf("error writing to temporary file: %v", err)
	}
	if err := tempFile.Close(); err != nil {
		return nil, fmt.Errorf("error closing temporary file: %v", err)
	}

	return tempFile, nil
}

func modifyContentStream(inputPath, outputPath string) error {
	logf("Reading PDF context from: %s", inputPath)
	ctx, err := api.ReadContextFile(inputPath)
	if err != nil {
		return fmt.Errorf("error reading PDF context: %v", err)
	}

	if ctx.PageCount > 1 {
		return fmt.Errorf("please provide a PDF with only one page")
	}

	if err := removeBackground(ctx); err != nil {
		return fmt.Errorf("error removing background: %v", err)
	}

	if err := cropBottomAndLeft(ctx); err != nil {
		return fmt.Errorf("error cropping bottom: %v", err)
	}

	if err := api.WriteContextFile(ctx, outputPath); err != nil {
		return fmt.Errorf("error writing modified PDF: %v", err)
	}

	logln("Successfully written to output.pdf")
	return nil
}

func cropBottomAndLeft(ctx *model.Context) error {
	pageDict, _, _, err := ctx.PageDict(1, false)
	if err != nil {
		return fmt.Errorf("error getting page dictionary for page 1: %v", err)
	}

	mediaBoxArray := pageDict.ArrayEntry("MediaBox")
	if mediaBoxArray == nil || len(mediaBoxArray) != 4 {
		return fmt.Errorf("error: MediaBox not found or invalid for page 1")
	}

	// Crop 2cm from the left
	if mediaBoxArray[0], err = adjustCoordinate(mediaBoxArray[0], 56.7); err != nil {
		return fmt.Errorf("error adjusting MediaBox for page 1: %v", err)
	}
	if mediaBoxArray[2], err = adjustCoordinate(mediaBoxArray[2], -56.7); err != nil {
		return fmt.Errorf("error adjusting MediaBox for page 1: %v", err)
	}

	// Crop from the bottom
	if mediaBoxArray[1], err = adjustCoordinate(mediaBoxArray[1], BOTTOM_CROP_POINTS); err != nil {
		return fmt.Errorf("error adjusting MediaBox for page 1: %v", err)
	}

	pageDict.Update("MediaBox", mediaBoxArray)

	return nil
}

func adjustCoordinate(coord types.Object, points float64) (types.Object, error) {
	switch v := coord.(type) {
	case types.Integer:
		return types.Integer(int(v) + int(points)), nil
	case types.Float:
		return types.Float(float64(v) + points), nil
	default:
		return nil, fmt.Errorf("unsupported coordinate type: %T", coord)
	}
}

func removeBackground(ctx *model.Context) error {
	pageDict, _, _, err := ctx.PageDict(1, false)
	if err != nil {
		return fmt.Errorf("error getting page dictionary for page 1: %v", err)
	}

	cont := pageDict["Contents"]
	streamDict, _, err := ctx.DereferenceStreamDict(cont)
	if err != nil {
		return fmt.Errorf("error dereferencing content stream: %v", err)
	}

	if err := streamDict.Decode(); err != nil {
		return fmt.Errorf("error decoding stream: %v", err)
	}

	contentString := strings.ReplaceAll(string(streamDict.Content), "\n", " ")
	logln(contentString)

	whiteRectRegex := regexp.MustCompile(`(/Cs1 cs 1 1 1 sc \d+(\.\d+)? \d+(\.\d+)? \d+(\.\d+)? \d+(\.\d+)? re f\*)`)
	whiteRectRegex2 := regexp.MustCompile(`\b1\s+1\s+1\s+rg\s+\d+(\.\d+)?\s+\d+(\.\d+)?\s+m\s+\d+(\.\d+)?\s+\d+(\.\d+)?\s+l\s+\d+(\.\d+)?\s+\d+(\.\d+)?\s+l\s+\d+(\.\d+)?\s+\d+(\.\d+)?\s+l\s+\d+(\.\d+)?\s+\d+(\.\d+)?\s+l\s+h\s+f\*`)
	blackRectRegex := regexp.MustCompile(`(\d+\.\d+\s+){3}(9\.\d+|0\.8\d*)\s+re\s+f\*`)
	imageRegex := regexp.MustCompile(`/Im\d+\s+Do`)

	if matches := whiteRectRegex.FindAllString(contentString, -1); len(matches) == 0 {
		logln("no white background found")
	}
	if matches := whiteRectRegex2.FindAllString(contentString, -1); len(matches) == 0 {
		logln("no white background found with the new method either")
	}

	modifiedContent := removeMatches(contentString, whiteRectRegex)
	modifiedContent = removeMatches(modifiedContent, whiteRectRegex2)
	modifiedContent = removeMatches(modifiedContent, blackRectRegex)
	modifiedContent = removeMatches(modifiedContent, imageRegex)

	streamDict.Content = []byte(modifiedContent)
	if err := streamDict.Encode(); err != nil {
		return fmt.Errorf("error encoding stream: %v", err)
	}

	sdRef, err := ctx.IndRefForNewObject(*streamDict)
	if err != nil {
		return fmt.Errorf("error creating indirect reference for new stream dict: %v", err)
	}

	pageDict.Update("Contents", *sdRef)
	logln("Updated content stream in page dictionary.")
	return nil
}

func removeMatches(content string, regex *regexp.Regexp) string {
	return regex.ReplaceAllString(content, "")
}
