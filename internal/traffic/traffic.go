package traffic

import (
	"fmt"
	"strconv"
	"strings"
)

// Usage 一次订阅流量快照。零值合法（未知）。
type Usage struct {
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
	Total    int64 `json:"total"`  // 0 = 无限
	Expire   int64 `json:"expire"` // unix 秒，0 = 未知
}

// ParseSubscriptionUserinfo 解析 "upload=1; download=2; total=3; expire=4"。
// 带 "Subscription-Userinfo: " 前缀也接受。返回 ok=false 表示这条头不可用。
// 未出现的键保持零值；单个键语法错误只跳过该键，不整体失败。
func ParseSubscriptionUserinfo(s string) (Usage, bool) {
	s = strings.TrimSpace(s)
	lower := strings.ToLower(s)
	const prefix = "subscription-userinfo:"
	if strings.HasPrefix(lower, prefix) {
		s = strings.TrimSpace(s[len(prefix):])
	}
	if s == "" {
		return Usage{}, false
	}
	u := Usage{}
	found := false
	for _, seg := range strings.Split(s, ";") {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		parts := strings.SplitN(seg, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])
		switch key {
		case "upload", "download", "total", "expire":
			n, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				continue
			}
			switch key {
			case "upload":
				u.Upload = n
			case "download":
				u.Download = n
			case "total":
				u.Total = n
			case "expire":
				u.Expire = n
			}
			found = true
		}
	}
	if !found {
		return Usage{}, false
	}
	return u, true
}

// FormatSubscriptionUserinfo 生成可直接写响应头的原文。
func FormatSubscriptionUserinfo(u Usage) string {
	return fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d", u.Upload, u.Download, u.Total, u.Expire)
}

// FieldStrategy 字段独立策略。
type FieldStrategy string

const (
	FieldSum   FieldStrategy = "sum"
	FieldMin   FieldStrategy = "min"
	FieldMax   FieldStrategy = "max"
	FieldAvg   FieldStrategy = "avg"
	FieldFirst FieldStrategy = "first"
	FieldLast  FieldStrategy = "last"
	FieldNone  FieldStrategy = "none" // 该字段不透传
)

// Strategy 四个字段各一个策略，加一个只用于回显的 Preset 名。
type Strategy struct {
	Preset   string        `json:"preset,omitempty"`
	Upload   FieldStrategy `json:"upload"`
	Download FieldStrategy `json:"download"`
	Total    FieldStrategy `json:"total"`
	Expire   FieldStrategy `json:"expire"`
}

// Normalize 填空为默认。
//
// 简单预设名（sum/min/max/avg/first/last/off）会被展开成对应的字段策略，
// 这样配置里写裸名字 "max" 与写 JSON {"preset":"max"} 语义一致——
// Aggregate 的 default 分支只看字段，不展开就等于把用户选的预设静默丢掉。
// min_quota/max_quota/weighted 由 Aggregate 按 Preset 特殊处理，字段仍填通用默认。
// 显式给出的字段永远优先于预设展开值。
func (s Strategy) Normalize() Strategy {
	base := Strategy{Upload: FieldSum, Download: FieldSum, Total: FieldSum, Expire: FieldMin}
	if p, ok := PresetStrategy(s.Preset); ok && p.Upload != "" {
		base = p
		s.Preset = p.Preset
	}
	if s.Upload == "" {
		s.Upload = base.Upload
	}
	if s.Download == "" {
		s.Download = base.Download
	}
	if s.Total == "" {
		s.Total = base.Total
	}
	if s.Expire == "" {
		s.Expire = base.Expire
	}
	return s
}

// PresetStrategy 映射（名字大小写不敏感）。
func PresetStrategy(name string) (Strategy, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "sum":
		return Strategy{Preset: "sum", Upload: FieldSum, Download: FieldSum, Total: FieldSum, Expire: FieldMin}, true
	case "min":
		return Strategy{Preset: "min", Upload: FieldMin, Download: FieldMin, Total: FieldMin, Expire: FieldMin}, true
	case "max":
		return Strategy{Preset: "max", Upload: FieldMax, Download: FieldMax, Total: FieldMax, Expire: FieldMax}, true
	case "avg":
		return Strategy{Preset: "avg", Upload: FieldAvg, Download: FieldAvg, Total: FieldAvg, Expire: FieldAvg}, true
	case "first":
		return Strategy{Preset: "first", Upload: FieldFirst, Download: FieldFirst, Total: FieldFirst, Expire: FieldFirst}, true
	case "last":
		return Strategy{Preset: "last", Upload: FieldLast, Download: FieldLast, Total: FieldLast, Expire: FieldLast}, true
	case "min_quota":
		return Strategy{Preset: "min_quota"}, true
	case "max_quota":
		return Strategy{Preset: "max_quota"}, true
	case "weighted":
		return Strategy{Preset: "weighted"}, true
	case "off":
		return Strategy{Preset: "off", Upload: FieldNone, Download: FieldNone, Total: FieldNone, Expire: FieldNone}, true
	}
	return Strategy{}, false
}

// Aggregate 聚合多机场快照。
func Aggregate(usages []Usage, nodeCounts []int, s Strategy) (Usage, bool) {
	if len(usages) == 0 {
		return Usage{}, false
	}
	s = s.Normalize()
	if s.Upload == FieldNone && s.Download == FieldNone && s.Total == FieldNone && s.Expire == FieldNone {
		return Usage{}, false
	}

	switch s.Preset {
	case "min_quota":
		bestIdx := -1
		for i, u := range usages {
			if u.Total == 0 {
				continue
			}
			if bestIdx == -1 || u.Total < usages[bestIdx].Total {
				bestIdx = i
			}
		}
		if bestIdx == -1 {
			bestIdx = 0
		}
		return usages[bestIdx], true
	case "max_quota":
		bestIdx := -1
		for i, u := range usages {
			if u.Total == 0 {
				bestIdx = i
				break
			}
		}
		if bestIdx == -1 {
			for i, u := range usages {
				if bestIdx == -1 || u.Total > usages[bestIdx].Total {
					bestIdx = i
				}
			}
		}
		return usages[bestIdx], true
	case "weighted":
		weights := make([]int, len(usages))
		sumW := 0
		if len(nodeCounts) == len(usages) {
			for i := range usages {
				weights[i] = nodeCounts[i]
				sumW += weights[i]
			}
		}
		result := Usage{}
		if sumW == 0 {
			result.Upload = applyField(usages, FieldAvg, "upload")
			result.Download = applyField(usages, FieldAvg, "download")
			result.Total = applyField(usages, FieldAvg, "total")
		} else {
			result.Upload = weightedSum(usages, weights, sumW, "upload")
			result.Download = weightedSum(usages, weights, sumW, "download")
			result.Total = weightedSum(usages, weights, sumW, "total")
		}
		result.Expire = applyField(usages, FieldMin, "expire")
		return result, true
	}

	result := Usage{}
	result.Upload = applyField(usages, s.Upload, "upload")
	result.Download = applyField(usages, s.Download, "download")
	result.Total = applyField(usages, s.Total, "total")
	result.Expire = applyField(usages, s.Expire, "expire")
	return result, true
}

func getField(u Usage, field string) int64 {
	switch field {
	case "upload":
		return u.Upload
	case "download":
		return u.Download
	case "total":
		return u.Total
	case "expire":
		return u.Expire
	}
	return 0
}

func applyField(usages []Usage, st FieldStrategy, field string) int64 {
	vals := make([]int64, len(usages))
	for i, u := range usages {
		vals[i] = getField(u, field)
	}
	switch st {
	case FieldSum:
		var sum int64
		for _, v := range vals {
			if v != 0 {
				sum += v
			}
		}
		return sum
	case FieldMin:
		var min int64
		found := false
		for _, v := range vals {
			if v == 0 {
				continue
			}
			if !found || v < min {
				min = v
				found = true
			}
		}
		if !found {
			return 0
		}
		return min
	case FieldMax:
		var max int64
		found := false
		for _, v := range vals {
			if !found || v > max {
				max = v
				found = true
			}
		}
		if !found {
			return 0
		}
		return max
	case FieldAvg:
		var sum int64
		count := 0
		for _, v := range vals {
			if v != 0 {
				sum += v
				count++
			}
		}
		if count == 0 {
			return 0
		}
		return sum / int64(count)
	case FieldFirst:
		for _, v := range vals {
			if v != 0 {
				return v
			}
		}
		return 0
	case FieldLast:
		for i := len(vals) - 1; i >= 0; i-- {
			if vals[i] != 0 {
				return vals[i]
			}
		}
		return 0
	case FieldNone:
		return 0
	}
	return 0
}

func weightedSum(usages []Usage, weights []int, sumW int, field string) int64 {
	var total int64
	for i, u := range usages {
		v := getField(u, field)
		total += v * int64(weights[i])
	}
	return total / int64(sumW)
}

// format 未导出：按策略拼接响应头，跳过 none 键；全 none 返回空串。
func format(u Usage, s Strategy) string {
	s = s.Normalize()
	if s.Upload == FieldNone && s.Download == FieldNone && s.Total == FieldNone && s.Expire == FieldNone {
		return ""
	}
	if s.Upload != FieldNone && s.Download != FieldNone && s.Total != FieldNone && s.Expire != FieldNone {
		return FormatSubscriptionUserinfo(u)
	}
	parts := make([]string, 0, 4)
	if s.Upload != FieldNone {
		parts = append(parts, fmt.Sprintf("upload=%d", u.Upload))
	}
	if s.Download != FieldNone {
		parts = append(parts, fmt.Sprintf("download=%d", u.Download))
	}
	if s.Total != FieldNone {
		parts = append(parts, fmt.Sprintf("total=%d", u.Total))
	}
	if s.Expire != FieldNone {
		parts = append(parts, fmt.Sprintf("expire=%d", u.Expire))
	}
	return strings.Join(parts, "; ")
}

// ToHeader 把聚合结果写成响应头原文。
func ToHeader(u Usage) string {
	return format(u, Strategy{Upload: FieldSum, Download: FieldSum, Total: FieldSum, Expire: FieldMin})
}

// FormatWithStrategy 按策略把聚合结果写成响应头原文。
// 与 ToHeader 的区别：本函数尊重字段级策略——FieldNone 的字段不出现；
// 四个字段全为 FieldNone（"不透传"）时返回空串，调用方据此跳过设置响应头。
func FormatWithStrategy(u Usage, s Strategy) string {
	return format(u, s)
}
