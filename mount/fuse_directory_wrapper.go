package mount

import (
	"context"
	"log"
	"path/filepath"
	"strings"
	"syscall"

	gofusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/tw1nk/renpyarchivetool"
)

type fuseDirectoryRootWrapper struct {
	gofusefs.Inode
	archiveFileMap map[string]string
	inodeMap       map[string]*gofusefs.Inode
}

type fuseRenpyArchiveDirectoryWrapper struct {
	gofusefs.Inode
	archileFilePath string

	dirEntries []fuse.DirEntry
	inodes     []*gofusefs.Inode
}

type fuseRenpyArchiveFileNode struct {
	gofusefs.Inode
	archive  *renpyarchivetool.RenPyArchive
	filePath string
	Attr     fuse.Attr
	info     *renpyarchivetool.Index

	data []byte
}

// Open implements fs.NodeOpener.
func (f *fuseRenpyArchiveFileNode) Open(ctx context.Context, flags uint32) (fh gofusefs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	return nil, fuse.FOPEN_KEEP_CACHE, gofusefs.OK

}

func (f *fuseRenpyArchiveFileNode) Getattr(ctx context.Context, fh gofusefs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Attr = f.Attr
	out.Attr.Size = uint64(f.info.Length)
	out.Attr.Mode = syscall.S_IFREG | 0444

	return gofusefs.OK
}

var _ = (gofusefs.NodeOpener)((*fuseRenpyArchiveFileNode)(nil))

func (f *fuseRenpyArchiveFileNode) Read(ctx context.Context, fh gofusefs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	if f.data == nil {
		data, err := f.archive.Read(f.filePath)
		if err != nil {
			return nil, syscall.EIO
		}
		f.data = data
	}

	end := off + int64(len(dest))
	if end > int64(len(f.data)) {
		end = int64(len(f.data))
	}

	return fuse.ReadResultData(f.data[off:end]), gofusefs.OK
}

type fuseRenpyArchiveDirNode struct {
	gofusefs.Inode
	archive *renpyarchivetool.RenPyArchive

	dirEntries []fuse.DirEntry
}

// Readdir implements fs.NodeReaddirer.
func (f *fuseRenpyArchiveDirectoryWrapper) Readdir(ctx context.Context) (gofusefs.DirStream, syscall.Errno) {
	if f.dirEntries != nil {
		return gofusefs.NewListDirStream(f.dirEntries), gofusefs.OK
	}

	out := make([]fuse.DirEntry, 0)
	archive, err := renpyarchivetool.Load(f.archileFilePath)
	if err != nil {
		return nil, syscall.EIO
	}

	for archiveFilePath, info := range archive.Indexes() {
		p := f.EmbeddedInode()

		dir, base := filepath.Split(archiveFilePath)
		if dir == "" {
			// create file node
			inodeIndex := len(f.inodes)
			out = append(out, fuse.DirEntry{
				Name: base,
				Mode: syscall.S_IFREG,
				Ino:  uint64(inodeIndex),
			})

			fileNode := &fuseRenpyArchiveFileNode{
				archive:  archive,
				filePath: archiveFilePath,
				info:     info,
			}

			ch := p.NewInode(ctx,
				fileNode,
				gofusefs.StableAttr{
					Mode: syscall.S_IFREG,
					Ino:  uint64(inodeIndex),
				})

			p.AddChild(base, ch, false)
			f.inodes = append(f.inodes, ch)

		} else {
			parts := strings.Split(dir, "/")

			// create directory node if not exists
			child := p.GetChild(parts[0])
			if child == nil {
				out = append(out, fuse.DirEntry{
					Name: parts[0],
					Mode: syscall.S_IFDIR,
				})

				inodeIndex := len(f.inodes)
				child = p.NewPersistentInode(ctx,
					&gofusefs.MemRegularFile{},
					gofusefs.StableAttr{
						Mode: syscall.S_IFDIR,
						Ino:  uint64(inodeIndex),
					},
				)

				p.AddChild(parts[0], child, false)
				f.inodes = append(f.inodes, child)

			}

			p = child

			for _, comp := range parts[1:] {
				if comp == "" {
					continue
				}

				child := p.GetChild(comp)
				if child == nil {
					// create file node
					inodeIndex := len(f.inodes)
					child = p.NewPersistentInode(ctx,
						&gofusefs.MemRegularFile{},
						gofusefs.StableAttr{
							Mode: syscall.S_IFDIR,
							Ino:  uint64(inodeIndex),
						},
					)

					p.AddChild(comp, child, false)
					f.inodes = append(f.inodes, child)
				}

				p = child

			}
			var attr fuse.Attr
			attr.Size = uint64(info.Length)

			fileNode := &fuseRenpyArchiveFileNode{
				archive:  archive,
				filePath: archiveFilePath,
				info:     info,
			}
			fileNode.Attr = attr
			inodeIndex := len(f.inodes)

			ch := p.NewInode(ctx,
				fileNode,
				gofusefs.StableAttr{
					Mode: syscall.S_IFREG,
					Ino:  uint64(inodeIndex),
				})
			p.AddChild(base, ch, false)
			f.inodes = append(f.inodes, ch)

		}
	}

	f.dirEntries = out

	return gofusefs.NewListDirStream(out), gofusefs.OK
}

var _ gofusefs.NodeReaddirer = (*fuseRenpyArchiveDirectoryWrapper)(nil)

func NewFuseDirectoryWrapper(archiveFiles []string) *fuseDirectoryRootWrapper {

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

	return &fuseDirectoryRootWrapper{
		archiveFileMap: archiveFileMap,
		inodeMap:       make(map[string]*gofusefs.Inode),
	}
}

// OnAdd implements fs.NodeOnAdder.
func (r *fuseDirectoryRootWrapper) OnAdd(ctx context.Context) {
	for mountPath, archileFilePath := range r.archiveFileMap {
		p := r.EmbeddedInode()
		child := p.GetChild(mountPath)
		if child == nil {
			rf := &fuseRenpyArchiveDirectoryWrapper{
				archileFilePath: archileFilePath,
			}
			/*
				var attr fuse.Attr
				rf.Attr = attr
			*/
			child := p.NewPersistentInode(
				ctx,
				rf,
				gofusefs.StableAttr{
					Mode: syscall.S_IFDIR,
				},
			)

			success := p.AddChild(mountPath, child, false)

			if !success {
				log.Println("failed to add child", mountPath)
			}
		}
	}
}
