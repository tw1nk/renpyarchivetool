package main

import (
	"github.com/spf13/cobra"
)

func main() {

	rootCmd := &cobra.Command{
		Use:   "rptool",
		Short: "Ren'Py Archive Tool",
		Long:  `A tool to extract and list files from Ren'Py archives.`,
	}

	rootCmd.AddCommand(extractCmd)
	rootCmd.AddCommand(listCmd)

	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}
