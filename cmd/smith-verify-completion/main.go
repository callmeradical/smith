package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"smith/internal/source/verification"
)

func main() {
	repoPath := flag.String("repo", "", "path to git repository fixture")
	expectedPath := flag.String("expected", "", "path to expected outcomes JSON")
	scenarioID := flag.String("scenario", "", "scenario id to verify")
	handoffPath := flag.String("handoff", "", "optional path to handoff JSON")
	phasePath := flag.String("phase", "", "optional path to phase-state JSON")
	outputPath := flag.String("output", "", "optional output JSON file path")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	report := verification.Verify(ctx, verification.VerifyInput{
		RepoPath:     *repoPath,
		ExpectedPath: *expectedPath,
		ScenarioID:   *scenarioID,
		HandoffPath:  *handoffPath,
		PhasePath:    *phasePath,
	})

	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal report: %v\n", err)
		os.Exit(1)
	}

	if *outputPath != "" {
		if err := os.WriteFile(*outputPath, payload, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write output file: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println(string(payload))
	if !report.Passed {
		os.Exit(1)
	}
}
