package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/finset/app/internal/models"
)

func (p *Pool) migrateQuiz(ctx context.Context) error {
	stmts := []string{
		`
		CREATE TABLE IF NOT EXISTS students (
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL,
			group_name  TEXT NOT NULL DEFAULT '',
			name_key    TEXT NOT NULL,
			group_key   TEXT NOT NULL DEFAULT '',
			created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (name_key, group_key)
		)
		`,
		`
		CREATE TABLE IF NOT EXISTS quiz_attempts (
			id                TEXT PRIMARY KEY,
			student_id        TEXT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
			quiz_key          TEXT NOT NULL DEFAULT 'sql-practice',
			quiz_title        TEXT NOT NULL DEFAULT 'SQL Практика',
			student_name      TEXT NOT NULL,
			student_group     TEXT NOT NULL DEFAULT '',
			difficulty_filter TEXT NOT NULL DEFAULT 'all',
			total_questions   INT NOT NULL CHECK (total_questions >= 0),
			correct_count     INT NOT NULL CHECK (correct_count >= 0),
			wrong_count       INT NOT NULL CHECK (wrong_count >= 0),
			score             INT NOT NULL DEFAULT 0,
			score_percent     NUMERIC(5,2) NOT NULL DEFAULT 0,
			duration_ms       BIGINT NOT NULL DEFAULT 0,
			started_at        TIMESTAMPTZ NOT NULL,
			finished_at       TIMESTAMPTZ NOT NULL,
			created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
		`,
		`
		CREATE TABLE IF NOT EXISTS quiz_attempt_answers (
			id              BIGSERIAL PRIMARY KEY,
			attempt_id      TEXT NOT NULL REFERENCES quiz_attempts(id) ON DELETE CASCADE,
			question_id     TEXT NOT NULL,
			topic_index     INT NOT NULL DEFAULT 0,
			topic_name      TEXT NOT NULL DEFAULT '',
			difficulty      TEXT NOT NULL DEFAULT 'easy',
			question_type   TEXT NOT NULL DEFAULT 'mcq',
			is_correct      BOOLEAN NOT NULL DEFAULT FALSE,
			selected_answer TEXT NOT NULL DEFAULT '',
			correct_answer  TEXT NOT NULL DEFAULT '',
			time_spent_ms   BIGINT NOT NULL DEFAULT 0,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (attempt_id, question_id)
		)
		`,
		`ALTER TABLE quiz_attempts ADD COLUMN IF NOT EXISTS quiz_key TEXT NOT NULL DEFAULT 'sql-practice'`,
		`ALTER TABLE quiz_attempts ADD COLUMN IF NOT EXISTS quiz_title TEXT NOT NULL DEFAULT 'SQL Практика'`,
		`CREATE INDEX IF NOT EXISTS idx_quiz_attempts_student_finished ON quiz_attempts (student_id, finished_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_quiz_attempts_finished ON quiz_attempts (finished_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_quiz_answers_difficulty ON quiz_attempt_answers (difficulty)`,
		`CREATE INDEX IF NOT EXISTS idx_quiz_answers_question ON quiz_attempt_answers (question_id)`,
	}

	for _, stmt := range stmts {
		if _, err := p.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("quiz migration: %w", err)
		}
	}
	return nil
}

func normalizeQuizKey(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func quizScoreForDifficulty(difficulty string) int {
	switch difficulty {
	case "hard":
		return 20
	case "medium":
		return 15
	default:
		return 10
	}
}

func (p *Pool) SaveQuizAttempt(ctx context.Context, attemptID, studentID string, req models.SubmitQuizAttemptRequest) (*models.SavedQuizAttempt, error) {
	tx, err := p.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	nameKey := normalizeQuizKey(req.StudentName)
	groupKey := normalizeQuizKey(req.StudentGroup)

	if err := tx.QueryRow(ctx, `
		INSERT INTO students (id, name, group_name, name_key, group_key)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (name_key, group_key)
		DO UPDATE SET name = EXCLUDED.name, group_name = EXCLUDED.group_name
		RETURNING id
	`, studentID, req.StudentName, req.StudentGroup, nameKey, groupKey).Scan(&studentID); err != nil {
		return nil, fmt.Errorf("upsert student: %w", err)
	}

	totalQuestions := len(req.Answers)
	correctCount := 0
	score := 0
	for _, answer := range req.Answers {
		if answer.IsCorrect {
			correctCount++
			score += quizScoreForDifficulty(answer.Difficulty)
		}
	}
	wrongCount := totalQuestions - correctCount

	startedAt := req.StartedAt
	finishedAt := req.FinishedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	if finishedAt.IsZero() || finishedAt.Before(startedAt) {
		finishedAt = startedAt
	}
	durationMS := finishedAt.Sub(startedAt).Milliseconds()
	if durationMS < 0 {
		durationMS = 0
	}

	scorePercent := 0.0
	if totalQuestions > 0 {
		scorePercent = float64(correctCount) * 100 / float64(totalQuestions)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO quiz_attempts (
			id, student_id, quiz_key, quiz_title, student_name, student_group,
			difficulty_filter, total_questions, correct_count, wrong_count, score,
			score_percent, duration_ms, started_at, finished_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`, attemptID, studentID, req.QuizKey, req.QuizTitle, req.StudentName, req.StudentGroup, req.DifficultyFilter,
		totalQuestions, correctCount, wrongCount, score, scorePercent,
		durationMS, startedAt, finishedAt); err != nil {
		return nil, fmt.Errorf("insert attempt: %w", err)
	}

	for _, answer := range req.Answers {
		if _, err := tx.Exec(ctx, `
			INSERT INTO quiz_attempt_answers (
				attempt_id, question_id, topic_index, topic_name, difficulty,
				question_type, is_correct, selected_answer, correct_answer, time_spent_ms
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		`, attemptID, answer.QuestionID, answer.TopicIndex, answer.TopicName, answer.Difficulty,
			answer.QuestionType, answer.IsCorrect, answer.SelectedAnswer, answer.CorrectAnswer, answer.TimeSpentMS); err != nil {
			return nil, fmt.Errorf("insert answer %s: %w", answer.QuestionID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit attempt: %w", err)
	}

	return &models.SavedQuizAttempt{
		AttemptID:        attemptID,
		StudentID:        studentID,
		QuizKey:          req.QuizKey,
		QuizTitle:        req.QuizTitle,
		StudentName:      req.StudentName,
		StudentGroup:     req.StudentGroup,
		DifficultyFilter: req.DifficultyFilter,
		TotalQuestions:   totalQuestions,
		CorrectCount:     correctCount,
		WrongCount:       wrongCount,
		Score:            score,
		ScorePercent:     scorePercent,
		DurationMS:       durationMS,
		FinishedAt:       finishedAt,
	}, nil
}

func (p *Pool) GetQuizDashboard(ctx context.Context) (*models.QuizDashboard, error) {
	var dashboard models.QuizDashboard

	if err := p.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM students)::int,
			(SELECT COUNT(*) FROM quiz_attempts)::int,
			(SELECT COUNT(*) FROM quiz_attempt_answers)::int,
			COALESCE((SELECT AVG(score_percent)::float8 FROM quiz_attempts), 0),
			COALESCE((
				SELECT AVG(
					CASE WHEN total_questions > 0 THEN correct_count::float8 * 100 / total_questions ELSE 0 END
				)::float8
				FROM quiz_attempts
			), 0)
	`).Scan(
		&dashboard.Overview.StudentCount,
		&dashboard.Overview.AttemptCount,
		&dashboard.Overview.AnswerCount,
		&dashboard.Overview.AverageScore,
		&dashboard.Overview.AverageAccuracy,
	); err != nil {
		return nil, fmt.Errorf("overview: %w", err)
	}

	diffRows, err := p.Query(ctx, `
		SELECT
			difficulty,
			COUNT(*)::int AS answers,
			COALESCE(SUM(CASE WHEN is_correct THEN 1 ELSE 0 END), 0)::int AS correct,
			COALESCE(AVG(CASE WHEN is_correct THEN 100 ELSE 0 END)::float8, 0) AS accuracy,
			COUNT(DISTINCT attempt_id)::int AS attempts
		FROM quiz_attempt_answers
		GROUP BY difficulty
		ORDER BY CASE difficulty WHEN 'easy' THEN 1 WHEN 'medium' THEN 2 WHEN 'hard' THEN 3 ELSE 4 END
	`)
	if err != nil {
		return nil, fmt.Errorf("difficulty stats: %w", err)
	}
	defer diffRows.Close()
	for diffRows.Next() {
		var row models.DifficultyDashboardRow
		if err := diffRows.Scan(&row.Difficulty, &row.Answers, &row.Correct, &row.Accuracy, &row.Attempts); err != nil {
			return nil, fmt.Errorf("scan difficulty stats: %w", err)
		}
		dashboard.Difficulties = append(dashboard.Difficulties, row)
	}
	if err := diffRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate difficulty stats: %w", err)
	}

	topicRows, err := p.Query(ctx, `
		SELECT
			topic_index,
			COALESCE(topic_name, '') AS topic_name,
			COUNT(*)::int AS answers,
			COALESCE(SUM(CASE WHEN is_correct THEN 1 ELSE 0 END), 0)::int AS correct,
			COALESCE(AVG(CASE WHEN is_correct THEN 100 ELSE 0 END)::float8, 0) AS accuracy
		FROM quiz_attempt_answers
		GROUP BY topic_index, topic_name
		ORDER BY topic_index
	`)
	if err != nil {
		return nil, fmt.Errorf("topic stats: %w", err)
	}
	defer topicRows.Close()
	for topicRows.Next() {
		var row models.TopicDashboardRow
		if err := topicRows.Scan(&row.TopicIndex, &row.TopicName, &row.Answers, &row.Correct, &row.Accuracy); err != nil {
			return nil, fmt.Errorf("scan topic stats: %w", err)
		}
		dashboard.Topics = append(dashboard.Topics, row)
	}
	if err := topicRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate topic stats: %w", err)
	}

	studentRows, err := p.Query(ctx, `
		SELECT
			s.id,
			s.name,
			s.group_name,
			COUNT(a.id)::int AS attempts,
			COALESCE(MAX(a.score_percent)::float8, 0) AS best_score,
			COALESCE(AVG(a.score_percent)::float8, 0) AS average_accuracy,
			COALESCE(MAX(a.finished_at), s.created_at) AS last_finished_at
		FROM students s
		LEFT JOIN quiz_attempts a ON a.student_id = s.id
		GROUP BY s.id, s.name, s.group_name, s.created_at
		ORDER BY best_score DESC, average_accuracy DESC, s.name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("student stats: %w", err)
	}
	defer studentRows.Close()

	studentsByID := map[string]*models.StudentDashboardRow{}
	for studentRows.Next() {
		var row models.StudentDashboardRow
		if err := studentRows.Scan(&row.StudentID, &row.StudentName, &row.StudentGroup, &row.Attempts, &row.BestScore, &row.AverageAccuracy, &row.LastFinishedAt); err != nil {
			return nil, fmt.Errorf("scan student stats: %w", err)
		}
		row.Difficulties = []models.StudentDifficultyRow{}
		dashboard.Students = append(dashboard.Students, row)
		studentsByID[row.StudentID] = &dashboard.Students[len(dashboard.Students)-1]
	}
	if err := studentRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate student stats: %w", err)
	}

	studentDiffRows, err := p.Query(ctx, `
		SELECT
			a.student_id,
			qa.difficulty,
			COUNT(*)::int AS answers,
			COALESCE(SUM(CASE WHEN qa.is_correct THEN 1 ELSE 0 END), 0)::int AS correct,
			COALESCE(AVG(CASE WHEN qa.is_correct THEN 100 ELSE 0 END)::float8, 0) AS accuracy
		FROM quiz_attempt_answers qa
		JOIN quiz_attempts a ON a.id = qa.attempt_id
		GROUP BY a.student_id, qa.difficulty
		ORDER BY a.student_id, CASE qa.difficulty WHEN 'easy' THEN 1 WHEN 'medium' THEN 2 WHEN 'hard' THEN 3 ELSE 4 END
	`)
	if err != nil {
		return nil, fmt.Errorf("student difficulty stats: %w", err)
	}
	defer studentDiffRows.Close()
	for studentDiffRows.Next() {
		var studentID string
		var diff models.StudentDifficultyRow
		if err := studentDiffRows.Scan(&studentID, &diff.Difficulty, &diff.Answers, &diff.Correct, &diff.Accuracy); err != nil {
			return nil, fmt.Errorf("scan student difficulty stats: %w", err)
		}
		if student := studentsByID[studentID]; student != nil {
			student.Difficulties = append(student.Difficulties, diff)
		}
	}
	if err := studentDiffRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate student difficulty stats: %w", err)
	}

	recentRows, err := p.Query(ctx, `
		SELECT
			id,
			student_id,
			quiz_key,
			quiz_title,
			student_name,
			student_group,
			difficulty_filter,
			score_percent::float8,
			correct_count,
			total_questions,
			finished_at
		FROM quiz_attempts
		ORDER BY finished_at DESC
		LIMIT 25
	`)
	if err != nil {
		return nil, fmt.Errorf("recent attempts: %w", err)
	}
	defer recentRows.Close()
	for recentRows.Next() {
		var row models.RecentAttemptRow
		if err := recentRows.Scan(&row.AttemptID, &row.StudentID, &row.QuizKey, &row.QuizTitle, &row.StudentName, &row.StudentGroup, &row.DifficultyFilter, &row.ScorePercent, &row.CorrectCount, &row.TotalQuestions, &row.FinishedAt); err != nil {
			return nil, fmt.Errorf("scan recent attempts: %w", err)
		}
		dashboard.RecentAttempts = append(dashboard.RecentAttempts, row)
	}
	if err := recentRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent attempts: %w", err)
	}

	missRows, err := p.Query(ctx, `
		SELECT
			question_id,
			COALESCE(topic_name, '') AS topic_name,
			difficulty,
			COUNT(*)::int AS attempts,
			COALESCE(SUM(CASE WHEN is_correct THEN 1 ELSE 0 END), 0)::int AS correct,
			COALESCE(AVG(CASE WHEN is_correct THEN 100 ELSE 0 END)::float8, 0) AS accuracy
		FROM quiz_attempt_answers
		GROUP BY question_id, topic_name, difficulty
		HAVING COUNT(*) >= 1
		ORDER BY accuracy ASC, attempts DESC, question_id ASC
		LIMIT 12
	`)
	if err != nil {
		return nil, fmt.Errorf("question misses: %w", err)
	}
	defer missRows.Close()
	for missRows.Next() {
		var row models.QuestionInsightRow
		if err := missRows.Scan(&row.QuestionID, &row.TopicName, &row.Difficulty, &row.Attempts, &row.Correct, &row.Accuracy); err != nil {
			return nil, fmt.Errorf("scan question misses: %w", err)
		}
		dashboard.QuestionMisses = append(dashboard.QuestionMisses, row)
	}
	if err := missRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate question misses: %w", err)
	}

	return &dashboard, nil
}
