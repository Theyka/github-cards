// Package server implements the HTTP layer: routing, query parsing,
// caching and SVG responses.
package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Theyka/github-cards/internal/cards"
	"github.com/Theyka/github-cards/internal/github"
)

const (
	defaultStatsTTL  = 6 * time.Hour
	defaultStreakTTL = 24 * time.Hour
	maxErrorTTL      = 15 * time.Minute
)

// Server serves the stats, pin and streak cards.
type Server struct {
	client    *github.Client
	cache     *Cache
	statsTTL  time.Duration
	streakTTL time.Duration
}

// New creates a Server. CACHE_SECONDS overrides both cache durations.
func New(client *github.Client) *Server {
	statsTTL := defaultStatsTTL
	streakTTL := defaultStreakTTL
	if v := strings.TrimSpace(os.Getenv("CACHE_SECONDS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			statsTTL = time.Duration(n) * time.Second
			streakTTL = statsTTL
		}
	}
	return &Server{
		client:    client,
		cache:     NewCache(),
		statsTTL:  statsTTL,
		streakTTL: streakTTL,
	}
}

// Handler returns the HTTP routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleRoot)
	mux.HandleFunc("GET /assets/{file}", serveBuilderAsset)
	mux.HandleFunc("GET /builder/themes", serveBuilderThemes)
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api", s.handleStats)
	mux.HandleFunc("GET /api/", s.handleStats)
	mux.HandleFunc("GET /api/pin", s.handlePin)
	mux.HandleFunc("GET /api/pin/", s.handlePin)
	mux.HandleFunc("GET /streak", s.handleStreak)
	mux.HandleFunc("GET /streak/", s.handleStreak)
	return mux
}

func writeSVG(w http.ResponseWriter, svg string, ttl time.Duration) {
	w.Header().Set("Content-Type", "image/svg+xml")
	if ttl > 0 {
		w.Header().Set("Cache-Control", "public, max-age="+strconv.Itoa(int(ttl.Seconds())))
		w.Header().Set("Expires", time.Now().Add(ttl).UTC().Format(http.TimeFormat))
		w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(svg))
}

func errorMessage(err error) string {
	var ghErr *github.Error
	if errors.As(err, &ghErr) {
		return ghErr.Message
	}
	var renderErr *cards.RenderError
	if errors.As(err, &renderErr) {
		return renderErr.Message
	}
	return err.Error()
}

func errorSecondary(err error) string {
	var ghErr *github.Error
	if errors.As(err, &ghErr) {
		return ghErr.SecondaryMessage()
	}
	var renderErr *cards.RenderError
	if errors.As(err, &renderErr) {
		return renderErr.Secondary
	}
	return ""
}

func errorShowRepoLink(err error) bool {
	var ghErr *github.Error
	if errors.As(err, &ghErr) {
		return ghErr.Kind != github.KindMissingParam
	}
	return true
}

func errorTTLFor(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return 0
	}
	if ttl > maxErrorTTL {
		return maxErrorTTL
	}
	return ttl
}

func buildErrorCard(err error, colors cards.ColorInput) []byte {
	return []byte(cards.RenderErrorCard(
		errorMessage(err),
		errorSecondary(err),
		colors,
		errorShowRepoLink(err),
	))
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Has("user") {
		http.Redirect(w, r, "/streak?"+r.URL.RawQuery, http.StatusFound)
		return
	}
	if q.Has("username") {
		http.Redirect(w, r, "/api?"+r.URL.RawQuery, http.StatusFound)
		return
	}
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' blob:; connect-src 'self'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	serveBuilderFile(w, r, "index.html", "text/html; charset=utf-8")
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	colorOptions := colorInputFromQuery(q)

	if invalid := findInvalidColorParam(q); invalid != "" {
		writeSVG(w, cards.RenderErrorCard(
			"Something went wrong",
			`Invalid color input for parameter "`+invalid+`"`,
			colorOptions,
			true,
		), errorTTLFor(s.statsTTL))
		return
	}

	username := q.Get("username")
	repo := q.Get("repo")
	owner := q.Get("owner")
	if (username != "" && !safePatternRe.MatchString(username)) ||
		(repo != "" && !safePatternRe.MatchString(repo)) ||
		(owner != "" && !safePatternRe.MatchString(owner)) {
		writeSVG(w, cards.RenderErrorCard(
			"Something went wrong",
			"Username, repository or owner contains unsafe characters",
			colorOptions,
			true,
		), errorTTLFor(s.statsTTL))
		return
	}

	var commitsYear *int
	if v, ok := stringParam(q, "commits_year"); ok {
		if !yearRe.MatchString(v) {
			writeSVG(w, cards.RenderErrorCard(
				"Something went wrong",
				`Invalid number input for parameter "commits_year"`,
				colorOptions,
				true,
			), errorTTLFor(s.statsTTL))
			return
		}
		y, _ := strconv.Atoi(v)
		commitsYear = &y
	}

	includeAllCommits := boolParam(q, "include_all_commits", false)

	params := github.StatsParams{
		Username:                username,
		IncludeAllCommits:       includeAllCommits,
		ExcludeRepos:            parseListParam(q.Get("exclude_repo")),
		CommitsYear:             commitsYear,
		OwnerAffiliations:       parseListParam(q.Get("role")),
		ContribsIncludeOwnRepos: boolParam(q, "contribs_include_own_repos", false),
	}

	options := cards.StatsCardOptions{
		Colors:            colorOptions,
		Hide:              parseListParam(q.Get("hide")),
		ShowIcons:         boolParam(q, "show_icons", false),
		HideTitle:         boolParam(q, "hide_title", false),
		HideBorder:        boolParam(q, "hide_border", false),
		CardWidth:         intLikeParam(q, "card_width"),
		HideRank:          boolParam(q, "hide_rank", false),
		IncludeAllCommits: includeAllCommits,
		CommitsYear:       commitsYear,
		LineHeight:        lineHeightParam(q, 25),
		TextBold:          boolParam(q, "text_bold", true),
		CustomTitle:       q.Get("custom_title"),
		BorderRadius:      floatParam(q, "border_radius"),
		NumberFormat:      valueOr(q, "number_format", "short"),
		NumberPrecision:   intParam(q, "number_precision"),
		DisableAnimations: boolParam(q, "disable_animations", false),
		RankIcon:          valueOr(q, "rank_icon", "default"),
	}

	key := "/api?" + r.URL.RawQuery
	svg := s.cache.GetOrCompute(key, func() ([]byte, time.Duration) {
		stats, err := s.client.FetchStats(r.Context(), params)
		if err != nil {
			return buildErrorCard(err, colorOptions), errorTTLFor(s.statsTTL)
		}
		rendered, err := cards.RenderStatsCard(stats, options)
		if err != nil {
			return buildErrorCard(err, colorOptions), errorTTLFor(s.statsTTL)
		}
		return []byte(rendered), s.statsTTL
	})
	writeSVG(w, string(svg), s.statsTTL)
}

func (s *Server) handlePin(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	colorOptions := colorInputFromQuery(q)

	if invalid := findInvalidColorParam(q); invalid != "" {
		writeSVG(w, cards.RenderErrorCard(
			"Something went wrong",
			`Invalid color input for parameter "`+invalid+`"`,
			colorOptions,
			true,
		), errorTTLFor(s.statsTTL))
		return
	}

	username := q.Get("username")
	reponame := q.Get("repo")
	if (username != "" && !safePatternRe.MatchString(username)) ||
		(reponame != "" && !safePatternRe.MatchString(reponame)) {
		writeSVG(w, cards.RenderErrorCard(
			"Something went wrong",
			"Username or repository contains unsafe characters",
			colorOptions,
			true,
		), errorTTLFor(s.statsTTL))
		return
	}

	options := cards.RepoCardOptions{
		Colors:                colorOptions,
		HideBorder:            boolParam(q, "hide_border", false),
		CardWidthInput:        intLikeParam(q, "card_width"),
		ShowOwner:             boolParam(q, "show_owner", false),
		ShowIcons:             boolPtrParam(q, "show_icons"),
		NumberFormat:          valueOr(q, "number_format", "short"),
		TextBold:              boolParam(q, "text_bold", false),
		LineHeight:            lineHeightParam(q, 22),
		BorderRadius:          floatParam(q, "border_radius"),
		DescriptionLinesCount: intParam(q, "description_lines_count"),
		DisableAnimations:     boolPtrParam(q, "disable_animations"),
	}

	key := "/api/pin?" + r.URL.RawQuery
	svg := s.cache.GetOrCompute(key, func() ([]byte, time.Duration) {
		repo, err := s.client.FetchRepo(r.Context(), username, reponame)
		if err != nil {
			return buildErrorCard(err, colorOptions), errorTTLFor(s.statsTTL)
		}
		rendered, err := cards.RenderRepoCard(repo, options)
		if err != nil {
			return buildErrorCard(err, colorOptions), errorTTLFor(s.statsTTL)
		}
		return []byte(rendered), s.statsTTL
	})
	writeSVG(w, string(svg), s.statsTTL)
}

func (s *Server) handleStreak(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	options := streakOptionsFromQuery(q)

	params := github.StreakParams{
		User:         q.Get("user"),
		StartingYear: intValParam(q, "starting_year"),
		ExcludeDays:  parseListParam(q.Get("exclude_days")),
		Timezone:     q.Get("timezone"),
		Mode:         q.Get("mode"),
	}

	if q.Get("type") == "json" {
		payload := s.cache.GetOrCompute("/streak/json?"+r.URL.RawQuery, func() ([]byte, time.Duration) {
			stats, err := s.client.FetchStreak(r.Context(), params)
			if err != nil {
				body, _ := json.Marshal(map[string]any{"error": errorMessage(err), "code": 400})
				return body, errorTTLFor(s.streakTTL)
			}
			body, err := json.Marshal(stats)
			if err != nil {
				return []byte(`{"error":"Could not encode stats","code":500}`), errorTTLFor(s.streakTTL)
			}
			return body, s.streakTTL
		})
		w.Header().Set("Content-Type", "application/json")
		if s.streakTTL > 0 {
			w.Header().Set("Cache-Control", "public, max-age="+strconv.Itoa(int(s.streakTTL.Seconds())))
			w.Header().Set("Expires", time.Now().Add(s.streakTTL).UTC().Format(http.TimeFormat))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
		return
	}

	if !q.Has("user") {
		writeSVG(w, cards.RenderStreakErrorCard("GitHub username is required.", options), errorTTLFor(s.streakTTL))
		return
	}

	key := "/streak?" + r.URL.RawQuery
	svg := s.cache.GetOrCompute(key, func() ([]byte, time.Duration) {
		stats, err := s.client.FetchStreak(r.Context(), params)
		if err != nil {
			return []byte(cards.RenderStreakErrorCard(errorMessage(err), options)), errorTTLFor(s.streakTTL)
		}
		return []byte(cards.RenderStreakCard(stats, options)), s.streakTTL
	})
	writeSVG(w, string(svg), s.streakTTL)
}

var streakColorProps = []string{
	"background", "border", "stroke", "ring", "fire",
	"currStreakNum", "sideNums", "currStreakLabel", "sideLabels",
	"dates", "excludeDaysLabel",
}

func streakColorParams(q url.Values) map[string]string {
	out := map[string]string{}
	for _, prop := range streakColorProps {
		if v, ok := stringParam(q, prop); ok {
			out[prop] = v
		}
	}
	return out
}

func streakOptionsFromQuery(q url.Values) cards.StreakCardOptions {
	return cards.StreakCardOptions{
		Theme:                  q.Get("theme"),
		ColorParams:            streakColorParams(q),
		HideBorder:             q.Get("hide_border") == "true",
		BorderRadius:           floatValParam(q, "border_radius", 4.5),
		CardWidth:              intValParam(q, "card_width"),
		CardHeight:             intValParam(q, "card_height"),
		HideTotalContributions: q.Get("hide_total_contributions") == "true",
		HideCurrentStreak:      q.Get("hide_current_streak") == "true",
		HideLongestStreak:      q.Get("hide_longest_streak") == "true",
		ShortNumbers:           q.Get("short_numbers") == "true",
		DisableAnimations:      q.Get("disable_animations") == "true",
	}
}
