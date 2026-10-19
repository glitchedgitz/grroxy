package app

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestMCPContextWithChatID(t *testing.T) {
	req := httptest.NewRequest("POST", "/mcp/http?chatId=chat-123", nil)
	ctx := mcpContextWithChatID(context.Background(), req)

	if got := ChatIDFromContext(ctx); got != "chat-123" {
		t.Fatalf("ChatIDFromContext() = %q, want %q", got, "chat-123")
	}
}

func TestMCPContextWithChatIDPreservesEmptyContext(t *testing.T) {
	req := httptest.NewRequest("POST", "/mcp/http", nil)
	ctx := mcpContextWithChatID(context.Background(), req)

	if got := ChatIDFromContext(ctx); got != "" {
		t.Fatalf("ChatIDFromContext() = %q, want empty", got)
	}
}

type testMCPSession struct{ id string }

func (s *testMCPSession) SessionID() string { return s.id }
func (s *testMCPSession) Initialize()       {}
func (s *testMCPSession) Initialized() bool { return true }
func (s *testMCPSession) NotificationChannel() chan<- mcp.JSONRPCNotification {
	return make(chan mcp.JSONRPCNotification, 10)
}

// startBlockingToolCall registers a tool that blocks until its context is
// cancelled, calls it on the given session and chat, and returns a channel
// that closes when the handler gives up.
func startBlockingToolCall(t *testing.T, sessionID, chatID string) (*Backend, <-chan struct{}) {
	t.Helper()
	backend := &Backend{}
	backend.mcpInit()
	s := backend.MCP.server

	started := make(chan struct{})
	stopped := make(chan struct{})
	s.AddTool(mcp.NewTool("blockForever"), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		close(started)
		<-ctx.Done()
		close(stopped)
		return nil, ctx.Err()
	})

	session := &testMCPSession{id: sessionID}
	if err := s.RegisterSession(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	ctx := s.WithContext(context.Background(), session)
	ctx = context.WithValue(ctx, chatIDCtxKey{}, chatID)

	msg, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "blockForever", "arguments": map[string]any{}},
	})
	// Detached from cancellation, the way mcp-go's SSE transport runs calls.
	go s.HandleMessage(context.WithoutCancel(ctx), msg)

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("tool call never started")
	}
	return backend, stopped
}

func waitStopped(t *testing.T, stopped <-chan struct{}) {
	t.Helper()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("tool call kept running")
	}
}

func TestMCPToolCallCancelledOnChatAbort(t *testing.T) {
	_, stopped := startBlockingToolCall(t, "sess-abort", "chat-abort")

	if n := CancelChatToolCalls("other-chat"); n != 0 {
		t.Fatalf("cancelled %d calls for an unrelated chat", n)
	}
	if n := CancelChatToolCalls("chat-abort"); n != 1 {
		t.Fatalf("CancelChatToolCalls() = %d, want 1", n)
	}
	waitStopped(t, stopped)
}

func TestMCPToolCallCancelledOnSessionClose(t *testing.T) {
	backend, stopped := startBlockingToolCall(t, "sess-close", "chat-close")

	backend.MCP.server.UnregisterSession(context.Background(), "sess-close")
	waitStopped(t, stopped)
}
