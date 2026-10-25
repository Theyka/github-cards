package cards

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

// ColorValue is either a single color ("#141321") or a gradient
// (["90", "f00", "0f0"], i.e. angle followed by bare hex stops).
type ColorValue struct {
	Single   string
	Gradient []string
}

// IsGradient reports whether the value is a gradient.
func (c ColorValue) IsGradient() bool { return c.Gradient != nil }

// Fill returns the SVG fill for the value ("#hex" or "url(#gradient)").
func (c ColorValue) Fill(gradientID string) string {
	if c.IsGradient() {
		return "url(#" + gradientID + ")"
	}
	return c.Single
}

// CardColors holds the resolved colors of one color scheme.
type CardColors struct {
	TitleColor     string
	IconColor      string
	TextColor      string
	BorderColor    string
	RingColor      string
	ProgBarBgColor string
	BGColor        ColorValue
}

// ColorInput holds the raw color params of a request. Pointer fields
// distinguish "not sent" from "sent empty".
type ColorInput struct {
	TitleColor     *string
	IconColor      *string
	TextColor      *string
	BGColor        *string
	BorderColor    *string
	RingColor      *string
	ProgBarBgColor *string
	Theme          *string
}

var hexColorRe = regexp.MustCompile(`^([A-Fa-f0-9]{8}|[A-Fa-f0-9]{6}|[A-Fa-f0-9]{4}|[A-Fa-f0-9]{3})$`)

// IsBareHexColor checks for a hex color with no leading "#".
func IsBareHexColor(value string) bool {
	return hexColorRe.MatchString(value)
}

// IsPrefixedHexColor checks for a "#"-prefixed hex color.
func IsPrefixedHexColor(value string) bool {
	return strings.HasPrefix(value, "#") && hexColorRe.MatchString(value[1:])
}

// IsValidGradient checks parts of the form [angle, stop1, stop2, ...].
func IsValidGradient(parts []string) bool {
	if len(parts) < 3 {
		return false
	}
	angle := parts[0]
	if strings.TrimSpace(angle) == "" {
		return false
	}
	if _, err := strconv.ParseFloat(angle, 64); err != nil {
		return false
	}
	for _, stop := range parts[1:] {
		if !IsBareHexColor(stop) {
			return false
		}
	}
	return true
}

// IsValidColorInput reports whether a present query param value is a valid
// color or gradient input.
func IsValidColorInput(value string) bool {
	return IsValidGradient(strings.Split(value, ",")) || IsBareHexColor(value)
}

// FallbackColor resolves a color param or theme value to a ColorValue,
// mirroring github-readme-stats' fallbackColor.
func FallbackColor(color string, fallback ColorValue) ColorValue {
	if color != "" {
		parts := strings.Split(color, ",")
		if len(parts) > 1 && IsValidGradient(parts) {
			return ColorValue{Gradient: parts}
		}
		if IsBareHexColor(color) {
			return ColorValue{Single: "#" + color}
		}
	}
	return fallback
}

var errGradientNotAllowed = errors.New("gradients are only allowed for bg_color")

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// GetCardColors resolves theme colors with overrides, mirroring
// github-readme-stats' getCardColors.
func GetCardColors(in ColorInput) (CardColors, error) {
	def := themesGhstats["default"]
	selected := def
	if in.Theme != nil {
		if t, ok := themesGhstats[*in.Theme]; ok {
			selected = t
		}
	}

	defaultBorderColor := def["border_color"]
	if v, ok := selected["border_color"]; ok {
		defaultBorderColor = v
	}

	first := func(a, b string) string {
		if a != "" {
			return a
		}
		return b
	}

	titleColor := FallbackColor(first(deref(in.TitleColor), selected["title_color"]), ColorValue{Single: "#" + def["title_color"]})
	iconColor := FallbackColor(first(deref(in.IconColor), selected["icon_color"]), ColorValue{Single: "#" + def["icon_color"]})
	textColor := FallbackColor(first(deref(in.TextColor), selected["text_color"]), ColorValue{Single: "#" + def["text_color"]})
	bgColor := FallbackColor(first(deref(in.BGColor), selected["bg_color"]), ColorValue{Single: "#" + def["bg_color"]})
	borderColor := FallbackColor(first(deref(in.BorderColor), defaultBorderColor), ColorValue{Single: "#" + defaultBorderColor})
	ringColor := FallbackColor(deref(in.RingColor), titleColor)
	progBarBgColor := FallbackColor(deref(in.ProgBarBgColor), ColorValue{Single: "#ddd"})

	if titleColor.IsGradient() || iconColor.IsGradient() || textColor.IsGradient() || borderColor.IsGradient() || ringColor.IsGradient() || progBarBgColor.IsGradient() {
		return CardColors{}, errGradientNotAllowed
	}

	return CardColors{
		TitleColor:     titleColor.Single,
		IconColor:      iconColor.Single,
		TextColor:      textColor.Single,
		BorderColor:    borderColor.Single,
		RingColor:      ringColor.Single,
		ProgBarBgColor: progBarBgColor.Single,
		BGColor:        bgColor,
	}, nil
}
