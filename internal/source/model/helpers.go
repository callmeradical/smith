package model

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

var ErrInvalidLoopID = errors.New("loop id is required")

type KVPair struct {
	Key   string
	Value []byte
}

type WatchEvent struct {
	Key   string
	Value []byte
}

type KVStore interface {
	Put(ctx context.Context, key string, value []byte) error
	Get(ctx context.Context, key string) (value []byte, found bool, err error)
	ListPrefix(ctx context.Context, prefix string) ([]KVPair, error)
	WatchPrefix(ctx context.Context, prefix string) <-chan WatchEvent
}

type StateWatchEvent struct {
	LoopID string
	State  StateRecord
}

func PutAnomaly(ctx context.Context, kv KVStore, anomaly Anomaly) error {
	if strings.TrimSpace(anomaly.ID) == "" {
		return ErrInvalidLoopID
	}
	anomaly.SchemaVersion = SchemaVersion
	now := time.Now().UTC()
	if anomaly.CreatedAt.IsZero() {
		anomaly.CreatedAt = now
	}
	anomaly.UpdatedAt = now
	payload, err := json.Marshal(anomaly)
	if err != nil {
		return err
	}
	return kv.Put(ctx, AnomalyKey(anomaly.ID), payload)
}

func GetAnomaly(ctx context.Context, kv KVStore, loopID string) (Anomaly, bool, error) {
	if strings.TrimSpace(loopID) == "" {
		return Anomaly{}, false, ErrInvalidLoopID
	}
	payload, found, err := kv.Get(ctx, AnomalyKey(loopID))
	if err != nil || !found {
		return Anomaly{}, found, err
	}
	var anomaly Anomaly
	if err := json.Unmarshal(payload, &anomaly); err != nil {
		return Anomaly{}, false, err
	}
	return anomaly, true, nil
}

func PutState(ctx context.Context, kv KVStore, state StateRecord) error {
	if strings.TrimSpace(state.LoopID) == "" {
		return ErrInvalidLoopID
	}
	state.SchemaVersion = SchemaVersion
	state.UpdatedAt = time.Now().UTC()
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return kv.Put(ctx, StateKey(state.LoopID), payload)
}

func GetState(ctx context.Context, kv KVStore, loopID string) (StateRecord, bool, error) {
	if strings.TrimSpace(loopID) == "" {
		return StateRecord{}, false, ErrInvalidLoopID
	}
	payload, found, err := kv.Get(ctx, StateKey(loopID))
	if err != nil || !found {
		return StateRecord{}, found, err
	}
	state, err := DecodeStateRecord(payload)
	if err != nil {
		return StateRecord{}, false, err
	}
	return state, true, nil
}

func AppendJournal(ctx context.Context, kv KVStore, entry JournalEntry) error {
	if strings.TrimSpace(entry.LoopID) == "" {
		return ErrInvalidLoopID
	}
	entry.SchemaVersion = SchemaVersion
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}
	if entry.Sequence <= 0 {
		next, err := nextSequence(ctx, kv, JournalPrefix(entry.LoopID))
		if err != nil {
			return err
		}
		entry.Sequence = next
	}
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return kv.Put(ctx, JournalKey(entry.LoopID, entry.Sequence), payload)
}

func ListJournal(ctx context.Context, kv KVStore, loopID string) ([]JournalEntry, error) {
	if strings.TrimSpace(loopID) == "" {
		return nil, ErrInvalidLoopID
	}
	pairs, err := kv.ListPrefix(ctx, JournalPrefix(loopID)+"/")
	if err != nil {
		return nil, err
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].Key < pairs[j].Key })
	out := make([]JournalEntry, 0, len(pairs))
	for _, pair := range pairs {
		var entry JournalEntry
		if err := json.Unmarshal(pair.Value, &entry); err != nil {
			continue
		}
		out = append(out, entry)
	}
	return out, nil
}

func AppendHandoff(ctx context.Context, kv KVStore, handoff Handoff) error {
	if strings.TrimSpace(handoff.LoopID) == "" {
		return ErrInvalidLoopID
	}
	handoff.SchemaVersion = SchemaVersion
	if handoff.Timestamp.IsZero() {
		handoff.Timestamp = time.Now().UTC()
	}
	if handoff.Sequence <= 0 {
		next, err := nextSequence(ctx, kv, HandoffPrefix(handoff.LoopID))
		if err != nil {
			return err
		}
		handoff.Sequence = next
	}
	payload, err := json.Marshal(handoff)
	if err != nil {
		return err
	}
	return kv.Put(ctx, HandoffKey(handoff.LoopID, handoff.Sequence), payload)
}

func WatchStates(ctx context.Context, kv KVStore) <-chan StateWatchEvent {
	out := make(chan StateWatchEvent)
	in := kv.WatchPrefix(ctx, PrefixState+"/")
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-in:
				if !ok {
					return
				}
				state, err := DecodeStateRecord(event.Value)
				if err != nil {
					continue
				}
				select {
				case <-ctx.Done():
					return
				case out <- StateWatchEvent{LoopID: state.LoopID, State: state}:
				}
			}
		}
	}()
	return out
}

func nextSequence(ctx context.Context, kv KVStore, prefix string) (int64, error) {
	pairs, err := kv.ListPrefix(ctx, prefix+"/")
	if err != nil {
		return 0, err
	}
	var max int64
	for _, pair := range pairs {
		base := pair.Key[strings.LastIndex(pair.Key, "/")+1:]
		if len(base) == 0 {
			continue
		}
		var seq int64
		for _, ch := range []byte(base) {
			if ch < '0' || ch > '9' {
				seq = 0
				break
			}
			seq = seq*10 + int64(ch-'0')
		}
		if seq > max {
			max = seq
		}
	}
	return max + 1, nil
}
