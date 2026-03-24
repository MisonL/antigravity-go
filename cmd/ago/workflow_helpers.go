package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type reviewTarget struct {
	Relative string
	Absolute string
	IsDir    bool
}

func resolveReviewTarget(cwd string, raw string) (reviewTarget, error) {
	target := strings.TrimSpace(raw)
	if target == "" {
		target = "."
	}

	abs := target
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(cwd, target)
	}
	abs = filepath.Clean(abs)

	info, err := os.Stat(abs)
	if err != nil {
		return reviewTarget{}, err
	}

	rel, err := filepath.Rel(cwd, abs)
	if err != nil {
		return reviewTarget{}, err
	}
	if rel == "" {
		rel = "."
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return reviewTarget{}, fmt.Errorf("目标 %q 超出当前工作区", raw)
	}

	return reviewTarget{
		Relative: filepath.ToSlash(rel),
		Absolute: abs,
		IsDir:    info.IsDir(),
	}, nil
}

func buildReviewPrompt(target reviewTarget) string {
	targetKind := "文件"
	if target.IsDir {
		targetKind = "目录"
	}

	return fmt.Sprintf(`请对目标%s执行静态代码审查，不要修改任何代码。
目标相对路径: %s
目标绝对路径: %s

要求:
1. 先调用 get_validation_states 和 get_core_diagnostics 收集证据；如果目标是单文件，可结合 get_diagnostics。
2. 仅在目标范围内使用 read_dir、read_file、search 读取必要上下文，不要扩散到无关文件。
3. 在形成最终结论前，调用 ask_specialist(role="reviewer") 汇总审查判断。
4. 最终输出必须先给 PASS 或 FAIL，再给最多 5 条关键发现；每条发现都要明确文件路径或触发原因。
5. 绝对不要调用任何会修改工作区的工具。`, targetKind, target.Relative, target.Absolute)
}

func buildAutoFixPrompt(validation string) string {
	return fmt.Sprintf(`请修复当前工作区的编译错误或 Lint 警告，并复用现有 Maker/Checker 自愈流程完成闭环。

执行要求:
1. 先阅读下面的 validation 快照，锁定最关键的问题。
2. 允许使用 apply_core_edit 或 write_file 修改代码，但只修改与当前错误直接相关的文件。
3. 每轮改动后都要再次调用 get_validation_states；必要时调用 run_command 执行 go test ./...。
4. 目标是让当前工作区的验证结果恢复为无 error/warning；如果无法完成，必须显式说明阻塞原因和剩余问题。

初始 validation 快照:
%s`, validation)
}

func validationReportHasIssues(raw string) bool {
	var payload any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return strings.TrimSpace(raw) != ""
	}
	return validationValueHasIssues("", payload)
}

func validationValueHasIssues(key string, value any) bool {
	lowerKey := strings.ToLower(strings.TrimSpace(key))

	switch typed := value.(type) {
	case map[string]any:
		for childKey, childValue := range typed {
			if validationValueHasIssues(childKey, childValue) {
				return true
			}
		}
	case []any:
		if len(typed) == 0 {
			return false
		}
		if strings.Contains(lowerKey, "error") ||
			strings.Contains(lowerKey, "warning") ||
			strings.Contains(lowerKey, "diagnostic") ||
			strings.Contains(lowerKey, "issue") {
			return true
		}
		for _, childValue := range typed {
			if validationValueHasIssues(lowerKey, childValue) {
				return true
			}
		}
	case string:
		lowerValue := strings.ToLower(strings.TrimSpace(typed))
		if (lowerKey == "status" || lowerKey == "state") &&
			(strings.Contains(lowerValue, "fail") || strings.Contains(lowerValue, "error") || strings.Contains(lowerValue, "warn")) {
			return true
		}
		if lowerKey == "severity" && (lowerValue == "error" || lowerValue == "warning") {
			return true
		}
	case bool:
		if (lowerKey == "ok" || lowerKey == "passed" || lowerKey == "success") && !typed {
			return true
		}
	case float64:
		if typed <= 0 {
			return false
		}
		if strings.Contains(lowerKey, "error") ||
			strings.Contains(lowerKey, "warning") ||
			strings.Contains(lowerKey, "diagnostic") ||
			strings.Contains(lowerKey, "issue") {
			return true
		}
	}

	return false
}
