package mount

import (
	"context"
	"path/filepath"
	"strings"
	"syscall"

	gofusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/tw1nk/renpyarchivetool"
)

type renpyArchiveFS struct {
	gofusefs.Inode
	archive *renpyarchivetool.RenPyArchive
}

// OnAdd implements fs.NodeOnAdder.
func (r *renpyArchiveFS) OnAdd(ctx context.Context) {
	for archiveFilePath, info := range r.archive.Indexes() {
		dir, base := filepath.Split(archiveFilePath)

		p := r.EmbeddedInode()
		for _, comp := range strings.Split(dir, "/") {
			if comp == "" {
				continue
			}

			child := p.GetChild(comp)
			if child == nil {
				child = p.NewPersistentInode(ctx,
					&gofusefs.Inode{},
					gofusefs.StableAttr{Mode: syscall.S_IFDIR},
				)

				p.AddChild(comp, child, false)
			}

			p = child
		}

		var attr fuse.Attr
		attr.Size = uint64(info.Length)
		data, err := r.archive.Read(archiveFilePath)
		if err != nil {
			continue
		}
		df := gofusefs.MemRegularFile{
			Data: data,
		}

		df.Attr = attr
		p.AddChild(base, r.NewPersistentInode(
			ctx,
			&df,
			gofusefs.StableAttr{
				Mode: syscall.S_IFREG,
			},
		), false)
	}

}

func NewRenpyArchiveFSFromPath(archivePath string) (gofusefs.InodeEmbedder, error) {
	archive, err := renpyarchivetool.Load(archivePath)
	if err != nil {
		return nil, err
	}

	return &renpyArchiveFS{
		archive: archive,
	}, nil
}

func NewRenpyArchiveFS(archive *renpyarchivetool.RenPyArchive) gofusefs.InodeEmbedder {
	return &renpyArchiveFS{
		archive: archive,
	}
}

var (
	_ gofusefs.InodeEmbedder = new(renpyArchiveFS)
	_ gofusefs.NodeOnAdder   = new(renpyArchiveFS)

	//_ gofusefs.NodeGetattrer = (*renpyArchiveFS)(nil)
	//_ gofusefs.NodeReader    = (*renpyArchiveFS)(nil)
	//_ gofusefs.NodeOpener     = (*renpyArchiveFS)(nil)
	// _ gofusefs.NodeLookuper  = (*renpyArchiveFS)(nil)
	// _ gofusefs.NodeReaddirer = (*renpyArchiveFS)(nil)
)
