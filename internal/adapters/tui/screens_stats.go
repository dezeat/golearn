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

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dezeat/golearn/internal/ports"
)

// TUI stats display constants.
const (
	statsTrendLimit       = 10 // number of recent sessions for trend sparklines
	statsTagMinAttempts   = 5  // minimum attempts for tag-based stats
	statsWeakMinAttempts  = 3  // minimum attempts for weak-question list
	statsWeakQuestionsMax = 10 // maximum weak questions to display
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
	m %= 60
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

// --- Stats Menu ---

func (m model) viewStatsMenu() string {
	var b strings.Builder

	m.writeCenteredLine(&b, styleHeader.Render("golearn — Stats"))
	m.writeCenteredLine(&b, "═══════════════")
	b.WriteString("\n")

	if m.currentUser != nil {
		m.writeCenteredLine(&b, fmt.Sprintf("Profile: %s", displayProfile(*m.currentUser)))
		b.WriteString("\n")
	}

	opts := m.statsMenuOptions()
	for i, opt := range opts {
		cursor := "  "
		if i == m.statsMenuCursor {
			cursor = "▸ "
		}
		m.writeCenteredLine(&b, cursor+opt)
	}

	m.writeFooter(&b, footerMenuSub)
	return b.String()
}

// --- Home Menu ---

func (m model) viewHomeMenu() string {
	var b strings.Builder

	m.writeCenteredLine(&b, styleHeader.Render("golearn — Home"))
	m.writeCenteredLine(&b, "══════════════")
	b.WriteString("\n")

	options := m.homeMenuOptions()
	for i, opt := range options {
		cursor := "  "
		if i == m.homeMenuCursor {
			cursor = "▸ "
		}
		label := opt
		// Mark "Review Wrong Answers" as disabled if no wrong answers.
		if opt == "Review Wrong Answers" && len(m.wrongAnswers) == 0 {
			label = styleDim.Render(opt)
		}
		m.writeCenteredLine(&b, cursor+label)
	}

	if len(m.wrongAnswers) > 0 {
		m.writeFooter(&b, footerMenuTopReview)
	} else {
		m.writeFooter(&b, footerMenuTop)
	}
	return b.String()
}

// --- Summary screen (updated) ---

func (m model) viewSummary() string {
	var b strings.Builder

	m.writeCenteredLine(&b, styleHeader.Render("golearn — Session Summary"))
	m.writeCenteredLine(&b, "═════════════════════════")
	b.WriteString("\n")

	lines := []string{
		fmt.Sprintf("Topic:          %s", m.selectedTopic.Topic.Name),
		fmt.Sprintf("Total answered: %d", m.answered),
		fmt.Sprintf("Correct:        %d", m.correctCount),
	}

	if m.answered > 0 {
		pct := float64(m.correctCount) / float64(m.answered) * 100
		lines = append(lines, fmt.Sprintf("Accuracy:       %.1f%%", pct))

		if m.totalLatency > 0 {
			avgMs := m.totalLatency / m.answered
			if avgMs >= 1000 {
				lines = append(lines, fmt.Sprintf("Avg response:   %.1fs", float64(avgMs)/1000))
			} else {
				lines = append(lines, fmt.Sprintf("Avg response:   %dms", avgMs))
			}
		}
	} else {
		lines = append(lines, "Accuracy:       —")
	}

	blockWidth := maxLineWidth(lines)
	for _, line := range lines {
		m.writeCenteredLeftLine(&b, blockWidth, line)
	}

	wrongCount := len(m.wrongAnswers)
	if wrongCount > 0 {
		b.WriteString("\n")
		m.writeCenteredLeftLine(&b, blockWidth, styleIncorrect.Render(fmt.Sprintf("%d question(s) answered incorrectly", wrongCount)))
	}

	// Menu options.
	b.WriteString("\n")
	options := m.summaryOptions()
	menuLines := make([]string, 0, len(options))
	for i, opt := range options {
		cursor := "  "
		if i == m.summaryCursor {
			cursor = "▸ "
		}
		menuLines = append(menuLines, cursor+opt)
	}
	menuWidth := maxLineWidth(menuLines)
	for _, line := range menuLines {
		m.writeCenteredLeftLine(&b, menuWidth, line)
	}

	if len(m.wrongAnswers) > 0 {
		m.writeFooter(&b, footerMenuSubReview)
	} else {
		m.writeFooter(&b, footerMenuSub)
	}
	return b.String()
}

// --- Global Stats Screen ---

func (m model) viewStatsGlobal() string {
	var b strings.Builder

	m.writeCenteredLine(&b, styleHeader.Render("golearn — Global Stats"))
	m.writeCenteredLine(&b, "══════════════════════")
	b.WriteString("\n")

	lines := make([]string, 0, 12)

	if m.currentUser != nil {
		lines = append(lines, fmt.Sprintf("Profile: %s", displayProfile(*m.currentUser)), "")
	}

	if m.statsError != "" {
		lines = append(lines, styleIncorrect.Render("Error: "+m.statsError))
		blockWidth := maxLineWidth(lines)
		for _, line := range lines {
			if line == "" {
				b.WriteString("\n")
				continue
			}
			m.writeCenteredLeftLine(&b, blockWidth, line)
		}
		m.writeFooter(&b, footerBackOnly)
		return b.String()
	}

	gs := m.statsGlobal
	if gs == nil {
		m.writeCenteredLeftLine(&b, lipgloss.Width("No stats data yet. Complete a practice session first."), "No stats data yet. Complete a practice session first.")
		m.writeFooter(&b, footerBackOnly)
		return b.String()
	}

	if gs.TotalAnswered == 0 && gs.TotalSkipped == 0 {
		m.writeCenteredLeftLine(&b, lipgloss.Width("No attempts recorded yet."), "No attempts recorded yet.")
		m.writeFooter(&b, footerBackOnly)
		return b.String()
	}

	lines = append(lines,
		fmt.Sprintf("Accuracy:        %.1f%%", gs.AccuracyPct),
		fmt.Sprintf("Answered:        %d", gs.TotalAnswered),
		fmt.Sprintf("Skipped:         %d", gs.TotalSkipped),
		fmt.Sprintf("Avg response:    %s", fmtDuration(gs.AvgLatencySeconds)),
		fmt.Sprintf("Total time:      %s", fmtDuration(gs.TotalTimeSeconds)),
	)

	if gs.MostPracticedTopic != "" {
		lines = append(lines, fmt.Sprintf("Most practiced:  %s", gs.MostPracticedTopic))
	}
	if gs.WeakestTopic != "" {
		lines = append(lines, fmt.Sprintf("Weakest pack:    %s", gs.WeakestTopic))
	} else {
		lines = append(lines, fmt.Sprintf("Weakest pack:    %s", styleDim.Render("N/A (not enough attempts)")))
	}
	if gs.StrongestTopic != "" {
		lines = append(lines, fmt.Sprintf("Strongest pack:  %s", gs.StrongestTopic))
	} else {
		lines = append(lines, fmt.Sprintf("Strongest pack:  %s", styleDim.Render("N/A (not enough attempts)")))
	}

	// Trend sparkline.
	if len(m.statsGlobalTrend) > 0 {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("Trend (last %d sessions): %s%s",
			len(m.statsGlobalTrend),
			sparkline(m.statsGlobalTrend),
			trendDelta(m.statsGlobalTrend)))
	}

	blockWidth := maxLineWidth(lines)
	for _, line := range lines {
		if line == "" {
			b.WriteString("\n")
			continue
		}
		m.writeCenteredLeftLine(&b, blockWidth, line)
	}

	m.writeFooter(&b, footerBackOnly)
	return b.String()
}

// --- Pack Stats List Screen ---

func (m model) viewStatsPackList() string {
	var b strings.Builder

	m.writeCenteredLine(&b, styleHeader.Render("golearn — Pack Stats"))
	m.writeCenteredLine(&b, "════════════════════")
	b.WriteString("\n")

	if m.statsError != "" {
		m.writeCenteredLine(&b, styleIncorrect.Render("Error: "+m.statsError))
		m.writeFooter(&b, footerMenuSub)
		return b.String()
	}

	if len(m.statsPacks) == 0 {
		m.writeCenteredLine(&b, "No packs found.")
		m.writeFooter(&b, footerMenuSub)
		return b.String()
	}

	// Fixed-column layout with bounded centered width so columns don't drift too far apart
	// on wide terminals.
	const packTableMaxW = 82
	const packTableTotalPadding = 20
	w := m.centeredBlockWidth(packTableMaxW, packTableTotalPadding, 46)
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
	m.writeCenteredLine(&b, fmt.Sprintf("  %-*s  %-*s  %-*s  %s",
		nameW, "Pack", qsColW, "Qs", accColW, "Acc", "Att"))
	m.writeCenteredLine(&b, fmt.Sprintf("  %s  %s  %s  %s",
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

		m.writeCenteredLine(&b, fmt.Sprintf("%s%-*s  %*s  %*s  %*s",
			cursor, nameW, name, qsColW, qsStr, accColW, accStr, attColW, attStr))
	}

	m.writeFooter(&b, footerMenuSub)
	return b.String()
}

// --- Pack Detail Stats Screen ---

func (m model) viewStatsPackDetail() string {
	var b strings.Builder

	m.writeCenteredLine(&b, styleHeader.Render("golearn — Pack Detail Stats"))
	m.writeCenteredLine(&b, "═══════════════════════════")
	b.WriteString("\n")

	if m.statsError != "" {
		m.writeCenteredLeftLine(&b, lipgloss.Width("Error: "+m.statsError), styleIncorrect.Render("Error: "+m.statsError))
		m.writeFooter(&b, footerBackOnly)
		return b.String()
	}

	ts := m.statsDetail
	if ts == nil {
		m.writeCenteredLeftLine(&b, lipgloss.Width("No stats loaded."), "No stats loaded.")
		m.writeFooter(&b, footerBackOnly)
		return b.String()
	}

	cw := m.contentWidth(2)
	lines := make([]string, 0, 32)

	// Header metrics.
	lines = append(lines, styleBold.Render(truncate(ts.TopicName, cw-2)), "")

	accStr := "—"
	if ts.AttemptsAnswered > 0 {
		accStr = fmt.Sprintf("%.1f%%", ts.AccuracyPct)
	}
	lines = append(lines,
		fmt.Sprintf("Accuracy:    %s", accStr),
		fmt.Sprintf("Attempts:    %d answered, %d skipped", ts.AttemptsAnswered, ts.AttemptsSkipped),
		fmt.Sprintf("Coverage:    %d/%d questions (%.0f%%)", ts.SeenQuestions, ts.TotalQuestions, ts.CoveragePct),
	)
	if ts.AttemptsAnswered > 0 {
		lines = append(lines, fmt.Sprintf("Avg time:    %s", fmtDuration(ts.AvgLatencySeconds)))
	}
	if ts.LastPracticedAt != "" {
		lines = append(lines, fmt.Sprintf("Last:        %s", formatStatsTimestamp(ts.LastPracticedAt)))
	}

	// Difficulty breakdown.
	lines = append(lines, "")
	if len(m.statsDifficulty) > 0 {
		lines = append(lines, styleBold.Render("Difficulty Breakdown"))
		lines = append(lines, fmt.Sprintf("%-10s  %8s  %8s  %8s", "Level", "Attempts", "Accuracy", "Avg Time"))
		lines = append(lines, fmt.Sprintf("%s  %s  %s  %s",
			strings.Repeat("─", 10), strings.Repeat("─", 8),
			strings.Repeat("─", 8), strings.Repeat("─", 8)))
		for _, ds := range m.statsDifficulty {
			lines = append(lines, fmt.Sprintf("%-10s  %8d  %7.0f%%  %8s",
				ds.Bucket, ds.AttemptsAnswered, ds.AccuracyPct, fmtDuration(ds.AvgLatencySeconds)))
		}
	} else {
		lines = append(lines, styleDim.Render("No difficulty ratings yet."))
	}

	// Tags.
	if len(m.statsWeakTags) > 0 {
		lines = append(lines, "", styleBold.Render("Weak Tags (accuracy < 70%)"))
		for _, t := range m.statsWeakTags {
			lines = append(lines, fmt.Sprintf("  %-20s  %.0f%% (%d attempts)",
				truncate(t.Tag, 20), t.AccuracyPct, t.AttemptsAnswered))
		}
	}
	if len(m.statsStrongTags) > 0 {
		lines = append(lines, "", styleBold.Render("Strong Tags (accuracy ≥ 70%)"))
		for _, t := range m.statsStrongTags {
			lines = append(lines, fmt.Sprintf("  %-20s  %.0f%% (%d attempts)",
				truncate(t.Tag, 20), t.AccuracyPct, t.AttemptsAnswered))
		}
	}

	// Weak questions.
	if len(m.statsWeakQs) > 0 {
		lines = append(lines, "", styleBold.Render("Weakest Questions"))
		previewW := cw - 30
		if previewW < 20 {
			previewW = 20
		}
		for i, wq := range m.statsWeakQs {
			preview := truncate(wq.PromptPreview, previewW)
			lines = append(lines, fmt.Sprintf("%2d. %s  (wrong %.0f%%, %d att)",
				i+1, preview, wq.WrongRate*100, wq.AttemptsAnswered))
		}
	}

	// Trend.
	if len(m.statsDetailTrend) > 0 {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("Trend (last %d sessions): %s%s",
			len(m.statsDetailTrend),
			sparkline(m.statsDetailTrend),
			trendDelta(m.statsDetailTrend)))
	}

	blockWidth := maxLineWidth(lines)
	for _, line := range lines {
		if line == "" {
			b.WriteString("\n")
			continue
		}
		m.writeCenteredLeftLine(&b, blockWidth, line)
	}

	m.writeFooter(&b, footerBackOnly)
	return b.String()
}

// --- Home Menu ---

func (m model) homeMenuOptions() []string {
	return []string{"Practice", "Review Wrong Answers", "Stats"}
}

func (m model) updateHomeMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	key := keyMsg.String()
	switch {
	case isQuitKey(key):
		m.quitting = true
		return m, tea.Quit
	case isBackKey(key):
		m.profileMenuCursor = 0
		m.screen = screenProfileMenu
	case isUpNav(key):
		if m.homeMenuCursor > 0 {
			m.homeMenuCursor--
		}
	case isDownNav(key):
		if m.homeMenuCursor < len(m.homeMenuOptions())-1 {
			m.homeMenuCursor++
		}
	case isEnterKey(key):
		options := m.homeMenuOptions()
		if m.homeMenuCursor >= len(options) {
			return m, nil
		}
		switch options[m.homeMenuCursor] {
		case "Practice":
			m.screen = screenTopicSelect
		case "Review Wrong Answers":
			if len(m.wrongAnswers) > 0 {
				m.startReviewSession(screenHomeMenu)
			}
		case "Stats":
			m.statsMenuCursor = 0
			m.screen = screenStatsMenu
		}
	case isReviewKey(key):
		if len(m.wrongAnswers) > 0 {
			m.startReviewSession(screenHomeMenu)
		}
	}
	return m, nil
}

// --- Stats Update Handlers ---

func (m model) statsMenuOptions() []string {
	return []string{"Global Stats", "Stats by Pack"}
}

func (m model) updateStatsMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	key := keyMsg.String()
	switch {
	case isBackKey(key):
		m.homeMenuCursor = 0
		m.screen = screenHomeMenu
	case isUpNav(key):
		if m.statsMenuCursor > 0 {
			m.statsMenuCursor--
		}
	case isDownNav(key):
		opts := m.statsMenuOptions()
		if m.statsMenuCursor < len(opts)-1 {
			m.statsMenuCursor++
		}
	case isEnterKey(key):
		opts := m.statsMenuOptions()
		if m.statsMenuCursor >= len(opts) {
			return m, nil
		}
		switch opts[m.statsMenuCursor] {
		case "Global Stats":
			m.loadGlobalStats()
			m.screen = screenStatsGlobal
		case "Stats by Pack":
			m.loadPackListStats()
			m.screen = screenStatsPackList
		}
	}
	return m, nil
}

func (m *model) loadGlobalStats() {
	if m.statsRepo == nil || m.userCtx == nil {
		return
	}
	uid := m.userCtx.CurrentUserID()
	gs, err := m.statsRepo.GlobalStats(uid)
	if err != nil {
		m.statsError = err.Error()
		m.statsGlobal = nil
		return
	}
	m.statsGlobal = gs
	m.statsError = ""

	// Load trend for global or most practiced topic.
	trend, err := m.statsRepo.SessionTrendGlobal(uid, statsTrendLimit)
	if err != nil {
		m.statsError = fmt.Sprintf("load trend: %v", err)
	}
	m.statsGlobalTrend = trend
}

func (m *model) loadPackListStats() {
	if m.statsRepo == nil || m.userCtx == nil {
		return
	}
	uid := m.userCtx.CurrentUserID()
	packs, err := m.statsRepo.TopicSummaries(uid)
	if err != nil {
		m.statsError = err.Error()
		m.statsPacks = nil
		return
	}

	// Sort by attempts descending for the current user.
	sortPacksByAttempts(packs)

	m.statsPacks = packs
	m.statsError = ""
	m.statsPackCursor = 0
}

func (m *model) loadPackDetailStats(topicID int64) {
	if m.statsRepo == nil || m.userCtx == nil {
		return
	}
	uid := m.userCtx.CurrentUserID()
	ts, err := m.statsRepo.TopicSummary(uid, topicID)
	if err != nil {
		m.statsError = err.Error()
		m.statsDetail = nil
		return
	}
	m.statsDetail = ts
	m.statsError = ""

	var errs []string

	diff, err := m.statsRepo.DifficultyStats(uid, topicID)
	if err != nil {
		errs = append(errs, fmt.Sprintf("difficulty: %v", err))
	}
	m.statsDifficulty = diff

	weakTags, err := m.statsRepo.TagStats(uid, topicID, statsTagMinAttempts)
	if err != nil {
		errs = append(errs, fmt.Sprintf("tags: %v", err))
	}
	var weak, strong []ports.TagStat
	for _, t := range weakTags {
		if t.AccuracyPct < 70 {
			weak = append(weak, t)
		} else {
			strong = append(strong, t)
		}
	}
	m.statsWeakTags = weak
	m.statsStrongTags = strong

	wqs, err := m.statsRepo.WeakQuestions(uid, topicID, statsWeakMinAttempts, statsWeakQuestionsMax)
	if err != nil {
		errs = append(errs, fmt.Sprintf("weak questions: %v", err))
	}
	m.statsWeakQs = wqs

	dt, err := m.statsRepo.SessionTrend(uid, topicID, statsTrendLimit)
	if err != nil {
		errs = append(errs, fmt.Sprintf("session trend: %v", err))
	}
	m.statsDetailTrend = dt

	if len(errs) > 0 {
		m.statsError = strings.Join(errs, "; ")
	}
}

func (m model) updateStatsGlobal(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	if isBackKey(keyMsg.String()) {
		m.statsMenuCursor = 0
		m.screen = screenStatsMenu
	}
	return m, nil
}

func (m model) updateStatsPackList(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	key := keyMsg.String()
	switch {
	case isBackKey(key):
		m.statsMenuCursor = 0
		m.screen = screenStatsMenu
	case isUpNav(key):
		if m.statsPackCursor > 0 {
			m.statsPackCursor--
		}
	case isDownNav(key):
		if m.statsPackCursor < len(m.statsPacks)-1 {
			m.statsPackCursor++
		}
	case isEnterKey(key):
		if len(m.statsPacks) > 0 {
			sel := m.statsPacks[m.statsPackCursor]
			m.loadPackDetailStats(sel.TopicID)
			m.screen = screenStatsPackDetail
		}
	}
	return m, nil
}

func (m model) updateStatsPackDetail(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	key := keyMsg.String()
	if isBackKey(key) {
		m.loadPackListStats()
		m.screen = screenStatsPackList
	}
	return m, nil
}

// sortPacksByAttempts sorts packs by AttemptsAnswered descending.
func sortPacksByAttempts(packs []ports.TopicSummary) {
	sort.Slice(packs, func(i, j int) bool {
		return packs[i].AttemptsAnswered > packs[j].AttemptsAnswered
	})
}
