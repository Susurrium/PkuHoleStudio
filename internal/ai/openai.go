package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Susurrium/PkuHoleStudio/internal/config"
)

type OpenAIProvider struct {
	name       string
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewOpenAIProvider(provider config.AIProviderConfig) (*OpenAIProvider, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(provider.BaseURL), "/")
	if baseURL == "" {
		return nil, errors.New("AI provider base URL is required")
	}
	if strings.TrimSpace(provider.Model) == "" {
		return nil, errors.New("AI provider model is required")
	}
	timeout := time.Duration(provider.RequestTimeout) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &OpenAIProvider{
		name: provider.Name, baseURL: baseURL, apiKey: strings.TrimSpace(provider.APIKey),
		model: provider.Model, httpClient: &http.Client{Timeout: timeout},
	}, nil
}

func (p *OpenAIProvider) Info() ProviderInfo {
	return ProviderInfo{Name: p.name, BaseURL: p.baseURL, Model: p.model, Configured: p.apiKey != ""}
}

func (p *OpenAIProvider) Chat(ctx context.Context, request ChatRequest) (ChatResponse, error) {
	response, err := p.do(ctx, request, false)
	if err != nil {
		return ChatResponse{}, err
	}
	defer response.Body.Close()
	var payload struct {
		Model   string `json:"model"`
		Choices []struct {
			Message ChatMessage `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return ChatResponse{}, fmt.Errorf("decode AI response: %w", err)
	}
	if len(payload.Choices) == 0 {
		return ChatResponse{}, errors.New("AI response did not contain a choice")
	}
	return ChatResponse{Content: payload.Choices[0].Message.Content, ToolCalls: payload.Choices[0].Message.ToolCalls, Model: payload.Model}, nil
}

func (p *OpenAIProvider) ChatStream(ctx context.Context, request ChatRequest) (<-chan StreamEvent, error) {
	response, err := p.do(ctx, request, true)
	if err != nil {
		return nil, err
	}
	events := make(chan StreamEvent, 32)
	go func() {
		defer close(events)
		defer response.Body.Close()
		scanner := bufio.NewScanner(response.Body)
		scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				sendStreamEvent(ctx, events, StreamEvent{Done: true})
				return
			}
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
					FinishReason *string `json:"finish_reason"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				sendStreamEvent(ctx, events, StreamEvent{Error: fmt.Errorf("decode AI stream: %w", err)})
				return
			}
			for _, choice := range chunk.Choices {
				if choice.Delta.Content != "" && !sendStreamEvent(ctx, events, StreamEvent{Delta: choice.Delta.Content}) {
					return
				}
				if choice.FinishReason != nil {
					if !sendStreamEvent(ctx, events, StreamEvent{Done: true}) {
						return
					}
				}
			}
		}
		if err := scanner.Err(); err != nil {
			sendStreamEvent(ctx, events, StreamEvent{Error: fmt.Errorf("read AI stream: %w", err)})
			return
		}
		sendStreamEvent(ctx, events, StreamEvent{Done: true})
	}()
	return events, nil
}

func (p *OpenAIProvider) do(ctx context.Context, request ChatRequest, stream bool) (*http.Response, error) {
	if p == nil || p.httpClient == nil {
		return nil, errors.New("AI provider is not configured")
	}
	model := strings.TrimSpace(request.Model)
	if model == "" {
		model = p.model
	}
	payload := struct {
		Model       string           `json:"model"`
		Messages    []ChatMessage    `json:"messages"`
		Tools       []ToolDefinition `json:"tools,omitempty"`
		Temperature float64          `json:"temperature,omitempty"`
		MaxTokens   int              `json:"max_tokens,omitempty"`
		Stream      bool             `json:"stream,omitempty"`
	}{model, request.Messages, request.Tools, request.Temperature, request.MaxOutputTokens, stream}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	response, err := p.httpClient.Do(httpRequest)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer response.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(response.Body, 64*1024))
		return nil, fmt.Errorf("AI provider returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return response, nil
}

func sendStreamEvent(ctx context.Context, target chan<- StreamEvent, event StreamEvent) bool {
	select {
	case target <- event:
		return true
	case <-ctx.Done():
		return false
	}
}
