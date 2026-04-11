package rpc

import (
	"errors"
	"testing"
)

func TestIsUnsupportedMethodErrorMarksDeprecatedAsUnsupported(t *testing.T) {
	err := errors.New("GetUserMemories: status 500: deprecated")
	if !IsUnsupportedMethodError(err) {
		t.Fatalf("expected deprecated rpc to be treated as unsupported")
	}
}

func TestIsUnsupportedMethodErrorDoesNotTreatBusinessNotFoundAsUnsupported(t *testing.T) {
	err := errors.New("ResolveOutstandingSteps: status 500: run state not found")
	if IsUnsupportedMethodError(err) {
		t.Fatalf("expected business not found to stay supported")
	}
}

func TestIsUnsupportedMethodErrorRecognizesMethodNotFound(t *testing.T) {
	err := errors.New("ListPages: status 500: method not found")
	if !IsUnsupportedMethodError(err) {
		t.Fatalf("expected method-not-found error to be unsupported")
	}
}
