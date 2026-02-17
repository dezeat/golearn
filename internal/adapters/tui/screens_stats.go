package tui

import (
	"fmt"
	"strings"
	"time"
)

// sparkline renders a unicode sparkline from a slice of float64 values (0–100).
func sparkline(values []float64) string {
	if len(values) == 0 {
		return "—"
	}
	blocks := []rune("▁▂▃▄▅▆▇█")
	var b strings.Builder
	for _, v := range values {
		if v < 0 {
			v = 0
		}
		if v > 100 {
			v = 100
		}
		idx := int(v / 100 * float64(len(blocks)-1))
		if idx >= len(blocks) {
			idx = len(blocks) - 1
		}
		b.WriteRune(blocks[idx])
	}
	return b.String()
}

// trendDelta returns the delta between first and last values as a formatted string.
func trendDelta(values []float64) string {
	if len(values) < 2 {
		return ""
	}
	delta := values[len(values)-1] - values[0]
	if delta > 0 {
		return fmt.Sprintf(" ↑%.0f%%", delta)
	} else if delta < 0 {
		return fmt.Sprintf(" ↓%.0f%%", -delta)
	}
	return " →0%"
}

// truncate truncates a string to maxLen, adding "…" if needed.
func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return "…"
	}
	return s[:maxLen-1] + "…"
}

// fmtDuration formats seconds into a human-readable duration.
func fmtDuration(totalSec float64) string {
	if totalSec < 60 {
		return fmt.Sprintf("%.1fs", totalSec)
	}
	m := int(totalSec) / 60
	s := int(totalSec) % 60
	if m < 60 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	h := m / 60
	m = m % 60
	return fmt.Sprintf("%dh %dm", h, m)
}

func formatStatsTimestamp(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	layouts := []string{
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.Format("2006-01-02 15:04")
		}
	}
	if len(raw) >= len("2006-01-02 15:04") {
		return raw[:len("2006-01-02 15:04")]
	}
	return raw
}

// --- Home Menu ---

func (m model) viewHomeMenu() string {
	var b strings.Builder

	b.WriteString(styleHeader.Render("golearn — Home") + "\n")
	b.WriteString("══════════════\n\n")

	if m.currentUser != nil {
		b.WriteString(fmt.Sprintf("  Profile: %s\n\n", displayProfile(*m.currentUser)))
	}

	options := m.homeMenuOptions()
	for i, opt := range options {
		cursor := "  "
		if i == m.homeMenuCursor {
			cursor = "▸ "
		}
		label := opt
		// Mark "Review Wrong Answers" as disabled if no wrong answers.
		if opt == "Review Wrong Answers" && len(m.wrongAnswers) == 0 {
			label = styleDim.Render(opt + " (none)")
		}
		b.WriteString(cursor + label + "\n")
	}

	b.WriteString("\n  ↑/↓ or j/k to navigate · enter to select · q to quit\n")
	return b.String()
}

// --- Summary screen (updated) ---

func (m model) viewSummary() string {
	var b strings.Builder

	b.WriteString(styleHeader.Render("golearn — Session Summary") + "\n")
	b.WriteString("═════════════════════════\n\n")

	b.WriteString(fmt.Sprintf("  Topic:          %s\n", m.selectedTopic.Topic.Name))
	b.WriteString(fmt.Sprintf("  Total answered: %d\n", m.answered))
	b.WriteString(fmt.Sprintf("  Correct:        %d\n", m.correctCount))

	if m.answered > 0 {
		pct := float64(m.correctCount) / float64(m.answered) * 100
		b.WriteString(fmt.Sprintf("  Accuracy:       %.1f%%\n", pct))

		if m.totalLatency > 0 {
			avgMs := m.totalLatency / m.answered
			if avgMs >= 1000 {
				b.WriteString(fmt.Sprintf("  Avg response:   %.1fs\n", float64(avgMs)/1000))
			} else {
				b.WriteString(fmt.Sprintf("  Avg response:   %dms\n", avgMs))
			}
		}
	} else {
		b.WriteString("  Accuracy:       —\n")
	}

	wrongCount := len(m.wrongAnswers)
	if wrongCount > 0 {
		b.WriteString(fmt.Sprintf("\n  %s\n",
			styleIncorrect.Render(fmt.Sprintf("  %d question(s) answered incorrectly", wrongCount))))
	}

	// Menu options.
	b.WriteString("\n")
	options := m.summaryOptions()
	for i, opt := range options {
		cursor := "  "
		if i == m.summaryCursor {
			cursor = "▸ "
		}
		b.WriteString(cursor + opt + "\n")
	}

	b.WriteString("\n  ↑/↓ navigate · enter select · r review · q quit\n")
	return b.String()
}

// --- Global Stats Screen ---

func (m model) viewStatsGlobal() string {
	var b strings.Builder

	b.WriteString(styleHeader.Render("golearn — Global Stats") + "\n")
	b.WriteString("══════════════════════\n\n")

	if m.currentUser != nil {
		b.WriteString(fmt.Sprintf("  Profile: %s\n\n", displayProfile(*m.currentUser)))
	}

	if m.statsError != "" {
		b.WriteString(styleIncorrect.Render("  Error: "+m.statsError) + "\n")
		b.WriteString("\n  esc back\n")
		return b.String()
	}

	gs := m.statsGlobal
	if gs == nil {
		b.WriteString("  No stats data yet. Complete a practice session first.\n")
		b.WriteString("\n  esc back\n")
		return b.String()
	}

	if gs.TotalAnswered == 0 && gs.TotalSkipped == 0 {
		b.WriteString("  No attempts recorded yet.\n")
		b.WriteString("\n  esc back\n")
		return b.String()
	}

	b.WriteString(fmt.Sprintf("  Accuracy:        %.1f%%\n", gs.AccuracyPct))
	b.WriteString(fmt.Sprintf("  Answered:        %d\n", gs.TotalAnswered))
	b.WriteString(fmt.Sprintf("  Skipped:         %d\n", gs.TotalSkipped))
	b.WriteString(fmt.Sprintf("  Avg response:    %s\n", fmtDuration(gs.AvgLatencySeconds)))
	b.WriteString(fmt.Sprintf("  Total time:      %s\n", fmtDuration(gs.TotalTimeSeconds)))

	if gs.MostPracticedTopic != "" {
		b.WriteString(fmt.Sprintf("  Most practiced:  %s\n", gs.MostPracticedTopic))
	}
	if gs.WeakestTopic != "" {
		b.WriteString(fmt.Sprintf("  Weakest pack:    %s\n", gs.WeakestTopic))
	}

	// Trend sparkline.
	if len(m.statsGlobalTrend) > 0 {
		b.WriteString(fmt.Sprintf("\n  Trend (last %d sessions): %s%s\n",
			len(m.statsGlobalTrend),
			sparkline(m.statsGlobalTrend),
			trendDelta(m.statsGlobalTrend)))
	}

	b.WriteString("\n  enter: pack stats · esc: back to home\n")
	return b.String()
}

// --- Pack Stats List Screen ---

func (m model) viewStatsPackList() string {
	var b strings.Builder

	b.WriteString(styleHeader.Render("golearn — Pack Stats") + "\n")
	b.WriteString("════════════════════\n\n")

	if m.statsError != "" {
		b.WriteString(styleIncorrect.Render("  Error: "+m.statsError) + "\n")
		b.WriteString("\n  esc back\n")
		return b.String()
	}

	if len(m.statsPacks) == 0 {
		b.WriteString("  No packs found.\n")
		b.WriteString("\n  esc back\n")
		return b.String()
	}

	// Fixed-column layout.
	w := m.width
	if w == 0 {
		w = 80
	}
	const cursorW = 2
	const qsColW = 8  // "999 qs"
	const accColW = 6 // "100.0%"
	const attColW = 8 // "99999 att"
	const gaps = 6    // three gaps of 2 chars
	nameW := w - cursorW - gaps - qsColW - accColW - attColW
	if nameW < 10 {
		nameW = 10
	}

	// Header row.
	b.WriteString(fmt.Sprintf("  %-*s  %-*s  %-*s  %s\n",
		nameW, "Pack", qsColW, "Qs", accColW, "Acc", "Att"))
	b.WriteString(fmt.Sprintf("  %s  %s  %s  %s\n",
		strings.Repeat("─", nameW), strings.Repeat("─", qsColW),
		strings.Repeat("─", accColW), strings.Repeat("─", attColW)))

	for i, ts := range m.statsPacks {
		cursor := "  "
		if i == m.statsPackCursor {
			cursor = "▸ "
		}

		name := truncate(ts.TopicName, nameW)
		qsStr := fmt.Sprintf("%d", ts.TotalQuestions)
		accStr := "—"
		if ts.AttemptsAnswered > 0 {
			accStr = fmt.Sprintf("%.0f%%", ts.AccuracyPct)
		}
		attStr := fmt.Sprintf("%d", ts.AttemptsAnswered)

		b.WriteString(fmt.Sprintf("%s%-*s  %*s  %*s  %*s\n",
			cursor, nameW, name, qsColW, qsStr, accColW, accStr, attColW, attStr))
	}

	b.WriteString("\n  ↑/↓ navigate · enter details · esc back\n")
	return b.String()
}

// --- Pack Detail Stats Screen ---

func (m model) viewStatsPackDetail() string {
	var b strings.Builder

	b.WriteString(styleHeader.Render("golearn — Pack Detail Stats") + "\n")
	b.WriteString("═══════════════════════════\n\n")

	if m.statsError != "" {
		b.WriteString(styleIncorrect.Render("  Error: "+m.statsError) + "\n")
		b.WriteString("\n  b back · q home\n")
		return b.String()
	}

	ts := m.statsDetail
	if ts == nil {
		b.WriteString("  No stats loaded.\n")
		b.WriteString("\n  b back · q home\n")
		return b.String()
	}

	cw := m.contentWidth(2)

	// Header metrics.
	b.WriteString(fmt.Sprintf("  %s\n\n", styleBold.Render(truncate(ts.TopicName, cw-2))))

	accStr := "—"
	if ts.AttemptsAnswered > 0 {
		accStr = fmt.Sprintf("%.1f%%", ts.AccuracyPct)
	}
	b.WriteString(fmt.Sprintf("  Accuracy:    %s\n", accStr))
	b.WriteString(fmt.Sprintf("  Attempts:    %d answered, %d skipped\n", ts.AttemptsAnswered, ts.AttemptsSkipped))
	b.WriteString(fmt.Sprintf("  Coverage:    %d/%d questions (%.0f%%)\n", ts.SeenQuestions, ts.TotalQuestions, ts.CoveragePct))
	if ts.AttemptsAnswered > 0 {
		b.WriteString(fmt.Sprintf("  Avg time:    %s\n", fmtDuration(ts.AvgLatencySeconds)))
	}
	if ts.LastPracticedAt != "" {
		b.WriteString(fmt.Sprintf("  Last:        %s\n", formatStatsTimestamp(ts.LastPracticedAt)))
	}

	// Difficulty breakdown.
	b.WriteString("\n")
	if len(m.statsDifficulty) > 0 {
		b.WriteString(styleBold.Render("  Difficulty Breakdown") + "\n")
		b.WriteString(fmt.Sprintf("  %-10s  %8s  %8s  %8s\n", "Level", "Attempts", "Accuracy", "Avg Time"))
		b.WriteString(fmt.Sprintf("  %s  %s  %s  %s\n",
			strings.Repeat("─", 10), strings.Repeat("─", 8),
			strings.Repeat("─", 8), strings.Repeat("─", 8)))
		for _, ds := range m.statsDifficulty {
			b.WriteString(fmt.Sprintf("  %-10s  %8d  %7.0f%%  %8s\n",
				ds.Bucket, ds.AttemptsAnswered, ds.AccuracyPct, fmtDuration(ds.AvgLatencySeconds)))
		}
	} else {
		b.WriteString(styleDim.Render("  No difficulty ratings yet.") + "\n")
	}

	// Tags.
	if len(m.statsWeakTags) > 0 {
		b.WriteString("\n" + styleBold.Render("  Weak Tags (accuracy < 70%)") + "\n")
		for _, t := range m.statsWeakTags {
			b.WriteString(fmt.Sprintf("    %-20s  %.0f%% (%d attempts)\n",
				truncate(t.Tag, 20), t.AccuracyPct, t.AttemptsAnswered))
		}
	}
	if len(m.statsStrongTags) > 0 {
		b.WriteString("\n" + styleBold.Render("  Strong Tags (accuracy ≥ 70%)") + "\n")
		for _, t := range m.statsStrongTags {
			b.WriteString(fmt.Sprintf("    %-20s  %.0f%% (%d attempts)\n",
				truncate(t.Tag, 20), t.AccuracyPct, t.AttemptsAnswered))
		}
	}

	// Weak questions.
	if len(m.statsWeakQs) > 0 {
		b.WriteString("\n" + styleBold.Render("  Weakest Questions") + "\n")
		previewW := cw - 30
		if previewW < 20 {
			previewW = 20
		}
		for i, wq := range m.statsWeakQs {
			preview := truncate(wq.PromptPreview, previewW)
			b.WriteString(fmt.Sprintf("  %2d. %s  (wrong %.0f%%, %d att)\n",
				i+1, preview, wq.WrongRate*100, wq.AttemptsAnswered))
		}
	}

	// Trend.
	if len(m.statsDetailTrend) > 0 {
		b.WriteString(fmt.Sprintf("\n  Trend (last %d sessions): %s%s\n",
			len(m.statsDetailTrend),
			sparkline(m.statsDetailTrend),
			trendDelta(m.statsDetailTrend)))
	}

	b.WriteString("\n  b/esc back · q home\n")
	return b.String()
}
