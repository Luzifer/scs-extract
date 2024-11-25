package scs

const offsetBlockSize = 16 // byte

type (
	iMetaEntry interface {
		Fill(*catalogMetaEntry)
	}

	metaEntry struct {
		t metaEntryType
		p iMetaEntry
	}

	metaEntryBrokenOctal      [3]byte
	metaEntryBrokenOctalImage [4]byte

	metaEntryType struct {
		Index metaEntryBrokenOctal
		Type  catalogMetaEntryType
	}

	metaEntryDir struct {
		CompressedSize metaEntryBrokenOctal
		Flags          byte
		Size           uint32
		Unknown2       uint32
		OffsetBlock    uint32
	}

	metaEntryFile struct {
		CompressedSize metaEntryBrokenOctal
		Flags          byte
		Size           uint32
		Unknown2       uint32
		OffsetBlock    uint32
	}

	metaEntryImage struct {
		Unknown1       uint64
		TextureWidth   uint16
		TextureHeight  uint16
		ImgFlags       uint32
		SampleFlags    uint32
		CompressedSize metaEntryBrokenOctalImage
		Unknown3       [8]byte
		OffsetBlock    uint32
	}
)

func (m metaEntry) Fill(c *catalogMetaEntry) {
	c.Index = m.t.Index.Uint32()
	m.p.Fill(c)
}

func (m metaEntryDir) Fill(c *catalogMetaEntry) {
	c.IsDirectory = true

	c.Offset = uint64(m.OffsetBlock) * offsetBlockSize
	c.CompressedSize = m.CompressedSize.Uint32()
	c.Size = m.Size
	c.Flags = m.Flags
}

func (m metaEntryFile) Fill(c *catalogMetaEntry) {
	c.Offset = uint64(m.OffsetBlock) * offsetBlockSize
	c.CompressedSize = m.CompressedSize.Uint32()
	c.Size = m.Size
	c.Flags = m.Flags
}

func (m metaEntryImage) Fill(c *catalogMetaEntry) {
	c.Offset = uint64(m.OffsetBlock) * offsetBlockSize
	c.CompressedSize = m.CompressedSize.Uint32()
	c.Size = m.CompressedSize.Uint32()
	c.IsCompressed = m.CompressedSize.IsCompressed()
}

func (m metaEntryBrokenOctal) Uint32() uint32 {
	return uint32(m[0]) + uint32(m[1])<<8 + uint32(m[2])<<16
}

func (m metaEntryBrokenOctalImage) IsCompressed() bool {
	return (m[3] & 0xf0) != 0 //nolint:mnd
}

func (m metaEntryBrokenOctalImage) Uint32() uint32 {
	return uint32(m[0]) + uint32(m[1])<<8 + uint32(m[2])<<16 + uint32(m[3])<<24
}
