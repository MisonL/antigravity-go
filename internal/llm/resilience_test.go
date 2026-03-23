package llm

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type retryableProviderError struct {
	code int
}

func (e retryableProviderError) Error() string {
	return fmt.Sprintf("provider returned %d", e.code)
}

func (e retryableProviderError) StatusCode() int {
	return e.code
}

type retryScriptProvider struct {
	chatCalls   int
	streamCalls int
	chatErrors  []error
	streamError []error
	response    Message
	chunks      []string
}

func (p *retryScriptProvider) Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (Message, error) {
	idx := p.chatCalls
	p.chatCalls++
	if idx < len(p.chatErrors) && p.chatErrors[idx] != nil {
		return Message{}, p.chatErrors[idx]
	}
	return p.response, nil
}

func (p *retryScriptProvider) StreamChat(ctx context.Context, messages []Message, tools []ToolDefinition, cb StreamCallback) (Message, error) {
	idx := p.streamCalls
	p.streamCalls++
	if idx < len(p.streamError) && p.streamError[idx] != nil {
		return Message{}, p.streamError[idx]
	}
	for _, chunk := range p.chunks {
		if cb != nil {
			cb(chunk, nil)
		}
	}
	return p.response, nil
}

func TestRetryBudgetProviderRetriesChatOn429(t *testing.T) {
	base := &retryScriptProvider{
		chatErrors: []error{
			retryableProviderError{code: 429},
			retryableProviderError{code: 429},
			nil,
		},
		response: Message{Role: RoleAssistant, Content: "ok"},
	}

	wrapped := WrapProviderWithRetryBudget(base, RetryBudget{
		MaxAttempts: 3,
		BaseDelay:   time.Millisecond,
		MaxDelay:    2 * time.Millisecond,
	})
	retrying, ok := wrapped.(*retryBudgetProvider)
	if !ok {
		t.Fatalf("expected retryBudgetProvider, got %T", wrapped)
	}

	var sleeps []time.Duration
	retrying.sleep = func(ctx context.Context, delay time.Duration) error {
		sleeps = append(sleeps, delay)
		return nil
	}

	resp, err := retrying.Chat(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("Chat returned error: %v", err)
	}
	if resp.Content != "ok" {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if base.chatCalls != 3 {
		t.Fatalf("expected 3 chat attempts, got %d", base.chatCalls)
	}
	if len(sleeps) != 2 {
		t.Fatalf("expected 2 retry sleeps, got %d", len(sleeps))
	}
	if sleeps[0] != time.Millisecond || sleeps[1] != 2*time.Millisecond {
		t.Fatalf("unexpected retry backoff: %#v", sleeps)
	}
}

func TestRetryBudgetProviderDoesNotRetryNonRetryableError(t *testing.T) {
	base := &retryScriptProvider{
		chatErrors: []error{retryableProviderError{code: 500}},
	}

	wrapped := WrapProviderWithRetryBudget(base, RetryBudget{
		MaxAttempts: 3,
		BaseDelay:   time.Millisecond,
		MaxDelay:    time.Millisecond,
	})

	_, err := wrapped.Chat(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected Chat to fail")
	}
	if base.chatCalls != 1 {
		t.Fatalf("expected a single chat attempt, got %d", base.chatCalls)
	}
}

func TestRetryBudgetProviderRetriesStreamOn503(t *testing.T) {
	base := &retryScriptProvider{
		streamError: []error{
			retryableProviderError{code: 503},
			nil,
		},
		response: Message{Role: RoleAssistant, Content: "stream-ok"},
		chunks:   []string{"stream-ok"},
	}

	wrapped := WrapProviderWithRetryBudget(base, RetryBudget{
		MaxAttempts: 3,
		BaseDelay:   time.Millisecond,
		MaxDelay:    time.Millisecond,
	})
	retrying := wrapped.(*retryBudgetProvider)
	retrying.sleep = func(ctx context.Context, delay time.Duration) error {
		return nil
	}

	var chunks []string
	resp, err := retrying.StreamChat(context.Background(), nil, nil, func(chunk string, err error) {
		if err != nil {
			t.Fatalf("unexpected callback error: %v", err)
		}
		chunks = append(chunks, chunk)
	})
	if err != nil {
		t.Fatalf("StreamChat returned error: %v", err)
	}
	if resp.Content != "stream-ok" {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if base.streamCalls != 2 {
		t.Fatalf("expected 2 stream attempts, got %d", base.streamCalls)
	}
	if len(chunks) != 1 || chunks[0] != "stream-ok" {
		t.Fatalf("unexpected stream chunks: %#v", chunks)
	}
}
