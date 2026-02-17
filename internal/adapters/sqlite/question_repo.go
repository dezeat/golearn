package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dezeat/golearn/internal/domain"
	"github.com/dezeat/golearn/internal/ports"
)

// QuestionRepo implements ports.QuestionRepository using SQLite.
type QuestionRepo struct {
	db *sql.DB
}

// NewQuestionRepo creates a QuestionRepo.
func NewQuestionRepo(db *sql.DB) *QuestionRepo {
	return &QuestionRepo{db: db}
}

// InsertMany inserts questions, skipping duplicates (by hash UNIQUE constraint).
func (r *QuestionRepo) InsertMany(questions []domain.Question) (*ports.InsertResult, error) {
	result := &ports.InsertResult{}

	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO questions (
		topic_id, type, intro, prompt,
		choices_json, correct_choice_ids_json, tags_json,
		difficulty, rationale_correct, rationale_per_choice_json,
		source, source_ref, confidence, hash, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return nil, fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	for _, q := range questions {
		choicesJSON, err := json.Marshal(q.Choices)
		if err != nil {
			return nil, fmt.Errorf("marshal choices: %w", err)
		}
		correctJSON, err := json.Marshal(q.CorrectChoiceIDs)
		if err != nil {
			return nil, fmt.Errorf("marshal correct_choice_ids: %w", err)
		}
		tagsJSON, err := json.Marshal(q.Tags)
		if err != nil {
			return nil, fmt.Errorf("marshal tags: %w", err)
		}
		rationalePerChoiceJSON, err := json.Marshal(q.Rationale.PerChoice)
		if err != nil {
			return nil, fmt.Errorf("marshal rationale_per_choice: %w", err)
		}

		createdAt := q.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now().UTC()
		}

		res, err := stmt.Exec(
			q.TopicID, string(q.Type), q.Intro, q.Prompt,
			string(choicesJSON), string(correctJSON), string(tagsJSON),
			string(q.Difficulty), q.Rationale.Correct, string(rationalePerChoiceJSON),
			q.Source, q.SourceRef, q.Confidence, q.Hash, createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("insert question (hash=%s): %w", q.Hash, err)
		}
		affected, _ := res.RowsAffected()
		if affected > 0 {
			result.Inserted++
		} else {
			result.Duplicates++
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return result, nil
}

// ListByTopic returns all questions for a given topic, ordered by created_at.
func (r *QuestionRepo) ListByTopic(topicID int64) ([]domain.Question, error) {
	rows, err := r.db.Query(`SELECT
		id, topic_id, type, intro, prompt,
		choices_json, correct_choice_ids_json, tags_json,
		difficulty, rationale_correct, rationale_per_choice_json,
		source, source_ref, confidence, hash, created_at
	FROM questions WHERE topic_id = ? ORDER BY created_at ASC`, topicID)
	if err != nil {
		return nil, fmt.Errorf("list questions by topic %d: %w", topicID, err)
	}
	defer rows.Close()

	var questions []domain.Question
	for rows.Next() {
		var q domain.Question
		var qtype string
		var diffStr string
		var choicesJSON, correctJSON, tagsJSON, rationalePerChoiceJSON string
		if err := rows.Scan(
			&q.ID, &q.TopicID, &qtype, &q.Intro, &q.Prompt,
			&choicesJSON, &correctJSON, &tagsJSON,
			&diffStr, &q.Rationale.Correct, &rationalePerChoiceJSON,
			&q.Source, &q.SourceRef, &q.Confidence, &q.Hash, &q.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan question: %w", err)
		}
		q.Type = domain.QuestionType(qtype)
		q.Difficulty = domain.Difficulty(diffStr)
		if err := json.Unmarshal([]byte(choicesJSON), &q.Choices); err != nil {
			return nil, fmt.Errorf("unmarshal choices: %w", err)
		}
		if err := json.Unmarshal([]byte(correctJSON), &q.CorrectChoiceIDs); err != nil {
			return nil, fmt.Errorf("unmarshal correct_choice_ids: %w", err)
		}
		if err := json.Unmarshal([]byte(tagsJSON), &q.Tags); err != nil {
			return nil, fmt.Errorf("unmarshal tags: %w", err)
		}
		if err := json.Unmarshal([]byte(rationalePerChoiceJSON), &q.Rationale.PerChoice); err != nil {
			return nil, fmt.Errorf("unmarshal rationale_per_choice: %w", err)
		}
		questions = append(questions, q)
	}
	return questions, rows.Err()
}
