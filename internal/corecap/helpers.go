package corecap

import (
	"fmt"
	"strings"

	"github.com/mison/antigravity-go/internal/rpc"
)

type managerWithClient interface {
	getClient() *rpc.Client
}

func requireClient(managerName string, client *rpc.Client) error {
	if client != nil {
		return nil
	}
	return fmt.Errorf("%s is not initialized", managerName)
}

func requireNonEmpty(value, fieldName string) error {
	_, err := normalizedRequired(value, fieldName)
	return err
}

func normalizedRequired(value, fieldName string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("%s is required", fieldName)
	}
	return trimmed, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func withManagerClient[T managerWithClient, R any](
	managerName string,
	manager T,
	call func(*rpc.Client) (R, error),
) (R, error) {
	var zero R

	client := managerClient(manager)
	if err := requireClient(managerName, client); err != nil {
		return zero, err
	}

	return call(client)
}

func firstNonEmptyString(payload map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := payload[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}
