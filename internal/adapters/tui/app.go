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

// Package tui implements the Bubble Tea terminal UI for golearn.
//
// The TUI provides an interactive experience for practising MCQs:
//   - ASCII intro splash screen
//   - Topic selection screen
//   - Session configuration screen
//   - Question answering screen with quiz-show feedback
//   - Session summary screen with review option
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dezeat/golearn/internal/app"
	"github.com/dezeat/golearn/internal/domain"
	"github.com/dezeat/golearn/internal/ports"
)

// Styles used across TUI screens.
var (
	styleCorrect   = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // green
	styleIncorrect = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))  // red
	styleSelected  = lipgloss.NewStyle().Foreground(lipgloss.Color("12")) // blue
	styleExplain   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))  // dim
	styleBold      = lipgloss.NewStyle().Bold(true)
	styleHeader    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14")) // cyan
	styleDim       = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

// RunParams holds all dependencies and initial state needed to launch the TUI.
// The caller (cmd/golearn) is responsible for constructing repos, resolving
// the current user, and injecting everything here.
type RunParams struct {
	TopicRepo    ports.TopicRepository
	QuestionRepo ports.QuestionRepository
	SessionRepo  ports.SessionRepository
	AttemptRepo  ports.AttemptRepository
	UserRepo     ports.UserRepository
	StatsRepo    ports.StatsRepository
	ConfigStore  ports.ConfigStore
	UserCtx      app.CurrentUserProvider
	CurrentUser  *domain.User
	HasContinue  bool
	Profiles     []domain.User
}

// Run launches the TUI application with the provided dependencies.
func Run(p RunParams) error {
	m := newModel(
		p.TopicRepo, p.QuestionRepo, p.SessionRepo, p.AttemptRepo,
		p.UserRepo, p.StatsRepo, p.ConfigStore, p.UserCtx,
	)
	m.currentUser = p.CurrentUser
	m.hasValidCurrentUser = p.HasContinue
	m.profiles = p.Profiles

	if err := m.reloadTopicsForCurrentUser(); err != nil {
		return err
	}

	if len(m.topics) == 0 {
		return fmt.Errorf("no topics found — import a question pack first: golearn import <path>")
	}

	prog := tea.NewProgram(m, tea.WithAltScreen())
	_, err := prog.Run()
	return err
}

// topicInfo extends a Topic with display metadata.
type topicInfo struct {
	Topic         domain.Topic
	QuestionCount int
	TotalAttempts int
	TotalCorrect  int
}

// wrongAnswer pairs a question with the user's selected choices
// for display in the review browse screen.
type wrongAnswer struct {
	Question    domain.Question
	SelectedIDs []string
}

// screen represents which screen is currently active.
type screen int

const (
	screenProfileMenu     screen = iota // profile menu with continue/login/register
	screenProfileLogin                  // profile picker
	screenProfileRegister               // profile registration form
	screenHomeMenu                      // post-login home menu
	screenTopicSelect                   // topic selection
	screenSessionConfig                 // session configuration
	screenQuestion                      // answering a question
	screenReview                        // quiz-show review mode (replaces screenFeedback)
	screenReviewBrowse                  // browse-only review of wrong answers
	screenSummary                       // session summary
	screenStatsMenu                     // stats menu: global / by pack / back
	screenStatsGlobal                   // global stats overview
	screenStatsPackList                 // per-pack stats list
	screenStatsPackDetail               // single pack detail stats
)

// model is the root Bubble Tea model that holds all TUI state.
type model struct {
	// Dependencies
	topicRepo    ports.TopicRepository
	questionRepo ports.QuestionRepository
	sessionRepo  ports.SessionRepository
	attemptRepo  ports.AttemptRepository
	userRepo     ports.UserRepository
	statsRepo    ports.StatsRepository
	configStore  ports.ConfigStore
	userCtx      app.CurrentUserProvider

	// Current screen
	screen screen

	// Profile menu state
	profiles            []domain.User
	currentUser         *domain.User
	hasValidCurrentUser bool
	profileMenuCursor   int
	profileLoginCursor  int
	profileError        string

	// Registration state
	registerHandle      string
	registerDisplayName string
	registerField       int

	// Home menu state
	homeMenuCursor int

	// Topic select state
	topics      []topicInfo
	topicCursor int

	// Session config state
	selectedTopic      topicInfo
	questionCount      int // number of questions for the session
	sessionMode        app.SelectionMode
	sessionModeCursor  int    // cursor within mode picker
	sessionDifficulty  string // "easy", "medium", "hard"
	sessionDiffCursor  int
	sessionWeakestSub  app.WeakestSubMode
	sessionWeakCursor  int
	sessionConfigField int // 0=questions, 1=mode, 2=sub-option (difficulty/weakest)

	// Session engine
	engine *app.SessionEngine

	// Question screen state
	currentQuestion        *app.SessionQuestion
	questionNum            int // 1-based index
	totalQuestions         int
	choiceCursor           int             // which choice is highlighted
	selected               map[string]bool // toggled choices for multi_select
	displayLabelByChoiceID map[string]string
	choiceIDByDisplayLabel map[string]string
	submitted              bool // whether answer has been submitted
	lastCorrect            bool // result of last submission
	lastSkipped            bool
	sessionModeLabel       string // e.g. "Mode: Balanced"
	sessionModeNote        string // informational note from selection

	// Review mode state
	showExplanations bool // toggled by 'e' in review mode

	// Summary state
	answered      int
	correctCount  int
	totalLatency  int // cumulative latency in milliseconds
	summaryCursor int // for summary menu navigation

	// Track wrong answers for review browse
	wrongAnswers []wrongAnswer

	// Review browse state
	reviewQueue        []wrongAnswer
	reviewCursor       int
	reviewReturnScreen screen

	// Stats state
	statsMenuCursor  int
	statsGlobal      *ports.GlobalStats
	statsGlobalTrend []float64
	statsPacks       []ports.TopicSummary
	statsPackCursor  int
	statsDetail      *ports.TopicSummary
	statsDifficulty  []ports.DifficultyStat
	statsWeakTags    []ports.TagStat
	statsStrongTags  []ports.TagStat
	statsWeakQs      []ports.QuestionWeakStat
	statsDetailTrend []float64
	statsError       string

	// Window size
	width  int
	height int

	// Error display
	lastError string

	// Quit flag
	quitting bool
}

// defaultQuestionCount is the initial number of questions per session in the TUI.
const defaultQuestionCount = 10

func newModel(
	topicRepo ports.TopicRepository,
	questionRepo ports.QuestionRepository,
	sessionRepo ports.SessionRepository,
	attemptRepo ports.AttemptRepository,
	userRepo ports.UserRepository,
	statsRepo ports.StatsRepository,
	configStore ports.ConfigStore,
	userCtx app.CurrentUserProvider,
) model {
	return model{
		topicRepo:         topicRepo,
		questionRepo:      questionRepo,
		sessionRepo:       sessionRepo,
		attemptRepo:       attemptRepo,
		userRepo:          userRepo,
		statsRepo:         statsRepo,
		configStore:       configStore,
		userCtx:           userCtx,
		screen:            screenProfileMenu,
		questionCount:     defaultQuestionCount,
		sessionMode:       app.ModeBalanced,
		sessionDifficulty: "easy",
		sessionWeakestSub: app.WeakestByQuestion,
		selected:          make(map[string]bool),
	}
}

func (m *model) reloadTopicsForCurrentUser() error {
	if m.userCtx == nil || m.userCtx.CurrentUserID() <= 0 {
		return fmt.Errorf("current user is not set")
	}

	topics, err := m.topicRepo.List()
	if err != nil {
		return fmt.Errorf("load topics: %w", err)
	}

	topicInfos := make([]topicInfo, 0, len(topics))
	for _, t := range topics {
		qs, err := m.questionRepo.ListByTopic(t.ID)
		if err != nil {
			return fmt.Errorf("list questions for topic %q: %w", t.Slug, err)
		}
		stats, err := m.attemptRepo.StatsByTopic(m.userCtx.CurrentUserID(), t.ID)
		if err != nil {
			return fmt.Errorf("load stats for topic %q: %w", t.Slug, err)
		}

		totalAttempts := 0
		totalCorrect := 0
		for _, s := range stats {
			totalAttempts += s.Attempts
			totalCorrect += s.Attempts - s.Wrong
		}

		topicInfos = append(topicInfos, topicInfo{
			Topic:         t,
			QuestionCount: len(qs),
			TotalAttempts: totalAttempts,
			TotalCorrect:  totalCorrect,
		})
	}

	m.topics = topicInfos
	if m.topicCursor >= len(m.topics) {
		m.topicCursor = 0
	}
	return nil
}

func (m *model) setCurrentUser(u *domain.User, saveConfig bool) error {
	m.currentUser = u
	m.userCtx.SetCurrentUserID(u.ID)
	if !saveConfig {
		return nil
	}
	if err := m.configStore.Save(ports.LocalConfig{CurrentUserID: u.ID}); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	m.hasValidCurrentUser = true
	return nil
}

func (m model) profileMenuOptions() []string {
	options := make([]string, 0, 3)
	if m.hasValidCurrentUser && m.currentUser != nil {
		var label string
		if m.currentUser.DisplayName != "" {
			label = fmt.Sprintf("Continue (%s — %s)", m.currentUser.Handle, m.currentUser.DisplayName)
		} else {
			label = fmt.Sprintf("Continue (%s)", m.currentUser.Handle)
		}
		options = append(options, label)
	}
	options = append(options, "Login", "Register")
	return options
}

func isValidHandle(handle string) bool {
	if handle == "" {
		return false
	}
	for _, r := range handle {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

func displayProfile(u domain.User) string {
	if strings.TrimSpace(u.DisplayName) == "" {
		return u.Handle
	}
	return fmt.Sprintf("%s — %s", u.Handle, u.DisplayName)
}
