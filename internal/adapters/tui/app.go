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
	"database/sql"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dezeat/golearn/internal/adapters/localconfig"
	"github.com/dezeat/golearn/internal/adapters/sqlite"
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

// Run launches the TUI application. It owns the DB connection and
// creates all necessary repos and services.
func Run(db *sql.DB) error {
	return RunWithConfigPath(db, localconfig.DefaultPath())
}

// RunWithConfigPath launches the TUI using a specific config path.
// This exists so tests can avoid writing to the real home directory.
func RunWithConfigPath(db *sql.DB, configPath string) error {
	topicRepo := sqlite.NewTopicRepo(db)
	questionRepo := sqlite.NewQuestionRepo(db)
	sessionRepo := sqlite.NewSessionRepo(db)
	attemptRepo := sqlite.NewAttemptRepo(db)
	userRepo := sqlite.NewUserRepo(db)
	statsRepo := sqlite.NewStatsRepo(db)
	configStore := localconfig.NewStore(configPath)

	profiles, err := userRepo.List()
	if err != nil {
		return fmt.Errorf("load users: %w", err)
	}
	if len(profiles) == 0 {
		localProfile, createErr := userRepo.Create("local", "Local")
		if createErr != nil {
			return fmt.Errorf("seed local user: %w", createErr)
		}
		profiles = append(profiles, *localProfile)
	}

	cfg, err := configStore.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	currentUser, hasContinue, err := resolveCurrentUser(userRepo, cfg.CurrentUserID)
	if err != nil {
		return err
	}

	if !hasContinue {
		cfg.CurrentUserID = currentUser.ID
		if err := configStore.Save(cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
		hasContinue = true
	}

	userCtx := app.NewUserContext(currentUser.ID)
	m := newModel(topicRepo, questionRepo, sessionRepo, attemptRepo, userRepo, statsRepo, configStore, userCtx)
	m.currentUser = currentUser
	m.hasValidCurrentUser = hasContinue
	m.profiles = profiles

	if err := m.reloadTopicsForCurrentUser(); err != nil {
		return err
	}

	if len(m.topics) == 0 {
		return fmt.Errorf("no topics found — import a question pack first: golearn import <path>")
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

func resolveCurrentUser(userRepo ports.UserRepository, configuredUserID int64) (*domain.User, bool, error) {
	if configuredUserID > 0 {
		u, found, err := userRepo.GetByID(configuredUserID)
		if err != nil {
			return nil, false, fmt.Errorf("get configured user %d: %w", configuredUserID, err)
		}
		if found {
			return u, true, nil
		}
	}

	localUser, found, err := userRepo.GetByHandle("local")
	if err != nil {
		return nil, false, fmt.Errorf("get local user: %w", err)
	}
	if found {
		return localUser, false, nil
	}

	users, err := userRepo.List()
	if err != nil {
		return nil, false, fmt.Errorf("list users: %w", err)
	}
	if len(users) == 0 {
		return nil, false, fmt.Errorf("no users available")
	}
	return &users[0], false, nil
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
	configStore  *localconfig.Store
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
	homeMenuCursor      int
	hasLastWrongAnswers bool // whether last completed session had wrong answers

	// Topic select state
	topics      []topicInfo
	topicCursor int

	// Session config state
	selectedTopic topicInfo
	questionCount int // number of questions for the session

	// Session engine
	engine *app.SessionEngine

	// Question screen state
	currentQuestion *domain.Question
	questionNum     int // 1-based index
	totalQuestions  int
	choiceCursor    int             // which choice is highlighted
	selected        map[string]bool // toggled choices for multi_select
	submitted       bool            // whether answer has been submitted
	lastCorrect     bool            // result of last submission
	lastSkipped     bool

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
	reviewQueue  []wrongAnswer
	reviewCursor int

	// Stats state
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

	// Quit flag
	quitting bool
}

func newModel(
	topicRepo ports.TopicRepository,
	questionRepo ports.QuestionRepository,
	sessionRepo ports.SessionRepository,
	attemptRepo ports.AttemptRepository,
	userRepo ports.UserRepository,
	statsRepo ports.StatsRepository,
	configStore *localconfig.Store,
	userCtx app.CurrentUserProvider,
) model {
	return model{
		topicRepo:     topicRepo,
		questionRepo:  questionRepo,
		sessionRepo:   sessionRepo,
		attemptRepo:   attemptRepo,
		userRepo:      userRepo,
		statsRepo:     statsRepo,
		configStore:   configStore,
		userCtx:       userCtx,
		screen:        screenProfileMenu,
		questionCount: 10,
		selected:      make(map[string]bool),
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
	if err := m.configStore.Save(localconfig.Config{CurrentUserID: u.ID}); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	m.hasValidCurrentUser = true
	return nil
}

func (m model) profileMenuOptions() []string {
	options := make([]string, 0, 4)
	if m.hasValidCurrentUser && m.currentUser != nil {
		label := "Continue"
		if m.currentUser.DisplayName != "" {
			label = fmt.Sprintf("Continue (%s — %s)", m.currentUser.Handle, m.currentUser.DisplayName)
		} else {
			label = fmt.Sprintf("Continue (%s)", m.currentUser.Handle)
		}
		options = append(options, label)
	}
	options = append(options, "Login", "Register", "Quit")
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
