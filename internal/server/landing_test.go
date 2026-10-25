package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Theyka/github-cards/internal/github"
)

func TestBuilderRoutes(t *testing.T) {
	handler := New(github.NewClient()).Handler()
	for _, test := range []struct {
		path, contentType, contains string
	}{
		{"/", "text/html", `id="builder-form"`},
		{"/assets/builder.css", "text/css", "@media (max-width: 760px)"},
		{"/assets/builder.mjs", "text/javascript", "function renderPreview"},
		{"/assets/builder-core.mjs", "text/javascript", "function buildOutputs"},
		{"/builder/themes", "application/json", `"radical"`},
	} {
		t.Run(test.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
			if response.Code != http.StatusOK {
				t.Fatalf("status: got %d, want 200", response.Code)
			}
			if !strings.HasPrefix(response.Header().Get("Content-Type"), test.contentType) {
				t.Errorf("unexpected Content-Type: %s", response.Header().Get("Content-Type"))
			}
			if !strings.Contains(response.Body.String(), test.contains) {
				t.Errorf("body is missing %q", test.contains)
			}
			if response.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Error("missing nosniff header")
			}
		})
	}
}

func TestBuilderThemeCatalog(t *testing.T) {
	response := httptest.NewRecorder()
	New(github.NewClient()).Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/builder/themes", nil))
	var themes map[string]map[string]map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &themes); err != nil {
		t.Fatal(err)
	}
	if themes["stats"]["radical"]["bg_color"] != "141321" || themes["streak"]["radical"]["background"] != "#141321" {
		t.Fatal("theme catalog must expose the renderer's palettes for each card family")
	}
	if len(themes["stats"]) < 70 || len(themes["streak"]) < 160 {
		t.Fatal("theme catalog must include the full embedded theme collections")
	}
}

func TestLegacyRootImageRedirects(t *testing.T) {
	handler := New(github.NewClient()).Handler()
	for _, test := range []struct{ path, location string }{
		{"/?username=Theyka&theme=radical", "/api?username=Theyka&theme=radical"},
		{"/?user=Theyka&theme=radical", "/streak?user=Theyka&theme=radical"},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
		if response.Code != http.StatusFound || response.Header().Get("Location") != test.location {
			t.Errorf("legacy URL %s: got status %d, Location %q", test.path, response.Code, response.Header().Get("Location"))
		}
	}
}

func TestBuilderAssetCachingAndIsolation(t *testing.T) {
	handler := New(github.NewClient()).Handler()
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/assets/builder.mjs", nil))
	if first.Header().Get("ETag") == "" || first.Header().Get("Cache-Control") != "no-cache" {
		t.Fatal("assets must revalidate across deployments")
	}
	request := httptest.NewRequest(http.MethodGet, "/assets/builder.mjs", nil)
	request.Header.Set("If-None-Match", first.Header().Get("ETag"))
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, request)
	if second.Code != http.StatusNotModified || second.Body.Len() != 0 {
		t.Fatal("unchanged assets must support conditional requests")
	}
	for _, path := range []string{"/assets/index.html", "/assets/landing.go", "/assets/.env", "/assets/missing.js"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Errorf("%s: got %d, want 404", path, response.Code)
		}
	}
	for _, path := range []string{"/", "/assets/builder.css"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodHead, path, nil))
		if response.Code != http.StatusOK || response.Body.Len() != 0 {
			t.Errorf("HEAD %s: got status %d, body length %d", path, response.Code, response.Body.Len())
		}
	}
}

func TestBuilderSecurityPolicy(t *testing.T) {
	response := httptest.NewRecorder()
	New(github.NewClient()).Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(response.Header().Get("Content-Security-Policy"), "script-src 'self'") {
		t.Fatal("builder scripts must be served locally")
	}
	if strings.Contains(response.Body.String(), "<style>") || strings.Contains(response.Body.String(), "<script>") {
		t.Fatal("the builder should not require unsafe inline scripts or styles")
	}
}
