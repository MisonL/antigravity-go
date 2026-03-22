package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/mison/antigravity-go/internal/llm"
)

// TerminalOutputKey is the context key for the terminal broadcaster function
type TerminalOutputKey struct{}

func NewRunCommandTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "run_command",
			Description: "Execute a shell command. Use this to run go tests, build commands, or file operations.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "The command line to execute.",
					},
				},
				"required": []string{"command"},
			},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			var params struct {
				Command string `json:"command"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", err
			}

			// Broadcast command start
			broadcast, hasBroadcast := ctx.Value(TerminalOutputKey{}).(func([]byte))
			if hasBroadcast {
				broadcast([]byte(fmt.Sprintf("\x1b[33m$ %s\x1b[0m\r\n", params.Command)))
			}

			// 5-minute timeout for shell commands
			timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()

			cmd := exec.CommandContext(timeoutCtx, "/bin/bash", "-c", params.Command)

			var outputBuf bytes.Buffer
			var writers []io.Writer
			writers = append(writers, &outputBuf)

			if hasBroadcast {
				writers = append(writers, &broadcastWriter{fn: broadcast})
			}

			multiWriter := io.MultiWriter(writers...)
			cmd.Stdout = multiWriter
			cmd.Stderr = multiWriter

			if err := cmd.Run(); err != nil {
				// Don't return error immediately, return output with error message
				// so LLM sees what happened
				return fmt.Sprintf("%s\nError: %v", outputBuf.String(), err), nil
			}

			return outputBuf.String(), nil
		},
		RequiresPermission: true,
	}
}

type broadcastWriter struct {
	fn func([]byte)
}

func (w *broadcastWriter) Write(p []byte) (n int, err error) {
	w.fn(p)
	return len(p), nil
}
