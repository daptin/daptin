package daptinid

import (
	"errors"
	"github.com/google/uuid"
	"github.com/json-iterator/go"
	"unsafe"
)

type DaptinReferenceId [16]byte

type DaptinReferenceEncoder struct{}

func (c *DaptinReferenceEncoder) Encode(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	src := *((*DaptinReferenceId)(ptr))

	//attachVal, _ := stream.Attachment.(DaptinReferenceId)
	stream.WriteRaw(`"`)
	stream.WriteRaw(src.String())
	stream.WriteRaw(`"`)
}

func (c *DaptinReferenceEncoder) IsEmpty(ptr unsafe.Pointer) bool {
	return false
}

func (d DaptinReferenceId) String() string {
	x, _ := uuid.FromBytes(d[:])
	return x.String()
}

func (d DaptinReferenceId) MarshalBinary() (data []byte, err error) {
	// Return a copy of the 16-byte array
	return d[:], nil
}

func (d *DaptinReferenceId) UnmarshalBinary(data []byte) error {
	if len(data) != 16 {
		return errors.New("invalid data length: expected exactly 16 bytes")
	}
	// Copy data into the DaptinReferenceId array
	copy(d[:], data)
	return nil
}

var NullReferenceId DaptinReferenceId
