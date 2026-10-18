package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type errTransport struct{}

func (errTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("boom")
}

type rewriteTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (t rewriteTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r.URL.Scheme = t.target.Scheme
	r.URL.Host = t.target.Host
	return t.base.RoundTrip(r)
}

func TestTelegramChannel_ConstructAndName(t *testing.T) {
	ch := NewTelegramChannel("tk", "cid")
	if ch.BotToken != "tk" {
		t.Errorf("BotToken = %q", ch.BotToken)
	}
	if ch.ChatID != "cid" {
		t.Errorf("ChatID = %q", ch.ChatID)
	}
	if ch.client == nil {
		t.Fatal("client nil")
	}
	if ch.client.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v", ch.client.Timeout)
	}
	if ch.Name() != "telegram" {
		t.Errorf("Name = %q", ch.Name())
	}
}

func TestDingTalkChannel_ConstructAndName(t *testing.T) {
	ch := NewDingTalkChannel("http://x")
	if ch.WebhookURL != "http://x" {
		t.Errorf("WebhookURL = %q", ch.WebhookURL)
	}
	if ch.client == nil {
		t.Fatal("client nil")
	}
	if ch.client.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v", ch.client.Timeout)
	}
	if ch.Name() != "dingtalk" {
		t.Errorf("Name = %q", ch.Name())
	}
}

func TestSMTPChannel_NameAndSend(t *testing.T) {
	ch := &SMTPChannel{}
	if ch.Name() != "smtp" {
		t.Errorf("Name = %q", ch.Name())
	}
	err := ch.Send(context.Background(), "x")
	if err == nil {
		t.Fatal("expected err")
	}
	if err.Error() != "SMTP 通道尚未实现，请使用 Telegram 或 DingTalk" {
		t.Errorf("err = %q", err.Error())
	}
}

func TestLogAlertChannel_NameAndLogs(t *testing.T) {
	ch := &LogAlertChannel{}
	if ch.Name() != "log" {
		t.Errorf("Name = %q", ch.Name())
	}
	if err := ch.Send(context.Background(), "m1"); err != nil {
		t.Fatalf("Send1: %v", err)
	}
	if err := ch.Send(context.Background(), "m2"); err != nil {
		t.Fatalf("Send2: %v", err)
	}
	logs := ch.Logs()
	if len(logs) != 2 {
		t.Fatalf("len = %d", len(logs))
	}
	if logs[0] != "m1" || logs[1] != "m2" {
		t.Errorf("logs = %v", logs)
	}
	logs[0] = "x"
	if ch.Logs()[0] != "m1" {
		t.Error("Logs() did not return a copy")
	}
}

func TestTelegramSend_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if r.URL.Path != "/botTOKEN/sendMessage" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
			t.Errorf("content-type = %s", r.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(r.Body)
		vals, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse body: %v", err)
		}
		if vals.Get("chat_id") != "cid" {
			t.Errorf("chat_id = %q", vals.Get("chat_id"))
		}
		if vals.Get("text") != "hello" {
			t.Errorf("text = %q", vals.Get("text"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(ts.Close)
	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	ch := NewTelegramChannel("TOKEN", "cid")
	ch.client = &http.Client{Transport: rewriteTransport{target: u, base: http.DefaultTransport}}
	if err := ch.Send(context.Background(), "hello"); err != nil {
		t.Fatalf("Send: %v", err)
	}
}

func TestTelegramSend_Non200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(ts.Close)
	u, _ := url.Parse(ts.URL)
	ch := NewTelegramChannel("TOKEN", "cid")
	ch.client = &http.Client{Transport: rewriteTransport{target: u, base: http.DefaultTransport}}
	err := ch.Send(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected err")
	}
	if err.Error() != "Telegram 响应码 500" {
		t.Errorf("err = %q", err.Error())
	}
}

func TestTelegramSend_TransportError(t *testing.T) {
	ch := NewTelegramChannel("TOKEN", "cid")
	ch.client = &http.Client{Transport: errTransport{}}
	err := ch.Send(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected err")
	}
	// 实测：http.Client.Do 会把 RoundTrip 错误包成 *url.Error，
	// 因此实际文案为 "Telegram 发送: Post \"https://api.telegram.org/botTOKEN/sendMessage\": boom"。
	if !strings.HasPrefix(err.Error(), "Telegram 发送: ") {
		t.Errorf("err = %q", err.Error())
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("err = %q", err.Error())
	}
}

func TestTelegramSend_ConstructFailure(t *testing.T) {
	ch := NewTelegramChannel("bad\n", "cid")
	err := ch.Send(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected err")
	}
	if !strings.Contains(err.Error(), "invalid control character") {
		t.Errorf("err = %q", err.Error())
	}
}

func TestDingTalkSend_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			t.Errorf("content-type = %s", r.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(r.Body)
		var got struct {
			MsgType string `json:"msgtype"`
			Text    struct {
				Content string `json:"content"`
			} `json:"text"`
		}
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if got.MsgType != "text" {
			t.Errorf("msgtype = %q", got.MsgType)
		}
		if got.Text.Content != "hello" {
			t.Errorf("content = %q", got.Text.Content)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(ts.Close)
	ch := NewDingTalkChannel(ts.URL)
	if err := ch.Send(context.Background(), "hello"); err != nil {
		t.Fatalf("Send: %v", err)
	}
}

func TestDingTalkSend_Non200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(ts.Close)
	ch := NewDingTalkChannel(ts.URL)
	err := ch.Send(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected err")
	}
	if err.Error() != "DingTalk 响应码 403" {
		t.Errorf("err = %q", err.Error())
	}
}

func TestDingTalkSend_TransportError(t *testing.T) {
	ch := NewDingTalkChannel("http://example.com/x")
	ch.client = &http.Client{Transport: errTransport{}}
	err := ch.Send(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected err")
	}
	// 实测：http.Client.Do 会把 RoundTrip 错误包成 *url.Error，
	// 因此实际文案为 "DingTalk 发送: Post \"http://example.com/x\": boom"。
	if !strings.HasPrefix(err.Error(), "DingTalk 发送: ") {
		t.Errorf("err = %q", err.Error())
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("err = %q", err.Error())
	}
}

func TestDingTalkSend_ConstructFailure(t *testing.T) {
	ch := NewDingTalkChannel("://bad\n")
	err := ch.Send(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected err")
	}
}

var _ = fmt.Sprintf
