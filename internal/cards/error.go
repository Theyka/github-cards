package cards

import (
	"strconv"
	"strings"
)

// Error card messages (github-stats-extended common/error.ts).
const (
	SecondaryTryAgainLater = "Please try again later"
	SecondaryMaxRetry      = "You can deploy own instance or wait until public will be no longer limited"
)

const errorCardLength = 576.5

// RenderError is a render failure with an optional secondary message,
// mirroring upstream's CustomError.
type RenderError struct {
	Message   string
	Secondary string
}

func (e *RenderError) Error() string { return e.Message }

// RenderErrorCard renders the "Something went wrong!" card, mirroring
// github-readme-stats' renderError.
func RenderErrorCard(message, secondaryMessage string, opts ColorInput, showRepoLink bool) string {
	colors, err := GetCardColors(opts)
	if err != nil {
		colors, _ = GetCardColors(ColorInput{})
	}

	title := "Something went wrong!"
	if secondaryMessage != SecondaryTryAgainLater && secondaryMessage != SecondaryMaxRetry && showRepoLink {
		title += " file an issue at https://tinyurl.com/github-stats"
	}

	bgFill := colors.BGColor.Fill("gradient")

	return `
    <svg width="` + fmtNum(errorCardLength) + `"  height="120" viewBox="0 0 ` + fmtNum(errorCardLength) + ` 120" fill="` + bgFill + `" xmlns="http://www.w3.org/2000/svg">
    <style>
    .text { font: 600 16px 'Segoe UI', Ubuntu, Sans-Serif; fill: ` + colors.TitleColor + ` }
    .small { font: 600 12px 'Segoe UI', Ubuntu, Sans-Serif; fill: ` + colors.TextColor + ` }
    .gray { fill: #858585 }
    </style>
    <rect x="0.5" y="0.5" width="` + fmtNum(errorCardLength-1) + `" height="99%" rx="4.5" fill="` + bgFill + `" stroke="` + colors.BorderColor + `"/>
    <text x="25" y="45" class="text">` + title + `</text>
    <text data-testid="message" x="25" y="55" class="text small">
      <tspan x="25" dy="18">` + EncodeHTML(message) + `</tspan>
      <tspan x="25" dy="18" class="gray">` + EncodeHTML(secondaryMessage) + `</tspan>
    </text>
    </svg>
  `
}

// textNodeParams are the inputs of createTextNode.
type textNodeParams struct {
	Icon            string
	Label           string
	Value           float64
	ID              string
	Index           int
	ShowIcons       bool
	ShiftValuePos   float64
	Bold            bool
	NumberFormat    string
	NumberPrecision *int
	LabelXOffset    float64
}

// createTextNode renders one label/value stat row, mirroring
// github-readme-stats' createTextNode.
func createTextNode(p textNodeParams) string {
	var precision *int
	if p.NumberPrecision != nil {
		v := *p.NumberPrecision
		if v < 0 {
			v = 0
		}
		if v > 2 {
			v = 2
		}
		precision = &v
	}

	kValue := KFormatter(p.Value, precision)
	if strings.EqualFold(p.NumberFormat, "long") {
		kValue = fmtNum(p.Value)
	}

	staggerDelay := (p.Index + 3) * 150
	boldClass := "not_bold"
	if p.Bold {
		boldClass = " bold"
	}
	valueX := 120.0
	if p.ShowIcons {
		valueX = 140
	}
	if p.Bold {
		valueX += 5
	}
	valueX += p.ShiftValuePos

	labelOffset := ""
	if p.ShowIcons {
		labelOffset = `x="` + fmtNum(p.LabelXOffset) + `"`
	}
	iconSvg := ""
	if p.ShowIcons {
		iconSvg = `
    <svg data-testid="icon" class="icon" viewBox="0 0 16 16" version="1.1" width="16" height="16">
      ` + p.Icon + `
    </svg>
  `
	}

	content := `
      ` + iconSvg + `
      <text class="stat ` + boldClass + `" ` + labelOffset + ` y="12.5">` + EncodeHTML(p.Label) + `:</text>
      <text
        class="stat ` + boldClass + `"
        x="` + fmtNum(valueX) + `"
        y="12.5"
        data-testid="` + p.ID + `"
      >` + kValue + `</text>`

	return `
    <g class="stagger" style="animation-delay: ` + strconv.Itoa(staggerDelay) + `ms" transform="translate(25, 0)">` + content + `
    </g>
  `
}
