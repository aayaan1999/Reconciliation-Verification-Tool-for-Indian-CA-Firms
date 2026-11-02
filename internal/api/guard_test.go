package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Requests a web page on another site could make through a staff member's browser are
// refused (security review of 28 October 2026), until logins with a CSRF token arrive.
func TestGuard(t *testing.T) {
	s := &Server{cfg: Config{Hosts: []string{"office-pc"}}}
	h := s.guard(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	for _, c := range []struct {
		name, method, host, origin, site, ctype string
		want                                    int
	}{
		{"a read by IP address", "GET", "127.0.0.1:8080", "", "", "", http.StatusNoContent},
		{"a read by localhost", "GET", "localhost:8080", "", "", "", http.StatusNoContent},
		{"a read by a LAN address", "GET", "192.168.1.10:8080", "", "", "", http.StatusNoContent},
		{"a read by IPv6", "GET", "[::1]:8080", "", "", "", http.StatusNoContent},
		{"a read by a name given with --host", "GET", "OFFICE-PC:8080", "", "", "", http.StatusNoContent},
		{"a read through another name (DNS rebinding)", "GET", "attacker.example:8080", "", "", "", http.StatusMisdirectedRequest},
		{"a post from the screens", "POST", "127.0.0.1:8080", "http://127.0.0.1:8080", "same-origin", "application/json", http.StatusNoContent},
		{"an upload from the screens", "POST", "127.0.0.1:8080", "http://127.0.0.1:8080", "same-origin", "multipart/form-data; boundary=x", http.StatusNoContent},
		{"a post from a program (no Origin)", "PUT", "127.0.0.1:8080", "", "", "application/json; charset=utf-8", http.StatusNoContent},
		{"a form on another site", "POST", "127.0.0.1:8080", "https://attacker.example", "cross-site", "multipart/form-data; boundary=x", http.StatusForbidden},
		{"another site, Origin only", "POST", "127.0.0.1:8080", "https://attacker.example", "", "application/json", http.StatusForbidden},
		{"another site, Sec-Fetch-Site only", "POST", "127.0.0.1:8080", "", "cross-site", "application/json", http.StatusForbidden},
		{"a text/plain body (a form posing as JSON)", "POST", "127.0.0.1:8080", "", "", "text/plain", http.StatusUnsupportedMediaType},
		{"a URL-encoded form", "POST", "127.0.0.1:8080", "", "", "application/x-www-form-urlencoded", http.StatusUnsupportedMediaType},
	} {
		r := httptest.NewRequest(c.method, "/api/v1/clients", strings.NewReader("{}"))
		r.Host = c.host
		for k, v := range map[string]string{"Origin": c.origin, "Sec-Fetch-Site": c.site, "Content-Type": c.ctype} {
			if v != "" {
				r.Header.Set(k, v)
			}
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != c.want {
			t.Errorf("%s: status %d, want %d (%s)", c.name, w.Code, c.want, w.Body.String())
		}
	}
}
