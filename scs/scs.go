// Package scs contains a reader for SCS# archive files
package scs

import (
	"bytes"
	"compress/flate"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/Luzifer/scs-extract/b0rkhash"
)

const (
	flagIsDirectory  = 0x10
	supportedVersion = 0x2
	zipHeaderSize    = 0x2
)

type (
	// File represents a file inside the SCS# archive
	File struct {
		Name string

		CompressedSize uint32
		Hash           uint64
		IsCompressed   bool
		IsDirectory    bool
		Size           uint32

		archiveReader io.ReaderAt
		offset        uint64
	}

	// Reader contains a parser for the archive and after creation will
	// hold a list of files ready to be opened from the archive
	Reader struct {
		Files []*File

		header        fileHeader
		entryTable    []catalogEntry
		metadataTable map[uint32]catalogMetaEntry

		archiveReader io.ReaderAt
	}

	fileHeader struct {
		Magic                    [4]byte
		Version                  uint16
		Salt                     uint16
		HashMethod               [4]byte
		EntryCount               uint32
		EntryTableLength         uint32
		MetadataEntriesCount     uint32
		MetadataTableLength      uint32
		EntryTableStart          uint64
		MetadataTableStart       uint64
		SecurityDescriptorOffset uint32
		Platform                 byte
	}

	catalogEntry struct {
		Hash          uint64
		MetadataIndex uint32
		MetadataCount uint16
		Flags         uint16
	}

	catalogMetaEntry struct {
		Index          uint32
		Offset         uint64
		CompressedSize uint32
		Size           uint32
		Flags          byte

		IsDirectory  bool
		IsCompressed bool
	}

	catalogMetaEntryType byte
)

const (
	metaEntryTypeImage           catalogMetaEntryType = 1
	metaEntryTypeSample          catalogMetaEntryType = 2
	metaEntryTypeMipProxy        catalogMetaEntryType = 3
	metaEntryTypeInlineDirectory catalogMetaEntryType = 4
	metaEntryTypePlain           catalogMetaEntryType = 128
	metaEntryTypeDirectory       catalogMetaEntryType = 129
	metaEntryTypeMip0            catalogMetaEntryType = 130
	metaEntryTypeMip1            catalogMetaEntryType = 131
	metaEntryTypeMipTail         catalogMetaEntryType = 132
)

var (
	scsMagic      = []byte("SCS#")
	scsHashMethod = []byte("CITY")

	localeRootPathHash = b0rkhash.CityHash64([]byte("locale"))
	rootPathHash       = b0rkhash.CityHash64([]byte(""))
)

// NewReader opens the archive from the given io.ReaderAt and parses
// the header information
func NewReader(r io.ReaderAt) (out *Reader, err error) {
	// Read the header
	var header fileHeader
	if err = binary.Read(
		io.NewSectionReader(r, 0, int64(binary.Size(fileHeader{}))),
		binary.LittleEndian,
		&header,
	); err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}

	// Sanity checks
	if !bytes.Equal(header.Magic[:], scsMagic) {
		return nil, fmt.Errorf("unexpected magic header")
	}

	if !bytes.Equal(header.HashMethod[:], scsHashMethod) {
		return nil, fmt.Errorf("unexpected hash method")
	}

	if header.Version != supportedVersion {
		return nil, fmt.Errorf("unsupported archive version: %d", header.Version)
	}

	// Do the real parsing
	out = &Reader{
		archiveReader: r,
		header:        header,
	}

	if err = out.parseEntryTable(); err != nil {
		return nil, fmt.Errorf("parsing entry table: %w", err)
	}

	if err = out.parseMetadataTable(); err != nil {
		return nil, fmt.Errorf("parsing metadata table: %w", err)
	}

	for _, e := range out.entryTable {
		meta := out.metadataTable[e.MetadataIndex+uint32(e.MetadataCount)]
		f := File{
			CompressedSize: meta.CompressedSize,
			Hash:           e.Hash,
			IsCompressed:   meta.IsCompressed || (meta.Flags&flagIsDirectory) != 0,
			IsDirectory:    meta.IsDirectory,
			Size:           meta.Size,
			archiveReader:  r,
			offset:         meta.Offset,
		}

		out.Files = append(out.Files, &f)
	}

	return out, out.populateFileNames()
}

// Open opens the file for reading
func (f *File) Open() (io.ReadCloser, error) {
	var rc io.ReadCloser

	if f.IsCompressed {
		r := io.NewSectionReader(f.archiveReader, int64(f.offset+zipHeaderSize), int64(f.CompressedSize)) //#nosec:G115 // int64 wraps at 9EB - We don't have to care for a LONG time
		rc = flate.NewReader(r)
	} else {
		r := io.NewSectionReader(f.archiveReader, int64(f.offset), int64(f.Size)) //#nosec:G115 // int64 wraps at 9EB - We don't have to care for a LONG time
		rc = io.NopCloser(r)
	}

	return rc, nil
}

func (r *Reader) parseEntryTable() error {
	etReader, err := zlib.NewReader(io.NewSectionReader(
		r.archiveReader,
		int64(r.header.EntryTableStart), //#nosec:G115 // int64 wraps at 9EB - We don't have to care for a LONG time
		int64(r.header.EntryTableLength),
	))
	if err != nil {
		return fmt.Errorf("opening entry-table reader: %w", err)
	}
	defer etReader.Close() //nolint:errcheck

	for i := uint32(0); i < r.header.EntryCount; i++ {
		var e catalogEntry
		if err = binary.Read(etReader, binary.LittleEndian, &e); err != nil {
			return fmt.Errorf("reading entry: %w", err)
		}
		r.entryTable = append(r.entryTable, e)
	}

	sort.Slice(r.entryTable, func(i, j int) bool {
		return r.entryTable[i].MetadataIndex < r.entryTable[j].MetadataIndex
	})

	return nil
}

func (r *Reader) parseMetadataTable() error {
	r.metadataTable = make(map[uint32]catalogMetaEntry)

	mtReader, err := zlib.NewReader(io.NewSectionReader(
		r.archiveReader,
		int64(r.header.MetadataTableStart), //#nosec:G115 // int64 wraps at 9EB - We don't have to care for a LONG time
		int64(r.header.MetadataTableLength),
	))
	if err != nil {
		return fmt.Errorf("opening metadata-table reader: %w", err)
	}
	defer mtReader.Close() //nolint:errcheck

	for {
		var metaType metaEntryType
		if err = binary.Read(mtReader, binary.LittleEndian, &metaType); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("reading meta-type-header: %w", err)
		}

		var payload iMetaEntry
		switch metaType.Type {
		case metaEntryTypeDirectory:
			var p metaEntryDir
			if err = binary.Read(mtReader, binary.LittleEndian, &p); err != nil {
				return fmt.Errorf("reading dir definition: %w", err)
			}
			payload = metaEntry{t: metaType, p: p}

		case metaEntryTypePlain:
			var p metaEntryFile
			if err = binary.Read(mtReader, binary.LittleEndian, &p); err != nil {
				return fmt.Errorf("reading file definition: %w", err)
			}
			payload = metaEntry{t: metaType, p: p}

		case metaEntryTypeImage:
			var p metaEntryImage
			if err = binary.Read(mtReader, binary.LittleEndian, &p); err != nil {
				return fmt.Errorf("reading image definition: %w", err)
			}
			payload = metaEntry{t: metaType, p: p}

		default:
			return fmt.Errorf("unhandled file type: %v", metaType.Type)
		}

		var e catalogMetaEntry
		payload.Fill(&e)

		r.metadataTable[e.Index] = e
	}
}

func (r *Reader) populateFileNames() (err error) {
	// first seek root entry, without the archive is not usable for us
	var entry *File
	for _, f := range r.Files {
		if f.Hash == rootPathHash {
			entry = f
			entry.Name = ""
			break
		} else if f.Hash == localeRootPathHash {
			entry = f
			entry.Name = "locale"
			break
		}
	}

	if entry == nil {
		// We found no suitable entrypoint
		return fmt.Errorf("no root entry found")
	}

	if err = r.setFilenamesFromDir(entry); err != nil {
		return fmt.Errorf("setting filenames: %w", err)
	}

	return nil
}

func (r *Reader) setFilenamesFromDir(node *File) error {
	f, err := node.Open()
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close() //nolint:errcheck

	var entryCount uint32
	if err = binary.Read(f, binary.LittleEndian, &entryCount); err != nil {
		return fmt.Errorf("reading entry count: %w", err)
	}

	if entryCount == 0 {
		// Listing without any files
		return fmt.Errorf("no entries in directory listing")
	}

	stringLengths := make([]byte, entryCount)
	if err = binary.Read(f, binary.LittleEndian, &stringLengths); err != nil {
		return fmt.Errorf("reading string lengths: %w", err)
	}

	for i := uint32(0); i < entryCount; i++ {
		var (
			hash    uint64
			name    = make([]byte, stringLengths[i])
			recurse bool
		)

		if err = binary.Read(f, binary.LittleEndian, &name); err != nil {
			return fmt.Errorf("reading name: %w", err)
		}

		if name[0] == '/' {
			// Directory entry
			recurse = true
			name = name[1:]
		}

		hash = b0rkhash.CityHash64([]byte(strings.TrimPrefix(path.Join(node.Name, string(name)), "/")))

		var next *File
		for _, rf := range r.Files {
			if rf.Hash == hash {
				next = rf
				break
			}
		}

		if next == nil {
			return fmt.Errorf("reference to void: %s", path.Join(node.Name, string(name)))
		}

		next.Name = strings.TrimPrefix(path.Join(node.Name, string(name)), "/")
		if recurse {
			if err = r.setFilenamesFromDir(next); err != nil {
				return err
			}
		}
	}

	return nil
}
