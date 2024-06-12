package main

import (
	"os"
	"os/signal"

	"github.com/spf13/cobra"
	"github.com/tw1nk/renpyarchivetool/mount"
)

var mountCmd *cobra.Command

func init() {
	mountCmd = &cobra.Command{
		Use:   "mount",
		Short: "Mount Ren'Py archives",
		RunE:  mountFunc,
		Args:  cobra.ExactArgs(2),
	}
}

func mountFunc(cmd *cobra.Command, args []string) error {
	filename := args[0]
	mountpoint := args[1]

	info, err := os.Stat(filename)
	if err != nil {
		return err
	}

	var ctrl mount.Controller

	if info.IsDir() {
		ctrl, err = mount.Directory(mountpoint, filename)
		if err != nil {
			return err
		}
	} else {
		ctrl, err = mount.Archive(mountpoint, filename)
		if err != nil {
			return err
		}
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	select {
	case <-signalChan:
		err := ctrl.Unmount()
		if err != nil {
			return err
		}

	case <-ctrl.Done():
		return nil
	}

	<-ctrl.Done()

	return nil
}
