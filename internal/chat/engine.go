package chat

import (
	"context"
)

type Engine interface {
	Stream(ctx context.Context, session *Session, message string, events chan<- ChatEvent) error
}
