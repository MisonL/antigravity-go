package tools

import (
	"testing"
)

func TestUniqueMcpToolName(t *testing.T) {
	used := make(map[string]string)

	// 测试正常前缀隔离
	name1 := uniqueMcpToolName("sqlite", "query", used)
	if name1 != "mcp__sqlite__query" {
		t.Errorf("expected mcp__sqlite__query, got %s", name1)
	}

	// 测试重名冲突解决
	name2 := uniqueMcpToolName("my-sqlite", "query", used)
	if name2 == name1 {
		t.Error("expected different name for different servers with same tool")
	}
}
