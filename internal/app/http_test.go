package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestBrowserBridgeAuthorization(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	h := Handler(s, fstest.MapFS{"index.html": {Data: []byte(`token=__API_TOKEN__`)}}, "127.0.0.1:1234")
	get := httptest.NewRequest("GET", "http://127.0.0.1:1234/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, get)
	token := strings.TrimPrefix(rec.Body.String(), "token=")
	if token == "__API_TOKEN__" || len(token) != 48 {
		t.Fatal("missing token", token)
	}
	call := func(token, origin, host string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "http://127.0.0.1:1234/api", bytes.NewBufferString(`{"method":"state.get","payload":{}}`))
		req.Host = host
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-OhMyMC-Token", token)
		req.Header.Set("Origin", origin)
		out := httptest.NewRecorder()
		h.ServeHTTP(out, req)
		return out
	}
	for _, out := range []*httptest.ResponseRecorder{call("", "", "127.0.0.1:1234"), call(token, "http://evil.test", "127.0.0.1:1234"), call(token, "", "evil.test")} {
		if out.Code != http.StatusForbidden {
			t.Fatal("unauthorized API permitted", out.Code)
		}
	}
	out := call(token, "http://127.0.0.1:1234", "127.0.0.1:1234")
	var response struct {
		Result State `json:"result"`
	}
	if err := json.Unmarshal(out.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Result.Plugins) != 2 || len(response.Result.Versions) != 0 {
		t.Fatal(out.Body.String())
	}
}
