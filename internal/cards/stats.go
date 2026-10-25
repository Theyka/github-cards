package cards

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Card size constants (github-stats-extended cards/stats.ts).
const (
	cardMinWidth             = 287
	cardDefaultWidth         = 287
	rankCardMinWidth         = 420
	rankCardDefaultWidth     = 450
	rankOnlyCardMinWidth     = 290
	rankOnlyCardDefaultWidth = 290
)

// Rank is a user's rank level and percentile.
type Rank struct {
	Level      string
	Percentile float64
}

// StatsData is the data a stats card renders.
type StatsData struct {
	Name          string
	TotalStars    float64
	TotalCommits  float64
	TotalIssues   float64
	TotalPRs      float64
	TotalReviews  float64
	ContributedTo float64
	Rank          Rank
}

// StatsCardOptions are the options of a stats card.
type StatsCardOptions struct {
	Colors            ColorInput
	Hide              []string
	ShowIcons         bool
	HideTitle         bool
	HideBorder        bool
	CardWidth         *float64 // nil: default; NaN: param sent but unparsable
	HideRank          bool
	IncludeAllCommits bool
	CommitsYear       *int
	LineHeight        float64
	TextBold          bool
	CustomTitle       string
	BorderRadius      *float64
	NumberFormat      string
	NumberPrecision   *int
	DisableAnimations bool
	RankIcon          string
}

var apostropheRe = regexp.MustCompile(`(?i)s$`)

func calculateCircleProgress(value float64) float64 {
	const radius = 40
	c := math.Pi * (radius * 2)
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	return ((100 - value) / 100) * c
}

func getProgressAnimation(progress float64) string {
	return `
    @keyframes rankAnimation {
      from {
        stroke-dashoffset: ` + fmtNum(calculateCircleProgress(0)) + `;
      }
      to {
        stroke-dashoffset: ` + fmtNum(calculateCircleProgress(progress)) + `;
      }
    }
  `
}

func statsCardStyles(colors CardColors, showIcons bool, progress float64) string {
	iconDisplay := "none"
	if showIcons {
		iconDisplay = "block"
	}
	return `
    .stat {
      font: 600 14px 'Segoe UI', Ubuntu, "Helvetica Neue", Sans-Serif; fill: ` + colors.TextColor + `;
    }
    @supports(-moz-appearance: auto) {
      /* Selector detects Firefox */
      .stat { font-size:12px; }
    }
    .stagger {
      opacity: 0;
      animation: fadeInAnimation 0.3s ease-in-out forwards;
    }
    .rank-text {
      font: 800 24px 'Segoe UI', Ubuntu, Sans-Serif; fill: ` + colors.TextColor + `;
      animation: scaleInAnimation 0.3s ease-in-out forwards;
    }
    .rank-percentile-header {
      font-size: 14px;
    }
    .rank-percentile-text {
      font-size: 16px;
    }
    
    .not_bold { font-weight: 400 }
    .bold { font-weight: 700 }
    .icon {
      fill: ` + colors.IconColor + `;
      display: ` + iconDisplay + `;
    }

    .rank-circle-rim {
      stroke: ` + colors.RingColor + `;
      fill: none;
      stroke-width: 6;
      opacity: 0.2;
    }
    .rank-circle {
      stroke: ` + colors.RingColor + `;
      stroke-dasharray: 250;
      fill: none;
      stroke-width: 6;
      stroke-linecap: round;
      opacity: 0.8;
      transform-origin: -10px 8px;
      transform: rotate(-90deg);
      animation: rankAnimation 1s forwards ease-in-out;
    }
    ` + getProgressAnimation(progress) + `
  `
}

// RenderStatsCard renders the stats card.
func RenderStatsCard(stats StatsData, opts StatsCardOptions) (string, error) {
	hide := make(map[string]bool, len(opts.Hide))
	for _, h := range opts.Hide {
		hide[h] = true
	}

	lineHeight := opts.LineHeight
	if lineHeight <= 0 || math.IsNaN(lineHeight) {
		lineHeight = 25
	}
	numberFormat := opts.NumberFormat
	if numberFormat == "" {
		numberFormat = "short"
	}
	rankIconMode := opts.RankIcon
	if rankIconMode == "" {
		rankIconMode = "default"
	}

	trimmedName := strings.TrimSpace(stats.Name)
	apostrophe := "s"
	if apostropheRe.MatchString(trimmedName) {
		apostrophe = ""
	}
	defaultTitle := trimmedName + "'" + apostrophe + " GitHub Stats"
	cardTitle := defaultTitle
	if opts.CustomTitle != "" {
		cardTitle = opts.CustomTitle
	}

	commitsYearLabel := ""
	if !opts.IncludeAllCommits {
		if opts.CommitsYear != nil {
			commitsYearLabel = " (" + strconv.Itoa(*opts.CommitsYear) + ")"
		} else {
			commitsYearLabel = " (last year)"
		}
	}
	commitsLabel := "Total Commits" + commitsYearLabel

	type statDef struct {
		icon  string
		label string
		id    string
		value float64
	}
	defs := []statDef{
		{iconStar, "Total Stars Earned", "stars", stats.TotalStars},
		{iconCommits, commitsLabel, "commits", stats.TotalCommits},
		{iconPRs, "Total PRs", "prs", stats.TotalPRs},
		{iconIssues, "Total Issues", "issues", stats.TotalIssues},
		{iconContribs, "Contributed to (last year)", "contribs", stats.ContributedTo},
	}

	longLabels := false
	for _, d := range defs {
		if !hide[d.id] && len([]rune(d.label)) > 18 {
			longLabels = true
		}
	}

	shiftValuePos := 29.01
	if longLabels {
		shiftValuePos += 50
	}

	var items []string
	index := 0
	for _, d := range defs {
		if hide[d.id] {
			continue
		}
		items = append(items, createTextNode(textNodeParams{
			Icon:            d.icon,
			Label:           d.label,
			Value:           d.value,
			ID:              d.id,
			Index:           index,
			ShowIcons:       opts.ShowIcons,
			ShiftValuePos:   shiftValuePos,
			Bold:            opts.TextBold,
			NumberFormat:    numberFormat,
			NumberPrecision: opts.NumberPrecision,
			LabelXOffset:    25,
		}))
		index++
	}

	if len(items) == 0 && opts.HideRank {
		return "", &RenderError{
			Message:   "Could not render stats card.",
			Secondary: "Either stats or rank are required.",
		}
	}

	minHeight := 0.0
	if !opts.HideRank {
		if len(items) > 0 {
			minHeight = 150
		} else {
			minHeight = 180
		}
	}
	height := math.Max(45+float64(len(items)+1)*lineHeight, minHeight)

	// The lower the user's percentile the better.
	progress := 100 - stats.Rank.Percentile

	iconWidth := 0.0
	if opts.ShowIcons && len(items) > 0 {
		iconWidth = 16 + 1
	}

	var minCardWidthValue, defaultCardWidthValue float64
	if opts.HideRank {
		titleForWidth := cardTitle
		if len(items) == 0 {
			titleForWidth = defaultTitle
		}
		minCardWidthValue = math.Max(cardMinWidth, 50+MeasureText(titleForWidth, 10)*2)
		defaultCardWidthValue = cardDefaultWidth
	} else if len(items) > 0 {
		minCardWidthValue = rankCardMinWidth
		defaultCardWidthValue = rankCardDefaultWidth
	} else {
		minCardWidthValue = rankOnlyCardMinWidth
		defaultCardWidthValue = rankOnlyCardDefaultWidth
	}
	minCardWidthValue += iconWidth
	defaultCardWidthValue += iconWidth

	width := math.Max(defaultCardWidthValue, minCardWidthValue)
	if opts.CardWidth != nil && *opts.CardWidth != 0 && !math.IsNaN(*opts.CardWidth) {
		width = *opts.CardWidth
	}

	// Rank-only cards use a shorter default title space (rankTitle == title here).
	colors, err := GetCardColors(opts.Colors)
	if err != nil {
		return "", &RenderError{Message: err.Error()}
	}

	card := NewCard(width, height, colors, cardTitle, "")
	card.SetBorderRadius(opts.BorderRadius)
	card.HideBorder = opts.HideBorder
	card.SetHideTitle(opts.HideTitle)
	card.CSS = statsCardStyles(colors, opts.ShowIcons, progress)
	if opts.DisableAnimations {
		card.DisableAnimations()
	}

	rankX := 0.0
	if len(items) > 0 {
		minXTranslation := rankCardMinWidth + iconWidth - 70
		if width > rankCardDefaultWidth {
			xMaxExpansion := minXTranslation + (rankCardDefaultWidth-minCardWidthValue)/2
			rankX = xMaxExpansion + width - rankCardDefaultWidth
		} else {
			rankX = minXTranslation + (width-minCardWidthValue)/2
		}
	} else {
		rankX = width/2 + 20 - 10
	}
	rankY := height/2 - 50

	rankCircle := ""
	if !opts.HideRank {
		rankCircle = `<g data-testid="rank-circle"
          transform="translate(` + fmtNum(rankX) + `, ` + fmtNum(rankY) + `)">
        <circle class="rank-circle-rim" cx="-10" cy="8" r="40" />
        <circle class="rank-circle" cx="-10" cy="8" r="40" />
        <g class="rank-text">
          ` + RankIcon(rankIconMode, stats.Rank.Level, stats.Rank.Percentile) + `
        </g>
      </g>`
	}

	// Accessibility labels.
	var labelParts []string
	for _, d := range defs {
		if hide[d.id] {
			continue
		}
		if d.id == "commits" {
			labelParts = append(labelParts, "Total Commits "+commitsYearLabel+" : "+fmtNum(d.value))
			continue
		}
		labelParts = append(labelParts, d.label+": "+fmtNum(d.value))
	}
	card.SetAccessibilityLabel(cardTitle+", Rank: "+stats.Rank.Level, strings.Join(labelParts, ", "))

	body := `
    ` + rankCircle + `
    <svg x="0" y="0">
      ` + strings.Join(FlexLayout(items, lineHeight, "column", nil), "") + `
    </svg>
  `

	return card.Render(body)
}
