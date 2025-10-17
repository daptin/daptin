package cache

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sync"
	"time"
)

// Buffer pool for reducing allocations during marshaling/unmarshaling
var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

// CachedFile represents a cached file with its metadata
type CachedFile struct {
	Data       []byte
	ETag       string
	Modtime    time.Time
	MimeType   string
	Path       string
	Size       int
	GzipData   []byte    // Pre-compressed version for text files
	IsDownload bool      // Whether file should be downloaded or displayed inline
	ExpiresAt  time.Time // When this cache entry expires
	FileStat   FileStat  // File stat information for validation
}

// MarshalBinary implements encoding.BinaryMarshaler interface for Olric compatibility
// Custom binary format without using gob or other encoders
func (cf *CachedFile) MarshalBinary() ([]byte, error) {
	// Get a buffer from the pool
	buf := bufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bufferPool.Put(buf)
	}()

	// Calculate the approximate size needed for the buffer
	bufSize := 8 + // Size for Data length
		len(cf.Data) + // Size for Data
		4 + // Size for ETag length
		len(cf.ETag) + // Size for ETag
		8 + // Size for ModTime (Unix timestamp)
		4 + // Size for MimeType length
		len(cf.MimeType) + // Size for MimeType
		4 + // Size for Path length
		len(cf.Path) + // Size for Path
		4 + // Size for Size int
		8 + // Size for GzipData length
		len(cf.GzipData) + // Size for GzipData
		1 + // Size for IsDownload bool
		8 + // Size for ExpiresAt (Unix timestamp)
		8 + // Size for FileStat.ModTime (Unix timestamp)
		8 + // Size for FileStat.Size (int64)
		1 // Size for FileStat.Exists (bool)

	// Grow the buffer if needed
	if buf.Cap() < bufSize {
		buf.Grow(bufSize)
	}

	// Write Data length and Data
	binary.Write(buf, binary.LittleEndian, int64(len(cf.Data)))
	buf.Write(cf.Data)

	// Write ETag length and ETag
	binary.Write(buf, binary.LittleEndian, int32(len(cf.ETag)))
	buf.WriteString(cf.ETag)

	// Write ModTime as Unix timestamp
	binary.Write(buf, binary.LittleEndian, cf.Modtime.Unix())

	// Write MimeType length and MimeType
	binary.Write(buf, binary.LittleEndian, int32(len(cf.MimeType)))
	buf.WriteString(cf.MimeType)

	// Write Path length and Path
	binary.Write(buf, binary.LittleEndian, int32(len(cf.Path)))
	buf.WriteString(cf.Path)

	// Write Size
	binary.Write(buf, binary.LittleEndian, int32(cf.Size))

	// Write GzipData length and GzipData
	binary.Write(buf, binary.LittleEndian, int64(len(cf.GzipData)))
	buf.Write(cf.GzipData)

	// Write IsDownload
	if cf.IsDownload {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}

	// Write ExpiresAt as Unix timestamp
	binary.Write(buf, binary.LittleEndian, cf.ExpiresAt.Unix())

	// Write FileStat
	binary.Write(buf, binary.LittleEndian, cf.FileStat.ModTime.Unix())
	binary.Write(buf, binary.LittleEndian, cf.FileStat.Size)
	if cf.FileStat.Exists {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}

	// Make a copy of the bytes to return (since we're returning the buffer to the pool)
	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())
	return result, nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler interface for Olric compatibility
// Custom binary format without using gob or other encoders
func (cf *CachedFile) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)

	// Read Data length and Data
	var dataLen int64
	if err := binary.Read(buf, binary.LittleEndian, &dataLen); err != nil {
		return fmt.Errorf("failed to read Data length: %v", err)
	}
	cf.Data = make([]byte, dataLen)
	if _, err := buf.Read(cf.Data); err != nil {
		return fmt.Errorf("failed to read Data: %v", err)
	}

	// Read ETag length and ETag
	var etagLen int32
	if err := binary.Read(buf, binary.LittleEndian, &etagLen); err != nil {
		return fmt.Errorf("failed to read ETag length: %v", err)
	}
	etagBytes := make([]byte, etagLen)
	if _, err := buf.Read(etagBytes); err != nil {
		return fmt.Errorf("failed to read ETag: %v", err)
	}
	cf.ETag = string(etagBytes)

	// Read ModTime
	var modTimeUnix int64
	if err := binary.Read(buf, binary.LittleEndian, &modTimeUnix); err != nil {
		return fmt.Errorf("failed to read ModTime: %v", err)
	}
	cf.Modtime = time.Unix(modTimeUnix, 0)

	// Read MimeType length and MimeType
	var mimeTypeLen int32
	if err := binary.Read(buf, binary.LittleEndian, &mimeTypeLen); err != nil {
		return fmt.Errorf("failed to read MimeType length: %v", err)
	}
	mimeTypeBytes := make([]byte, mimeTypeLen)
	if _, err := buf.Read(mimeTypeBytes); err != nil {
		return fmt.Errorf("failed to read MimeType: %v", err)
	}
	cf.MimeType = string(mimeTypeBytes)

	// Read Path length and Path
	var pathLen int32
	if err := binary.Read(buf, binary.LittleEndian, &pathLen); err != nil {
		return fmt.Errorf("failed to read Path length: %v", err)
	}
	pathBytes := make([]byte, pathLen)
	if _, err := buf.Read(pathBytes); err != nil {
		return fmt.Errorf("failed to read Path: %v", err)
	}
	cf.Path = string(pathBytes)

	// Read Size
	var size int32
	if err := binary.Read(buf, binary.LittleEndian, &size); err != nil {
		return fmt.Errorf("failed to read Size: %v", err)
	}
	cf.Size = int(size)

	// Read GzipData length and GzipData
	var gzipDataLen int64
	if err := binary.Read(buf, binary.LittleEndian, &gzipDataLen); err != nil {
		return fmt.Errorf("failed to read GzipData length: %v", err)
	}
	cf.GzipData = make([]byte, gzipDataLen)
	if _, err := buf.Read(cf.GzipData); err != nil {
		return fmt.Errorf("failed to read GzipData: %v", err)
	}

	// Read IsDownload
	isDownloadByte, err := buf.ReadByte()
	if err != nil {
		return fmt.Errorf("failed to read IsDownload: %v", err)
	}
	cf.IsDownload = isDownloadByte == 1

	// Read ExpiresAt
	var expiresAtUnix int64
	if err := binary.Read(buf, binary.LittleEndian, &expiresAtUnix); err != nil {
		return fmt.Errorf("failed to read ExpiresAt: %v", err)
	}
	cf.ExpiresAt = time.Unix(expiresAtUnix, 0)

	// Read FileStat
	var fileStatModTimeUnix int64
	if err := binary.Read(buf, binary.LittleEndian, &fileStatModTimeUnix); err != nil {
		return fmt.Errorf("failed to read FileStat.ModTime: %v", err)
	}
	cf.FileStat.ModTime = time.Unix(fileStatModTimeUnix, 0)

	if err := binary.Read(buf, binary.LittleEndian, &cf.FileStat.Size); err != nil {
		return fmt.Errorf("failed to read FileStat.Size: %v", err)
	}

	existsByte, err := buf.ReadByte()
	if err != nil {
		return fmt.Errorf("failed to read FileStat.Exists: %v", err)
	}
	cf.FileStat.Exists = existsByte == 1

	return nil
}
