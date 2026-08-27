package studio

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egekocabas/kick-sim/internal/app"
	kickopenapi "github.com/egekocabas/kick-sim/internal/openapi"
	"github.com/egekocabas/kick-sim/internal/scenario"
	"github.com/egekocabas/kick-sim/internal/workspace"
)

func TestStudioSecurityBoundary(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".kick-sim")
	if _, err := workspace.Init(root); err != nil {
		t.Fatal(err)
	}
	service, err := app.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	running, err := New(service, "127.0.0.1:4321")
	if err != nil {
		t.Fatal(err)
	}

	home := request(t, running.Handler, http.MethodGet, "/", "127.0.0.1:4321", nil, "", "")
	if home.Code != http.StatusOK {
		t.Fatalf("GET / status = %d: %s", home.Code, home.Body.String())
	}
	for name, want := range map[string]string{
		"Content-Security-Policy": "frame-ancestors 'none'",
		"Permissions-Policy":      "camera=()",
		"X-Content-Type-Options":  "nosniff",
	} {
		if !strings.Contains(home.Header().Get(name), want) {
			t.Errorf("%s = %q, want to contain %q", name, home.Header().Get(name), want)
		}
	}
	cookies := home.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != controlCookie || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("control cookie = %#v", cookies)
	}

	badHost := request(t, running.Handler, http.MethodGet, "/api/bootstrap", "attacker.invalid", nil, "", "")
	if badHost.Code != http.StatusForbidden {
		t.Fatalf("bad host status = %d", badHost.Code)
	}
	api := request(t, running.Handler, http.MethodGet, "/api/bootstrap", "127.0.0.1:4321", nil, "", "")
	if api.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("API Cache-Control = %q", api.Header().Get("Cache-Control"))
	}

	missingToken := request(t, running.Handler, http.MethodPost, "/api/workspace/validate", "127.0.0.1:4321", nil, running.URL, "application/json")
	if missingToken.Code != http.StatusUnauthorized {
		t.Fatalf("missing token status = %d", missingToken.Code)
	}
	badContentType := request(t, running.Handler, http.MethodPost, "/api/workspace/validate", "127.0.0.1:4321", cookies[0], running.URL, "application/jsonp")
	if badContentType.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("JSON-like content type status = %d", badContentType.Code)
	}
	wrongOrigin := request(t, running.Handler, http.MethodPost, "/api/workspace/validate", "127.0.0.1:4321", cookies[0], "http://attacker.invalid", "application/json")
	if wrongOrigin.Code != http.StatusForbidden {
		t.Fatalf("wrong origin status = %d", wrongOrigin.Code)
	}
	missingOrigin := request(t, running.Handler, http.MethodPost, "/api/workspace/validate", "127.0.0.1:4321", cookies[0], "", "application/json")
	if missingOrigin.Code != http.StatusUnauthorized {
		t.Fatalf("missing browser origin status = %d", missingOrigin.Code)
	}
	oversized := requestBody(t, running.Handler, http.MethodPost, "/api/workspace/validate", "127.0.0.1:4321", cookies[0], running.URL, "application/json", `{"padding":"`+strings.Repeat("x", maxRequestBody)+`"}`)
	if oversized.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized request status = %d", oversized.Code)
	}

	allowed := request(t, running.Handler, http.MethodPost, "/api/workspace/validate", "127.0.0.1:4321", cookies[0], running.URL, "application/json")
	if allowed.Code != http.StatusOK {
		t.Fatalf("authorized mutation status = %d: %s", allowed.Code, allowed.Body.String())
	}
}

func TestScenarioSourceSaveReturnsConflictWithoutOverwriting(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".kick-sim")
	if _, err := workspace.Init(root); err != nil {
		t.Fatal(err)
	}
	service, err := app.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	store := scenario.NewStore(root, service.EventRegistry(), service.Configuration())
	entry, err := store.Copy("builtin:chat/basic-message", "editing/conflict", "kick-sim@test")
	if err != nil {
		t.Fatal(err)
	}
	running, err := New(service, "127.0.0.1:4321")
	if err != nil {
		t.Fatal(err)
	}
	home := request(t, running.Handler, http.MethodGet, "/", "127.0.0.1:4321", nil, "", "")
	cookie := home.Result().Cookies()[0]
	external := strings.Replace(string(entry.Source), "Hello from Kick Sim", "External edit", 1)
	if err := os.WriteFile(entry.Path, []byte(external), 0o644); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(kickopenapi.ScenarioSourceSaveRequest{ID: entry.ID, Revision: entry.Revision, Source: string(entry.Source)})
	if err != nil {
		t.Fatal(err)
	}
	response := requestBody(t, running.Handler, http.MethodPut, "/api/scenario", "127.0.0.1:4321", cookie, running.URL, "application/json", string(body))
	if response.Code != http.StatusConflict {
		t.Fatalf("save status = %d: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), entry.Revision) || !strings.Contains(response.Body.String(), "sha256:") {
		t.Fatalf("conflict response does not include revisions: %s", response.Body.String())
	}
	onDisk, err := os.ReadFile(entry.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(onDisk) != external {
		t.Fatal("conflicting API save overwrote the external file")
	}
}

func request(t *testing.T, handler http.Handler, method, path, host string, cookie *http.Cookie, origin, contentType string) *httptest.ResponseRecorder {
	return requestBody(t, handler, method, path, host, cookie, origin, contentType, "{}")
}

func requestBody(t *testing.T, handler http.Handler, method, path, host string, cookie *http.Cookie, origin, contentType, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Host = host
	if cookie != nil {
		request.AddCookie(cookie)
	}
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}
