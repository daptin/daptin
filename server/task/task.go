package task

import daptinid "github.com/daptin/daptin/server/id"

type Task struct {
	Id                int64
	ReferenceId       daptinid.DaptinReferenceId
	Schedule          string
	Active            bool
	Name              string
	Attributes        map[string]interface{}
	AsUserEmail       string                     // Schema input, normalized to the persisted relation during synchronization.
	AsUserReferenceId daptinid.DaptinReferenceId // Runtime identity from the persisted relation.
	ActionName        string
	EntityName        string
	JobType           string
	AttributesJson    string
}
