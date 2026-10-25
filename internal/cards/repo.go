package cards

import (
	"math"
	"strings"
)

const (
	repoCardDefaultWidth    = 400
	repoXOffset             = 25
	repoDescriptionFontSize = 13
	repoIconSize            = 16
	repoDescriptionMaxLines = 3
)

// PrimaryLanguage is a repository's primary language.
type PrimaryLanguage struct {
	Name  *string
	Color *string
}

// RepositoryData is the data a repo pin card renders.
type RepositoryData struct {
	Name            string
	NameWithOwner   string
	Description     *string
	PrimaryLanguage *PrimaryLanguage
	IsArchived      bool
	IsTemplate      bool
	StargazerCount  float64
	ForkCount       float64
}

// RepoCardOptions are the options of a repo pin card.
type RepoCardOptions struct {
	Colors                ColorInput
	HideBorder            bool
	CardWidthInput        *float64
	ShowOwner             bool
	ShowIcons             *bool
	NumberFormat          string
	TextBold              bool
	LineHeight            float64
	BorderRadius          *float64
	DescriptionLinesCount *int
	DisableAnimations     *bool
}

func getBadgeSVG(label string, xOffset float64) string {
	return `
    <g data-testid="badge" class="badge" transform="translate(` + fmtNum(320+xOffset) + `, -18)">
      <rect stroke-width="1" width="70" height="20" x="-12" y="-14" ry="10" rx="10"></rect>
      <text
        x="23" y="-5"
        alignment-baseline="central"
        dominant-baseline="central"
        text-anchor="middle"
      >
        ` + EncodeHTML(label) + `
      </text>
    </g>
  `
}

func iconWithLabel(icon string, count float64, testid string, iconSize float64) string {
	if count <= 0 {
		return ""
	}
	iconSvg := `
      <svg
        class="icon"
        y="-12"
        viewBox="0 0 16 16"
        version="1.1"
        width="` + fmtNum(iconSize) + `"
        height="` + fmtNum(iconSize) + `"
      >
        ` + icon + `
      </svg>
    `
	text := `<text data-testid="` + testid + `" class="gray">` + EncodeHTML(KFormatter(count, nil)) + `</text>`
	return strings.Join(FlexLayout([]string{iconSvg, text}, 20, "", nil), "")
}

func repoCardStyles(colors CardColors) string {
	return `
    .description {
      font: 400 13px 'Segoe UI', Ubuntu, Sans-Serif;fill: ` + colors.TextColor + `;
      
    }
    .gray { font: 400 12px 'Segoe UI', Ubuntu, Sans-Serif; fill: ` + colors.TextColor + ` }
    .badge { font: 600 11px 'Segoe UI', Ubuntu, Sans-Serif; }
    .badge rect { opacity: 0.2; stroke: ` + colors.TextColor + ` }
    .badge text { fill: ` + colors.TextColor + ` }

    .stat { font: 400 12px 'Segoe UI', Ubuntu, Sans-Serif; fill: ` + colors.TextColor + ` }
    .stagger {
      opacity: 0;
      animation: fadeInAnimation 0.3s ease-in-out forwards;
    }
    .not_bold { font-weight: 400 }
    .bold { font-weight: 700 }
    .icon {
      fill: ` + colors.IconColor + `;
      display: block;
    }
  `
}

// RenderRepoCard renders the repository pin card.
func RenderRepoCard(repo RepositoryData, opts RepoCardOptions) (string, error) {
	cardWidth := float64(repoCardDefaultWidth)
	if opts.CardWidthInput != nil && *opts.CardWidthInput != 0 && !math.IsNaN(*opts.CardWidthInput) {
		cardWidth = *opts.CardWidthInput
	}

	showIcons := true
	if opts.ShowIcons != nil {
		showIcons = *opts.ShowIcons
	}

	header := repo.Name
	if opts.ShowOwner {
		header = repo.NameWithOwner
	}
	if headerRunes := []rune(header); len(headerRunes) > 35 {
		header = string(headerRunes[:35]) + "..."
	}

	langName := "Unspecified"
	langColor := "#333"
	if repo.PrimaryLanguage != nil {
		if repo.PrimaryLanguage.Name != nil && *repo.PrimaryLanguage.Name != "" {
			langName = *repo.PrimaryLanguage.Name
		}
		if repo.PrimaryLanguage.Color != nil && IsPrefixedHexColor(*repo.PrimaryLanguage.Color) {
			langColor = *repo.PrimaryLanguage.Color
		}
	}

	desc := "No description provided"
	if repo.Description != nil && *repo.Description != "" {
		desc = *repo.Description
	}

	descriptionMaxLines := repoDescriptionMaxLines
	if opts.DescriptionLinesCount != nil && *opts.DescriptionLinesCount != 0 {
		v := *opts.DescriptionLinesCount
		if v < 1 {
			v = 1
		}
		if v > repoDescriptionMaxLines {
			v = repoDescriptionMaxLines
		}
		descriptionMaxLines = v
	}

	descriptionBoxWidth := cardWidth - 2*float64(repoXOffset)
	multiLineDescription := WrapTextMultiline(desc, descriptionBoxWidth, repoDescriptionFontSize, descriptionMaxLines)

	descriptionLinesCount := len(multiLineDescription)
	if opts.DescriptionLinesCount != nil && *opts.DescriptionLinesCount != 0 {
		descriptionLinesCount = descriptionMaxLines
	}

	var tspans strings.Builder
	for _, line := range multiLineDescription {
		tspans.WriteString(`<tspan dy="1.2em" x="` + fmtNum(repoXOffset) + `">` + line + `</tspan>`)
	}
	descriptionSvg := `<text class="description" x="` + fmtNum(repoXOffset) + `" y="-5"> 
      ` + tspans.String() + `
    </text>`

	height := 110.0
	if descriptionLinesCount > 1 {
		height = 120
	}
	height += float64(descriptionLinesCount) * 10

	colorsIn := opts.Colors
	if colorsIn.Theme == nil {
		theme := "default_repocard"
		colorsIn.Theme = &theme
	}
	colors, err := GetCardColors(colorsIn)
	if err != nil {
		return "", &RenderError{Message: err.Error()}
	}

	svgLanguage := ""
	if repo.PrimaryLanguage != nil {
		svgLanguage = `
    <g data-testid="primary-lang">
      <circle data-testid="lang-color" cx="0" cy="-5" r="6" fill="` + langColor + `" />
      <text data-testid="lang-name" class="gray" x="15">` + EncodeHTML(langName) + `</text>
    </g>
    `
	}

	totalStars := KFormatter(repo.StargazerCount, nil)
	totalForks := KFormatter(repo.ForkCount, nil)
	svgStars := iconWithLabel(iconStar, repo.StargazerCount, "stargazers", repoIconSize)
	svgForks := iconWithLabel(iconFork, repo.ForkCount, "forkcount", repoIconSize)
	_ = showIcons

	starAndForkCount := strings.Join(FlexLayout(
		[]string{svgLanguage, svgStars, svgForks},
		25,
		"",
		[]float64{
			MeasureText(langName, 12),
			repoIconSize + MeasureText(totalStars, 12),
			repoIconSize + MeasureText(totalForks, 12),
		},
	), "")

	const extraHeight = 0.0
	extraItems := `
  <svg x="0" y="0"><g transform="translate(-3, ` + fmtNum(height-52-extraHeight) + `)">
      
    </g></svg>
    `

	card := NewCard(cardWidth, height, colors, header, iconContribs)
	card.SetBorderRadius(opts.BorderRadius)
	card.HideBorder = opts.HideBorder
	if opts.DisableAnimations == nil || *opts.DisableAnimations {
		card.DisableAnimations()
	}
	card.CSS = repoCardStyles(colors)

	badge := ""
	if repo.IsTemplate {
		badge = getBadgeSVG("Template", cardWidth-repoCardDefaultWidth)
	} else if repo.IsArchived {
		badge = getBadgeSVG("Archived", cardWidth-repoCardDefaultWidth)
	}

	body := `
    ` + badge + `

    ` + descriptionSvg + `

    <g transform="translate(30, ` + fmtNum(height-75-extraHeight) + `)">
      ` + starAndForkCount + `
    </g>
    ` + extraItems

	return card.Render(body)
}
