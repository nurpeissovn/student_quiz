package handlers

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/finset/app/internal/models"
)

type memoryQuizStore struct {
	students       map[string]*memoryStudent
	attempts       []memoryAttempt
	attemptAnswers []memoryAttemptAnswer
}

type memoryStudent struct {
	ID        string
	Name      string
	Group     string
	NameKey   string
	GroupKey  string
	CreatedAt time.Time
}

type memoryAttempt struct {
	models.SavedQuizAttempt
	StartedAt time.Time
}

type memoryAttemptAnswer struct {
	AttemptID string
	models.QuizAnswerInput
}

func newMemoryQuizStore() *memoryQuizStore {
	return &memoryQuizStore{
		students:       map[string]*memoryStudent{},
		attempts:       []memoryAttempt{},
		attemptAnswers: []memoryAttemptAnswer{},
	}
}

func normalizeMemoryQuizKey(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func memoryQuizScoreForDifficulty(difficulty string) int {
	switch difficulty {
	case "hard":
		return 20
	case "medium":
		return 15
	default:
		return 10
	}
}

func (h *Handler) saveQuizAttemptMemory(attemptID, studentID string, req models.SubmitQuizAttemptRequest) *models.SavedQuizAttempt {
	h.quizMu.Lock()
	defer h.quizMu.Unlock()

	store := h.quizStore
	if store == nil {
		store = newMemoryQuizStore()
		h.quizStore = store
	}

	nameKey := normalizeMemoryQuizKey(req.StudentName)
	groupKey := normalizeMemoryQuizKey(req.StudentGroup)
	studentKey := nameKey + "\x00" + groupKey

	now := time.Now().UTC()
	student, ok := store.students[studentKey]
	if !ok {
		student = &memoryStudent{
			ID:        studentID,
			Name:      req.StudentName,
			Group:     req.StudentGroup,
			NameKey:   nameKey,
			GroupKey:  groupKey,
			CreatedAt: now,
		}
		store.students[studentKey] = student
	} else {
		student.Name = req.StudentName
		student.Group = req.StudentGroup
	}

	totalQuestions := len(req.Answers)
	correctCount := 0
	score := 0
	for _, answer := range req.Answers {
		if answer.IsCorrect {
			correctCount++
			score += memoryQuizScoreForDifficulty(answer.Difficulty)
		}
	}
	wrongCount := totalQuestions - correctCount

	startedAt := req.StartedAt
	finishedAt := req.FinishedAt
	if startedAt.IsZero() {
		startedAt = now
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

	saved := models.SavedQuizAttempt{
		AttemptID:        attemptID,
		StudentID:        student.ID,
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
	}

	store.attempts = append(store.attempts, memoryAttempt{
		SavedQuizAttempt: saved,
		StartedAt:        startedAt,
	})
	for _, answer := range req.Answers {
		store.attemptAnswers = append(store.attemptAnswers, memoryAttemptAnswer{
			AttemptID:       attemptID,
			QuizAnswerInput: answer,
		})
	}

	return &saved
}

func (h *Handler) getQuizDashboardMemory() *models.QuizDashboard {
	h.quizMu.RLock()
	defer h.quizMu.RUnlock()

	store := h.quizStore
	if store == nil {
		return &models.QuizDashboard{}
	}

	dashboard := &models.QuizDashboard{}
	dashboard.Overview.StudentCount = len(store.students)
	dashboard.Overview.AttemptCount = len(store.attempts)
	dashboard.Overview.AnswerCount = len(store.attemptAnswers)

	if len(store.attempts) > 0 {
		var scoreSum float64
		for _, attempt := range store.attempts {
			scoreSum += attempt.ScorePercent
		}
		dashboard.Overview.AverageScore = scoreSum / float64(len(store.attempts))
		dashboard.Overview.AverageAccuracy = dashboard.Overview.AverageScore
	}

	diffStats := map[string]*models.DifficultyDashboardRow{}
	topicStats := map[string]*models.TopicDashboardRow{}
	diffAttempts := map[string]map[string]struct{}{}
	missStats := map[string]*models.QuestionInsightRow{}

	type studentAccumulator struct {
		row        models.StudentDashboardRow
		scoreTotal float64
		diffStats  map[string]*models.StudentDifficultyRow
	}
	studentStats := map[string]*studentAccumulator{}

	for _, attempt := range store.attempts {
		acc, ok := studentStats[attempt.StudentID]
		if !ok {
			acc = &studentAccumulator{
				row: models.StudentDashboardRow{
					StudentID:      attempt.StudentID,
					StudentName:    attempt.StudentName,
					StudentGroup:   attempt.StudentGroup,
					LastFinishedAt: attempt.FinishedAt,
					Difficulties:   []models.StudentDifficultyRow{},
				},
				diffStats: map[string]*models.StudentDifficultyRow{},
			}
			studentStats[attempt.StudentID] = acc
		}
		acc.row.Attempts++
		acc.scoreTotal += attempt.ScorePercent
		if attempt.ScorePercent > acc.row.BestScore {
			acc.row.BestScore = attempt.ScorePercent
		}
		if attempt.FinishedAt.After(acc.row.LastFinishedAt) {
			acc.row.LastFinishedAt = attempt.FinishedAt
		}

		dashboard.RecentAttempts = append(dashboard.RecentAttempts, models.RecentAttemptRow{
			AttemptID:        attempt.AttemptID,
			StudentID:        attempt.StudentID,
			StudentName:      attempt.StudentName,
			StudentGroup:     attempt.StudentGroup,
			DifficultyFilter: attempt.DifficultyFilter,
			ScorePercent:     attempt.ScorePercent,
			CorrectCount:     attempt.CorrectCount,
			TotalQuestions:   attempt.TotalQuestions,
			FinishedAt:       attempt.FinishedAt,
		})
	}

	attemptToStudent := map[string]string{}
	for _, attempt := range store.attempts {
		attemptToStudent[attempt.AttemptID] = attempt.StudentID
	}

	for _, answer := range store.attemptAnswers {
		diff := answer.Difficulty
		if diff == "" {
			diff = "easy"
		}
		diffRow, ok := diffStats[diff]
		if !ok {
			diffRow = &models.DifficultyDashboardRow{Difficulty: diff}
			diffStats[diff] = diffRow
		}
		diffRow.Answers++
		if answer.IsCorrect {
			diffRow.Correct++
		}
		if diffAttempts[diff] == nil {
			diffAttempts[diff] = map[string]struct{}{}
		}
		diffAttempts[diff][answer.AttemptID] = struct{}{}

		topicKey := fmt.Sprintf("%d\x00%s", answer.TopicIndex, answer.TopicName)
		topicRow, ok := topicStats[topicKey]
		if !ok {
			topicRow = &models.TopicDashboardRow{
				TopicIndex: answer.TopicIndex,
				TopicName:  answer.TopicName,
			}
			topicStats[topicKey] = topicRow
		}
		topicRow.Answers++
		if answer.IsCorrect {
			topicRow.Correct++
		}

		missKey := answer.QuestionID + "\x00" + answer.TopicName + "\x00" + diff
		missRow, ok := missStats[missKey]
		if !ok {
			missRow = &models.QuestionInsightRow{
				QuestionID: answer.QuestionID,
				TopicName:  answer.TopicName,
				Difficulty: diff,
			}
			missStats[missKey] = missRow
		}
		missRow.Attempts++
		if answer.IsCorrect {
			missRow.Correct++
		}

		studentID := attemptToStudent[answer.AttemptID]
		if acc := studentStats[studentID]; acc != nil {
			diffRow, ok := acc.diffStats[diff]
			if !ok {
				diffRow = &models.StudentDifficultyRow{Difficulty: diff}
				acc.diffStats[diff] = diffRow
			}
			diffRow.Answers++
			if answer.IsCorrect {
				diffRow.Correct++
			}
		}
	}

	for _, row := range diffStats {
		if row.Answers > 0 {
			row.Accuracy = float64(row.Correct) * 100 / float64(row.Answers)
		}
		row.Attempts = len(diffAttempts[row.Difficulty])
		dashboard.Difficulties = append(dashboard.Difficulties, *row)
	}
	sort.Slice(dashboard.Difficulties, func(i, j int) bool {
		order := map[string]int{"easy": 1, "medium": 2, "hard": 3}
		li := order[dashboard.Difficulties[i].Difficulty]
		lj := order[dashboard.Difficulties[j].Difficulty]
		if li == lj {
			return dashboard.Difficulties[i].Difficulty < dashboard.Difficulties[j].Difficulty
		}
		return li < lj
	})

	for _, row := range topicStats {
		if row.Answers > 0 {
			row.Accuracy = float64(row.Correct) * 100 / float64(row.Answers)
		}
		dashboard.Topics = append(dashboard.Topics, *row)
	}
	sort.Slice(dashboard.Topics, func(i, j int) bool {
		if dashboard.Topics[i].TopicIndex == dashboard.Topics[j].TopicIndex {
			return dashboard.Topics[i].TopicName < dashboard.Topics[j].TopicName
		}
		return dashboard.Topics[i].TopicIndex < dashboard.Topics[j].TopicIndex
	})

	for _, acc := range studentStats {
		if acc.row.Attempts > 0 {
			acc.row.AverageAccuracy = acc.scoreTotal / float64(acc.row.Attempts)
		}
		for _, diff := range acc.diffStats {
			if diff.Answers > 0 {
				diff.Accuracy = float64(diff.Correct) * 100 / float64(diff.Answers)
			}
			acc.row.Difficulties = append(acc.row.Difficulties, *diff)
		}
		sort.Slice(acc.row.Difficulties, func(i, j int) bool {
			order := map[string]int{"easy": 1, "medium": 2, "hard": 3}
			return order[acc.row.Difficulties[i].Difficulty] < order[acc.row.Difficulties[j].Difficulty]
		})
		dashboard.Students = append(dashboard.Students, acc.row)
	}
	sort.Slice(dashboard.Students, func(i, j int) bool {
		if dashboard.Students[i].BestScore == dashboard.Students[j].BestScore {
			if dashboard.Students[i].AverageAccuracy == dashboard.Students[j].AverageAccuracy {
				return dashboard.Students[i].StudentName < dashboard.Students[j].StudentName
			}
			return dashboard.Students[i].AverageAccuracy > dashboard.Students[j].AverageAccuracy
		}
		return dashboard.Students[i].BestScore > dashboard.Students[j].BestScore
	})

	sort.Slice(dashboard.RecentAttempts, func(i, j int) bool {
		return dashboard.RecentAttempts[i].FinishedAt.After(dashboard.RecentAttempts[j].FinishedAt)
	})
	if len(dashboard.RecentAttempts) > 25 {
		dashboard.RecentAttempts = dashboard.RecentAttempts[:25]
	}

	for _, row := range missStats {
		if row.Attempts > 0 {
			row.Accuracy = float64(row.Correct) * 100 / float64(row.Attempts)
		}
		dashboard.QuestionMisses = append(dashboard.QuestionMisses, *row)
	}
	sort.Slice(dashboard.QuestionMisses, func(i, j int) bool {
		if dashboard.QuestionMisses[i].Accuracy == dashboard.QuestionMisses[j].Accuracy {
			if dashboard.QuestionMisses[i].Attempts == dashboard.QuestionMisses[j].Attempts {
				return dashboard.QuestionMisses[i].QuestionID < dashboard.QuestionMisses[j].QuestionID
			}
			return dashboard.QuestionMisses[i].Attempts > dashboard.QuestionMisses[j].Attempts
		}
		return dashboard.QuestionMisses[i].Accuracy < dashboard.QuestionMisses[j].Accuracy
	})
	if len(dashboard.QuestionMisses) > 12 {
		dashboard.QuestionMisses = dashboard.QuestionMisses[:12]
	}

	return dashboard
}
