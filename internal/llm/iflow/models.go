package iflow

import (
	"sort"
	"strconv"
	"strings"
)

type ModelSpec struct {
	Name             string
	DisplayName      string
	Family           string // iflow / qwen / deepseek / other ...
	Status           string // online / offline
	MaxOutputTokens  int
	MaxContextTokens int
}

// Models 是“心流开放平台-模型库”页面的公开模型清单（抓取时间：2026-01-27）。
//
// 来源：POST https://platform.iflow.cn/api/platform/models/list
// 注意：平台模型可能会随时间变更；这里以“预置可选项 + 最常用约束（最大上下文/最大输出）”为主。
func Models() []ModelSpec {
	out := make([]ModelSpec, 0, len(models))
	out = append(out, models...)
	sort.Slice(out, func(i, j int) bool {
		// 在线优先，其次按 family，再按 name
		ai := out[i].Status == "online"
		aj := out[j].Status == "online"
		if ai != aj {
			return ai
		}
		if out[i].Family != out[j].Family {
			return out[i].Family < out[j].Family
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func Find(name string) (ModelSpec, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return ModelSpec{}, false
	}
	for _, m := range models {
		if m.Name == name {
			return m, true
		}
	}
	return ModelSpec{}, false
}

func DefaultModelName() string {
	// 优先选择一个更“通用/稳妥”的在线模型；如后续平台变更，可按需要调整。
	for _, m := range Models() {
		if m.Name == "qwen3-32b" && m.Status == "online" {
			return m.Name
		}
	}
	for _, m := range Models() {
		if m.Status == "online" {
			return m.Name
		}
	}
	if len(models) == 0 {
		return ""
	}
	return models[0].Name
}

func ParseTokenLimit(v string) int {
	v = strings.TrimSpace(strings.ToUpper(v))
	v = strings.TrimRight(v, "B") // 兼容 128KB/128K 等写法（宽松处理）
	if v == "" {
		return 0
	}

	mul := 1
	switch {
	case strings.HasSuffix(v, "K"):
		mul = 1024
		v = strings.TrimSuffix(v, "K")
	case strings.HasSuffix(v, "M"):
		mul = 1024 * 1024
		v = strings.TrimSuffix(v, "M")
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0
	}
	return n * mul
}

var models = []ModelSpec{
	{Family: "DeepSeek", DisplayName: "DeepSeek-V3.2-Exp", Name: "deepseek-v3.2", Status: "offline", MaxOutputTokens: ParseTokenLimit("64K"), MaxContextTokens: ParseTokenLimit("128K")},
	{Family: "Deepseek", DisplayName: "DeepSeek-R1", Name: "deepseek-r1", Status: "offline", MaxOutputTokens: ParseTokenLimit("32K"), MaxContextTokens: ParseTokenLimit("128K")},
	{Family: "Deepseek", DisplayName: "DeepSeek-V3-671B", Name: "deepseek-v3", Status: "offline", MaxOutputTokens: ParseTokenLimit("32K"), MaxContextTokens: ParseTokenLimit("128K")},
	{Family: "OTHER", DisplayName: "GLM-4.6", Name: "glm-4.6", Status: "offline", MaxOutputTokens: ParseTokenLimit("128K"), MaxContextTokens: ParseTokenLimit("200K")},
	{Family: "OTHER", DisplayName: "Kimi-K2", Name: "kimi-k2", Status: "offline", MaxOutputTokens: ParseTokenLimit("64K"), MaxContextTokens: ParseTokenLimit("128K")},
	{Family: "OTHER", DisplayName: "Kimi-K2-Instruct-0905", Name: "kimi-k2-0905", Status: "offline", MaxOutputTokens: ParseTokenLimit("64K"), MaxContextTokens: ParseTokenLimit("256K")},
	{Family: "QWEN", DisplayName: "Qwen3-235B-A22B", Name: "qwen3-235b", Status: "offline", MaxOutputTokens: ParseTokenLimit("32K"), MaxContextTokens: ParseTokenLimit("128K")},
	{Family: "QWEN", DisplayName: "Qwen3-235B-A22B-Instruct", Name: "qwen3-235b-a22b-instruct", Status: "offline", MaxOutputTokens: ParseTokenLimit("64K"), MaxContextTokens: ParseTokenLimit("256K")},
	{Family: "QWEN", DisplayName: "Qwen3-235B-A22B-Thinking", Name: "qwen3-235b-a22b-thinking-2507", Status: "online", MaxOutputTokens: ParseTokenLimit("64K"), MaxContextTokens: ParseTokenLimit("256K")},
	{Family: "QWEN", DisplayName: "Qwen3-32B", Name: "qwen3-32b", Status: "online", MaxOutputTokens: ParseTokenLimit("32K"), MaxContextTokens: ParseTokenLimit("128K")},
	{Family: "QWEN", DisplayName: "Qwen3-Max", Name: "qwen3-max", Status: "offline", MaxOutputTokens: ParseTokenLimit("32K"), MaxContextTokens: ParseTokenLimit("256K")},
	{Family: "QWEN", DisplayName: "Qwen3-Max-Preview", Name: "qwen3-max-preview", Status: "online", MaxOutputTokens: ParseTokenLimit("32K"), MaxContextTokens: ParseTokenLimit("256K")},
	{Family: "QWEN", DisplayName: "Qwen3-VL-Plus", Name: "qwen3-vl-plus", Status: "offline", MaxOutputTokens: ParseTokenLimit("32K"), MaxContextTokens: ParseTokenLimit("256K")},
	{Family: "iflow", DisplayName: "iFlow-ROME", Name: "iflow-rome-30ba3b", Status: "offline", MaxOutputTokens: ParseTokenLimit("64K"), MaxContextTokens: ParseTokenLimit("256K")},
	{Family: "qwen", DisplayName: "Qwen3-Coder-Plus", Name: "qwen3-coder-plus", Status: "offline", MaxOutputTokens: ParseTokenLimit("64K"), MaxContextTokens: ParseTokenLimit("1M")},
}
