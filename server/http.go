package main

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

func respond(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func reject(w http.ResponseWriter, code int, message string) {
	respond(w, code, map[string]string{"error": message})
}

func (p *Plugin) ServeHTTP(_ *plugin.Context, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// Mattermost authenticates the request and supplies this header. Never take
	// identity from a query parameter, path component, or submitted JSON field.
	id := r.Header.Get("Mattermost-User-ID")
	if !model.IsValidId(id) { reject(w, http.StatusUnauthorized, "authentication required"); return }
	user, appErr := p.API.GetUser(id)
	if appErr != nil {
		if appErr.StatusCode == 404 { reject(w, http.StatusUnauthorized, "authentication required") } else { reject(w, http.StatusServiceUnavailable, "user lookup unavailable") }
		return
	}
	if user == nil || user.DeleteAt != 0 || user.IsBot { reject(w, http.StatusForbidden, "active user required"); return }
	if r.URL.RawQuery != "" { reject(w, http.StatusBadRequest, "query parameters are not supported"); return }
	if r.URL.Path != "/api/v1/config" && r.URL.Path != "/api/v1/skip" { reject(w, http.StatusNotFound, "not found"); return }
	if r.Method != http.MethodGet {
		// A cross-site HTML form cannot supply this header. No CORS opt-in is
		// provided. Mattermost's own authentication/CSRF handling remains in place.
		if r.Header.Get("X-Requested-With") != "XMLHttpRequest" || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			reject(w, http.StatusForbidden, "same-origin application request required"); return
		}
	}
	if r.URL.Path == "/api/v1/skip" {
		if r.Method != http.MethodPost { w.Header().Set("Allow", "POST"); reject(w, http.StatusMethodNotAllowed, "use POST"); return }
		// No body (and therefore no client-supplied identity) is accepted.
		if r.Body != nil {
			body, err := io.ReadAll(io.LimitReader(r.Body, 1))
			if err != nil || len(body) != 0 { reject(w, http.StatusBadRequest, "skip takes no body"); return }
		}
		if err := p.service.Skip(r.Context(), id); err != nil { p.writeError(w, err); return }
		p.writeView(w, id)
		return
	}
	switch r.Method {
	case http.MethodGet:
		p.writeView(w, id)
	case http.MethodPut:
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" { reject(w, http.StatusUnsupportedMediaType, "use application/json"); return }
		r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		var input struct {
			Config *Config `json:"config"`
			Revision *uint64 `json:"revision"`
		}
		if err := decoder.Decode(&input); err != nil { reject(w, http.StatusBadRequest, "invalid configuration JSON"); return }
		if err := decoder.Decode(new(any)); err != io.EOF || input.Config == nil || input.Revision == nil {
			reject(w, http.StatusBadRequest, "provide one config object and its revision"); return
		}
		if err := input.Config.Validate(); err != nil { reject(w, http.StatusBadRequest, err.Error()); return }
		if err := p.service.Save(r.Context(), id, *input.Config, *input.Revision); err != nil { p.writeError(w, err); return }
		p.writeView(w, id)
	case http.MethodDelete:
		match := r.Header.Get("If-Match")
		if len(match) < 3 || !strings.HasPrefix(match, "\"") || !strings.HasSuffix(match, "\"") {
			reject(w, http.StatusPreconditionRequired, "If-Match with the configuration revision is required"); return
		}
		revision, err := strconv.ParseUint(match[1:len(match)-1], 10, 64)
		if err != nil { reject(w, http.StatusBadRequest, "invalid revision"); return }
		if err := p.service.Save(r.Context(), id, defaultConfig(), revision); err != nil { p.writeError(w, err); return }
		p.writeView(w, id)
	default:
		w.Header().Set("Allow", "GET, PUT, DELETE")
		reject(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (p *Plugin) writeError(w http.ResponseWriter, err error) {
	if errors.Is(err, errConflict) { reject(w, http.StatusConflict, errConflict.Error()); return }
	if errors.Is(err, errNoInterval) { reject(w, http.StatusConflict, errNoInterval.Error()); return }
	p.API.LogWarn("Scheduled DND request failed", "error", err.Error())
	reject(w, http.StatusInternalServerError, "request failed; retry or check the server logs")
}

func (p *Plugin) writeView(w http.ResponseWriter, id string) {
	view, err := p.service.View(id)
	if err != nil { p.writeError(w, err); return }
	w.Header().Set("ETag", "\""+strconv.FormatUint(view.Revision, 10)+"\"")
	respond(w, http.StatusOK, view)
}
