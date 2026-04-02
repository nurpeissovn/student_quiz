package models

import (
	"strings"
	"time"
)

type QuizAnswerInput struct {
	QuestionID     string `json:"question_id"`
	TopicIndex     int    `json:"topic_index"`
	TopicName      string `json:"topic_name"`
	Difficulty     string `json:"difficulty"`
	QuestionType   string `json:"question_type"`
	IsCorrect      bool   `json:"is_correct"`
	SelectedAnswer string `json:"selected_answer"`
	CorrectAnswer  string `json:"correct_answer"`
	TimeSpentMS    int64  `json:"time_spent_ms"`
}

type SubmitQuizAttemptRequest struct {
	QuizKey          string            `json:"quiz_key"`
	QuizTitle        string            `json:"quiz_title"`
	StudentName      string            `json:"student_name"`
	StudentGroup     string            `json:"student_group"`
	DifficultyFilter string            `json:"difficulty_filter"`
	TotalQuestions   int               `json:"total_questions"`
	StartedAt        time.Time         `json:"started_at"`
	FinishedAt       time.Time         `json:"finished_at"`
	Answers          []QuizAnswerInput `json:"answers"`
}

func (r *SubmitQuizAttemptRequest) Normalize() {
	r.QuizKey = strings.ToLower(strings.TrimSpace(r.QuizKey))
	r.QuizTitle = strings.TrimSpace(r.QuizTitle)
	r.StudentName = strings.TrimSpace(r.StudentName)
	r.StudentGroup = strings.TrimSpace(r.StudentGroup)
	r.DifficultyFilter = strings.ToLower(strings.TrimSpace(r.DifficultyFilter))
	for i := range r.Answers {
		r.Answers[i].QuestionID = strings.TrimSpace(r.Answers[i].QuestionID)
		r.Answers[i].TopicName = strings.TrimSpace(r.Answers[i].TopicName)
		r.Answers[i].Difficulty = strings.ToLower(strings.TrimSpace(r.Answers[i].Difficulty))
		r.Answers[i].QuestionType = strings.ToLower(strings.TrimSpace(r.Answers[i].QuestionType))
		r.Answers[i].SelectedAnswer = strings.TrimSpace(r.Answers[i].SelectedAnswer)
		r.Answers[i].CorrectAnswer = strings.TrimSpace(r.Answers[i].CorrectAnswer)
		if r.Answers[i].Difficulty == "" {
			r.Answers[i].Difficulty = "easy"
		}
	}
	if r.DifficultyFilter == "" {
		r.DifficultyFilter = "all"
	}
	if r.QuizKey == "" {
		r.QuizKey = "sql-practice"
	}
	if r.QuizTitle == "" {
		r.QuizTitle = "SQL Практика"
	}
	if r.TotalQuestions < 0 {
		r.TotalQuestions = 0
	}
}

func (r SubmitQuizAttemptRequest) Validate() string {
	if r.QuizKey == "" {
		return "quiz_key is required"
	}
	if r.QuizTitle == "" {
		return "quiz_title is required"
	}
	if r.StudentName == "" {
		return "student_name is required"
	}
	if len(r.Answers) == 0 {
		return "at least one answer is required"
	}
	if r.TotalQuestions > 0 && r.TotalQuestions < len(r.Answers) {
		return "total_questions cannot be smaller than answers length"
	}
	for _, a := range r.Answers {
		if a.QuestionID == "" {
			return "question_id is required for every answer"
		}
	}
	return ""
}

type SavedQuizAttempt struct {
	AttemptID        string    `json:"attempt_id"`
	StudentID        string    `json:"student_id"`
	QuizKey          string    `json:"quiz_key"`
	QuizTitle        string    `json:"quiz_title"`
	StudentName      string    `json:"student_name"`
	StudentGroup     string    `json:"student_group"`
	DifficultyFilter string    `json:"difficulty_filter"`
	TotalQuestions   int       `json:"total_questions"`
	CorrectCount     int       `json:"correct_count"`
	WrongCount       int       `json:"wrong_count"`
	Score            int       `json:"score"`
	ScorePercent     float64   `json:"score_percent"`
	DurationMS       int64     `json:"duration_ms"`
	FinishedAt       time.Time `json:"finished_at"`
}

type QuizOverview struct {
	StudentCount    int     `json:"student_count"`
	AttemptCount    int     `json:"attempt_count"`
	AnswerCount     int     `json:"answer_count"`
	AverageScore    float64 `json:"average_score"`
	AverageAccuracy float64 `json:"average_accuracy"`
}

type DifficultyDashboardRow struct {
	Difficulty string  `json:"difficulty"`
	Answers    int     `json:"answers"`
	Correct    int     `json:"correct"`
	Accuracy   float64 `json:"accuracy"`
	Attempts   int     `json:"attempts"`
}

type TopicDashboardRow struct {
	TopicIndex int     `json:"topic_index"`
	TopicName  string  `json:"topic_name"`
	Answers    int     `json:"answers"`
	Correct    int     `json:"correct"`
	Accuracy   float64 `json:"accuracy"`
}

type RecentAttemptRow struct {
	AttemptID        string    `json:"attempt_id"`
	StudentID        string    `json:"student_id"`
	QuizKey          string    `json:"quiz_key"`
	QuizTitle        string    `json:"quiz_title"`
	StudentName      string    `json:"student_name"`
	StudentGroup     string    `json:"student_group"`
	DifficultyFilter string    `json:"difficulty_filter"`
	ScorePercent     float64   `json:"score_percent"`
	CorrectCount     int       `json:"correct_count"`
	TotalQuestions   int       `json:"total_questions"`
	FinishedAt       time.Time `json:"finished_at"`
}

type StudentDifficultyRow struct {
	Difficulty string  `json:"difficulty"`
	Answers    int     `json:"answers"`
	Correct    int     `json:"correct"`
	Accuracy   float64 `json:"accuracy"`
}

type StudentDashboardRow struct {
	StudentID       string                 `json:"student_id"`
	StudentName     string                 `json:"student_name"`
	StudentGroup    string                 `json:"student_group"`
	Attempts        int                    `json:"attempts"`
	BestScore       float64                `json:"best_score"`
	AverageAccuracy float64                `json:"average_accuracy"`
	LastFinishedAt  time.Time              `json:"last_finished_at"`
	Difficulties    []StudentDifficultyRow `json:"difficulties"`
}

type QuestionInsightRow struct {
	QuestionID string  `json:"question_id"`
	TopicName  string  `json:"topic_name"`
	Difficulty string  `json:"difficulty"`
	Attempts   int     `json:"attempts"`
	Correct    int     `json:"correct"`
	Accuracy   float64 `json:"accuracy"`
}

type QuizDashboard struct {
	Overview       QuizOverview             `json:"overview"`
	Difficulties   []DifficultyDashboardRow `json:"difficulties"`
	Topics         []TopicDashboardRow      `json:"topics"`
	Students       []StudentDashboardRow    `json:"students"`
	RecentAttempts []RecentAttemptRow       `json:"recent_attempts"`
	QuestionMisses []QuestionInsightRow     `json:"question_misses"`
}
