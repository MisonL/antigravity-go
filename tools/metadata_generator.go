//go:build ignore

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// encodeVarint 编码 Protobuf varint
func encodeVarint(value uint64) []byte {
	var buf [10]byte
	n := 0
	for value >= 0x80 {
		buf[n] = byte(value) | 0x80
		value >>= 7
		n++
	}
	buf[n] = byte(value)
	n++
	return buf[:n]
}

// encodeTag 编码 Protobuf tag
func encodeTag(fieldNumber int, wireType int) byte {
	return byte((fieldNumber << 3) | wireType)
}

// generateMetadataMessage 生成一个基本的 metadata 消息
// 基于从二进制文件中提取的信息
func generateMetadataMessage() []byte {
	buf := new(bytes.Buffer)

	// 尝试创建一个包含基本字段的 metadata 消息
	// 基于我们找到的 metadata_provider.MetadataProvider 接口

	// 方案 1: 创建一个包含 workspace_id 的消息
	// Tag: field 1, wire type 2 (length-delimited string)
	buf.WriteByte(encodeTag(1, 2))
	workspaceID := []byte("test_workspace")
	buf.Write(encodeVarint(uint64(len(workspaceID))))
	buf.Write(workspaceID)

	// Tag: field 2, wire type 2 (length-delimited string) - 可能是 user_id
	buf.WriteByte(encodeTag(2, 2))
	userID := []byte("test_user")
	buf.Write(encodeVarint(uint64(len(userID))))
	buf.Write(userID)

	// Tag: field 3, wire type 0 (varint) - 可能是 process_id
	buf.WriteByte(encodeTag(3, 0))
	buf.Write(encodeVarint(12345))

	// Tag: field 4, wire type 2 (length-delimited) - 可能是 root_uri
	buf.WriteByte(encodeTag(4, 2))
	rootURI := []byte("file:///tmp/test")
	buf.Write(encodeVarint(uint64(len(rootURI))))
	buf.Write(rootURI)

	// Tag: field 5, wire type 2 (length-delimited) - 可能是 client_info
	buf.WriteByte(encodeTag(5, 2))
	clientInfo := []byte("antigravity-go-proxy")
	buf.Write(encodeVarint(uint64(len(clientInfo))))
	buf.Write(clientInfo)

	return buf.Bytes()
}

// generateSimpleMetadata 生成一个更简单的 metadata 消息
func generateSimpleMetadata() []byte {
	buf := new(bytes.Buffer)

	// 只包含最基本的字段
	// Tag: field 1, wire type 2 (length-delimited)
	buf.WriteByte(encodeTag(1, 2))
	value := []byte("{}") // 空的 JSON 对象
	buf.Write(encodeVarint(uint64(len(value))))
	buf.Write(value)

	return buf.Bytes()
}

// generateEmptyMetadata 生成一个包含空字段的 metadata 消息
func generateEmptyMetadata() []byte {
	buf := new(bytes.Buffer)

	// Tag: field 1, wire type 2 (length-delimited) with empty string
	buf.WriteByte(encodeTag(1, 2))
	buf.Write(encodeVarint(0))

	return buf.Bytes()
}

// testMetadataMessage 测试 metadata 消息
func testMetadataMessage(metadata []byte, name string) bool {
	fmt.Printf("\n[Test] %s...\n", name)
	fmt.Printf("  消息内容 (hex): %x\n", metadata)

	// 启动 antigravity_core
	cmd := exec.Command("../antigravity_core",
		"--enable_lsp",
		"--app_data_dir", "test_data",
		"--cloud_code_endpoint", "https://daily-cloudcode-pa.googleapis.com",
		"--api_server_url", "http://127.0.0.1:50001",
		"--extension_server_port", "0")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
		return false
	}

	_, err = cmd.StdoutPipe()
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
		return false
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
		return false
	}

	if err := cmd.Start(); err != nil {
		fmt.Printf("  Error: %v\n", err)
		return false
	}

	// 发送 metadata 消息
	_, err = stdin.Write(metadata)
	if err != nil {
		fmt.Printf("  Write error: %v\n", err)
		cmd.Process.Kill()
		return false
	}
	stdin.Close()

	// 等待输出
	time.Sleep(2 * time.Second)

	// 读取 stderr（带超时）
	stderrBytes := make([]byte, 4096)
	n, _ := stderr.Read(stderrBytes)
	stderrBytes = stderrBytes[:n]
	stderrStr := string(stderrBytes)

	fmt.Printf("  输出: %s\n", stderrStr[:min(200, len(stderrStr))])

	// 检查是否成功
	if bytes.Contains(stderrBytes, []byte("metadata cannot be empty")) {
		fmt.Println("  Failed: metadata cannot be empty")
		cmd.Process.Kill()
		return false
	}

	if bytes.Contains(stderrBytes, []byte("Language server listening")) ||
		bytes.Contains(stderrBytes, []byte("Language server will attempt")) {
		fmt.Println("  Success")
		cmd.Process.Kill()
		return true
	}

	if bytes.Contains(stderrBytes, []byte("Failed to unmarshal")) {
		fmt.Println("  Failed: protobuf could not be decoded")
		cmd.Process.Kill()
		return false
	}

	// 检查进程是否还在运行
	if cmd.Process != nil && cmd.ProcessState == nil {
		fmt.Println("  Process still running, likely success")
		cmd.Process.Kill()
		return true
	}

	// 确保进程被终止
	if cmd.Process != nil {
		cmd.Process.Kill()
	}

	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func main() {
	fmt.Println("生成和测试 Protobuf Metadata 消息")

	// 切换到正确的目录
	os.Chdir("/Volumes/Work/code/antigravity-go/tools")

	// 添加整体超时控制
	done := make(chan bool)
	go func() {
		// 测试不同的 metadata 消息
		messages := []struct {
			name     string
			metadata []byte
		}{
			{"简单 metadata", generateSimpleMetadata()},
			{"详细 metadata", generateMetadataMessage()},
			{"空字段 metadata", generateEmptyMetadata()},
		}

		successCount := 0
		for _, msg := range messages {
			if testMetadataMessage(msg.metadata, msg.name) {
				successCount++
			}
			time.Sleep(1 * time.Second)
		}

		fmt.Printf("\nResult: %d/%d passed\n", successCount, len(messages))

		if successCount > 0 {
			fmt.Println("Found a usable metadata format.")
		} else {
			fmt.Println("All metadata tests failed.")
		}

		done <- true
	}()

	// 等待完成或超时
	select {
	case <-done:
		// 正常完成
	case <-time.After(60 * time.Second):
		fmt.Println("Timeout reached, forcing shutdown.")
		exec.Command("pkill", "-9", "antigravity_core").Run()
		os.Exit(1)
	}
}
