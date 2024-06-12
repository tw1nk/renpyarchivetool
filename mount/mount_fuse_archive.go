package mount

import (
	"path/filepath"
	"time"

	gofusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/tw1nk/renpyarchivetool"
)

// we're serving read-only filesystem, cache some attributes for 30 seconds.
//
//nolint:gochecknoglobals
var cacheTimeout = 30 * time.Second

type fuseController struct {
	mountPoint     string
	fuseConnection *fuse.Server
	done           chan struct{}
}

func (fc *fuseController) MountPath() string {
	return fc.mountPoint
}

func (fc *fuseController) Unmount() error {
	if err := fc.fuseConnection.Unmount(); err != nil {
		return err
	}

	return nil
}

func (fc fuseController) Done() <-chan struct{} {
	return fc.done
}

func Archive(
	mountPath string,
	archivePath string,
) (Controller, error) {

	if mountPath == "." {
		_, fileName := filepath.Split(archivePath)
		mountPath += "/" + fileName + "_mount"
	}

	mountPath, err := filepath.Abs(mountPath)
	if err != nil {
		return nil, err
	}

	archivePath, err = filepath.Abs(archivePath)
	if err != nil {
		return nil, err
	}

	archive, err := renpyarchivetool.Load(archivePath)
	if err != nil {
		return nil, err
	}

	_ = archive

	rootfs := NewRenpyArchiveFS(archive)

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
