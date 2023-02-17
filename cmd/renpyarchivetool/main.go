package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"github.com/tw1nk/renpyarchivetool"
)

type argParams struct {
	extract      bool
	list         bool
	outputFolder string
}

func main() {
	params := argParams{
		list:         true,
		outputFolder: "./",
	}

	flag.BoolVar(&params.list, "list", true, "list files")
	flag.BoolVar(&params.extract, "extract", false, "extract files")
	flag.StringVar(&params.outputFolder, "out", ".", "where to extract files")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		log.Println("filename must be provided")
	}
	filename := args[0]

	archive := renpyarchivetool.RenPyArchive{}
	err := archive.Load(filename)
	if err != nil {
		log.Fatal(err)
	}

	if params.list {
		log.Println("Files in archive:")
		for _, fileName := range archive.FileNames() {
			log.Println("\t", fileName)
		}
	}

	if params.extract {
		var err error
		params.outputFolder, err = filepath.Abs(params.outputFolder)
		if err != nil {
			panic(err)
		}

		for _, filename := range archive.FileNames() {

			outputPath := filepath.Join(params.outputFolder, filename)
			dirName := filepath.Dir(outputPath)
			if err := os.MkdirAll(dirName, os.ModePerm); err != nil {
				panic(err)
			}

			fileData, err := archive.Read(filename)
			if err != nil {
				panic(err)
			}

			// handle things where the filetype doesn't match the extension
			fileType := mimetype.Detect(fileData)
			if !strings.HasSuffix(outputPath, fileType.Extension()) {
				outputPath += fileType.Extension()
			}

			log.Printf("Extracting %s filetype: %v", outputPath, fileType)

			err = os.WriteFile(outputPath, fileData, os.ModePerm)
			if err != nil {
				panic(err)
			}
		}
	}
}
