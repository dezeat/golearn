// Copyright 2026 dezeat
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tui

import "strings"

// wrapText hard-wraps s so no visual line exceeds width characters.
// It wraps at word boundaries when possible and breaks mid-word only
// when a single word exceeds width. Existing newlines are preserved.
func wrapText(s string, width int) string {
	if width <= 0 {
		return s
	}

	var b strings.Builder
	for i, line := range strings.Split(s, "\n") {
		if i > 0 {
			b.WriteByte('\n')
		}
		writeWrappedLine(&b, line, width)
	}
	return b.String()
}

// writeWrappedLine writes a single logical line, word-wrapping to width.
func writeWrappedLine(b *strings.Builder, line string, width int) {
	if len(line) <= width {
		b.WriteString(line)
		return
	}

	words := strings.Fields(line)
	if len(words) == 0 {
		return
	}

	col := 0
	for _, w := range words {
		wLen := len(w)

		// If a single word exceeds width, break it mid-word.
		if wLen > width {
			if col > 0 {
				b.WriteByte('\n')
				col = 0
			}
			for j := 0; j < wLen; j += width {
				if j > 0 {
					b.WriteByte('\n')
				}
				end := j + width
				if end > wLen {
					end = wLen
				}
				b.WriteString(w[j:end])
				col = end - j
			}
			continue
		}

		switch {
		case col == 0:
			b.WriteString(w)
			col = wLen
		case col+1+wLen <= width:
			b.WriteByte(' ')
			b.WriteString(w)
			col += 1 + wLen
		default:
			b.WriteByte('\n')
			b.WriteString(w)
			col = wLen
		}
	}
}

// wrapAndIndent wraps text to contentWidth and prepends indent to each
// resulting line. Use contentWidth = available columns minus indent size.
func wrapAndIndent(text string, contentWidth int, indent string) string {
	wrapped := wrapText(text, contentWidth)
	lines := strings.Split(wrapped, "\n")
	for i, l := range lines {
		lines[i] = indent + l
	}
	return strings.Join(lines, "\n")
}
