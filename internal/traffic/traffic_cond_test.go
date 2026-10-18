package traffic

import (
	"testing"
)

// traffic_cond_test.go — 条件覆盖补充

func TestParseSubscriptionUserinfoEmptySeg(t *testing.T) {
	// traffic.go:34 未覆盖：空分段（seg==""）
	u, ok := ParseSubscriptionUserinfo("upload=10;;download=20")
	if !ok {
		t.Fatalf("parse 应成功")
	}
	if u.Upload != 10 || u.Download != 20 {
		t.Fatalf("got %+v", u)
	}
}

func TestParseSubscriptionUserinfoUnknownKey(t *testing.T) {
	// traffic.go:49 未覆盖：switch default（未知 key）
	u, ok := ParseSubscriptionUserinfo("foo=1;upload=5")
	if !ok {
		t.Fatalf("parse 应成功")
	}
	if u.Upload != 5 {
		t.Fatalf("got %+v", u)
	}
}

func TestPresetStrategyUnknown(t *testing.T) {
	// traffic.go:125 未覆盖：switch default（未知 preset）
	_, ok := PresetStrategy("nope")
	if ok {
		t.Fatalf("未知 preset 应返回 false")
	}
}

func TestAggregateMinQuotaNoQuota(t *testing.T) {
	// traffic.go:171 未覆盖：min_quota 无配额时 bestIdx==-1
	us := []Usage{{Upload: 1, Total: 5}, {Upload: 2, Total: 3}}
	strat := Strategy{Preset: "min_quota"}
	r, ok := Aggregate(us, nil, strat)
	if !ok {
		t.Fatalf("Aggregate 应成功")
	}
	_ = r
}

func TestAggregateMaxQuotaInner(t *testing.T) {
	// traffic.go:185 未覆盖：max_quota 内层 bestIdx==-1
	us := []Usage{{Upload: 1, Total: 5}, {Upload: 2, Total: 3}}
	strat := Strategy{Preset: "max_quota"}
	r, ok := Aggregate(us, nil, strat)
	if !ok {
		t.Fatalf("Aggregate 应成功")
	}
	_ = r
}

func TestAggregateWeightedNodeCounts(t *testing.T) {
	// traffic.go:195 未覆盖：weighted nodeCounts 长度匹配
	us := []Usage{{Upload: 10, Total: 10}, {Upload: 20, Total: 20}}
	strat := Strategy{Preset: "weighted"}
	r, ok := Aggregate(us, []int{1, 2}, strat)
	if !ok {
		t.Fatalf("Aggregate 应成功")
	}
	if r.Upload == 0 {
		t.Fatalf("weighted 应非零")
	}
}

func TestAggregateWeightedZeroSum(t *testing.T) {
	// traffic.go:201 未覆盖：sumW==0
	us := []Usage{{Upload: 10, Total: 10}, {Upload: 20, Total: 20}}
	strat := Strategy{Preset: "weighted"}
	r, ok := Aggregate(us, []int{0, 0}, strat)
	if !ok {
		t.Fatalf("Aggregate 应成功")
	}
	if r.Upload == 0 {
		t.Fatalf("avg 应非零")
	}
}

func TestGetFieldUnknown(t *testing.T) {
	// traffic.go:223 未覆盖：getField switch default
	u := Usage{Upload: 1}
	if v := getField(u, "nope"); v != 0 {
		t.Fatalf("getField unknown 应返回 0, got %d", v)
	}
}

func TestApplyFieldAvgZeroCount(t *testing.T) {
	// traffic.go:288 未覆盖：count==0
	us := []Usage{{Upload: 0}, {Upload: 0}}
	v := applyField(us, FieldAvg, "upload")
	if v != 0 {
		t.Fatalf("avg zero count 应返回 0, got %d", v)
	}
}

func TestApplyFieldAvgNonZero(t *testing.T) {
	// traffic.go:283 未覆盖：v!=0
	us := []Usage{{Upload: 4}, {Upload: 0}, {Upload: 6}}
	v := applyField(us, FieldAvg, "upload")
	if v != 5 {
		t.Fatalf("avg 应为 5, got %d", v)
	}
}

func TestApplyFieldFirstNonZero(t *testing.T) {
	// traffic.go:293,294 未覆盖：FieldFirst v!=0 / return v
	us := []Usage{{Upload: 0}, {Upload: 7}, {Upload: 9}}
	v := applyField(us, FieldFirst, "upload")
	if v != 7 {
		t.Fatalf("first 应为 7, got %d", v)
	}
}

func TestFormatWithStrategyTotalExpire(t *testing.T) {
	// traffic.go:337,340 未覆盖：s.Total != FieldNone / s.Expire != FieldNone
	u := Usage{Upload: 1, Download: 2, Total: 3, Expire: 4}
	s := Strategy{Upload: FieldNone, Download: FieldNone, Total: FieldSum, Expire: FieldSum}
	out := FormatWithStrategy(u, s)
	if out == "" {
		t.Fatalf("输出不应为空")
	}
	if !contains(out, "total=3") || !contains(out, "expire=4") {
		t.Fatalf("输出应含 total/expire: %s", out)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

// --- 补充：覆盖 switch 各 case 与 default ---

func TestParseSubscriptionUserinfoAllKeys(t *testing.T) {
	// traffic.go:49 未覆盖：download/total/expire case
	u, ok := ParseSubscriptionUserinfo("upload=1;download=2;total=3;expire=4")
	if !ok {
		t.Fatalf("parse 应成功")
	}
	if u.Upload != 1 || u.Download != 2 || u.Total != 3 || u.Expire != 4 {
		t.Fatalf("got %+v", u)
	}
}

func TestPresetStrategyAllCond(t *testing.T) {
	// traffic.go:125 未覆盖：min/max/avg/first/last case
	for _, name := range []string{"min", "max", "avg", "first", "last"} {
		if _, ok := PresetStrategy(name); !ok {
			t.Fatalf("preset %s 应成功", name)
		}
	}
}

func TestGetFieldAll(t *testing.T) {
	// traffic.go:223 未覆盖：upload/download/total/expire case
	u := Usage{Upload: 1, Download: 2, Total: 3, Expire: 4}
	if getField(u, "upload") != 1 {
		t.Fatalf("upload")
	}
	if getField(u, "download") != 2 {
		t.Fatalf("download")
	}
	if getField(u, "total") != 3 {
		t.Fatalf("total")
	}
	if getField(u, "expire") != 4 {
		t.Fatalf("expire")
	}
}

func TestApplyFieldDefault(t *testing.T) {
	// traffic.go:241 未覆盖：applyField switch default
	us := []Usage{{Upload: 1}}
	v := applyField(us, FieldStrategy("bogus"), "upload")
	if v != 0 {
		t.Fatalf("default 应返回 0, got %d", v)
	}
}

func TestApplyFieldFirstAllZero(t *testing.T) {
	// traffic.go:293,294 未覆盖：循环完成未提前返回
	us := []Usage{{Upload: 0}, {Upload: 0}}
	v := applyField(us, FieldFirst, "upload")
	if v != 0 {
		t.Fatalf("first all zero 应返回 0, got %d", v)
	}
}

func TestApplyFieldLastAllZero(t *testing.T) {
	// traffic.go:300,301 未覆盖：循环完成未提前返回
	us := []Usage{{Upload: 0}, {Upload: 0}}
	v := applyField(us, FieldLast, "upload")
	if v != 0 {
		t.Fatalf("last all zero 应返回 0, got %d", v)
	}
}
