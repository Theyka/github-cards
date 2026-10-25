package cards

import (
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// StreakLength is one streak range.
type StreakLength struct {
	Start  string `json:"start"`
	End    string `json:"end"`
	Length int    `json:"length"`
}

// StreakStats is the data a streak card renders.
type StreakStats struct {
	Mode               string       `json:"mode"`
	TotalContributions float64      `json:"totalContributions"`
	FirstContribution  string       `json:"firstContribution"`
	CurrentStreak      StreakLength `json:"currentStreak"`
	LongestStreak      StreakLength `json:"longestStreak"`
	ExcludedDays       []string     `json:"excludedDays,omitempty"`
}

// MarshalJSON matches upstream's `type=json` output: longestStreak before
// currentStreak, and excludedDays present (as [] when empty) in daily mode only.
func (s StreakStats) MarshalJSON() ([]byte, error) {
	type streakStatsJSON struct {
		Mode               string       `json:"mode"`
		TotalContributions float64      `json:"totalContributions"`
		FirstContribution  string       `json:"firstContribution"`
		LongestStreak      StreakLength `json:"longestStreak"`
		CurrentStreak      StreakLength `json:"currentStreak"`
		ExcludedDays       *[]string    `json:"excludedDays,omitempty"`
	}
	out := streakStatsJSON{
		Mode:               s.Mode,
		TotalContributions: s.TotalContributions,
		FirstContribution:  s.FirstContribution,
		LongestStreak:      s.LongestStreak,
		CurrentStreak:      s.CurrentStreak,
	}
	if s.Mode == "daily" {
		days := s.ExcludedDays
		if days == nil {
			days = []string{}
		}
		out.ExcludedDays = &days
	}
	return json.Marshal(out)
}

// StreakCardOptions are the options of a streak card.
type StreakCardOptions struct {
	Theme                  string
	ColorParams            map[string]string
	HideBorder             bool
	BorderRadius           float64
	CardWidth              *int
	CardHeight             *int
	HideTotalContributions bool
	HideCurrentStreak      bool
	HideLongestStreak      bool
	ShortNumbers           bool
	DisableAnimations      bool
}

var (
	streakHexColorRe   = regexp.MustCompile(`^([a-f0-9]{3}|[a-f0-9]{4}|[a-f0-9]{6}|[a-f0-9]{8})$`)
	streakGradientRe   = regexp.MustCompile(`^-?[0-9]+,[a-f0-9]{3,8}(,[a-f0-9]{3,8})+$`)
	transparentAttrRe  = regexp.MustCompile(`(fill|stroke)=['"]transparent['"]`)
	shortHexAttrRe     = regexp.MustCompile(`(fill|stroke|stop-color)=['"]#([0-9a-fA-F]{4}|[0-9a-fA-F]{8})['"]`)
	animationStyleRe   = regexp.MustCompile(`(?s)<style>.*?</style>`)
	animationOpacityRe = regexp.MustCompile(`opacity: 0;`)
	animationFadeinRe  = regexp.MustCompile(`animation: fadein[^;'"]+`)
	animationStreakRe  = regexp.MustCompile(`animation: currstreak[^;'"]+`)
	animationLinkRe    = regexp.MustCompile(`(?s)<a .*?>(.*?)</a>`)
)

func normalizeThemeName(theme string) string {
	return strings.ReplaceAll(strings.ToLower(theme), "_", "-")
}

// getRequestedStreakTheme mirrors github-readme-streak-stats' getRequestedTheme.
func getRequestedStreakTheme(themeName string, colorParams map[string]string, hideBorder bool) map[string]string {
	selected := normalizeThemeName(themeName)
	if selected == "" {
		selected = "default"
	}
	base, ok := themesStreak[selected]
	if !ok {
		base = themesStreak["default"]
	}

	theme := make(map[string]string, len(base)+1)
	for k, v := range base {
		theme[k] = v
	}

	for prop := range theme {
		param, present := colorParams[prop]
		if !present {
			continue
		}
		param = strings.ToLower(param)
		switch {
		case streakHexColorRe.MatchString(param):
			theme[prop] = "#" + param
		case streakCSSColors[param]:
			theme[prop] = param
		case prop == "background" && streakGradientRe.MatchString(param):
			theme[prop] = param
		}
	}

	if hideBorder {
		theme["border"] = "#0000"
	}

	backgroundGradient := ""
	parts := strings.Split(theme["background"], ",")
	if len(parts) >= 3 {
		theme["background"] = "url(#gradient)"
		var b strings.Builder
		b.WriteString("<linearGradient id='gradient' gradientTransform='rotate(" + parts[0] + ")' gradientUnits='userSpaceOnUse'>")
		colors := parts[1:]
		for i, c := range colors {
			offset := float64(i) * 100 / float64(len(colors)-1)
			b.WriteString("<stop offset='" + fmtNum(offset) + "%' stop-color='#" + c + "' />")
		}
		b.WriteString("</linearGradient>")
		backgroundGradient = b.String()
	}
	theme["backgroundGradient"] = backgroundGradient

	return theme
}

// escapeXML mirrors htmlspecialchars with ENT_QUOTES | ENT_XML1.
func escapeXML(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#039;",
	)
	return r.Replace(s)
}

func utf8WordWrap(s string, width int, breakStr string, cutLongWords bool) string {
	words := strings.Fields(s)
	var lines []string
	cur := ""
	for _, word := range words {
		runes := []rune(word)
		if cutLongWords {
			for len(runes) > width {
				if cur != "" {
					lines = append(lines, cur)
					cur = ""
				}
				lines = append(lines, string(runes[:width]))
				runes = runes[width:]
			}
		}
		word = string(runes)
		candidate := word
		if cur != "" {
			candidate = cur + " " + word
		}
		if utf8.RuneCountInString(candidate) <= width {
			cur = candidate
		} else {
			if cur != "" {
				lines = append(lines, cur)
			}
			cur = word
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return strings.Join(lines, breakStr)
}

// splitLines mirrors github-readme-streak-stats' splitLines.
func splitLines(text string, maxChars int, line1Offset float64) string {
	if maxChars > 0 && utf8.RuneCountInString(text) > maxChars && !strings.Contains(text, "\n") {
		if strings.Contains(text, " - ") {
			text = strings.ReplaceAll(text, " - ", "\n- ")
		} else {
			text = utf8WordWrap(text, maxChars, "\n", true)
		}
	}
	text = escapeXML(text)
	if idx := strings.Index(text, "\n"); idx >= 0 {
		line1 := text[:idx]
		line2 := text[idx+1:]
		return "<tspan x='0' dy='" + fmtNum(line1Offset) + "'>" + line1 + "</tspan><tspan x='0' dy='16'>" + line2 + "</tspan>"
	}
	return text
}

func formatStreakDate(dateStr string, currentYear int) string {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return dateStr
	}
	if t.Year() == currentYear {
		return t.Format("Jan 2")
	}
	return t.Format("Jan 2, 2006")
}

func groupNumber(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		if neg {
			return "-" + s
		}
		return s
	}
	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	lead := len(s) % 3
	if lead == 0 {
		lead = 3
	}
	b.WriteString(s[:lead])
	for i := lead; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

func formatStreakNumber(num float64, short bool) string {
	if short {
		units := []string{"", "K", "M", "B", "T"}
		i := 0
		for num >= 1000 && i < len(units)-1 {
			num /= 1000
			i++
		}
		num = math.Round(num*10) / 10
		return fmtNum(num) + units[i]
	}
	return groupNumber(int64(math.Round(num)))
}

func convertHexColor(hex string) (string, string) {
	cleaned := regexp.MustCompile(`[^0-9a-fA-F]`).ReplaceAllString(hex, "")
	switch len(cleaned) {
	case 3:
		cleaned = string([]byte{cleaned[0], cleaned[0], cleaned[1], cleaned[1], cleaned[2], cleaned[2]})
	case 4:
		cleaned = string([]byte{cleaned[0], cleaned[0], cleaned[1], cleaned[1], cleaned[2], cleaned[2], cleaned[3], cleaned[3]})
	}
	if len(cleaned) == 6 {
		return "#" + cleaned, "1"
	}
	if len(cleaned) == 8 {
		alpha, _ := strconv.ParseInt(cleaned[6:8], 16, 64)
		return "#" + cleaned[:6], strconv.FormatFloat(float64(alpha)/255, 'f', -1, 64)
	}
	return "#" + cleaned, "1"
}

// convertHexColors mirrors github-readme-streak-stats' convertHexColors.
func convertHexColors(svg string) string {
	svg = transparentAttrRe.ReplaceAllString(svg, `$1="#0000"`)
	return shortHexAttrRe.ReplaceAllStringFunc(svg, func(match string) string {
		sub := shortHexAttrRe.FindStringSubmatch(match)
		attr, hex := sub[1], sub[2]
		opacityAttr := attr + "-opacity"
		if attr == "stop-color" {
			opacityAttr = "stop-opacity"
		}
		color, opacity := convertHexColor(hex)
		return attr + "='" + color + "' " + opacityAttr + "='" + opacity + "'"
	})
}

// RemoveAnimations mirrors github-readme-streak-stats' removeAnimations.
func RemoveAnimations(svg string) string {
	svg = animationStyleRe.ReplaceAllString(svg, "")
	svg = animationOpacityRe.ReplaceAllString(svg, "opacity: 1;")
	svg = animationFadeinRe.ReplaceAllString(svg, "opacity: 1;")
	svg = animationStreakRe.ReplaceAllString(svg, "font-size: 28px;")
	svg = animationLinkRe.ReplaceAllString(svg, "$1")
	return svg
}

// RenderStreakCard renders the streak card, mirroring
// github-readme-streak-stats' generateCard.
func RenderStreakCard(stats StreakStats, opts StreakCardOptions) string {
	theme := getRequestedStreakTheme(opts.Theme, opts.ColorParams, opts.HideBorder)

	borderRadius := opts.BorderRadius
	if borderRadius == 0 {
		borderRadius = 4.5
	}

	showTotal := !opts.HideTotalContributions
	showCurrent := !opts.HideCurrentStreak
	showLongest := !opts.HideLongestStreak
	numColumns := 0
	for _, show := range []bool{showTotal, showCurrent, showLongest} {
		if show {
			numColumns++
		}
	}

	cardWidth := 495
	if opts.CardWidth != nil {
		cardWidth = *opts.CardWidth
	}
	if minWidth := 100 * numColumns; cardWidth < minWidth {
		cardWidth = minWidth
	}
	rectWidth := cardWidth - 1
	columnWidth := 0.0
	if numColumns > 0 {
		columnWidth = float64(cardWidth) / float64(numColumns)
	}

	cardHeight := 195
	if opts.CardHeight != nil {
		cardHeight = *opts.CardHeight
	}
	if cardHeight < 170 {
		cardHeight = 170
	}
	rectHeight := cardHeight - 1
	heightOffset := float64(cardHeight-195) / 2

	barOffsets := [2]float64{-999, -999}
	for i := 0; i < numColumns-1; i++ {
		barOffsets[i] = columnWidth * float64(i+1)
	}

	columnOffsets := make([]float64, 0, numColumns)
	for i := 0; i < numColumns; i++ {
		columnOffsets = append(columnOffsets, columnWidth/2+columnWidth*float64(i))
	}

	next := 0
	totalOffset, currentOffset, longestOffset := -999.0, -999.0, -999.0
	if showTotal {
		totalOffset = columnOffsets[next]
		next++
	}
	if showCurrent {
		currentOffset = columnOffsets[next]
		next++
	}
	if showLongest {
		longestOffset = columnOffsets[next]
	}

	barHeightOffsets := [2]float64{28 + heightOffset/2, 170 + heightOffset}
	totalHeightOffsets := [3]float64{48 + heightOffset, 84 + heightOffset, 114 + heightOffset}
	currentHeightOffsets := [5]float64{48 + heightOffset, 108 + heightOffset, 145 + heightOffset, 71 + heightOffset, 19.5 + heightOffset}

	currentYear := time.Now().Year()

	totalContributions := formatStreakNumber(stats.TotalContributions, opts.ShortNumbers)
	firstContribution := formatStreakDate(stats.FirstContribution, currentYear)
	totalRange := firstContribution + " - Present"

	currentStreak := formatStreakNumber(float64(stats.CurrentStreak.Length), opts.ShortNumbers)
	currentStart := formatStreakDate(stats.CurrentStreak.Start, currentYear)
	currentEnd := formatStreakDate(stats.CurrentStreak.End, currentYear)
	currentRange := currentStart
	if currentStart != currentEnd {
		currentRange += " - " + currentEnd
	}

	longestStreak := formatStreakNumber(float64(stats.LongestStreak.Length), opts.ShortNumbers)
	longestStart := formatStreakDate(stats.LongestStreak.Start, currentYear)
	longestEnd := formatStreakDate(stats.LongestStreak.End, currentYear)
	longestRange := longestStart
	if longestStart != longestEnd {
		longestRange += " - " + longestEnd
	}

	maxCharsLabels := 0
	maxCharsDates := 0
	if numColumns > 0 {
		maxCharsLabels = int(math.Floor(float64(cardWidth) / float64(numColumns) / 7.5))
		maxCharsDates = int(math.Floor(float64(cardWidth) / float64(numColumns) / 6))
	}

	weekly := stats.Mode == "weekly"
	totalText := splitLines("Total Contributions", maxCharsLabels, -9)
	currentLabel, longestLabel := "Current Streak", "Longest Streak"
	if weekly {
		currentLabel, longestLabel = "Week Streak", "Longest Week Streak"
	}
	currentText := splitLines(currentLabel, maxCharsLabels, -9)
	longestText := splitLines(longestLabel, maxCharsLabels, -9)
	totalRange = splitLines(totalRange, maxCharsDates, 0)
	currentRange = splitLines(currentRange, maxCharsDates, 0)
	longestRange = splitLines(longestRange, maxCharsDates, 0)

	excludedDays := ""
	if len(stats.ExcludedDays) > 0 {
		daysText := escapeXML("Excluding " + strings.Join(stats.ExcludedDays, ", "))
		excludedDays = `<g style='isolation: isolate'>
                <!-- Excluded Days -->
                <g transform='translate(5,187)'>
                    <text stroke-width='0' text-anchor='right' fill='` + theme["excludeDaysLabel"] + `' stroke='none' font-family='"Segoe UI", Ubuntu, sans-serif' font-weight='400' font-size='10px' font-style='normal' style='opacity: 0; animation: fadein 0.5s linear forwards 0.9s'>
                        * ` + daysText + `
                    </text>
                </g>
            </g>`
	}

	var b strings.Builder
	b.WriteString("<svg xmlns='http://www.w3.org/2000/svg' xmlns:xlink='http://www.w3.org/1999/xlink'\n                style='isolation: isolate' viewBox='0 0 " + strconv.Itoa(cardWidth) + " " + strconv.Itoa(cardHeight) + "' width='" + strconv.Itoa(cardWidth) + "px' height='" + strconv.Itoa(cardHeight) + "px' direction='ltr'>\n        <style>\n            @keyframes currstreak {\n                0% { font-size: 3px; opacity: 0.2; }\n                80% { font-size: 34px; opacity: 1; }\n                100% { font-size: 28px; opacity: 1; }\n            }\n            @keyframes fadein {\n                0% { opacity: 0; }\n                100% { opacity: 1; }\n            }\n        </style>\n        <defs>\n            <clipPath id='outer_rectangle'>\n                <rect width='" + strconv.Itoa(cardWidth) + "' height='" + strconv.Itoa(cardHeight) + "' rx='" + fmtNum(borderRadius) + "'/>\n            </clipPath>\n            <mask id='mask_out_ring_behind_fire'>\n                <rect width='" + strconv.Itoa(cardWidth) + "' height='" + strconv.Itoa(cardHeight) + "' fill='white'/>\n                <ellipse id='mask-ellipse' cx='" + fmtNum(currentOffset) + "' cy='32' rx='13' ry='18' fill='black'/>\n            </mask>\n            " + theme["backgroundGradient"] + "\n        </defs>\n        <g clip-path='url(#outer_rectangle)'>\n            <g style='isolation: isolate'>\n                <rect stroke='" + theme["border"] + "' fill='" + theme["background"] + "' rx='" + fmtNum(borderRadius) + "' x='0.5' y='0.5' width='" + strconv.Itoa(rectWidth) + "' height='" + strconv.Itoa(rectHeight) + "'/>\n            </g>\n            <g style='isolation: isolate'>\n                <line x1='" + fmtNum(barOffsets[0]) + "' y1='" + fmtNum(barHeightOffsets[0]) + "' x2='" + fmtNum(barOffsets[0]) + "' y2='" + fmtNum(barHeightOffsets[1]) + "' vector-effect='non-scaling-stroke' stroke-width='1' stroke='" + theme["stroke"] + "' stroke-linejoin='miter' stroke-linecap='square' stroke-miterlimit='3'/>\n                <line x1='" + fmtNum(barOffsets[1]) + "' y1='" + fmtNum(barHeightOffsets[0]) + "' x2='" + fmtNum(barOffsets[1]) + "' y2='" + fmtNum(barHeightOffsets[1]) + "' vector-effect='non-scaling-stroke' stroke-width='1' stroke='" + theme["stroke"] + "' stroke-linejoin='miter' stroke-linecap='square' stroke-miterlimit='3'/>\n            </g>\n            <g style='isolation: isolate'>\n                <!-- Total Contributions big number -->\n                <g transform='translate(" + fmtNum(totalOffset) + ", " + fmtNum(totalHeightOffsets[0]) + ")'>\n                    <text x='0' y='32' stroke-width='0' text-anchor='middle' fill='" + theme["sideNums"] + "' stroke='none' font-family='\"Segoe UI\", Ubuntu, sans-serif' font-weight='700' font-size='28px' font-style='normal' style='opacity: 0; animation: fadein 0.5s linear forwards 0.6s'>\n                        " + totalContributions + "\n                    </text>\n                </g>\n\n                <!-- Total Contributions label -->\n                <g transform='translate(" + fmtNum(totalOffset) + ", " + fmtNum(totalHeightOffsets[1]) + ")'>\n                    <text x='0' y='32' stroke-width='0' text-anchor='middle' fill='" + theme["sideLabels"] + "' stroke='none' font-family='\"Segoe UI\", Ubuntu, sans-serif' font-weight='400' font-size='14px' font-style='normal' style='opacity: 0; animation: fadein 0.5s linear forwards 0.7s'>\n                        " + totalText + "\n                    </text>\n                </g>\n\n                <!-- Total Contributions range -->\n                <g transform='translate(" + fmtNum(totalOffset) + ", " + fmtNum(totalHeightOffsets[2]) + ")'>\n                    <text x='0' y='32' stroke-width='0' text-anchor='middle' fill='" + theme["dates"] + "' stroke='none' font-family='\"Segoe UI\", Ubuntu, sans-serif' font-weight='400' font-size='12px' font-style='normal' style='opacity: 0; animation: fadein 0.5s linear forwards 0.8s'>\n                        " + totalRange + "\n                    </text>\n                </g>\n            </g>\n            <g style='isolation: isolate'>\n                <!-- Current Streak label -->\n                <g transform='translate(" + fmtNum(currentOffset) + ", " + fmtNum(currentHeightOffsets[1]) + ")'>\n                    <text x='0' y='32' stroke-width='0' text-anchor='middle' fill='" + theme["currStreakLabel"] + "' stroke='none' font-family='\"Segoe UI\", Ubuntu, sans-serif' font-weight='700' font-size='14px' font-style='normal' style='opacity: 0; animation: fadein 0.5s linear forwards 0.9s'>\n                        " + currentText + "\n                    </text>\n                </g>\n\n                <!-- Current Streak range -->\n                <g transform='translate(" + fmtNum(currentOffset) + ", " + fmtNum(currentHeightOffsets[2]) + ")'>\n                    <text x='0' y='21' stroke-width='0' text-anchor='middle' fill='" + theme["dates"] + "' stroke='none' font-family='\"Segoe UI\", Ubuntu, sans-serif' font-weight='400' font-size='12px' font-style='normal' style='opacity: 0; animation: fadein 0.5s linear forwards 0.9s'>\n                        " + currentRange + "\n                    </text>\n                </g>\n\n                <!-- Ring around number -->\n                <g mask='url(#mask_out_ring_behind_fire)'>\n                    <circle cx='" + fmtNum(currentOffset) + "' cy='" + fmtNum(currentHeightOffsets[3]) + "' r='40' fill='none' stroke='" + theme["ring"] + "' stroke-width='5' style='opacity: 0; animation: fadein 0.5s linear forwards 0.4s'></circle>\n                </g>\n                <!-- Fire icon -->\n                <g transform='translate(" + fmtNum(currentOffset) + ", " + fmtNum(currentHeightOffsets[4]) + ")' stroke-opacity='0' style='opacity: 0; animation: fadein 0.5s linear forwards 0.6s'>\n                    <path d='M -12 -0.5 L 15 -0.5 L 15 23.5 L -12 23.5 L -12 -0.5 Z' fill='none'/>\n                    <path d='M 1.5 0.67 C 1.5 0.67 2.24 3.32 2.24 5.47 C 2.24 7.53 0.89 9.2 -1.17 9.2 C -3.23 9.2 -4.79 7.53 -4.79 5.47 L -4.76 5.11 C -6.78 7.51 -8 10.62 -8 13.99 C -8 18.41 -4.42 22 0 22 C 4.42 22 8 18.41 8 13.99 C 8 8.6 5.41 3.79 1.5 0.67 Z M -0.29 19 C -2.07 19 -3.51 17.6 -3.51 15.86 C -3.51 14.24 -2.46 13.1 -0.7 12.74 C 1.07 12.38 2.9 11.53 3.92 10.16 C 4.31 11.45 4.51 12.81 4.51 14.2 C 4.51 16.85 2.36 19 -0.29 19 Z' fill='" + theme["fire"] + "' stroke-opacity='0'/>\n                </g>\n\n                <!-- Current Streak big number -->\n                <g transform='translate(" + fmtNum(currentOffset) + ", " + fmtNum(currentHeightOffsets[0]) + ")'>\n                    <text x='0' y='32' stroke-width='0' text-anchor='middle' fill='" + theme["currStreakNum"] + "' stroke='none' font-family='\"Segoe UI\", Ubuntu, sans-serif' font-weight='700' font-size='28px' font-style='normal' style='animation: currstreak 0.6s linear forwards'>\n                        " + currentStreak + "\n                    </text>\n                </g>\n\n            </g>\n            <g style='isolation: isolate'>\n                <!-- Longest Streak big number -->\n                <g transform='translate(" + fmtNum(longestOffset) + ", " + fmtNum(totalHeightOffsets[0]) + ")'>\n                    <text x='0' y='32' stroke-width='0' text-anchor='middle' fill='" + theme["sideNums"] + "' stroke='none' font-family='\"Segoe UI\", Ubuntu, sans-serif' font-weight='700' font-size='28px' font-style='normal' style='opacity: 0; animation: fadein 0.5s linear forwards 1.2s'>\n                        " + longestStreak + "\n                    </text>\n                </g>\n\n                <!-- Longest Streak label -->\n                <g transform='translate(" + fmtNum(longestOffset) + ", " + fmtNum(totalHeightOffsets[1]) + ")'>\n                    <text x='0' y='32' stroke-width='0' text-anchor='middle' fill='" + theme["sideLabels"] + "' stroke='none' font-family='\"Segoe UI\", Ubuntu, sans-serif' font-weight='400' font-size='14px' font-style='normal' style='opacity: 0; animation: fadein 0.5s linear forwards 1.3s'>\n                        " + longestText + "\n                    </text>\n                </g>\n\n                <!-- Longest Streak range -->\n                <g transform='translate(" + fmtNum(longestOffset) + ", " + fmtNum(totalHeightOffsets[2]) + ")'>\n                    <text x='0' y='32' stroke-width='0' text-anchor='middle' fill='" + theme["dates"] + "' stroke='none' font-family='\"Segoe UI\", Ubuntu, sans-serif' font-weight='400' font-size='12px' font-style='normal' style='opacity: 0; animation: fadein 0.5s linear forwards 1.4s'>\n                        " + longestRange + "\n                    </text>\n                </g>\n            </g>\n            " + excludedDays + "\n        </g>\n    </svg>\n")

	svg := convertHexColors(b.String())
	if opts.DisableAnimations {
		svg = RemoveAnimations(svg)
	}
	return svg
}

// RenderStreakErrorCard renders the streak error card, mirroring
// github-readme-streak-stats' generateErrorCard.
func RenderStreakErrorCard(message string, opts StreakCardOptions) string {
	theme := getRequestedStreakTheme(opts.Theme, opts.ColorParams, opts.HideBorder)

	borderRadius := opts.BorderRadius
	if borderRadius == 0 {
		borderRadius = 4.5
	}

	cardWidth := 495
	if opts.CardWidth != nil {
		cardWidth = *opts.CardWidth
	}
	if cardWidth < 300 {
		cardWidth = 300
	}
	rectWidth := cardWidth - 1
	centerOffset := float64(cardWidth) / 2

	cardHeight := 195
	if opts.CardHeight != nil {
		cardHeight = *opts.CardHeight
	}
	if cardHeight < 170 {
		cardHeight = 170
	}
	rectHeight := cardHeight - 1
	heightOffset := float64(cardHeight-195) / 2
	errorLabelOffset := float64(cardHeight)/2 + 10.5

	escaped := escapeXML(message)

	var b strings.Builder
	b.WriteString("<svg xmlns='http://www.w3.org/2000/svg' xmlns:xlink='http://www.w3.org/1999/xlink' style='isolation: isolate' viewBox='0 0 " + strconv.Itoa(cardWidth) + " " + strconv.Itoa(cardHeight) + "' width='" + strconv.Itoa(cardWidth) + "px' height='" + strconv.Itoa(cardHeight) + "px'>\n        <style>\n            a {\n                fill: " + theme["dates"] + ";\n            }\n        </style>\n        <defs>\n            <clipPath id='outer_rectangle'>\n                <rect width='" + strconv.Itoa(cardWidth) + "' height='" + strconv.Itoa(cardHeight) + "' rx='" + fmtNum(borderRadius) + "'/>\n            </clipPath>\n            " + theme["backgroundGradient"] + "\n        </defs>\n        <g clip-path='url(#outer_rectangle)'>\n            <g style='isolation: isolate'>\n                <rect stroke='" + theme["border"] + "' fill='" + theme["background"] + "' rx='" + fmtNum(borderRadius) + "' x='0.5' y='0.5' width='" + strconv.Itoa(rectWidth) + "' height='" + strconv.Itoa(rectHeight) + "'/>\n            </g>\n            <g style='isolation: isolate'>\n                <!-- Error lable -->\n                <g transform='translate(" + fmtNum(centerOffset) + ", " + fmtNum(errorLabelOffset) + ")'>\n                    <text x='0' y='50' dy='0.25em' stroke-width='0' text-anchor='middle' fill='" + theme["sideLabels"] + "' stroke='none' font-family='\"Segoe UI\", Ubuntu, sans-serif' font-weight='400' font-size='14px' font-style='normal'>\n                        " + escaped + "\n                    </text>\n                </g>\n\n                <!-- Mask for background behind face -->\n                <defs>\n                    <mask id='cut-off-area'>\n                        <rect x='0' y='0' width='500' height='500' fill='white' />\n                        <ellipse cx='" + fmtNum(centerOffset) + "' cy='31' rx='13' ry='18'/>\n                    </mask>\n                </defs>\n                <!-- Sad face -->\n                <g transform='translate(" + fmtNum(centerOffset) + ", " + fmtNum(heightOffset) + ")'>\n                    <path fill='" + theme["fire"] + "' d='M0,35.8c-25.2,0-45.7,20.5-45.7,45.7s20.5,45.8,45.7,45.8s45.7-20.5,45.7-45.7S25.2,35.8,0,35.8z M0,122.3c-11.2,0-21.4-4.5-28.8-11.9c-2.9-2.9-5.4-6.3-7.4-10c-3-5.7-4.6-12.1-4.6-18.9c0-22.5,18.3-40.8,40.8-40.8 c10.7,0,20.4,4.1,27.7,10.9c3.8,3.5,6.9,7.7,9.1,12.4c2.6,5.3,4,11.3,4,17.6C40.8,104.1,22.5,122.3,0,122.3z'/>\n                    <path fill='" + theme["fire"] + "' d='M4.8,93.8c5.4,1.1,10.3,4.2,13.7,8.6l3.9-3c-4.1-5.3-10-9-16.6-10.4c-10.6-2.2-21.7,1.9-28.3,10.4l3.9,3 C-13.1,95.3-3.9,91.9,4.8,93.8z'/>\n                    <circle fill='" + theme["fire"] + "' cx='-15' cy='71' r='4.9'/>\n                    <circle fill='" + theme["fire"] + "' cx='15' cy='71' r='4.9'/>\n                </g>\n            </g>\n        </g>\n    </svg>\n")

	svg := convertHexColors(b.String())
	if opts.DisableAnimations {
		svg = RemoveAnimations(svg)
	}
	return svg
}
