package app

import (
	"context"
	"net/http/httptest"
	"testing"
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
