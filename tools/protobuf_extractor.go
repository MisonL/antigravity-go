//go:build ignore

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// ProtoMessage represents a protobuf message structure
type ProtoMessage struct {
	Name   string
	Fields []ProtoField
}

// ProtoField represents a protobuf field
type ProtoField struct {
	Number int
	Type   string
	Name   string
}

func main() {
	fmt.Println("提取 Protobuf Schema 信息...")

	// 1. 使用 strings 提取可能的 proto 定义
	extractProtoDefinitions()

	// 2. 查找 metadata 相关的 proto 文件
	findMetadataProto()

	// 3. 尝试从二进制中提取 protobuf descriptor
	extractDescriptor()
}

func extractProtoDefinitions() {
	fmt.Println("\n提取 Proto 定义...")

	// 运行 strings 命令
	cmd := exec.Command("strings", "./antigravity_core")
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("错误: %v\n", err)
		return
	}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	protoPatterns := []string{
		`message\s+\w+`,
		`proto\s+file\s+.*\.proto`,
		`google/protobuf/.*\.proto`,
		`metadata.*proto`,
		`MetadataProvider`,
		`LanguageServerMetadata`,
	}

	protoDefs := make(map[string]bool)
	for scanner.Scan() {
		line := scanner.Text()
		for _, pattern := range protoPatterns {
			matched, err := regexp.MatchString(pattern, line)
			if err != nil {
				fmt.Printf("正则匹配失败: %v\n", err)
				continue
			}
			if matched {
				if !protoDefs[line] {
					protoDefs[line] = true
					fmt.Printf("  找到: %s\n", line)
				}
			}
		}
	}
}

func findMetadataProto() {
	fmt.Println("\n查找 Metadata 相关的 Proto...")

	// 查找包含 metadata 的字符串
	cmd := exec.Command("strings", "./antigravity_core")
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("错误: %v\n", err)
		return
	}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	metadataPatterns := []string{
		`metadata\.\w+`,
		`\w+Metadata`,
		`metadata_provider`,
		`metadataprovider`,
	}

	metadataItems := make(map[string]bool)
	for scanner.Scan() {
		line := scanner.Text()
		for _, pattern := range metadataPatterns {
			matched, err := regexp.MatchString(pattern, line)
			if err != nil {
				fmt.Printf("正则匹配失败: %v\n", err)
				continue
			}
			if matched {
				if !metadataItems[line] && len(line) < 100 {
					metadataItems[line] = true
					fmt.Printf("  找到: %s\n", line)
				}
			}
		}
	}
}

func extractDescriptor() {
	fmt.Println("\n尝试提取 Protobuf Descriptor...")

	// 读取二进制文件
	data, err := os.ReadFile("./antigravity_core")
	if err != nil {
		fmt.Printf("错误: %v\n", err)
		return
	}

	// 查找可能的 protobuf descriptor
	// Protobuf descriptor 通常以特定的字节序列开始

	// 查找包含 "metadata" 的字符串
	metadataIndex := bytes.Index(data, []byte("metadata_provider"))
	if metadataIndex != -1 {
		fmt.Printf("  找到 metadata_provider 在偏移量: %d\n", metadataIndex)

		// 提取周围的数据
		start := metadataIndex - 100
		if start < 0 {
			start = 0
		}
		end := metadataIndex + 100
		if end > len(data) {
			end = len(data)
		}

		fmt.Printf("  周围数据 (hex): %x\n", data[start:end])
	}

	// 查找可能的 protobuf 消息
	searchForProtobufMessages(data)
}

func searchForProtobufMessages(data []byte) {
	fmt.Println("\n搜索 Protobuf 消息模式...")

	// Protobuf varint 编码模式
	// 查找连续的 varint 编码
	varintCount := 0
	for i := 0; i < len(data)-1; i++ {
		b := data[i]
		if b&0x80 != 0 {
			// 这是 varint 的继续字节
			varintCount++
		} else if varintCount > 0 {
			// varint 结束
			varintCount++
			if varintCount > 3 && varintCount < 10 {
				fmt.Printf("  可能的 varint 序列在偏移量 %d, 长度 %d\n", i-varintCount+1, varintCount)
			}
			varintCount = 0
		}
	}

	// 查找 tag 字段 (field_number << 3 | wire_type)
	fmt.Println("\n  查找 Tag 字段...")
	for i := 0; i < len(data)-1; i++ {
		b := data[i]
		if b < 0x80 {
			// 这是一个完整的 tag
			fieldNum := b >> 3
			wireType := b & 0x07
			if fieldNum > 0 && fieldNum < 100 && wireType < 6 {
				fmt.Printf("  Tag: field=%d, wire_type=%d (0x%02x) 在偏移量 %d\n", fieldNum, wireType, b, i)
			}
		}
	}
}

// 生成示例 Protobuf 消息
func generateSampleMessage() {
	fmt.Println("\n生成示例 Protobuf 消息...")

	// 尝试生成一个简单的 protobuf 消息
	// 基于我们找到的信息，尝试创建一个 metadata 消息

	// 示例：创建一个包含基本字段的 protobuf 消息
	buf := new(bytes.Buffer)

	// Tag: field 1, wire type 2 (length-delimited)
	tag := byte((1 << 3) | 2)
	buf.WriteByte(tag)

	// Length: 假设有一个空的字符串
	length := byte(0)
	buf.WriteByte(length)

	fmt.Printf("  生成的消息 (hex): %x\n", buf.Bytes())

	// 保存到文件
	if err := os.WriteFile("sample_metadata.bin", buf.Bytes(), 0644); err != nil {
		fmt.Printf("  保存 sample_metadata.bin 失败: %v\n", err)
		return
	}
	fmt.Println("  已保存到 sample_metadata.bin")
}

// 分析现有的 Antigravity 扩展
func analyzeAntigravityExtensions() {
	fmt.Println("\n分析 Antigravity 扩展...")

	// 查找扩展目录
	extensionsDir := os.Getenv("HOME") + "/.ago/extensions"
	if _, err := os.Stat(extensionsDir); os.IsNotExist(err) {
		fmt.Printf("  扩展目录不存在: %s\n", extensionsDir)
		return
	}

	// 列出扩展
	entries, err := os.ReadDir(extensionsDir)
	if err != nil {
		fmt.Printf("  错误: %v\n", err)
		return
	}

	fmt.Printf("  找到 %d 个扩展:\n", len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			fmt.Printf("    - %s\n", entry.Name())

			// 查找 proto 文件
			extPath := extensionsDir + "/" + entry.Name()
			findProtoFiles(extPath)
		}
	}
}

func findProtoFiles(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".proto") {
			fmt.Printf("      找到 proto 文件: %s\n", entry.Name())
		}
		if entry.IsDir() {
			findProtoFiles(dir + "/" + entry.Name())
		}
	}
}

func init() {
	// 确保在正确的目录
	if err := os.Chdir("/Volumes/Work/code/antigravity-go"); err != nil {
		fmt.Printf("切换目录失败: %v\n", err)
	}
}
