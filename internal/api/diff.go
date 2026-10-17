// Package api — 简单的行级 unified diff 生成，供 Convert 页的"diff 预览"使用。
package api

import "strings"

// DiffLine 是一行 diff 结果。
type DiffLine struct {
	Type string `json:"type"` // " " / "+" / "-"
	Text string `json:"text"`
}

// DiffLines 生成按行对比的 diff：相等行 type=" "，左边独有 type="-"，右边独有 type="+"。
func DiffLines(a, b string) []DiffLine {
	al := splitLines(a)
	bl := splitLines(b)
	m, n := len(al), len(bl)
	// LCS 动态规划
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := m - 1; i >= 0; i-- {
		for j := n - 1; j >= 0; j-- {
			if al[i] == bl[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	var out []DiffLine
	i, j := 0, 0
	for i < m && j < n {
		if al[i] == bl[j] {
			out = append(out, DiffLine{Type: " ", Text: al[i]})
			i++
			j++
		} else if dp[i+1][j] >= dp[i][j+1] {
			out = append(out, DiffLine{Type: "-", Text: al[i]})
			i++
		} else {
			out = append(out, DiffLine{Type: "+", Text: bl[j]})
			j++
		}
	}
	for i < m {
		out = append(out, DiffLine{Type: "-", Text: al[i]})
		i++
	}
	for j < n {
		out = append(out, DiffLine{Type: "+", Text: bl[j]})
		j++
	}
	return out
}

// splitLines 把文本切成待比较的行。
//
// 空串表示"没有任何行"，返回空切片——否则 Convert 页在基线为空时会看到一行
// 内容为空的删除行（幽灵行），两侧都空时也会显示一行空"相等"行而不是"无差异"。
// 非空文本保留末尾空行的差异，只去掉结尾那一个由 Split 产生的空串。
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}
