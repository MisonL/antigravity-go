package llm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

type RetryBudget struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

type retryBudgetProvider struct {
	base   Provider
	budget RetryBudget
	sleep  func(context.Context, time.Duration) error
}

func WrapProviderWithRetryBudget(provider Provider, budget RetryBudget) Provider {
	if provider == nil {
		return nil
	}

	normalized := normalizeRetryBudget(budget)
	if normalized.MaxAttempts <= 1 {
		return provider
	}

	return &retryBudgetProvider{
		base:   provider,
		budget: normalized,
		sleep:  sleepWithContext,
	}
}

func (p *retryBudgetProvider) Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (Message, error) {
	var resp Message
	err := p.withRetry(ctx, func() error {
		var err error
		resp, err = p.base.Chat(ctx, messages, tools)
		return err
	})
	return resp, err
}

func (p *retryBudgetProvider) StreamChat(ctx context.Context, messages []Message, tools []ToolDefinition, cb StreamCallback) (Message, error) {
	var resp Message

	err := p.withRetry(ctx, func() error {
		sawChunk := false
		proxy := func(chunk string, err error) {
			if chunk != "" {
				sawChunk = true
			}
			if cb != nil {
				cb(chunk, err)
			}
		}

		var err error
		resp, err = p.base.StreamChat(ctx, messages, tools, proxy)
		if err != nil && sawChunk {
			return nonRetryableProviderError{cause: err}
		}
		return err
	})
	return resp, unwrapNonRetryableProviderError(err)
}

func (p *retryBudgetProvider) withRetry(ctx context.Context, operation func() error) error {
	var lastErr error

	for attempt := 1; attempt <= p.budget.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := operation()
		if err == nil {
			return nil
		}
		if isNonRetryableProviderError(err) {
			return unwrapNonRetryableProviderError(err)
		}
		if !shouldRetryProviderError(err) || attempt == p.budget.MaxAttempts {
			return err
		}

		lastErr = err
		if sleepErr := p.sleep(ctx, retryDelay(attempt, p.budget)); sleepErr != nil {
			if errors.Is(sleepErr, context.Canceled) || errors.Is(sleepErr, context.DeadlineExceeded) {
				return sleepErr
			}
			return fmt.Errorf("retry budget sleep failed after %w: %v", err, sleepErr)
		}
	}

	return lastErr
}

func normalizeRetryBudget(budget RetryBudget) RetryBudget {
	if budget.MaxAttempts <= 0 {
		budget.MaxAttempts = 3
	}
	if budget.BaseDelay <= 0 {
		budget.BaseDelay = 200 * time.Millisecond
	}
	if budget.MaxDelay <= 0 {
		budget.MaxDelay = 2 * time.Second
	}
	if budget.MaxDelay < budget.BaseDelay {
		budget.MaxDelay = budget.BaseDelay
	}
	return budget
}

func retryDelay(attempt int, budget RetryBudget) time.Duration {
	delay := budget.BaseDelay
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay >= budget.MaxDelay {
			return budget.MaxDelay
		}
	}
	if delay > budget.MaxDelay {
		return budget.MaxDelay
	}
	return delay
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func shouldRetryProviderError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	statusCode := providerStatusCode(err)
	return statusCode == http.StatusTooManyRequests || statusCode == http.StatusServiceUnavailable
}

func providerStatusCode(err error) int {
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		return apiErr.HTTPStatusCode
	}

	type statusCoder interface {
		StatusCode() int
	}
	var sc statusCoder
	if errors.As(err, &sc) {
		return sc.StatusCode()
	}

	type httpStatusCoder interface {
		HTTPStatusCode() int
	}
	var hsc httpStatusCoder
	if errors.As(err, &hsc) {
		return hsc.HTTPStatusCode()
	}

	value := reflect.ValueOf(err)
	if value.Kind() == reflect.Pointer && !value.IsNil() {
		elem := value.Elem()
		if elem.Kind() == reflect.Struct {
			for _, fieldName := range []string{"StatusCode", "HTTPStatusCode"} {
				field := elem.FieldByName(fieldName)
				if field.IsValid() && field.CanInt() {
					return int(field.Int())
				}
			}
		}
	}

	msg := err.Error()
	if strings.Contains(msg, "429") {
		return http.StatusTooManyRequests
	}
	if strings.Contains(msg, "503") {
		return http.StatusServiceUnavailable
	}
	return 0
}

type nonRetryableProviderError struct {
	cause error
}

func (e nonRetryableProviderError) Error() string {
	if e.cause == nil {
		return ""
	}
	return e.cause.Error()
}

func (e nonRetryableProviderError) Unwrap() error {
	return e.cause
}

func isNonRetryableProviderError(err error) bool {
	var wrapped nonRetryableProviderError
	return errors.As(err, &wrapped)
}

func unwrapNonRetryableProviderError(err error) error {
	var wrapped nonRetryableProviderError
	if errors.As(err, &wrapped) {
		return wrapped.cause
	}
	return err
}
