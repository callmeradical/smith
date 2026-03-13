package journalpolicy

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidPolicy = errors.New("invalid journal policy")
)

type RetentionMode string

const (
	RetentionKeepForever RetentionMode = "keep_forever"
	RetentionTTL         RetentionMode = "ttl"
)

type ArchiveMode string

const (
	ArchiveNone ArchiveMode = "none"
	ArchiveS3   ArchiveMode = "s3"
)

type Policy struct {
	RetentionMode RetentionMode
	RetentionTTL  time.Duration
	ArchiveMode   ArchiveMode
	ArchiveBucket string
}

func DefaultPolicy() Policy {
	return Policy{
		RetentionMode: RetentionKeepForever,
		RetentionTTL:  0,
		ArchiveMode:   ArchiveNone,
	}
}

func (p Policy) Validate() error {
	switch p.RetentionMode {
	case RetentionKeepForever:
		if p.RetentionTTL != 0 {
			return fmt.Errorf("%w: retention ttl must be zero when retention mode is keep_forever", ErrInvalidPolicy)
		}
	case RetentionTTL:
		if p.RetentionTTL <= 0 {
			return fmt.Errorf("%w: retention ttl must be > 0 when retention mode is ttl", ErrInvalidPolicy)
		}
	default:
		return fmt.Errorf("%w: unsupported retention mode %q", ErrInvalidPolicy, p.RetentionMode)
	}

	switch p.ArchiveMode {
	case ArchiveNone:
		if strings.TrimSpace(p.ArchiveBucket) != "" {
			return fmt.Errorf("%w: archive bucket requires archive mode s3", ErrInvalidPolicy)
		}
	case ArchiveS3:
		if strings.TrimSpace(p.ArchiveBucket) == "" {
			return fmt.Errorf("%w: archive mode s3 requires archive bucket", ErrInvalidPolicy)
		}
	default:
		return fmt.Errorf("%w: unsupported archive mode %q", ErrInvalidPolicy, p.ArchiveMode)
	}
	return nil
}
