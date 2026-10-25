package server

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Theyka/github-cards/internal/cards"
)

// These files travel with the binary: the production image needs no Node,
// frontend build, external CDN, or separately mounted static directory.
//
//go:embed web/index.html web/builder.css web/builder.mjs web/builder-core.mjs
var builderFiles embed.FS

func serveBuilderFile(w http.ResponseWriter, r *http.Request, name, contentType string) {
	body, err := builderFiles.ReadFile("web/" + name)
	if err != nil {
		http.Error(w, "Builder asset unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// Revalidate across deployments rather than mixing old JS with new HTML.
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("ETag", fmt.Sprintf(`"%x"`, sha256.Sum256(body)))
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(body))
}

func serveBuilderAsset(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	switch name {
	case "builder.css":
		serveBuilderFile(w, r, name, "text/css; charset=utf-8")
	case "builder.mjs", "builder-core.mjs":
		serveBuilderFile(w, r, name, "text/javascript; charset=utf-8")
	default:
		http.NotFound(w, r)
	}
}

func serveBuilderThemes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_ = json.NewEncoder(w).Encode(cards.ThemeCatalog())
}
