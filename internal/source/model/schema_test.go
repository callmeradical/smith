package model

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestSchemaVersionReadWriteCompatibility(t *testing.T) {
	if !IsReadableSchemaVersion(SchemaVersionV1) {
		t.Fatalf("expected %s to be readable", SchemaVersionV1)
	}
	if !IsReadableSchemaVersion(SchemaVersionV1Alpha1) {
		t.Fatalf("expected %s to be readable", SchemaVersionV1Alpha1)
	}
	if IsReadableSchemaVersion("v2") {
		t.Fatal("expected v2 to be unreadable")
	}

	if !IsWritableSchemaVersion(SchemaVersionV1) {
		t.Fatalf("expected %s to be writable", SchemaVersionV1)
	}
	if IsWritableSchemaVersion(SchemaVersionV1Alpha1) {
		t.Fatalf("expected %s to be write-rejected", SchemaVersionV1Alpha1)
	}
}

func TestDecodeStateRecordCurrentSchema(t *testing.T) {
	input := []byte(`{
		"loop_id":"loop-1",
		"state":"unresolved",
		"attempt":2,
		"observed_revision":8,
		"correlation_id":"corr-1",
		"schema_version":"v1"
	}`)

	got, err := DecodeStateRecord(input)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if got.State != LoopStateUnresolved {
		t.Fatalf("expected unresolved state, got %q", got.State)
	}
	if got.SchemaVersion != SchemaVersionV1 {
		t.Fatalf("expected schema version %q, got %q", SchemaVersionV1, got.SchemaVersion)
	}
}

func TestDecodeStateRecordMigratesV1Alpha1(t *testing.T) {
	input := []byte(`{
		"loop_id":"loop-2",
		"status":"overwriting",
		"attempt":1,
		"observed_revision":9,
		"correlation_id":"corr-2",
		"schema_version":"v1alpha1"
	}`)

	got, err := DecodeStateRecord(input)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if got.State != LoopStateOverwriting {
		t.Fatalf("expected overwriting state after migration, got %q", got.State)
	}
	if got.SchemaVersion != SchemaVersionV1 {
		t.Fatalf("expected migrated schema version %q, got %q", SchemaVersionV1, got.SchemaVersion)
	}
}

func TestDecodeStateRecordMissingVersionDefaultsToLegacyMigration(t *testing.T) {
	input := []byte(`{
		"loop_id":"loop-3",
		"status":"unresolved",
		"attempt":0,
		"observed_revision":1,
		"correlation_id":"corr-3"
	}`)

	got, err := DecodeStateRecord(input)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if got.State != LoopStateUnresolved {
		t.Fatalf("expected unresolved state after default migration, got %q", got.State)
	}
	if got.SchemaVersion != SchemaVersionV1 {
		t.Fatalf("expected migrated schema version %q, got %q", SchemaVersionV1, got.SchemaVersion)
	}
}

func TestDecodeStateRecordRejectsUnsupportedSchema(t *testing.T) {
	raw := map[string]any{
		"loop_id":        "loop-4",
		"state":          "unresolved",
		"attempt":        1,
		"schema_version": "v2",
	}
	input, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	_, err = DecodeStateRecord(input)
	if err == nil {
		t.Fatal("expected unsupported schema error")
	}
	if !errors.Is(err, ErrUnsupportedSchemaVersion) {
		t.Fatalf("expected ErrUnsupportedSchemaVersion, got %v", err)
	}
}
