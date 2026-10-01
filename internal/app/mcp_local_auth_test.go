package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMCPLocalPeerAuthorization(t *testing.T) {
	a := &App{restServerToken: "test-token"}
	handler := a.withRESTSecurity(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	for _, tc := range []struct {
		name, peer, path, token string
		allowed                 bool
	}{
		{"本機 MCP", "127.0.0.1:43210", "/mcp", "", true},
		{"IPv4 映射", "[::ffff:127.0.0.1]:43210", "/mcp", "", true},
		{"IPv6 未授權", "[::1]:43210", "/mcp", "", false},
		{"其他 loopback", "127.0.0.2:43210", "/mcp", "", false},
		{"遠端無金鑰", "192.0.2.5:43210", "/mcp", "", false},
		{"遠端錯誤金鑰", "192.0.2.5:43210", "/mcp", "wrong", false},
		{"遠端正確金鑰", "192.0.2.5:43210", "/mcp", "test-token", true},
		{"REST 保留驗證", "127.0.0.1:43210", "/api/sites", "", false},
		{"子路徑不放行", "127.0.0.1:43210", "/mcp/other", "", false},
		{"缺少來源", "", "/mcp", "", false},
		{"非 TCP 來源", "127.0.0.1", "/mcp", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "http://localhost"+tc.path, nil)
			r.RemoteAddr = tc.peer
			r.Header.Set("X-Forwarded-For", "127.0.0.1")
			r.Header.Set("Forwarded", "for=127.0.0.1")
			r.Header.Set("X-Real-IP", "127.0.0.1")
			if tc.token != "" {
				r.Header.Set("Authorization", "Bearer "+tc.token)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			want := http.StatusUnauthorized
			if tc.allowed {
				want = http.StatusOK
			}
			if w.Code != want {
				t.Fatalf("status=%d want=%d", w.Code, want)
			}
		})
	}
}
