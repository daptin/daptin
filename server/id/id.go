package daptinid

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/json-iterator/go"
	log "github.com/sirupsen/logrus"
	"unsafe"
)

type DaptinReferenceId [16]byte

type DaptinReferenceEncoder struct{}

func (dr *DaptinReferenceId) Scan(value interface{}) error {
	asBytes, ok := value.([]uint8)
	if !ok {
		asStr, isStr := value.(string)
		if isStr {
			asUUid, err := uuid.Parse(asStr)
			if err != nil {
				return err
			}
			asBytes = asUUid[:]
		} else {
			return fmt.Errorf("value couldne be parsed at []uint8 => [%v] failed", value)
		}
	}
	// Convert asBytes into the appropriate type for DaptinReferenceId
	// You may need to interpret the bytes accordingly (e.g., converting them to a string, parsing them, etc.)
	copy(dr[:], asBytes)
	return nil
}

func (c DaptinReferenceEncoder) Encode(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	src := *((*DaptinReferenceId)(ptr))

	//attachVal, _ := stream.Attachment.(DaptinReferenceId)
	stream.WriteRaw(`"`)
	stream.WriteRaw(src.String())
	stream.WriteRaw(`"`)
}

func (c DaptinReferenceEncoder) IsEmpty(ptr unsafe.Pointer) bool {
	return false
}

func (d DaptinReferenceId) String() string {
	x, _ := uuid.FromBytes(d[:])
	return x.String()
}

func (d DaptinReferenceId) MarshalJSON() ([]byte, error) {
	x, _ := uuid.FromBytes(d[:])
	return []byte("\"" + x.String() + "\""), nil
}

func (d *DaptinReferenceId) UnmarshalJSON(val []byte) error {
	s := string(val)
	if len(s) > 2 {
		if s[0] == '"' && s[len(s)-1] == '"' {
			s = s[1 : len(s)-1] // unquoted s
		}
		if s[0] == '\'' && s[len(s)-1] == '\'' {
			s = s[1 : len(s)-1] // unquoted s
		}
	}
	x, err := uuid.Parse(s)
	if err != nil {
		return err
	}
	copy(d[:], x[:16])
	return nil
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

func InterfaceToDIR(valueToConvert interface{}) DaptinReferenceId {
	if valueToConvert == nil {
		return NullReferenceId
	}
	valueAsDir, isDir := valueToConvert.(DaptinReferenceId)
	if isDir {
		return valueAsDir
	}

	asUuid, isUuid := valueToConvert.(uuid.UUID)
	if isUuid {
		return DaptinReferenceId(asUuid)
	}

	asStr, isStr := valueToConvert.(string)
	if isStr {
		if asStr == "<nil>" {
			log.Printf("[100] No reference id is <nil> target store")
			return NullReferenceId
		} else {
			oauthTokenId, err := uuid.Parse(asStr)
			if err != nil {
				log.Errorf("[105] Failed to parse string as uuid [%s]: %v", asStr, err)
				return NullReferenceId
			}
			return DaptinReferenceId(oauthTokenId)
		}
	}
	asUint8Array, isIntArray := valueToConvert.([]uint8)
	if isIntArray {
		uuidFromBytes, err := uuid.FromBytes([]byte(asUint8Array))
		if err != nil {
			log.Printf("[100] reference id is invalid")
			return NullReferenceId
		}
		return DaptinReferenceId(uuidFromBytes)
	}
	return NullReferenceId
}
