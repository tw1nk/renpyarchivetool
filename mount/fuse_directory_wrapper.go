package mount

import (
	"context"
	"log"
	"path/filepath"
	"syscall"
	"time"

	gofusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

type fuseDirectoryWrapper struct {
	gofusefs.Inode
	archiveFileMap map[string]string
	inodeMap       map[string]*gofusefs.Inode
}

// Lookup implements fs.NodeLookuper.
func (r *fuseDirectoryWrapper) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*gofusefs.Inode, syscall.Errno) {
	archiveFilePath, found := r.archiveFileMap[name]
	if !found {
		return nil, syscall.ENOENT
	}

	foundInode, found := r.inodeMap[name]
	if found {
		return foundInode, gofusefs.OK
	}

	log.Println("fuseDirectoryWrapper.Lookup", name)

	archiveFS, err := NewRenpyArchiveFSFromPath(archiveFilePath)
	if err != nil {

		log.Println("failed to load archive", archiveFilePath, err)

		return nil, syscall.ENOENT
	}

	out.EntryValidNsec = uint32(time.Hour / 1000)
	out.AttrValidNsec = uint32(time.Hour / 1000)
	var attr fuse.Attr
	attr.Mode = syscall.S_IFDIR | 0644
	out.Attr = attr

	inode := archiveFS.EmbeddedInode()
	r.AddChild(name, inode, true)
	r.inodeMap[name] = inode
	return inode, gofusefs.OK
}

func NewFuseDirectoryWrapper(archiveFiles []string) *fuseDirectoryWrapper {

	archiveFileMap := make(map[string]string)
	for _, archiveFilePath := range archiveFiles {
		dir, base := filepath.Split(archiveFilePath)
		for {
			_, found := archiveFileMap[base]
			if !found {
				archiveFileMap[base] = archiveFilePath
				break
			}
			if dir == "" {
				panic("duplicate file name for: " + archiveFilePath)
			}
			var newbase string
			dir, newbase = filepath.Split(dir)
			base = newbase + "_" + base
		}
	}

	return &fuseDirectoryWrapper{
		archiveFileMap: archiveFileMap,
		inodeMap:       make(map[string]*gofusefs.Inode),
	}
}

// OnAdd implements fs.NodeOnAdder.
func (r *fuseDirectoryWrapper) OnAdd(ctx context.Context) {
	for mountPath, _ := range r.archiveFileMap {
		p := r.EmbeddedInode()
		log.Println("fuseDirectoryWrapper.OnAdd", mountPath)

		child := p.NewPersistentInode(
			ctx,
			&gofusefs.Inode{},
			gofusefs.StableAttr{Mode: syscall.S_IFDIR},
		)

		success := p.AddChild(mountPath, child, true)
		if !success {
			log.Println("failed to add child", mountPath)
		}
	}
}

var _ gofusefs.NodeLookuper = (*fuseDirectoryWrapper)(nil)
