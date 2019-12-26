package scs

import (
	"bufio"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"io"
	"io/ioutil"
	"path"
	"reflect"
	"strings"

	"github.com/pkg/errors"

	"github.com/Luzifer/scs-extract/b0rkhash"
)

var (
	localeRootPathHash = b0rkhash.CityHash64([]byte("locale"))
	rootPathHash       = b0rkhash.CityHash64([]byte(""))
)

type CatalogEntry struct {
	HashedPath uint64
	Offset     int32
	_          int32
	Type       EntryType
	CRC        uint32
	Size       int32
	ZSize      int32
}

type EntryType int32

// See https://forum.scssoft.com/viewtopic.php?p=644638#p644638
const (
	EntryTypeUncompressedFile EntryType = iota
	EntryTypeUncompressedNames
	EntryTypeCompressedFile
	EntryTypeCompressedNames
	EntryTypeUncompressedFileCopy
	EntryTypeUncompressedNamesCopy
	EntryTypeCompressedFileCopy
	EntryTypeCompressedNamesCopy
)

type File struct {
	Name string

	archiveReader io.ReaderAt

	CatalogEntry
}

func (f *File) Open() (io.ReadCloser, error) {
	var rc io.ReadCloser

	switch f.Type {

	case EntryTypeCompressedFile, EntryTypeCompressedFileCopy, EntryTypeCompressedNames, EntryTypeCompressedNamesCopy:
		r := io.NewSectionReader(f.archiveReader, int64(f.Offset+2), int64(f.ZSize))
		rc = flate.NewReader(r)

	case EntryTypeUncompressedFile, EntryTypeUncompressedFileCopy, EntryTypeUncompressedNames, EntryTypeUncompressedNamesCopy:
		r := io.NewSectionReader(f.archiveReader, int64(f.Offset), int64(f.Size))
		rc = ioutil.NopCloser(r)

	}
	return rc, nil
}

type Reader struct {
	Files []*File
}

func NewReader(r io.ReaderAt, size int64) (*Reader, error) {
	var magic = make([]byte, 4)
	n, err := r.ReadAt(magic, 0)
	if err != nil || n != 4 {
		return nil, errors.Wrap(err, "Unable to read file magic")
	}

	if !reflect.DeepEqual(magic, []byte{0x53, 0x43, 0x53, 0x23}) {
		return nil, errors.New("Did not receive expected file magic")
	}

	var entries = make([]byte, 4)
	n, err = r.ReadAt(entries, 0xC)
	if err != nil || n != 4 {
		return nil, errors.Wrap(err, "Unable to read entry count")
	}

	var entryCount int32
	if err = binary.Read(bytes.NewReader(entries), binary.LittleEndian, &entryCount); err != nil {
		return nil, errors.Wrap(err, "Unable to parse entry count")
	}

	out := &Reader{}

	var offset int64 = 0x1000
	for i := int32(0); i < entryCount; i++ {
		var hdr = make([]byte, 32)
		n, err = r.ReadAt(hdr, offset)
		if err != nil || n != 32 {
			return nil, errors.Wrap(err, "Unable to read file header")
		}

		var e = CatalogEntry{}
		if err = binary.Read(bytes.NewReader(hdr), binary.LittleEndian, &e); err != nil {
			return nil, errors.Wrap(err, "Unable to parse file header")
		}

		out.Files = append(out.Files, &File{
			CatalogEntry:  e,
			archiveReader: r,
		})
		offset += 32
	}

	return out, out.populateFileNames()
}

func (r *Reader) populateFileNames() error {
	// first seek root entry, without the archive is not usable for us
	var entry *File
	for _, e := range r.Files {
		if e.HashedPath == rootPathHash {
			entry = e
			entry.Name = ""
			break
		} else if e.HashedPath == localeRootPathHash {
			entry = e
			entry.Name = "locale"
			break
		}
	}

	if entry == nil ||
		(entry.ZSize == 0 && entry.Size == 0) ||
		(entry.Type != EntryTypeCompressedNames &&
			entry.Type != EntryTypeCompressedNamesCopy &&
			entry.Type != EntryTypeUncompressedNames &&
			entry.Type != EntryTypeUncompressedNamesCopy) {
		return errors.New("No root path entry found or root path empty")
	}

	return r.populateFileTree(entry)
}

func (r *Reader) populateFileTree(node *File) error {
	f, err := node.Open()
	if err != nil {
		return errors.Wrap(err, "Unable to open file")
	}
	defer f.Close()

	var entries []string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		entries = append(entries, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return errors.Wrap(err, "Unable to read from file")
	}

	for _, entry := range entries {
		var (
			hash    uint64
			recurse bool
		)

		if entry[0] == '*' {
			// Directory here
			recurse = true
			entry = entry[1:]
		}

		hash = b0rkhash.CityHash64([]byte(strings.TrimPrefix(path.Join(node.Name, entry), "/")))

		var next *File
		for _, rf := range r.Files {
			if rf.HashedPath == hash {
				next = rf
				break
			}
		}

		if next == nil {
			return errors.Errorf("Found missing reference: %s", path.Join(node.Name, entry))
		}

		next.Name = strings.TrimPrefix(path.Join(node.Name, entry), "/")
		if recurse {
			if err = r.populateFileTree(next); err != nil {
				return err
			}
		}

	}

	return nil
}
