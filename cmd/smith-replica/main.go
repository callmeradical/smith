package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

type startupContext struct {
	Anomaly      model.Anomaly
	PriorHandoff *model.Handoff
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

	loadedFromFile, fileErr := readHandoffFile(handoffPath)
	if fileErr != nil {
		log.Printf("failed to parse handoff file at %s: %v", handoffPath, fileErr)
	} else if loadedFromFile != nil {
		log.Printf("loaded mounted handoff for loop_id=%s", loadedFromFile.LoopID)
	} else {
		log.Printf("handoff not found at %s; continuing", handoffPath)
	}

	startup, startupErr := loadStartupContext(ctx, storeClient, loopID)
	if startupErr != nil {
		recordStartupFailure(ctx, storeClient, loopID, correlationID, startupErr)
		log.Fatalf("startup context load failed: %v", startupErr)
	}
	if startup.PriorHandoff != nil {
		log.Printf("loaded prior handoff sequence=%d for loop_id=%s", startup.PriorHandoff.Sequence, loopID)
	} else {
		log.Printf("no prior handoff found in etcd for loop_id=%s; treating as first-run", loopID)
	}

	workspace := strings.TrimSpace(os.Getenv("SMITH_WORKSPACE"))
	if workspace == "" {
		workspace = "/workspace"
	}
	envMeta, setupErr := setupLoopEnvironment(ctx, startup.Anomaly.Environment, workspace, commandRunner{})
	if setupErr != nil {
		_ = storeClient.AppendJournal(ctx, model.JournalEntry{
			LoopID:        loopID,
			Phase:         "environment",
			Level:         "error",
			ActorType:     "replica",
			ActorID:       hostnameOr("smith-replica"),
			Message:       "loop environment setup failed",
			CorrelationID: correlationID,
			Metadata: map[string]string{
				"error": setupErr.Error(),
			},
		})
		_, _ = storeClient.PutStateFromCurrent(ctx, loopID, func(current model.StateRecord) (model.StateRecord, error) {
			if current.State == model.LoopStateSynced || current.State == model.LoopStateFlatline || current.State == model.LoopStateCancelled {
				return current, nil
			}
			current.State = model.LoopStateFlatline
			current.Reason = "environment-setup-failed"
			return current, nil
		})
		log.Fatalf("environment setup failed: %v", setupErr)
	}
	_ = storeClient.AppendJournal(ctx, model.JournalEntry{
		LoopID:        loopID,
		Phase:         "environment",
		Level:         "info",
		ActorType:     "replica",
		ActorID:       hostnameOr("smith-replica"),
		Message:       "loop environment resolved",
		CorrelationID: correlationID,
		Metadata:      envMeta,
	})

	_ = storeClient.AppendJournal(ctx, model.JournalEntry{
		LoopID:        loopID,
		Phase:         "replica",
		Level:         "info",
		ActorType:     "replica",
		ActorID:       hostnameOr("smith-replica"),
		Message:       "replica execution started",
		CorrelationID: correlationID,
		Metadata:      runtimeMetadataFromEnv(),
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
	for k, v := range runtimeMetadataFromEnv() {
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

func runtimeMetadataFromEnv() map[string]string {
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
	skillCount := strings.TrimSpace(os.Getenv("SMITH_SKILL_MOUNT_COUNT"))
	if skillCount != "" {
		out["skill_mount_count"] = skillCount
	}
	skillMounts := strings.TrimSpace(os.Getenv("SMITH_SKILL_MOUNTS"))
	if skillMounts != "" {
		out["skill_mounts"] = skillMounts
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func readHandoffFile(path string) (*handoffFile, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var parsed handoffFile
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}

func loadStartupContext(ctx context.Context, storeClient *store.Store, loopID string) (startupContext, error) {
	anomaly, found, err := storeClient.GetAnomaly(ctx, loopID)
	if err != nil {
		return startupContext{}, err
	}
	if !found {
		return startupContext{}, fmt.Errorf("anomaly not found for loop_id=%s", loopID)
	}
	handoff, foundHandoff, err := storeClient.GetLatestHandoff(ctx, loopID)
	if err != nil {
		return startupContext{}, err
	}
	startup := startupContext{Anomaly: anomaly}
	if foundHandoff {
		startup.PriorHandoff = &handoff
	}
	return startup, nil
}

func recordStartupFailure(ctx context.Context, storeClient *store.Store, loopID, correlationID string, startupErr error) {
	_ = storeClient.AppendJournal(ctx, model.JournalEntry{
		LoopID:        loopID,
		Phase:         "startup",
		Level:         "error",
		ActorType:     "replica",
		ActorID:       hostnameOr("smith-replica"),
		Message:       "replica startup context load failed",
		CorrelationID: correlationID,
		Metadata: map[string]string{
			"error": startupErr.Error(),
		},
	})
	_, _ = storeClient.PutStateFromCurrent(ctx, loopID, func(current model.StateRecord) (model.StateRecord, error) {
		if current.State == model.LoopStateSynced || current.State == model.LoopStateFlatline || current.State == model.LoopStateCancelled {
			return current, nil
		}
		current.State = model.LoopStateFlatline
		current.Reason = "startup-context-load-failed"
		return current, nil
	})
}
