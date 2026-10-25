package cards

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// fmtNum formats a float the way JavaScript's String(number) does for the
// values that appear in the cards (no exponent notation, shortest round-trip).
func fmtNum(v float64) string {
	if math.IsNaN(v) {
		return "NaN"
	}
	if math.IsInf(v, 0) {
		return "Infinity"
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func floatPtr(v float64) *float64 { return &v }

var cjkRange = regexp.MustCompile(`[\x{3000}-\x{9FFF}\x{FF00}-\x{FFEF}]`)

// runeWidth returns the Segoe UI width factor of a single rune.
func runeWidth(r rune) float64 {
	if cjkRange.MatchString(string(r)) || r == '\u3000' {
		return 1
	}
	if int(r) >= 0 && int(r) < len(segoeWidths) {
		return segoeWidths[r]
	}
	return segoeAvg
}

// MeasureText estimates the rendered width of s in the Segoe UI font,
// mirroring github-readme-stats' measureText.
func MeasureText(s string, fontSize float64) float64 {
	total := 0.0
	for _, r := range s {
		total += runeWidth(r)
	}
	return total * fontSize
}

// FlexLayout lays out items horizontally (default) or vertically with a gap,
// mirroring github-readme-stats' flexLayout.
func FlexLayout(items []string, gap float64, direction string, sizes []float64) []string {
	out := make([]string, 0, len(items))
	lastSize := 0.0
	idx := 0
	for _, item := range items {
		if item == "" {
			continue
		}
		size := 0.0
		if idx < len(sizes) {
			size = sizes[idx]
		}
		var transform string
		if direction == "column" {
			transform = "translate(0, " + fmtNum(lastSize) + ")"
		} else {
			transform = "translate(" + fmtNum(lastSize) + ", 0)"
		}
		lastSize += size + gap
		out = append(out, `<g transform="`+transform+`">`+item+`</g>`)
		idx++
	}
	return out
}

var (
	normalizeWhitespaceRe = regexp.MustCompile(`[\t\n\r ]+`)
	tokenRe               = regexp.MustCompile(`\s|[\x{3000}-\x{9FFF}\x{FF00}-\x{FFEF}]|[^\s\x{3000}-\x{9FFF}\x{FF00}-\x{FFEF}]+`)
	trailingSpaceRe       = regexp.MustCompile(`[\t\n\r ]+$`)
)

func takeFittingSegment(token string, fontSize, availableWidth float64) (string, float64) {
	var sb strings.Builder
	width := 0.0
	for _, r := range token {
		cw := runeWidth(r) * fontSize
		if sb.Len() > 0 && width+cw > availableWidth {
			break
		}
		sb.WriteRune(r)
		width += cw
	}
	return sb.String(), width
}

// SplitWrappedText splits text into the lines it would wrap to, mirroring
// github-readme-stats' splitWrappedText.
func SplitWrappedText(text string, fontSize, maxWidth float64) []string {
	if text == "" {
		return nil
	}

	normalized := normalizeWhitespaceRe.ReplaceAllString(text, " ")
	tokens := tokenRe.FindAllString(normalized, -1)
	if tokens == nil {
		return nil
	}

	lines := []string{""}
	currentWidth := 0.0

	for _, token := range tokens {
		if token == " " {
			if currentWidth == 0 {
				continue
			}
			lines[len(lines)-1] += token
			currentWidth += MeasureText(token, fontSize)
			continue
		}

		remaining := token
		for remaining != "" {
			w := MeasureText(remaining, fontSize)
			if currentWidth+w <= maxWidth {
				lines[len(lines)-1] += remaining
				currentWidth += w
				break
			}
			if currentWidth > 0 {
				lines = append(lines, "")
				currentWidth = 0
				continue
			}
			segment, width := takeFittingSegment(remaining, fontSize, maxWidth)
			lines[len(lines)-1] += segment
			currentWidth = width
			remaining = remaining[len(segment):]
		}
	}

	for i := range lines {
		lines[i] = trailingSpaceRe.ReplaceAllString(lines[i], "")
	}
	return lines
}

// WrapTextMultiline splits text over multiple lines, encodes each line as
// HTML, adds an ellipsis when the text exceeds maxLines and drops empty lines.
func WrapTextMultiline(text string, width, fontSize float64, maxLines int) []string {
	wrapped := SplitWrappedText(text, fontSize, width)
	lines := make([]string, 0, len(wrapped))
	for _, line := range wrapped {
		lines = append(lines, EncodeHTML(strings.TrimSpace(line)))
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	if len(wrapped) > maxLines {
		last := maxLines - 1
		if last >= 0 && last < len(lines) {
			lines[last] = lines[last] + "..."
		}
	}
	// Remove empty lines if text fits in less than maxLines lines.
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// KFormatter abbreviates a number with a "k" suffix, mirroring
// github-readme-stats' kFormatter. precision switches to always-suffix mode.
func KFormatter(num float64, precision *int) string {
	abs := math.Abs(num)
	sign := 1.0
	if num < 0 {
		sign = -1
	}

	if precision != nil {
		return strconv.FormatFloat(sign*(abs/1000), 'f', *precision, 64) + "k"
	}

	if abs < 1000 {
		return fmtNum(num)
	}

	v := math.Round((abs/1000)*10) / 10
	return fmtNum(sign*v) + "k"
}

// EncodeHTML encodes a string for safe inclusion in the SVGs, mirroring
// github-readme-stats' encodeHTML.
func EncodeHTML(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\u0008' {
			continue
		}
		if (r >= 0x00A0 && r <= 0x9999) || r == '<' || r == '>' || r == '&' || r == '"' || r == '\'' {
			if i+1 < len(runes) && runes[i+1] == '#' {
				b.WriteRune(r)
				continue
			}
			b.WriteString("&#")
			b.WriteString(strconv.Itoa(int(r)))
			b.WriteString(";")
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
