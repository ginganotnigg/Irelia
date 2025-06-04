package features

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
	"go.uber.org/zap"

	"irelia/pkg/ent"
)

type QuestionPreparationJob struct {
	InterviewID    string
	UserID         uint64
	NextQuestionID int32
	Interview      *ent.Interview
	Questions      []*ent.Question
	EnqueuedAt     time.Time
}

type QuestionWorkerPool struct {
	jobQueue          chan QuestionPreparationJob
	workerCount       int
	maxTasksPerWorker int
	maxIdleTime       time.Duration
	maxTaskWaitTime   time.Duration
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup
	// Metrics
	totalJobsEnqueued  int64
	totalJobsProcessed int64
	totalJobsDropped   int64
	activeWorkers      int64
}

func NewQuestionWorkerPool(size, maxTasksPerWorker, maxIdleTime, maxTaskWaitTime int) *QuestionWorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	pool := &QuestionWorkerPool{
		jobQueue:          make(chan QuestionPreparationJob, size*maxTasksPerWorker),
		workerCount:       size,
		maxTasksPerWorker: maxTasksPerWorker,
		maxIdleTime:       time.Duration(maxIdleTime) * time.Second,
		maxTaskWaitTime:   time.Duration(maxTaskWaitTime) * time.Second,
		ctx:               ctx,
		cancel:            cancel,
	}

	return pool
}

func (wp *QuestionWorkerPool) Start(service *Irelia) {
	service.logger.Info("Starting question worker pool",
		zap.Int("workerCount", wp.workerCount),
		zap.Int("queueCapacity", cap(wp.jobQueue)),
		zap.Duration("maxIdleTime", wp.maxIdleTime))

	for i := 0; i < wp.workerCount; i++ {
		wp.wg.Add(1)
		go wp.worker(service, i)
	}
}

func (wp *QuestionWorkerPool) Stop() {
	wp.cancel()
	close(wp.jobQueue)
	wp.wg.Wait()
}

func (wp *QuestionWorkerPool) worker(service *Irelia, workerID int) {
	defer wp.wg.Done()
	atomic.AddInt64(&wp.activeWorkers, 1)
	defer atomic.AddInt64(&wp.activeWorkers, -1)

	idleTimer := time.NewTimer(wp.maxIdleTime)
	defer idleTimer.Stop()

	jobsProcessed := 0

	for {
		select {
		case job, ok := <-wp.jobQueue:
			if !ok {
				service.logger.Info("Worker stopping - job queue closed",
					zap.Int("workerID", workerID),
					zap.Int("jobsProcessed", jobsProcessed))
				return
			}

			// Calculate wait time in queue
			waitTime := time.Since(job.EnqueuedAt)
			service.logger.Debug("Worker processing job",
				zap.Int("workerID", workerID),
				zap.String("interviewID", job.InterviewID),
				zap.Int32("questionID", job.NextQuestionID),
				zap.Duration("waitTime", waitTime))

			// Process the job
			startTime := time.Now()
			service.prepareQuestionSafe(job)
			processingTime := time.Since(startTime)

			atomic.AddInt64(&wp.totalJobsProcessed, 1)
			jobsProcessed++

			service.logger.Debug("Worker completed job",
				zap.Int("workerID", workerID),
				zap.String("interviewID", job.InterviewID),
				zap.Int32("questionID", job.NextQuestionID),
				zap.Duration("processingTime", processingTime),
				zap.Duration("totalTime", time.Since(job.EnqueuedAt)))

			// Reset idle timer
			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			idleTimer.Reset(wp.maxIdleTime)

		case <-idleTimer.C:
			service.logger.Info("Worker idle timeout, exiting", zap.Int("workerID", workerID),
				zap.Int("jobsProcessed", jobsProcessed))
			return

		case <-wp.ctx.Done():
			service.logger.Info("Worker stopping - context cancelled", zap.Int("workerID", workerID),
				zap.Int("jobsProcessed", jobsProcessed))
			return
		}
	}
}

func (wp *QuestionWorkerPool) EnqueueJob(logger *zap.Logger, job QuestionPreparationJob) bool {
	job.EnqueuedAt = time.Now()
	logger.Info("Enqueuing question preparation job", zap.String("interviewID", job.InterviewID),
		zap.Int32("nextQuestionID", job.NextQuestionID))

	select {
	case wp.jobQueue <- job:
		atomic.AddInt64(&wp.totalJobsEnqueued, 1)
		logger.Debug("Successfully enqueued question preparation job", zap.String("interviewID", job.InterviewID),
			zap.Int32("nextQuestionID", job.NextQuestionID),
			zap.Int("queueSize", len(wp.jobQueue)),
			zap.Int("queueCapacity", cap(wp.jobQueue)))
		return true

	case <-time.After(wp.maxTaskWaitTime):
		atomic.AddInt64(&wp.totalJobsDropped, 1)
		logger.Error("Job enqueue timeout - queue may be full or workers unavailable", zap.String("interviewID", job.InterviewID),
			zap.Int32("nextQuestionID", job.NextQuestionID),
			zap.Duration("timeout", wp.maxTaskWaitTime),
			zap.Int("queueSize", len(wp.jobQueue)),
			zap.Int("queueCapacity", cap(wp.jobQueue)),
			zap.Int64("activeWorkers", atomic.LoadInt64(&wp.activeWorkers)))
		return false

	default:
		atomic.AddInt64(&wp.totalJobsDropped, 1)
		logger.Warn("Job queue is full, dropping job", zap.String("interviewID", job.InterviewID),
			zap.Int32("nextQuestionID", job.NextQuestionID),
			zap.Int("queueSize", len(wp.jobQueue)),
			zap.Int("queueCapacity", cap(wp.jobQueue)),
			zap.Int64("activeWorkers", atomic.LoadInt64(&wp.activeWorkers)))
		return false
	}
}

// GetMetrics returns worker pool metrics
func (wp *QuestionWorkerPool) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"total_jobs_enqueued":  atomic.LoadInt64(&wp.totalJobsEnqueued),
		"total_jobs_processed": atomic.LoadInt64(&wp.totalJobsProcessed),
		"total_jobs_dropped":   atomic.LoadInt64(&wp.totalJobsDropped),
		"active_workers":       atomic.LoadInt64(&wp.activeWorkers),
		"queue_size":           len(wp.jobQueue),
		"queue_capacity":       cap(wp.jobQueue),
	}
}

