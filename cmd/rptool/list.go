package main

import (
	"os"
	"path/filepath"

	"github.com/mattn/go-zglob"
	"github.com/spf13/cobra"
	"github.com/tw1nk/renpyarchivetool"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List files from Ren'Py archives",
	RunE:  list,
	Args:  cobra.MinimumNArgs(1),
}

func list(cmd *cobra.Command, args []string) error {

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
			listFile(filename)
		}

	} else {
		listFile(filename)
	}

	return nil
}

func listFile(filename string) error {
	archive, err := renpyarchivetool.Load(filename)
	if err != nil {
		return err
	}

	for _, file := range archive.FileNames() {
		println(file)
	}

	return nil
}
