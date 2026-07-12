package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Susurrium/PkuHoleStudio/internal/config"
)

func TestOpenAIProviderChatAndStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/chat/completions" || request.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("request = %s auth=%q", request.URL.Path, request.Header.Get("Authorization"))
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if streaming, _ := body["stream"].(bool); streaming {
			response.Header().Set("Content-Type", "text/event-stream")
			_, _ = response.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hello \"}}]}\n\n"))
			_, _ = response.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"world\"},\"finish_reason\":\"stop\"}]}\n\n"))
			_, _ = response.Write([]byte("data: [DONE]\n\n"))
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"model":"test-model","choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"call-1","type":"function","function":{"name":"search_archive","arguments":"{\"query\":\"alpha\"}"}}]}}]}`))
	}))
	defer server.Close()
	provider, err := NewOpenAIProvider(config.AIProviderConfig{Name: "test", BaseURL: server.URL, APIKey: "secret", Model: "test-model", RequestTimeout: 5})
	if err != nil {
		t.Fatal(err)
	}
	chat, err := provider.Chat(context.Background(), ChatRequest{Messages: []ChatMessage{{Role: "user", Content: "question"}}})
	if err != nil || len(chat.ToolCalls) != 1 || chat.ToolCalls[0].Function.Name != "search_archive" {
		t.Fatalf("Chat() = %+v, %v", chat, err)
	}
	stream, err := provider.ChatStream(context.Background(), ChatRequest{Messages: []ChatMessage{{Role: "user", Content: "question"}}})
	if err != nil {
		t.Fatal(err)
	}
	var answer strings.Builder
	for event := range stream {
		if event.Error != nil {
			t.Fatal(event.Error)
		}
		answer.WriteString(event.Delta)
	}
	if answer.String() != "hello world" {
		t.Fatalf("stream answer = %q", answer.String())
	}
}

func TestOpenAIProviderIncludesHTTPFailureBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		http.Error(response, "bad model", http.StatusBadRequest)
	}))
	defer server.Close()
	provider, _ := NewOpenAIProvider(config.AIProviderConfig{BaseURL: server.URL, Model: "bad"})
	_, err := provider.Chat(context.Background(), ChatRequest{})
	if err == nil || !strings.Contains(err.Error(), "bad model") {
		t.Fatalf("Chat() error = %v", err)
	}
}
