package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

type httpFake struct {
	plugin.API
	fake *fakeAPI
}

func (f httpFake) GetUser(id string) (*model.User, *model.AppError) { return f.fake.GetUser(id) }
func (f httpFake) LogWarn(string, ...any) {}

func httpPlugin(t *testing.T) (*Plugin, *fakeAPI) {
	t.Helper()
	f := newFake()
	p := &Plugin{service: serviceFor(f, instant(t, "2026-10-26T18:00:00Z"))}
	p.SetAPI(httpFake{fake: f})
	return p, f
}

func request(p *Plugin, method, path, id, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	r.Header.Set("Mattermost-User-ID", id)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Requested-With", "XMLHttpRequest")
	w := httptest.NewRecorder()
	p.ServeHTTP(nil, w, r)
	return w
}

func TestHTTPAuthenticationAndIsolation(t *testing.T) {
	p, f := httpPlugin(t)
	for _, id := range []string{"", "invalid", "cccccccccccccccccccccccccc"} {
		if w := request(p, "GET", "/api/v1/config", id, ""); w.Code != 401 { t.Fatalf("got %d for unauthorized ID", w.Code) }
	}
	f.users[userID].DeleteAt = 1
	if w := request(p, "GET", "/api/v1/config", userID, ""); w.Code != 403 { t.Fatal(w.Code) }
	f.users[userID].DeleteAt = 0
	body, _ := json.Marshal(map[string]any{"config": configFor([]string{"monday"}, "17:00", "08:00"), "revision": 0})
	if w := request(p, "PUT", "/api/v1/config", userID, string(body)); w.Code != 200 { t.Fatal(w.Code, w.Body.String()) }
	w := request(p, "GET", "/api/v1/config", otherID, "")
	var view View
	if err := json.Unmarshal(w.Body.Bytes(), &view); err != nil { t.Fatal(err) }
	if w.Code != 200 || view.Config.Enabled || view.Revision != 0 { t.Fatal("another user's config was exposed") }
	if w.Header().Get("Cache-Control") != "no-store" { t.Fatal("private data may be cached") }
	if w := request(p, "GET", "/api/v1/config?user_id="+userID, otherID, ""); w.Code != 400 { t.Fatal("query identity accepted") }
	if w := request(p, "GET", "/api/v1/users/"+userID, otherID, ""); w.Code != 404 { t.Fatal("path identity accepted") }
	if w := request(p, "PUT", "/api/v1/config", userID, string(body)); w.Code != 409 { t.Fatal("stale save accepted") }
}

func TestHTTPMalformedRequestsAndCSRF(t *testing.T) {
	p, _ := httpPlugin(t)
	for _, body := range []string{
		"", "null", "{}", "{", `{"config":null,"revision":0}`,
		`{"config":{"enabled":true,"timezone":"Europe/Warsaw","schedule":[]},"revision":0}`,
		`{"config":{"enabled":false,"timezone":"Local","schedule":[]},"revision":0}`,
		`{"config":{"timezone":"UTC","user_id":"x"},"revision":0}`,
		`{"config":{"timezone":"UTC"},"revision":0,"user_id":"x"}`,
		`{"config":{"timezone":"UTC"},"revision":-1}`,
		`{"config":{"timezone":"UTC"},"revision":0} {}`,
		`{"config":{"timezone":"UTC"},"revision":0,"unknown":true}`,
	} {
		if w := request(p, "PUT", "/api/v1/config", userID, body); w.Code != 400 { t.Fatalf("accepted %q: %d", body, w.Code) }
	}
	large := `{"config":{"timezone":"`+string(bytes.Repeat([]byte{'x'}, 17000))+`"},"revision":0}`
	if w := request(p, "PUT", "/api/v1/config", userID, large); w.Code != 400 { t.Fatal("oversized request accepted") }
	for _, method := range []string{"PUT", "DELETE", "POST"} {
		r := httptest.NewRequest(method, "/api/v1/config", nil)
		r.Header.Set("Mattermost-User-ID", userID)
		w := httptest.NewRecorder(); p.ServeHTTP(nil, w, r)
		if w.Code != 403 { t.Fatal("missing CSRF header accepted", method, w.Code) }
	}
	r := httptest.NewRequest("PUT", "/api/v1/config", nil)
	r.Header.Set("Mattermost-User-ID", userID)
	r.Header.Set("X-Requested-With", "XMLHttpRequest")
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	w := httptest.NewRecorder(); p.ServeHTTP(nil, w, r)
	if w.Code != 403 { t.Fatal("cross-site request accepted") }
	r.Header.Del("Sec-Fetch-Site")
	r.Header.Set("Content-Type", "text/plain")
	w = httptest.NewRecorder(); p.ServeHTTP(nil, w, r)
	if w.Code != 415 { t.Fatal("non-JSON accepted") }
	if w := request(p, "PATCH", "/api/v1/config", userID, ""); w.Code != 405 { t.Fatal(w.Code) }
}

func TestHTTPResetAndSkip(t *testing.T) {
	p, _ := httpPlugin(t)
	saveSchedule(t, p.service, userID)
	if w := request(p, "POST", "/api/v1/skip", userID, ""); w.Code != 200 { t.Fatal(w.Code, w.Body.String()) }
	if w := request(p, "POST", "/api/v1/skip", userID, `{}`); w.Code != 400 { t.Fatal("skip body accepted") }
	if w := request(p, "GET", "/api/v1/skip", userID, ""); w.Code != 405 { t.Fatal(w.Code) }
	if w := request(p, "DELETE", "/api/v1/config", userID, ""); w.Code != 428 { t.Fatal("reset without revision accepted") }
	r := httptest.NewRequest("DELETE", "/api/v1/config", nil)
	r.Header.Set("Mattermost-User-ID", userID)
	r.Header.Set("X-Requested-With", "XMLHttpRequest")
	r.Header.Set("If-Match", `"1"`)
	w := httptest.NewRecorder(); p.ServeHTTP(nil, w, r)
	if w.Code != http.StatusOK { t.Fatal(w.Code, w.Body.String()) }
	var view View
	if err := json.Unmarshal(w.Body.Bytes(), &view); err != nil { t.Fatal(err) }
	if view.Config.Enabled || view.Revision != 2 || len(view.Config.Schedule) != 0 { t.Fatal("reset did not persist defaults") }
	if view.Runtime.SuppressUntil == 0 { t.Fatal("reset erased the manual override") }
}

func TestSkipAcceptsEmptyStreamWithoutContentLength(t *testing.T) {
	p, _ := httpPlugin(t)
	saveSchedule(t, p.service, userID)
	r := httptest.NewRequest("POST", "/api/v1/skip", bytes.NewBuffer(nil))
	r.ContentLength = -1
	r.Header.Set("Mattermost-User-ID", userID)
	r.Header.Set("X-Requested-With", "XMLHttpRequest")
	w := httptest.NewRecorder()
	p.ServeHTTP(nil, w, r)
	if w.Code != 200 { t.Fatal(w.Code, w.Body.String()) }
}
