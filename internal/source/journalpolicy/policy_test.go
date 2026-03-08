package journalpolicy

import (
	"testing"
	"time"
)

func TestDefaultPolicyValidates(t *testing.T) {
	p := DefaultPolicy()
	if err := p.Validate(); err != nil {
		t.Fatalf("default policy should validate: %v", err)
	}
}

func TestValidateRejectsInvalidRetentionMode(t *testing.T) {
	p := DefaultPolicy()
	p.RetentionMode = "windowed"
	if err := p.Validate(); err == nil {
		t.Fatal("expected invalid retention mode error")
	}
}

func TestValidateRejectsTTLWithoutDuration(t *testing.T) {
	p := DefaultPolicy()
	p.RetentionMode = RetentionTTL
	p.RetentionTTL = 0
	if err := p.Validate(); err == nil {
		t.Fatal("expected invalid ttl error")
	}
}

func TestValidateRejectsArchiveBucketWithoutS3(t *testing.T) {
	p := DefaultPolicy()
	p.ArchiveBucket = "smith-archive"
	if err := p.Validate(); err == nil {
		t.Fatal("expected invalid archive bucket error")
	}
}

func TestValidateRequiresBucketForS3(t *testing.T) {
	p := DefaultPolicy()
	p.RetentionMode = RetentionTTL
	p.RetentionTTL = 7 * 24 * time.Hour
	p.ArchiveMode = ArchiveS3
	p.ArchiveBucket = "smith-archive"
	if err := p.Validate(); err != nil {
		t.Fatalf("expected valid policy, got %v", err)
	}
}
