package studio

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egekocabas/kick-sim/internal/app"
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
	cookies := home.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != controlCookie || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("control cookie = %#v", cookies)
	}

	badHost := request(t, running.Handler, http.MethodGet, "/api/bootstrap", "attacker.invalid", nil, "", "")
	if badHost.Code != http.StatusForbidden {
		t.Fatalf("bad host status = %d", badHost.Code)
	}

	missingToken := request(t, running.Handler, http.MethodPost, "/api/workspace/validate", "127.0.0.1:4321", nil, running.URL, "application/json")
	if missingToken.Code != http.StatusUnauthorized {
		t.Fatalf("missing token status = %d", missingToken.Code)
	}

	allowed := request(t, running.Handler, http.MethodPost, "/api/workspace/validate", "127.0.0.1:4321", cookies[0], running.URL, "application/json")
	if allowed.Code != http.StatusOK {
		t.Fatalf("authorized mutation status = %d: %s", allowed.Code, allowed.Body.String())
	}
}

func request(t *testing.T, handler http.Handler, method, path, host string, cookie *http.Cookie, origin, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader("{}"))
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
