package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"strings"
	"time"

	"smith/internal/source/model"
	"smith/internal/source/store"
)

type handoffFile struct {
	LoopID string `json:"loop_id"`
}

func main() {
	loopID := strings.TrimSpace(os.Getenv("SMITH_LOOP_ID"))
	if loopID == "" {
		log.Fatal("SMITH_LOOP_ID is required")
	}
	correlationID := strings.TrimSpace(os.Getenv("SMITH_CORRELATION_ID"))

	handoffPath := strings.TrimSpace(os.Getenv("SMITH_HANDOFF_PATH"))
	if handoffPath == "" {
		handoffPath = "/smith/handoff/latest.json"
	}

	ctx := context.Background()
	storeClient, err := store.New(ctx, splitCSV(os.Getenv("SMITH_ETCD_ENDPOINTS")), 5*time.Second)
	if err != nil {
		log.Fatalf("failed to connect etcd: %v", err)
	}
	defer func() { _ = storeClient.Close() }()

	payload, err := os.ReadFile(handoffPath)
	if err == nil {
		var h handoffFile
		if unmarshalErr := json.Unmarshal(payload, &h); unmarshalErr == nil {
			log.Printf("loaded handoff for loop_id=%s", h.LoopID)
		}
	} else {
		log.Printf("handoff not found at %s; continuing", handoffPath)
	}

	_ = storeClient.AppendJournal(ctx, model.JournalEntry{
		LoopID:        loopID,
		Phase:         "replica",
		Level:         "info",
		ActorType:     "replica",
		ActorID:       hostnameOr("smith-replica"),
		Message:       "replica execution started",
		CorrelationID: correlationID,
		Metadata:      executionImageMetadataFromEnv(),
	})

	time.Sleep(250 * time.Millisecond)

	state, found, getErr := storeClient.GetState(ctx, loopID)
	if getErr != nil {
		log.Fatalf("failed to read state: %v", getErr)
	}
	if !found {
		log.Fatalf("state not found for loop_id=%s", loopID)
	}
	if state.Record.State != model.LoopStateOverwriting {
		log.Fatalf("unexpected state for loop_id=%s: %s", loopID, state.Record.State)
	}

	next := state.Record
	next.State = model.LoopStateSynced
	next.Reason = "replica-complete"
	next.LockHolder = ""
	if _, putErr := storeClient.PutState(ctx, next, state.Revision); putErr != nil {
		if errors.Is(putErr, store.ErrRevisionMismatch) {
			log.Fatalf("state conflict finalizing loop_id=%s", loopID)
		}
		log.Fatalf("failed to finalize state: %v", putErr)
	}

	handoffMetadata := map[string]string{
		"executor": hostnameOr("smith-replica"),
	}
	for k, v := range executionImageMetadataFromEnv() {
		handoffMetadata[k] = v
	}

	_ = storeClient.AppendHandoff(ctx, model.Handoff{
		LoopID:           loopID,
		FinalDiffSummary: "replica completed autonomous cycle",
		ValidationState:  "passed",
		NextSteps:        "operator review optional",
		CorrelationID:    correlationID,
		Metadata:         handoffMetadata,
	})

	_ = storeClient.AppendJournal(ctx, model.JournalEntry{
		LoopID:        loopID,
		Phase:         "replica",
		Level:         "info",
		ActorType:     "replica",
		ActorID:       hostnameOr("smith-replica"),
		Message:       "replica execution completed",
		CorrelationID: correlationID,
		Metadata: map[string]string{
			"token_total":  "0",
			"token_prompt": "0",
			"token_output": "0",
			"cost_usd":     "0",
		},
	})

	log.Printf("smith-replica startup complete for loop_id=%s", loopID)
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{"http://127.0.0.1:2379"}
	}
	return out
}

func hostnameOr(fallback string) string {
	h, err := os.Hostname()
	if err != nil || strings.TrimSpace(h) == "" {
		return fallback
	}
	return h
}

func executionImageMetadataFromEnv() map[string]string {
	out := map[string]string{}
	ref := strings.TrimSpace(os.Getenv("SMITH_EXECUTION_IMAGE_REF"))
	if ref != "" {
		out["execution_image_ref"] = ref
	}
	source := strings.TrimSpace(os.Getenv("SMITH_EXECUTION_IMAGE_SOURCE"))
	if source != "" {
		out["execution_image_source"] = source
	}
	digest := strings.TrimSpace(os.Getenv("SMITH_EXECUTION_IMAGE_DIGEST"))
	if digest != "" {
		out["execution_image_digest"] = digest
	}
	pullPolicy := strings.TrimSpace(os.Getenv("SMITH_EXECUTION_IMAGE_PULL_POLICY"))
	if pullPolicy != "" {
		out["execution_image_pull_policy"] = pullPolicy
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
