// subscribe_traffic_test.go：/subscribe 透传 subscription-userinfo（TF-6）。
package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/tktogether/subforge/internal/traffic"
)

func TestSubscribeTrafficHeaderDefaultSum(t *testing.T) {
	ts, mgr := uiSubServer(t)
	ctx := context.Background()

	u := fixtureURL(t)
	a, err := mgr.AddAirport(ctx, "a1", u, "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	if err := mgr.UpdateAirportTraffic(ctx, a.ID, &traffic.Usage{
		Upload: 100, Download: 200, Total: 1000, Expire: 1700000000,
	}, 5, ""); err != nil {
		t.Fatalf("UpdateAirportTraffic: %v", err)
	}
	if err := mgr.CreateSubToken(ctx, "sub-token-1", "测试"); err != nil {
		t.Fatalf("CreateSubToken: %v", err)
	}

	code, hdr, body := uiGet(t, ts, "/subscribe?token=sub-token-1")
	if code != http.StatusOK {
		t.Fatalf("status=%d body=%s", code, body)
	}
	want := "upload=100; download=200; total=1000; expire=1700000000"
	if got := hdr.Get("Subscription-Userinfo"); got != want {
		t.Fatalf("header=%q want=%q", got, want)
	}
}

func TestSubscribeTrafficHeaderSumsAirports(t *testing.T) {
	ts, mgr := uiSubServer(t)
	ctx := context.Background()

	u := fixtureURL(t)
	a1, err := mgr.AddAirport(ctx, "a1", u, "", true)
	if err != nil {
		t.Fatalf("AddAirport1: %v", err)
	}
	a2, err := mgr.AddAirport(ctx, "a2", u, "", true)
	if err != nil {
		t.Fatalf("AddAirport2: %v", err)
	}
	if err := mgr.UpdateAirportTraffic(ctx, a1.ID, &traffic.Usage{
		Upload: 100, Download: 200, Total: 1000, Expire: 1700000000,
	}, 5, ""); err != nil {
		t.Fatalf("UpdateAirportTraffic1: %v", err)
	}
	if err := mgr.UpdateAirportTraffic(ctx, a2.ID, &traffic.Usage{
		Upload: 1, Download: 2, Total: 30, Expire: 1600000000,
	}, 2, ""); err != nil {
		t.Fatalf("UpdateAirportTraffic2: %v", err)
	}
	if err := mgr.CreateSubToken(ctx, "sub-token-1", "测试"); err != nil {
		t.Fatalf("CreateSubToken: %v", err)
	}

	code, hdr, body := uiGet(t, ts, "/subscribe?token=sub-token-1")
	if code != http.StatusOK {
		t.Fatalf("status=%d body=%s", code, body)
	}
	want := "upload=101; download=202; total=1030; expire=1600000000"
	if got := hdr.Get("Subscription-Userinfo"); got != want {
		t.Fatalf("header=%q want=%q", got, want)
	}
}

func TestSubscribeTrafficHeaderOffPreset(t *testing.T) {
	ts, mgr := uiSubServer(t)
	ctx := context.Background()

	u := fixtureURL(t)
	a, err := mgr.AddAirport(ctx, "a1", u, "", true)
	if err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	if err := mgr.UpdateAirportTraffic(ctx, a.ID, &traffic.Usage{
		Upload: 100, Download: 200, Total: 1000, Expire: 1700000000,
	}, 5, ""); err != nil {
		t.Fatalf("UpdateAirportTraffic: %v", err)
	}
	if err := mgr.SetTrafficStrategy(ctx, traffic.Strategy{
		Preset: "off", Upload: traffic.FieldNone, Download: traffic.FieldNone,
		Total: traffic.FieldNone, Expire: traffic.FieldNone,
	}); err != nil {
		t.Fatalf("SetTrafficStrategy: %v", err)
	}
	if err := mgr.CreateSubToken(ctx, "sub-token-1", "测试"); err != nil {
		t.Fatalf("CreateSubToken: %v", err)
	}

	code, hdr, body := uiGet(t, ts, "/subscribe?token=sub-token-1")
	if code != http.StatusOK {
		t.Fatalf("status=%d body=%s", code, body)
	}
	if got := hdr.Get("Subscription-Userinfo"); got != "" {
		t.Fatalf("header=%q want empty", got)
	}
}

func TestSubscribeTrafficHeaderFirstPreset(t *testing.T) {
	ts, mgr := uiSubServer(t)
	ctx := context.Background()

	u := fixtureURL(t)
	a1, err := mgr.AddAirport(ctx, "a1", u, "", true)
	if err != nil {
		t.Fatalf("AddAirport1: %v", err)
	}
	a2, err := mgr.AddAirport(ctx, "a2", u, "", true)
	if err != nil {
		t.Fatalf("AddAirport2: %v", err)
	}
	if err := mgr.UpdateAirportTraffic(ctx, a1.ID, &traffic.Usage{
		Upload: 100, Download: 200, Total: 1000, Expire: 1700000000,
	}, 5, ""); err != nil {
		t.Fatalf("UpdateAirportTraffic1: %v", err)
	}
	if err := mgr.UpdateAirportTraffic(ctx, a2.ID, &traffic.Usage{
		Upload: 1, Download: 2, Total: 30, Expire: 1600000000,
	}, 2, ""); err != nil {
		t.Fatalf("UpdateAirportTraffic2: %v", err)
	}
	if err := mgr.SetTrafficStrategy(ctx, traffic.Strategy{
		Preset: "first", Upload: traffic.FieldFirst, Download: traffic.FieldFirst,
		Total: traffic.FieldFirst, Expire: traffic.FieldFirst,
	}); err != nil {
		t.Fatalf("SetTrafficStrategy: %v", err)
	}
	if err := mgr.CreateSubToken(ctx, "sub-token-1", "测试"); err != nil {
		t.Fatalf("CreateSubToken: %v", err)
	}

	code, hdr, body := uiGet(t, ts, "/subscribe?token=sub-token-1")
	if code != http.StatusOK {
		t.Fatalf("status=%d body=%s", code, body)
	}
	want := "upload=100; download=200; total=1000; expire=1700000000"
	if got := hdr.Get("Subscription-Userinfo"); got != want {
		t.Fatalf("header=%q want=%q", got, want)
	}
}

func TestSubscribeTrafficHeaderAbsentWithoutTraffic(t *testing.T) {
	ts, mgr := uiSubServer(t)
	ctx := context.Background()

	u := fixtureURL(t)
	if _, err := mgr.AddAirport(ctx, "a1", u, "", true); err != nil {
		t.Fatalf("AddAirport: %v", err)
	}
	if err := mgr.CreateSubToken(ctx, "sub-token-1", "测试"); err != nil {
		t.Fatalf("CreateSubToken: %v", err)
	}

	code, hdr, body := uiGet(t, ts, "/subscribe?token=sub-token-1")
	if code != http.StatusOK {
		t.Fatalf("status=%d body=%s", code, body)
	}
	if got := hdr.Get("Subscription-Userinfo"); got != "" {
		t.Fatalf("header=%q want empty", got)
	}
}
