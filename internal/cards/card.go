package cards

import (
	"fmt"
	"math"
	"strings"
)

// Card builds the SVG shell shared by the stats and repo cards, mirroring
// github-readme-stats' Card class.
type Card struct {
	Width  float64
	Height float64

	HideBorder   bool
	HideTitle    bool
	BorderRadius float64

	Colors CardColors
	Title  string
	CSS    string

	PaddingX float64
	PaddingY float64

	TitlePrefixIcon string
	Animations      bool

	A11yTitle string
	A11yDesc  string
}

// NewCard creates a card with the upstream defaults.
func NewCard(width, height float64, colors CardColors, title, titlePrefixIcon string) *Card {
	return &Card{
		Width:           width,
		Height:          height,
		BorderRadius:    4.5,
		Colors:          colors,
		Title:           title,
		PaddingX:        25,
		PaddingY:        35,
		TitlePrefixIcon: titlePrefixIcon,
		Animations:      true,
	}
}

// SetBorderRadius overrides the default border radius when a value is given.
func (c *Card) SetBorderRadius(v *float64) {
	if v != nil {
		c.BorderRadius = *v
	}
}

// DisableAnimations turns off animations.
func (c *Card) DisableAnimations() {
	c.Animations = false
}

// SetHideTitle adjusts the height like upstream's setHideTitle.
func (c *Card) SetHideTitle(value bool) {
	if value && !c.HideTitle {
		c.Height -= 30
	}
	if !value && c.HideTitle {
		c.Height += 30
	}
	c.HideTitle = value
}

// SetAccessibilityLabel sets the a11y title and description.
func (c *Card) SetAccessibilityLabel(title, desc string) {
	c.A11yTitle = title
	c.A11yDesc = desc
}

func (c *Card) renderTitle() string {
	titleText := `
      <text
        x="0"
        y="0"
        class="header"
        data-testid="header"
      >` + EncodeHTML(c.Title) + `</text>
    `

	prefixIcon := `
      <svg
        class="icon"
        x="0"
        y="-13"
        viewBox="0 0 16 16"
        version="1.1"
        width="16"
        height="16"
      >
        ` + c.TitlePrefixIcon + `
      </svg>
    `

	first := ""
	if c.TitlePrefixIcon != "" {
		first = prefixIcon
	}
	return `
      <g
        data-testid="card-title"
        transform="translate(` + fmtNum(c.PaddingX) + `, ` + fmtNum(c.PaddingY) + `)"
      >
        ` + strings.Join(FlexLayout([]string{first, titleText}, 25, "", nil), "") + `
      </g>
    `
}

func (c *Card) renderGradient() string {
	buildGradientDef := func(id string, bg []string) string {
		gradients := bg[1:]
		if len(gradients) < 2 {
			return ""
		}
		stops := make([]string, 0, len(gradients))
		for i, g := range gradients {
			offset := float64(i) * 100 / float64(len(gradients)-1)
			stops = append(stops, `<stop offset="`+fmtNum(offset)+`%" stop-color="#`+g+`" />`)
		}
		return `
          <linearGradient
            id="` + id + `"
            gradientTransform="rotate(` + bg[0] + `)"
            gradientUnits="userSpaceOnUse"
          >
            ` + strings.Join(stops, ",") + `
          </linearGradient>`
	}

	gradientDef := ""
	if c.Colors.BGColor.IsGradient() {
		gradientDef = buildGradientDef("gradient", c.Colors.BGColor.Gradient)
	}

	return `
        <defs>
          ` + gradientDef + `
        </defs>
        `
}

func (c *Card) getAnimations() string {
	return `
      /* Animations */
      @keyframes scaleInAnimation {
        from {
          transform: translate(-5px, 5px) scale(0);
        }
        to {
          transform: translate(-5px, 5px) scale(1);
        }
      }
      @keyframes fadeInAnimation {
        from {
          opacity: 0;
        }
        to {
          opacity: 1;
        }
      }
    `
}

// Render assembles the final SVG.
func (c *Card) Render(body string) (string, error) {
	if math.IsNaN(c.BorderRadius) || math.IsInf(c.BorderRadius, 0) {
		return "", fmt.Errorf("Invalid border radius: %q", fmtNum(c.BorderRadius))
	}

	disableAnimationsCSS := ""
	if !c.Animations {
		disableAnimationsCSS = `* { animation-duration: 0s !important; animation-delay: 0s !important; }`
	}

	bodyTop := c.PaddingY + 20
	if c.HideTitle {
		bodyTop = c.PaddingX
	}

	titleBlock := ""
	if !c.HideTitle {
		titleBlock = c.renderTitle()
	}

	return `
      <svg
        width="` + fmtNum(c.Width) + `"
        height="` + fmtNum(c.Height) + `"
        viewBox="0 0 ` + fmtNum(c.Width) + ` ` + fmtNum(c.Height) + `"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
        role="img"
        aria-labelledby="descId"
      >
        <title id="titleId">` + EncodeHTML(c.A11yTitle) + `</title>
        <desc id="descId">` + EncodeHTML(c.A11yDesc) + `</desc>
        <style>
          .header {
            font: 600 18px 'Segoe UI', Ubuntu, Sans-Serif;
            fill: ` + c.Colors.TitleColor + `;
            animation: fadeInAnimation 0.8s ease-in-out forwards;
          }
          @supports(-moz-appearance: auto) {
            /* Selector detects Firefox */
            .header { font-size: 15.5px; }
          }
          ` + c.CSS + `

          ` + c.getAnimations() + `
          ` + disableAnimationsCSS + `
        </style>

        ` + c.renderGradient() + `

        <rect
          data-testid="card-bg"
          class="card-bg"
          x="0.5"
          y="0.5"
          rx="` + fmtNum(c.BorderRadius) + `"
          height="99%"
          stroke="` + c.Colors.BorderColor + `"
          width="` + fmtNum(c.Width-1) + `"
          fill="` + c.Colors.BGColor.Fill("gradient") + `"
          stroke-opacity="` + boolInt(!c.HideBorder) + `"
        />

        ` + titleBlock + `

        <g
          data-testid="main-card-body"
          transform="translate(0, ` + fmtNum(bodyTop) + `)"
        >
          ` + body + `
        </g>
      </svg>
    `, nil
}

func boolInt(v bool) string {
	if v {
		return "1"
	}
	return "0"
}
