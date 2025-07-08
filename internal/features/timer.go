package features

import (
	"context"
	"fmt"
	"sync"
	"time"
	
	"go.uber.org/zap"
)

// QuestionTimer represents a timer for a specific question
type QuestionTimer struct {
	InterviewID   string
	QuestionIndex int32
	UserID        uint64
	StartTime     time.Time
	CancelFunc    context.CancelFunc
	Done          chan struct{}
}

// QuestionTimerManager manages all question timers
type QuestionTimerManager struct {
	timers  sync.Map // key: "interviewID:questionIndex", value: *QuestionTimer
	logger  *zap.Logger
	timeout time.Duration
}

// NewQuestionTimerManager creates a new timer manager
func NewQuestionTimerManager(logger *zap.Logger, timeout time.Duration) *QuestionTimerManager {
	return &QuestionTimerManager{
		logger:  logger,
		timeout: timeout,
	}
}

// startTimer starts a new timer for a question
func (qtm *QuestionTimerManager) startTimer(interviewID string, questionIndex int32, userID uint64, onTimeout func(string, int32, uint64)) {
	timerKey := fmt.Sprintf("%s:%d", interviewID, questionIndex)
	
	// Cancel any existing timer for this question
	qtm.cancelTimer(timerKey)
	
	// Create new timer context
	ctx, cancel := context.WithCancel(context.Background())
	
	timer := &QuestionTimer{
		InterviewID:   interviewID,
		QuestionIndex: questionIndex,
		UserID:        userID,
		StartTime:     time.Now(),
		CancelFunc:    cancel,
		Done:          make(chan struct{}),
	}
	
	// Store the timer
	qtm.timers.Store(timerKey, timer)
	
	// Start the timeout goroutine
	go qtm.runTimer(ctx, timer, onTimeout)
}

// cancelTimer cancels a timer if it exists
func (qtm *QuestionTimerManager) cancelTimer(timerKey string) bool {
	if val, ok := qtm.timers.LoadAndDelete(timerKey); ok {
		if timer, ok := val.(*QuestionTimer); ok {
			timer.CancelFunc()
			// Wait for goroutine to finish (with timeout to prevent blocking)
			select {
			case <-timer.Done:
				qtm.logger.Debug("Timer cancelled successfully", zap.String("timerKey", timerKey))
			case <-time.After(100 * time.Millisecond):
				qtm.logger.Warn("Timer cancellation timeout", zap.String("timerKey", timerKey))
			}
			return true
		}
	}
	return false
}

// runTimer executes the timer logic
func (qtm *QuestionTimerManager) runTimer(ctx context.Context, timer *QuestionTimer, onTimeout func(string, int32, uint64)) {
	defer close(timer.Done)
	
	timerKey := fmt.Sprintf("%s:%d", timer.InterviewID, timer.QuestionIndex)
	
	select {
	case <-time.After(qtm.timeout):
		// Timeout reached - check if timer still exists (wasn't cancelled)
		if _, exists := qtm.timers.Load(timerKey); exists {
			qtm.logger.Info("Question timeout reached", 
				zap.String("interviewID", timer.InterviewID),
				zap.Int32("questionIndex", timer.QuestionIndex),
				zap.Uint64("userID", timer.UserID))
			
			// Call the timeout handler
			onTimeout(timer.InterviewID, timer.QuestionIndex, timer.UserID)
			
			// Clean up
			qtm.timers.Delete(timerKey)
		}
	case <-ctx.Done():
		// Timer was cancelled
		qtm.logger.Debug("Timer cancelled", 
			zap.String("interviewID", timer.InterviewID),
			zap.Int32("questionIndex", timer.QuestionIndex))
	}
}

// getRemainingTime returns the remaining time for a question
func (qtm *QuestionTimerManager) getRemainingTime(interviewID string, questionIndex int32) time.Duration {
	timerKey := fmt.Sprintf("%s:%d", interviewID, questionIndex)
	if val, ok := qtm.timers.Load(timerKey); ok {
		if timer, ok := val.(*QuestionTimer); ok {
			elapsed := time.Since(timer.StartTime)
			remaining := qtm.timeout - elapsed
			if remaining < 0 {
				return 0
			}
			return remaining
		}
	}
	return 0
}

// cleanupInterviewTimers removes all timers for a specific interview
func (qtm *QuestionTimerManager) cleanupInterviewTimers(interviewID string) {
	qtm.timers.Range(func(key, value interface{}) bool {
		if timer, ok := value.(*QuestionTimer); ok {
			if timer.InterviewID == interviewID {
				qtm.cancelTimer(key.(string))
			}
		}
		return true
	})
}

// Shutdown cancels all timers and cleans up
func (qtm *QuestionTimerManager) shutdown() {
	qtm.logger.Info("Shutting down question timer manager")
	qtm.timers.Range(func(key, value interface{}) bool {
		if timer, ok := value.(*QuestionTimer); ok {
			timer.CancelFunc()
		}
		return true
	})
}