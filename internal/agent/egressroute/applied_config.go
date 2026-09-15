package egressroute

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// marksReferencedByConfigPaths 解析"已应用的核心配置"，提取仍被引用的出口集分发 fwmark：
//   - sing-box: outbound.routing_mark
//   - xray:     outbound.streamSettings.sockopt.mark
//
// paths 可以是文件，也可以是目录（目录下所有 *.json 都会被扫描——sing-box 的 apply 产物是
// ManagedDir 下的一批 fragment，不存在单文件 merged 输出）。
//
// 返回 (marks, ok)。ok=false 表示无法判定（无可用文件/全部解析失败），调用方应退回保守策略。
//
// 为什么需要它：拆除陈旧规则/表的安全条件是"核心已不再引用该 mark"。
// 用 revision 前进做代理条件在两种真实序列下会失效（GAP-2 残留）：
//  1. agent 进程重启后 applyRevision 归零，旧 revision 被重新应用并立刻满足"前进"；
//  2. 面板在移除成员前渲染、排在队列里的 apply 完成后才轮到本轮。
//
// 直接读已应用配置则不存在歧义：只要配置还在引用该 mark，就绝不拆除。
func marksReferencedByConfigPaths(paths []string) (map[int]bool, bool) {
	marks := map[int]bool{}
	parsedAny := false
	for _, raw := range paths {
		path := strings.TrimSpace(raw)
		if path == "" {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.IsDir() {
			entries, err := os.ReadDir(path)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
					continue
				}
				if collectMarksFromFile(filepath.Join(path, entry.Name()), marks) {
					parsedAny = true
				}
			}
			continue
		}
		if collectMarksFromFile(path, marks) {
			parsedAny = true
		}
	}
	if !parsedAny {
		return nil, false
	}
	return marks, true
}

// collectMarksFromFile 解析单个 JSON 文件并收集 mark；成功解析返回 true。
func collectMarksFromFile(path string, out map[int]bool) bool {
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var doc any
	if err := json.Unmarshal(content, &doc); err != nil {
		return false
	}
	collectReferencedMarks(doc, "", out)
	return true
}

// collectReferencedMarks 递归收集 mark 值：`routing_mark` 任意层级生效；
// 裸 `mark` 仅在 `sockopt` 对象内生效（避免误判无关字段）。
func collectReferencedMarks(node any, parentKey string, out map[int]bool) {
	switch typed := node.(type) {
	case map[string]any:
		for key, child := range typed {
			if number, ok := child.(float64); ok {
				if key == "routing_mark" || (key == "mark" && parentKey == "sockopt") {
					out[int(number)] = true
				}
			}
			collectReferencedMarks(child, key, out)
		}
	case []any:
		for _, child := range typed {
			collectReferencedMarks(child, parentKey, out)
		}
	}
}
