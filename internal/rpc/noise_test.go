package rpc

import (
	"errors"
	"testing"
)

func TestShouldSilenceDeprecatedMethodError(t *testing.T) {
	err := errors.New("GetUserMemories: status 500: deprecated")
	if !ShouldSilenceDeprecatedMethodError("GetUserMemories", err) {
		t.Fatalf("expected deprecated memory RPC error to be silenced")
	}

	if ShouldSilenceDeprecatedMethodError("GetUserMemories", errors.New("status 500: unavailable")) {
		t.Fatalf("unexpected silence for non-deprecated memory RPC error")
	}

	if ShouldSilenceDeprecatedMethodError("GetDiagnostics", errors.New("GetDiagnostics: status 500: deprecated")) {
		t.Fatalf("unexpected silence for unrelated deprecated RPC error")
	}
}

func TestShouldSilenceDeprecatedLogLine(t *testing.T) {
	line := "rpc error: GetAllCascadeTrajectories returned deprecated and will be removed"
	if !ShouldSilenceDeprecatedLogLine(line) {
		t.Fatalf("expected deprecated trajectory log to be silenced")
	}

	if ShouldSilenceDeprecatedLogLine("rpc error: GetAllCascadeTrajectories returned unimplemented") {
		t.Fatalf("unexpected silence without deprecated marker")
	}
}
