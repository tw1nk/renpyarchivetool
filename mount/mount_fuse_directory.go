package mount

import (
	"path/filepath"

	gofusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/mattn/go-zglob"
)

func Directory(mountPath string, dirPath string) (Controller, error) {
	dirPath, err := filepath.Abs(dirPath)
	if err != nil {
		return nil, err
	}

	files, err := zglob.Glob(filepath.Join(dirPath, "*.rpa"))
	if err != nil {
		return nil, err
	}

	rootfs := NewFuseDirectoryWrapper(files)

	fuseServer, err := gofusefs.Mount(mountPath, rootfs, &gofusefs.Options{
		MountOptions: fuse.MountOptions{
			AllowOther: true,
			Name:       "rptool",
			FsName:     "rptool",
			Debug:      false,
		},
		EntryTimeout:    &cacheTimeout,
		AttrTimeout:     &cacheTimeout,
		NegativeTimeout: &cacheTimeout,
	})

	if err != nil {
		return nil, err
	}

	done := make(chan struct{})
	go func() {
		fuseServer.Wait()
		close(done)
	}()

	return &fuseController{
		mountPoint:     mountPath,
		fuseConnection: fuseServer,
		done:           done,
	}, nil
}
