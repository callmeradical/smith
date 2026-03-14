package goosed

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"smith/internal/chat"
	"strings"
)

type Engine struct {
	agentCmd string
}

func NewEngine() *Engine {
	return &Engine{
		agentCmd: "goose acp",
	}
}

func (e *Engine) Stream(ctx context.Context, session *chat.Session, message string, events chan<- chat.ChatEvent) error {
	fields := strings.Fields(e.agentCmd)
	cmd := exec.CommandContext(ctx, fields[0], fields[1:]...)

	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start goose: %w", err)
	}

	sendRPC := func(method string, params any, id any) {
		req := map[string]any{
			"jsonrpc": "2.0",
			"method":  method,
			"params":  params,
		}
		if id != nil {
			req["id"] = id
		}
		data, _ := json.Marshal(req)
		_, _ = stdin.Write(data)
		_, _ = stdin.Write([]byte("\n"))
	}

	// Step 1: Initialize
	sendRPC("initialize", map[string]any{
		"capabilities": map[string]any{"text": true},
		"client_info":  map[string]any{"name": "smith-chat", "version": "1.0.0"},
	}, 1)

	// Proxy stderr to logs (optional)
	go func() {
		s := bufio.NewScanner(stderr)
		for s.Scan() {
			// events <- chat.ChatEvent{Event: chat.EventError, Data: s.Text()}
		}
	}()

	var sessionID string
	done := make(chan struct{})

	go func() {
		defer close(done)
		s := bufio.NewScanner(stdout)
		for s.Scan() {
			var rpc map[string]any
			if err := json.Unmarshal(s.Bytes(), &rpc); err != nil {
				continue
			}

			id, _ := rpc["id"]
			method, _ := rpc["method"].(string)

			if id == float64(1) {
				// Initialized, create session
				sendRPC("session/new", map[string]any{"working_directory": "/tmp"}, 2)
			} else if id == float64(2) {
				// Session created
				if res, ok := rpc["result"].(map[string]any); ok {
					sessionID, _ = res["session_id"].(string)
					// Now send the prompt
					sendRPC("session/prompt", map[string]any{
						"session_id": sessionID,
						"message": map[string]any{
							"role": "user",
							"parts": []map[string]any{
								{"content_type": "text/plain", "content": message},
							},
						},
					}, 3)
				}
			} else if method == "session/append" {
				// Delta received
				if params, ok := rpc["params"].(map[string]any); ok {
					if parts, ok := params["parts"].([]any); ok && len(parts) > 0 {
						if part, ok := parts[0].(map[string]any); ok {
							if content, ok := part["content"].(string); ok {
								events <- chat.ChatEvent{Event: chat.EventMessageDelta, Data: chat.MessageDelta{Delta: content}}
							}
						}
					}
				}
			} else if id == float64(3) {
				// Prompt completed
				events <- chat.ChatEvent{Event: chat.EventMessageCompleted, Data: map[string]any{}}
				return
			}
		}
	}()

	select {
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		return ctx.Err()
	case <-done:
		return nil
	}
}
