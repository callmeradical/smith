package prompts

import (
	"context"
	"fmt"
	"smith/internal/chat"
	"smith/internal/chat/smithbridge"
	"strings"
)

type Manager struct {
	bridge smithbridge.Bridge
}

func NewManager(b smithbridge.Bridge) *Manager {
	return &Manager{bridge: b}
}

func (m *Manager) BuildSystemPrompt(ctx context.Context, session *chat.Session) (string, error) {
	var sb strings.Builder
	sb.WriteString("You are the Smith Interactive Chat Assistant. ")
	sb.WriteString(fmt.Sprintf("You are currently in a %s session.\n\n", session.Type))

	switch session.Type {
	case chat.SessionTypePRDRefinement:
		m.injectPRDContext(ctx, &sb, session.Context)
	case chat.SessionTypeLoopAssist:
		m.injectLoopContext(ctx, &sb, session.Context)
	case chat.SessionTypeDocumentAssist:
		m.injectDocumentContext(ctx, &sb, session.Context)
	}

	sb.WriteString("\nFollow the operator's instructions and provide helpful, concise responses.")
	return sb.String(), nil
}

func (m *Manager) injectPRDContext(ctx context.Context, sb *strings.Builder, sCtx map[string]string) {
	sb.WriteString("CONTEXT: PRD Refinement\n")
	if docID, ok := sCtx["documentId"]; ok {
		doc, err := m.bridge.GetDocument(ctx, docID)
		if err == nil {
			sb.WriteString(fmt.Sprintf("Document Title: %s\n", doc.Title))
			sb.WriteString(fmt.Sprintf("Current Content:\n%s\n", doc.Content))
		}
	}
}

func (m *Manager) injectLoopContext(ctx context.Context, sb *strings.Builder, sCtx map[string]string) {
	sb.WriteString("CONTEXT: Loop Assist\n")
	if loopID, ok := sCtx["loopId"]; ok {
		loop, err := m.bridge.GetLoop(ctx, loopID)
		if err == nil {
			sb.WriteString(fmt.Sprintf("Loop ID: %s\n", loop.LoopID))
			sb.WriteString(fmt.Sprintf("Status: %s\n", loop.State))
			sb.WriteString(fmt.Sprintf("Reason: %s\n", loop.Reason))
		}

		journal, err := m.bridge.GetJournal(ctx, loopID, 10)
		if err == nil && len(journal) > 0 {
			sb.WriteString("Recent Journal Entries:\n")
			for _, entry := range journal {
				sb.WriteString(fmt.Sprintf("- [%s] %s\n", entry.Timestamp.Format("15:04:05"), entry.Message))
			}
		}
	}
}

func (m *Manager) injectDocumentContext(ctx context.Context, sb *strings.Builder, sCtx map[string]string) {
	sb.WriteString("CONTEXT: Document Assist\n")
	if docID, ok := sCtx["documentId"]; ok {
		doc, err := m.bridge.GetDocument(ctx, docID)
		if err == nil {
			sb.WriteString(fmt.Sprintf("Document: %s\n", doc.Title))
			sb.WriteString(fmt.Sprintf("Content:\n%s\n", doc.Content))
		}
	}
}
