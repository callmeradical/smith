package main

import "testing"

func TestIngressSummary(t *testing.T) {
	summary := ingressSummary([]ingressResult{
		{Status: "unresolved", Created: true},
		{Status: "error", Created: false},
		{Status: "unresolved", Created: false},
	})
	meta := summary["summary"].(map[string]int)
	if meta["requested"] != 3 || meta["created"] != 1 || meta["existing"] != 1 || meta["errors"] != 1 {
		t.Fatalf("unexpected summary: %#v", meta)
	}
}
