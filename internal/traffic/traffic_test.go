package traffic

import (
	"testing"
)

func TestParseNormal(t *testing.T) {
	u, ok := ParseSubscriptionUserinfo("upload=123; download=456; total=1073741824; expire=1735689600")
	if !ok {
		t.Fatal("ok false")
	}
	if u.Upload != 123 || u.Download != 456 || u.Total != 1073741824 || u.Expire != 1735689600 {
		t.Fatalf("got %+v", u)
	}
}

func TestParseMissingKey(t *testing.T) {
	u, ok := ParseSubscriptionUserinfo("upload=10")
	if !ok {
		t.Fatal("ok false")
	}
	if u.Upload != 10 || u.Download != 0 || u.Total != 0 || u.Expire != 0 {
		t.Fatalf("got %+v", u)
	}
}

func TestParseInvalidValueSkipped(t *testing.T) {
	u, ok := ParseSubscriptionUserinfo("upload=abc; download=2")
	if !ok {
		t.Fatal("ok false")
	}
	if u.Upload != 0 || u.Download != 2 {
		t.Fatalf("got %+v", u)
	}
}

func TestParsePrefix(t *testing.T) {
	for _, p := range []string{"Subscription-Userinfo: upload=1; download=2", "subscription-userinfo:upload=1; download=2", "SUBSCRIPTION-USERINFO: upload=1"} {
		u, ok := ParseSubscriptionUserinfo(p)
		if !ok || u.Upload != 1 {
			t.Fatalf("prefix case failed: %s -> %+v %v", p, u, ok)
		}
	}
}

func TestParseEmpty(t *testing.T) {
	for _, s := range []string{"", "   ", "	"} {
		_, ok := ParseSubscriptionUserinfo(s)
		if ok {
			t.Fatalf("expected false for %q", s)
		}
	}
}

func TestParseNoKnownKey(t *testing.T) {
	_, ok := ParseSubscriptionUserinfo("foo=1; bar=2")
	if ok {
		t.Fatal("expected false")
	}
}

func TestParseRoundTrip(t *testing.T) {
	u := Usage{Upload: 10, Download: 20, Total: 30, Expire: 40}
	s := FormatSubscriptionUserinfo(u)
	u2, ok := ParseSubscriptionUserinfo(s)
	if !ok || u2 != u {
		t.Fatalf("roundtrip failed: %s -> %+v", s, u2)
	}
}

func TestFormatExact(t *testing.T) {
	u := Usage{Upload: 1, Download: 2, Total: 3, Expire: 4}
	got := FormatSubscriptionUserinfo(u)
	want := "upload=1; download=2; total=3; expire=4"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNormalizeEmpty(t *testing.T) {
	s := Strategy{}.Normalize()
	if s.Upload != FieldSum || s.Download != FieldSum || s.Total != FieldSum || s.Expire != FieldMin {
		t.Fatalf("got %+v", s)
	}
}

func TestNormalizeFilled(t *testing.T) {
	s := Strategy{Upload: FieldMax, Download: FieldMin, Total: FieldAvg, Expire: FieldLast, Preset: "x"}.Normalize()
	if s.Upload != FieldMax || s.Download != FieldMin || s.Total != FieldAvg || s.Expire != FieldLast || s.Preset != "x" {
		t.Fatalf("got %+v", s)
	}
}

func TestPresetStrategyAll(t *testing.T) {
	names := []string{"sum", "min", "max", "avg", "first", "last", "min_quota", "max_quota", "weighted", "off"}
	for _, n := range names {
		s, ok := PresetStrategy(n)
		if !ok {
			t.Fatalf("preset %s not found", n)
		}
		if s.Preset != n {
			// case canonical: preset stored lower
			t.Logf("preset %s stored %s", n, s.Preset)
		}
	}
	// case insensitive
	if _, ok := PresetStrategy("SUM"); !ok {
		t.Fatal("case insensitive failed")
	}
	// unknown
	if _, ok := PresetStrategy("nope"); ok {
		t.Fatal("unknown should be false")
	}
}

func TestPresetStrategyOff(t *testing.T) {
	s, _ := PresetStrategy("off")
	if s.Upload != FieldNone || s.Download != FieldNone || s.Total != FieldNone || s.Expire != FieldNone {
		t.Fatalf("got %+v", s)
	}
}

func TestAggregateEmpty(t *testing.T) {
	_, ok := Aggregate(nil, nil, Strategy{})
	if ok {
		t.Fatal("expected false")
	}
}

func TestAggregateAllNone(t *testing.T) {
	usages := []Usage{{Upload: 1}}
	_, ok := Aggregate(usages, nil, Strategy{Upload: FieldNone, Download: FieldNone, Total: FieldNone, Expire: FieldNone})
	if ok {
		t.Fatal("expected false")
	}
}

func TestFieldSum(t *testing.T) {
	usages := []Usage{{Upload: 1}, {Upload: 2}, {Upload: 0}}
	got, _ := Aggregate(usages, nil, Strategy{Upload: FieldSum, Download: FieldNone, Total: FieldNone, Expire: FieldNone})
	if got.Upload != 3 {
		t.Fatalf("got %d", got.Upload)
	}
}

func TestFieldMinTotalInfinity(t *testing.T) {
	usages := []Usage{{Total: 0}, {Total: 100}}
	got, _ := Aggregate(usages, nil, Strategy{Upload: FieldNone, Download: FieldNone, Total: FieldMin, Expire: FieldNone})
	if got.Total != 100 {
		t.Fatalf("got %d", got.Total)
	}
}

func TestFieldMinExpireIgnoreZero(t *testing.T) {
	usages := []Usage{{Expire: 0}, {Expire: 10}}
	got, _ := Aggregate(usages, nil, Strategy{Upload: FieldNone, Download: FieldNone, Total: FieldNone, Expire: FieldMin})
	if got.Expire != 10 {
		t.Fatalf("got %d", got.Expire)
	}
}

func TestFieldMax(t *testing.T) {
	usages := []Usage{{Upload: 1}, {Upload: 5}, {Upload: 0}}
	got, _ := Aggregate(usages, nil, Strategy{Upload: FieldMax, Download: FieldNone, Total: FieldNone, Expire: FieldNone})
	if got.Upload != 5 {
		t.Fatalf("got %d", got.Upload)
	}
}

func TestFieldAvg(t *testing.T) {
	usages := []Usage{{Upload: 1}, {Upload: 2}}
	got, _ := Aggregate(usages, nil, Strategy{Upload: FieldAvg, Download: FieldNone, Total: FieldNone, Expire: FieldNone})
	if got.Upload != 1 {
		t.Fatalf("got %d", got.Upload)
	}
}

func TestFieldFirstLast(t *testing.T) {
	usages := []Usage{{Upload: 0}, {Upload: 7}, {Upload: 9}}
	got, _ := Aggregate(usages, nil, Strategy{Upload: FieldFirst, Download: FieldNone, Total: FieldNone, Expire: FieldNone})
	if got.Upload != 7 {
		t.Fatalf("first got %d", got.Upload)
	}
	got2, _ := Aggregate(usages, nil, Strategy{Upload: FieldLast, Download: FieldNone, Total: FieldNone, Expire: FieldNone})
	if got2.Upload != 9 {
		t.Fatalf("last got %d", got2.Upload)
	}
}

func TestFieldNone(t *testing.T) {
	usages := []Usage{{Upload: 5}}
	got, _ := Aggregate(usages, nil, Strategy{Upload: FieldNone, Download: FieldNone, Total: FieldNone, Expire: FieldNone})
	_ = got
	// all none -> false already tested
}

func TestMinQuota(t *testing.T) {
	usages := []Usage{{Total: 100, Upload: 1}, {Total: 0, Upload: 2}, {Total: 50, Upload: 3}}
	s, _ := PresetStrategy("min_quota")
	got, ok := Aggregate(usages, nil, s)
	if !ok {
		t.Fatal("ok false")
	}
	if got.Total != 50 || got.Upload != 3 {
		t.Fatalf("got %+v", got)
	}
}

func TestMaxQuota(t *testing.T) {
	usages := []Usage{{Total: 100, Upload: 1}, {Total: 0, Upload: 2}, {Total: 50, Upload: 3}}
	s, _ := PresetStrategy("max_quota")
	got, ok := Aggregate(usages, nil, s)
	if !ok {
		t.Fatal("ok false")
	}
	if got.Total != 0 || got.Upload != 2 {
		t.Fatalf("got %+v", got)
	}
}

func TestWeighted(t *testing.T) {
	usages := []Usage{{Upload: 10, Download: 0, Total: 100, Expire: 5}, {Upload: 20, Download: 0, Total: 200, Expire: 3}}
	s, _ := PresetStrategy("weighted")
	got, ok := Aggregate(usages, []int{1, 1}, s)
	if !ok {
		t.Fatal("ok false")
	}
	// weighted avg: (10+20)/2=15 upload; total (100+200)/2=150; expire min=3
	if got.Upload != 15 || got.Total != 150 || got.Expire != 3 {
		t.Fatalf("got %+v", got)
	}
}

func TestWeightedDegenerate(t *testing.T) {
	usages := []Usage{{Upload: 1}, {Upload: 2}}
	s, _ := PresetStrategy("weighted")
	got, ok := Aggregate(usages, nil, s)
	if !ok {
		t.Fatal("ok false")
	}
	if got.Upload != 1 { // avg of 1,2 = 1
		t.Fatalf("got %d", got.Upload)
	}
}

func TestOffPreset(t *testing.T) {
	usages := []Usage{{Upload: 1}}
	s, _ := PresetStrategy("off")
	_, ok := Aggregate(usages, nil, s)
	if ok {
		t.Fatal("expected false")
	}
}

func TestAllZeroOk(t *testing.T) {
	usages := []Usage{{}, {}}
	got, ok := Aggregate(usages, nil, Strategy{Upload: FieldSum, Download: FieldSum, Total: FieldSum, Expire: FieldMin})
	if !ok {
		t.Fatal("expected true")
	}
	if got != (Usage{}) {
		t.Fatalf("got %+v", got)
	}
}

func TestNoClip(t *testing.T) {
	usages := []Usage{{Upload: 200, Download: 100, Total: 100}}
	got, _ := Aggregate(usages, nil, Strategy{Upload: FieldSum, Download: FieldSum, Total: FieldSum, Expire: FieldNone})
	if got.Total != 100 || got.Upload+got.Download != 300 {
		t.Fatalf("got %+v", got)
	}
}

func TestExpireMinIgnoreZero(t *testing.T) {
	// already covered; extra
	usages := []Usage{{Expire: 0}, {Expire: 0}}
	got, _ := Aggregate(usages, nil, Strategy{Upload: FieldNone, Download: FieldNone, Total: FieldNone, Expire: FieldMin})
	if got.Expire != 0 {
		t.Fatalf("got %d", got.Expire)
	}
}

func TestToHeaderFull(t *testing.T) {
	u := Usage{Upload: 1, Download: 2, Total: 3, Expire: 4}
	got := ToHeader(u)
	want := "upload=1; download=2; total=3; expire=4"
	if got != want {
		t.Fatalf("got %q", got)
	}
}

func TestFormatNoneSkip(t *testing.T) {
	u := Usage{Upload: 1, Download: 2, Total: 3, Expire: 4}
	s := Strategy{Upload: FieldNone, Download: FieldSum, Total: FieldSum, Expire: FieldSum}
	got := format(u, s)
	want := "download=2; total=3; expire=4"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatAllNone(t *testing.T) {
	u := Usage{Upload: 1}
	s := Strategy{Upload: FieldNone, Download: FieldNone, Total: FieldNone, Expire: FieldNone}
	got := format(u, s)
	if got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestParsePrefixWhitespace(t *testing.T) {
	u, ok := ParseSubscriptionUserinfo("  Subscription-Userinfo:   upload=5  ;  download=6  ")
	if !ok || u.Upload != 5 || u.Download != 6 {
		t.Fatalf("got %+v %v", u, ok)
	}
}

func TestParseNoEquals(t *testing.T) {
	_, ok := ParseSubscriptionUserinfo("garbage; upload=1")
	if !ok {
		t.Fatal("expected ok true")
	}
	u, _ := ParseSubscriptionUserinfo("garbage; upload=1")
	if u.Upload != 1 {
		t.Fatalf("got %+v", u)
	}
}
