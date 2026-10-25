package server

import (
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/Theyka/github-cards/internal/cards"
)

var (
	safePatternRe = regexp.MustCompile(`^[-\w/.,]+$`)
	yearRe        = regexp.MustCompile(`^\d{4}$`)
	parseFloatRe  = regexp.MustCompile(`^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?`)
)

func stringParam(q url.Values, key string) (string, bool) {
	if !q.Has(key) {
		return "", false
	}
	return q.Get(key), true
}

func valueOr(q url.Values, key, def string) string {
	if v, ok := stringParam(q, key); ok {
		return v
	}
	return def
}

// boolParam mirrors github-readme-stats' parseBoolean: exact \"true\"/\"false\"
// matches, otherwise the default.
func boolParam(q url.Values, key string, def bool) bool {
	v, ok := stringParam(q, key)
	if !ok {
		return def
	}
	switch v {
	case "true":
		return true
	case "false":
		return false
	default:
		return def
	}
}

// boolPtrParam returns nil for absent or unparsable values so callers can
// apply their own defaults.
func boolPtrParam(q url.Values, key string) *bool {
	v, ok := stringParam(q, key)
	if !ok {
		return nil
	}
	switch v {
	case "true":
		b := true
		return &b
	case "false":
		b := false
		return &b
	default:
		return nil
	}
}

// parseIntJS parses the leading integer of s like JavaScript's parseInt(s, 10).
func parseIntJS(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	i := 0
	sign := 1
	if s[i] == '+' || s[i] == '-' {
		if s[i] == '-' {
			sign = -1
		}
		i++
	}
	start := i
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == start {
		return 0, false
	}
	n, err := strconv.Atoi(s[start:i])
	if err != nil {
		return 0, false
	}
	return sign * n, true
}

// floatPrefix parses the leading float of s like JavaScript's parseFloat.
func floatPrefix(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	m := parseFloatRe.FindString(s)
	if m == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(m, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// intLikeParam returns nil when absent, NaN when present but unparsable
// (mirroring parseInt), otherwise the integer value.
func intLikeParam(q url.Values, key string) *float64 {
	v, ok := stringParam(q, key)
	if !ok {
		return nil
	}
	if n, ok2 := parseIntJS(v); ok2 {
		f := float64(n)
		return &f
	}
	nan := math.NaN()
	return &nan
}

// intParam returns nil when absent or unparsable.
func intParam(q url.Values, key string) *int {
	v, ok := stringParam(q, key)
	if !ok {
		return nil
	}
	n, ok2 := parseIntJS(v)
	if !ok2 {
		return nil
	}
	return &n
}

// intValParam mirrors PHP intval: prefix integer, unparsable becomes 0.
func intValParam(q url.Values, key string) *int {
	v, ok := stringParam(q, key)
	if !ok {
		return nil
	}
	n, ok2 := parseIntJS(v)
	if !ok2 {
		n = 0
	}
	return &n
}

// floatParam returns nil when absent and NaN when present but unparsable
// (mirroring parseFloat, which the stats/repo cards validate).
func floatParam(q url.Values, key string) *float64 {
	v, ok := stringParam(q, key)
	if !ok {
		return nil
	}
	f, ok2 := floatPrefix(v)
	if !ok2 {
		nan := math.NaN()
		return &nan
	}
	return &f
}

// floatValParam mirrors PHP floatval: prefix float, unparsable becomes 0.
func floatValParam(q url.Values, key string, def float64) float64 {
	v, ok := stringParam(q, key)
	if !ok {
		return def
	}
	f, ok2 := floatPrefix(v)
	if !ok2 {
		return 0
	}
	return f
}

// lineHeightParam parses an integer line height, falling back to def.
func lineHeightParam(q url.Values, def float64) float64 {
	v, ok := stringParam(q, "line_height")
	if !ok {
		return def
	}
	n, ok2 := parseIntJS(v)
	if !ok2 {
		return def
	}
	return float64(n)
}

// parseListParam splits a comma-separated list like upstream's parseArray.
func parseListParam(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, ",")
}

var grsColorParams = []string{
	"title_color", "icon_color", "text_color", "bg_color",
	"border_color", "ring_color", "prog_bar_bg_color",
}

func colorInputFromQuery(q url.Values) cards.ColorInput {
	in := cards.ColorInput{}
	if v, ok := stringParam(q, "title_color"); ok {
		in.TitleColor = &v
	}
	if v, ok := stringParam(q, "icon_color"); ok {
		in.IconColor = &v
	}
	if v, ok := stringParam(q, "text_color"); ok {
		in.TextColor = &v
	}
	if v, ok := stringParam(q, "bg_color"); ok {
		in.BGColor = &v
	}
	if v, ok := stringParam(q, "border_color"); ok {
		in.BorderColor = &v
	}
	if v, ok := stringParam(q, "ring_color"); ok {
		in.RingColor = &v
	}
	if v, ok := stringParam(q, "prog_bar_bg_color"); ok {
		in.ProgBarBgColor = &v
	}
	if v, ok := stringParam(q, "theme"); ok {
		in.Theme = &v
	}
	return in
}

// findInvalidColorParam returns the first color parameter with an invalid
// value, or "" when all are valid.
func findInvalidColorParam(q url.Values) string {
	for _, key := range grsColorParams {
		if v, ok := stringParam(q, key); ok && !cards.IsValidColorInput(v) {
			return key
		}
	}
	return ""
}
