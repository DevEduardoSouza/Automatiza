package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestInjectProxyMergesConfig(t *testing.T) {
	sessionProxies = []ProxyCfg{
		{Server: "1.1.1.1:12323", Username: "u", Password: "p"},
		{Server: "2.2.2.2:12323", Username: "u", Password: "p"},
	}
	proxyCursor = 0
	defer func() { sessionProxies = nil; proxyCursor = 0 }()

	// existing config keys (e.g. metadata) must survive
	orig := `{"name":"sess_abc","start":true,"config":{"metadata":{"x":"y"}}}`
	r, _ := http.NewRequest("POST", "/api/sessions", bytes.NewReader([]byte(orig)))
	injectProxy(r)
	out, _ := io.ReadAll(r.Body)

	var m map[string]interface{}
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("bad json out: %v", err)
	}
	if m["name"] != "sess_abc" || m["start"] != true {
		t.Errorf("top-level fields lost: %s", out)
	}
	cfg := m["config"].(map[string]interface{})
	if _, ok := cfg["metadata"]; !ok {
		t.Errorf("existing config.metadata dropped: %s", out)
	}
	px := cfg["proxy"].(map[string]interface{})
	if px["server"] != "1.1.1.1:12323" || px["username"] != "u" || px["password"] != "p" {
		t.Errorf("proxy not injected correctly: %v", px)
	}
	if r.ContentLength != int64(len(out)) {
		t.Errorf("ContentLength %d != body %d", r.ContentLength, len(out))
	}
}

func TestNextProxyRoundRobin(t *testing.T) {
	sessionProxies = []ProxyCfg{{Server: "a"}, {Server: "b"}, {Server: "c"}}
	proxyCursor = 0
	defer func() { sessionProxies = nil; proxyCursor = 0 }()
	want := []string{"a", "b", "c", "a", "b", "c"}
	for i, w := range want {
		if got := nextProxy().Server; got != w {
			t.Errorf("call %d: got %s want %s", i, got, w)
		}
	}
}

func TestInjectProxyNoPoolIsNoop(t *testing.T) {
	sessionProxies = nil
	orig := `{"name":"s","start":true}`
	r, _ := http.NewRequest("POST", "/api/sessions", bytes.NewReader([]byte(orig)))
	injectProxy(r)
	out, _ := io.ReadAll(r.Body)
	if string(out) != orig {
		t.Errorf("no-op expected, got %s", out)
	}
}

func TestSessionFromPath(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		// The bug: QR + scoped endpoints must resolve the session from segment 1.
		{"/api/session_01kx3a28w9944tj2mkk5rw2ban/auth/qr", "session_01kx3a28w9944tj2mkk5rw2ban"},
		{"/api/session_abc/auth/request-code", "session_abc"},
		{"/api/session_abc/chats", "session_abc"},
		{"/api/session_abc/groups/count", "session_abc"},
		{"/api/session_abc/presence", "session_abc"},
		{"/api/default/auth/qr", "default"},
		// Management form keeps working.
		{"/api/sessions/session_abc", "session_abc"},
		{"/api/sessions/session_abc/start", "session_abc"},
		{"/api/sessions/session_abc/me", "session_abc"},
		{"/api/sessions/session_abc/logout", "session_abc"},
		// Send endpoints carry session in body -> must NOT be read from path.
		{"/api/sendText", ""},
		{"/api/sendImage", ""},
		{"/api/send/buttons/reply", ""},
		{"/api/sendSeen", ""},
		// Global / reserved endpoints -> not session-scoped.
		{"/api/server/status", ""},
		{"/api/contacts/check-exists", ""},
		{"/api/checkNumberStatus", ""},
		{"/api/version", ""},
		{"/api/sessions", ""},
		{"/health", ""},
		{"/ping", ""},
	}
	for _, c := range cases {
		if got := sessionFromPath(c.path); got != c.want {
			t.Errorf("sessionFromPath(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}
