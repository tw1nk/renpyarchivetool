package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"github.com/mattn/go-zglob"
	"github.com/spf13/cobra"
	"github.com/tw1nk/renpyarchivetool"
)

var extractCmd *cobra.Command

func init() {
	extractCmd = &cobra.Command{
		Use:   "extract",
		Short: "Extract files from Ren'Py archives",
		RunE:  extract,
		Args:  cobra.MinimumNArgs(1),
	}

	extractCmd.Flags().StringP("output", "o", ".", "output folder")
	extractCmd.Flags().BoolP("use-mime-detector", "m", false, "use mime detector to give files correct extensions")
}

func extract(cmd *cobra.Command, args []string) error {
	filename := args[0]

	info, err := os.Stat(filename)
	if err != nil {
		return err
	}

	if info.IsDir() {
		files, err := zglob.Glob(filepath.Join(filename, "*.rpa"))
		if err != nil {
			return err
		}

		for _, filename := range files {
			extractFile(filename)
		}

	} else {
		extractFile(filename)
	}

	return nil
}

func extractFile(filename string) error {
	outputFolder, err := extractCmd.Flags().GetString("output")
	if err != nil {
		return err
	}

	outputFolder, err = filepath.Abs(outputFolder)
	if err != nil {
		return err
	}

	useMimeDetector, err := extractCmd.Flags().GetBool("use-mime-detector")
	if err != nil {
		return err
	}

	archive, err := renpyarchivetool.Load(filename)
	if err != nil {
		return err
	}

	return exctractArchive(archive, outputFolder, useMimeDetector)
}

func exctractArchive(
	archive *renpyarchivetool.RenPyArchive,
	outputFolder string,
	useMimeDetector bool,
) error {
	for _, filename := range archive.FileNames() {
		outputPath := filepath.Join(outputFolder, filename)
		dirName := filepath.Dir(outputPath)
		if err := os.MkdirAll(dirName, os.ModePerm); err != nil {
			return err
		}

		fileData, err := archive.Read(filename)
		if err != nil {
			return err
		}

		if useMimeDetector && !strings.HasSuffix(outputPath, ".rpy") {
			fileType := mimetype.Detect(fileData)
			if !strings.HasSuffix(outputPath, fileType.Extension()) {
				log.Printf("Correcting file extension of: %s to: %s",
					outputPath,
					fileType.Extension(),
				)
				outputPath += fileType.Extension()
			}

			log.Printf("Extracting %s", outputPath)

			err = os.WriteFile(outputPath, fileData, os.ModePerm)
			if err != nil {
				return err
			}

		}

	}

	return nil
}
