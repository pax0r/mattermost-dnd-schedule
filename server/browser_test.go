package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// This host fixture exercises the production HTTP handler and compiled webapp
// against fake Mattermost storage/presence. It is never included in the plugin.
func TestBrowserIntegration(t *testing.T) {
	if os.Getenv("SCHEDULED_DND_BROWSER_TESTS") != "1" { t.Skip("run make test-browser") }
	p, f := httpPlugin(t)
	root, err := filepath.Abs("../webapp")
	if err != nil { t.Fatal(err) }
	mux := http.NewServeMux()
	mux.HandleFunc("/fixture/reset", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" { w.WriteHeader(405); return }
		f.Lock()
		f.data = map[string][]byte{}
		f.kvFailure = false
		f.Unlock()
		if err := p.service.Save(context.Background(), userID, configFor([]string{"monday"}, "17:00", "08:00"), 0); err != nil { t.Error(err); w.WriteHeader(500) }
	})
	mux.HandleFunc("/fixture/unavailable", func(w http.ResponseWriter, r *http.Request) {
		f.Lock(); f.kvFailure = true; f.Unlock()
	})
	mux.HandleFunc("/mattermost/plugins/org.mattermost.scheduled-dnd/", func(w http.ResponseWriter, r *http.Request) {
		r.Header.Del("Mattermost-User-ID")
		if cookie, err := r.Cookie("session"); err == nil && cookie.Value == "fixture-session" { r.Header.Set("Mattermost-User-ID", userID) }
		r.URL.Path = strings.TrimPrefix(r.URL.Path, "/mattermost/plugins/org.mattermost.scheduled-dnd")
		p.ServeHTTP(nil, w, r)
	})
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir(root))))
	mux.HandleFunc("/mattermost/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!doctype html><html><head><meta charset="utf-8"><title>Plugin host fixture</title></head><body>
<button id="open">Settings</button><div id="tabs"></div><main id="panel"></main>
<script src="/assets/node_modules/react/umd/react.development.js"></script>
<script src="/assets/node_modules/react-dom/umd/react-dom.development.js"></script>
<script>
window.registerPlugin = (id, plugin) => {
 if (id !== 'org.mattermost.scheduled-dnd') throw new Error('Unexpected plugin ID');
 plugin.initialize({registerUserSettings(settings) {
  if (settings.id !== id || settings.sections.length !== 1) throw new Error('Invalid settings registration');
  document.getElementById('open').onclick = () => {
   const button = document.createElement('button'); button.textContent = settings.uiName;
   button.onclick = () => ReactDOM.render(React.createElement(settings.sections[0].component), document.getElementById('panel'));
   document.getElementById('tabs').replaceChildren(button);
  };
 }}, {getState: () => ({entities: {general: {config: {SiteURL: location.origin + '/mattermost/'}}}})});
};
</script><script src="/assets/dist/main.js"></script></body></html>`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	cmd := exec.Command("npm", "exec", "--", "playwright", "test")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "BROWSER_BASE_URL="+server.URL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil { t.Fatal(err) }
}
