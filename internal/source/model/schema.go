package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	SchemaVersionV1Alpha1 = "v1alpha1"
	SchemaVersionV1       = "v1"
)

var ErrUnsupportedSchemaVersion = errors.New("unsupported schema version")

func CurrentSchemaVersion() string {
	return SchemaVersion
}

func IsReadableSchemaVersion(version string) bool {
	switch normalizeVersion(version) {
	case SchemaVersionV1, SchemaVersionV1Alpha1:
		return true
	default:
		return false
	}
}

func IsWritableSchemaVersion(version string) bool {
	return normalizeVersion(version) == CurrentSchemaVersion()
}

// DecodeStateRecord accepts both the current state schema and one legacy
// schema revision to support rolling upgrades and in-flight anomalies.
func DecodeStateRecord(payload []byte) (StateRecord, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(payload, &raw); err != nil {
		return StateRecord{}, err
	}

	version := schemaVersionFromRaw(raw)
	switch normalizeVersion(version) {
	case SchemaVersionV1:
		var current StateRecord
		if err := json.Unmarshal(payload, &current); err != nil {
			return StateRecord{}, err
		}
		current.SchemaVersion = SchemaVersionV1
		return current, nil
	case SchemaVersionV1Alpha1:
		var legacy stateRecordV1Alpha1
		if err := json.Unmarshal(payload, &legacy); err != nil {
			return StateRecord{}, err
		}
		return legacy.ToCurrent(), nil
	default:
		return StateRecord{}, fmt.Errorf("%w: %q", ErrUnsupportedSchemaVersion, version)
	}
}

func normalizeVersion(version string) string {
	v := strings.TrimSpace(strings.ToLower(version))
	if v == "" {
		return SchemaVersionV1Alpha1
	}
	return v
}

func schemaVersionFromRaw(raw map[string]json.RawMessage) string {
	rawVersion, ok := raw["schema_version"]
	if !ok {
		return ""
	}
	var version string
	if err := json.Unmarshal(rawVersion, &version); err != nil {
		return ""
	}
	return version
}

type stateRecordV1Alpha1 struct {
	LoopID           string     `json:"loop_id"`
	Status           LoopState  `json:"status"`
	Attempt          int        `json:"attempt"`
	Reason           string     `json:"reason,omitempty"`
	WorkerJobName    string     `json:"worker_job_name,omitempty"`
	LockHolder       string     `json:"lock_holder,omitempty"`
	ObservedRevision int64      `json:"observed_revision"`
	UpdatedAt        time.Time  `json:"updated_at"`
	LastHeartbeatAt  *time.Time `json:"last_heartbeat_at,omitempty"`
	CorrelationID    string     `json:"correlation_id"`
	SchemaVersion    string     `json:"schema_version,omitempty"`
}

func (s stateRecordV1Alpha1) ToCurrent() StateRecord {
	return StateRecord{
		LoopID:           s.LoopID,
		State:            s.Status,
		Attempt:          s.Attempt,
		Reason:           s.Reason,
		WorkerJobName:    s.WorkerJobName,
		LockHolder:       s.LockHolder,
		ObservedRevision: s.ObservedRevision,
		UpdatedAt:        s.UpdatedAt,
		LastHeartbeatAt:  s.LastHeartbeatAt,
		CorrelationID:    s.CorrelationID,
		SchemaVersion:    SchemaVersionV1,
	}
}
