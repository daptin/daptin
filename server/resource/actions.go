package resource

import (
	"bytes"
	"encoding/binary"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/google/uuid"
)

// ActionRow represents an action instance on the database
// Can be retrieved using GetActionByName
type ActionRow struct {
	Name             string
	Label            string
	OnType           string
	InstanceOptional bool `db:"instance_optional"`
	ReferenceId      daptinid.DaptinReferenceId
	ActionSchema     string `db:"action_schema"`
}

// MarshalBinary encodes the struct into binary format manually
func (e ActionRow) MarshalBinary() (data []byte, err error) {
	buffer := new(bytes.Buffer)

	// Encode Name
	if err := encodeString(buffer, e.Name); err != nil {
		return nil, err
	}

	// Encode Label
	if err := encodeString(buffer, e.Label); err != nil {
		return nil, err
	}

	// Encode OnType
	if err := encodeString(buffer, e.OnType); err != nil {
		return nil, err
	}

	// Encode InstanceOptional
	if err := binary.Write(buffer, binary.BigEndian, e.InstanceOptional); err != nil {
		return nil, err
	}

	// Encode ReferenceId
	if err := encodeString(buffer, e.ReferenceId.String()); err != nil {
		return nil, err
	}

	// Encode ActionSchema
	if err := encodeString(buffer, e.ActionSchema); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

// UnmarshalBinary decodes the data into the struct using manual binary decoding
func (e *ActionRow) UnmarshalBinary(data []byte) error {
	buffer := bytes.NewBuffer(data)

	// Decode Name
	if name, err := decodeString(buffer); err != nil {
		return err
	} else {
		e.Name = name
	}

	// Decode Label
	if label, err := decodeString(buffer); err != nil {
		return err
	} else {
		e.Label = label
	}

	// Decode OnType
	if onType, err := decodeString(buffer); err != nil {
		return err
	} else {
		e.OnType = onType
	}

	// Decode InstanceOptional
	if err := binary.Read(buffer, binary.BigEndian, &e.InstanceOptional); err != nil {
		return err
	}

	// Decode ReferenceId
	if referenceId, err := decodeString(buffer); err != nil {
		return err
	} else {
		e.ReferenceId = daptinid.DaptinReferenceId(uuid.MustParse(referenceId))
	}

	// Decode ActionSchema
	if actionSchema, err := decodeString(buffer); err != nil {
		return err
	} else {
		e.ActionSchema = actionSchema
	}

	return nil
}
