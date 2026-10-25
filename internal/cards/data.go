// Package cards renders README card SVGs.
//
// The renderers are ports of github-readme-stats (github-stats-extended fork)
// and github-readme-streak-stats, both MIT licensed. See LICENSE and README.md.
package cards

import (
	_ "embed"
	"encoding/json"
)

//go:embed data/themes_ghstats.json
var themesGhstatsJSON []byte

//go:embed data/themes_streak.json
var themesStreakJSON []byte

//go:embed data/segoe_widths.json
var segoeWidthsJSON []byte

//go:embed data/streak_css_colors.json
var streakCSSColorsJSON []byte

var (
	// themesGhstats maps theme name -> color key -> bare hex value
	// (github-readme-stats themes, e.g. "radical", "default_repocard").
	themesGhstats map[string]map[string]string

	// themesStreak maps theme name -> property -> CSS color
	// (github-readme-streak-stats themes, e.g. "radical").
	themesStreak map[string]map[string]string

	segoeWidths     []float64
	segoeAvg        float64
	streakCSSColors map[string]bool
)

func init() {
	mustUnmarshal(themesGhstatsJSON, &themesGhstats)
	mustUnmarshal(themesStreakJSON, &themesStreak)

	var w struct {
		Widths []float64 `json:"widths"`
		Avg    float64   `json:"avg"`
	}
	mustUnmarshal(segoeWidthsJSON, &w)
	segoeWidths = w.Widths
	segoeAvg = w.Avg

	var colors []string
	mustUnmarshal(streakCSSColorsJSON, &colors)
	streakCSSColors = make(map[string]bool, len(colors))
	for _, c := range colors {
		streakCSSColors[c] = true
	}
}

func mustUnmarshal(data []byte, v any) {
	if err := json.Unmarshal(data, v); err != nil {
		panic("cards: invalid embedded data: " + err.Error())
	}
}

// IsThemeName reports whether the name is a known github-readme-stats theme.
func IsThemeName(name string) bool {
	_, ok := themesGhstats[name]
	return ok
}
