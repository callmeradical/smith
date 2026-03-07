package main

import (
	"encoding/json"
	"log"
	"os"
	"strings"
)

type handoff struct {
	LoopID string `json:"loop_id"`
}

func main() {
	loopID := strings.TrimSpace(os.Getenv("SMITH_LOOP_ID"))
	if loopID == "" {
		log.Fatal("SMITH_LOOP_ID is required")
	}

	handoffPath := strings.TrimSpace(os.Getenv("SMITH_HANDOFF_PATH"))
	if handoffPath == "" {
		handoffPath = "/smith/handoff/latest.json"
	}

	payload, err := os.ReadFile(handoffPath)
	if err == nil {
		var h handoff
		if unmarshalErr := json.Unmarshal(payload, &h); unmarshalErr == nil {
			log.Printf("loaded handoff for loop_id=%s", h.LoopID)
		}
	} else {
		log.Printf("handoff not found at %s; continuing", handoffPath)
	}

	log.Printf("smith-replica startup complete for loop_id=%s", loopID)
}
