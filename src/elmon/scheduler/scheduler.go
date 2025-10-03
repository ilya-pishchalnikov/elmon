package scheduler

import (
	"context"
	"elmon/logger"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// TaskFunc now accepts interface{}, making the scheduler universal
type TaskFunc func(ctx context.Context, taskPayload interface{}) error

type TaskScheduler struct {
	Interval   time.Duration
	MaxRetries int
	RetryDelay time.Duration
	Task       TaskFunc
	Payload    interface{} // Task payload
	Logger     *logger.Logger

	// Fields for atomic ID generation and tracking
	taskIDCounter     uint64 // Atomically incremented counter for unique task IDs
	currentTaskID     uint64 // ID of the currently running task, protected by mutex

	ticker            *time.Ticker
	stopChan          chan struct{} // Used to signal the main runLoop to stop
	isRunning         bool
	isDisabled        bool
	mutex             sync.Mutex // Protected state fields
	currentTaskCancel context.CancelFunc // Used to abort the currently running task
}

// NewTaskScheduler creates and returns a new TaskScheduler instance
// It requires an initialized slog.Logger instance
func NewTaskScheduler(interval time.Duration, maxRetries int, retryDelay time.Duration, task TaskFunc, payload interface{}, logger *logger.Logger) *TaskScheduler {
	return &TaskScheduler{
		Interval:   interval,
		MaxRetries: maxRetries,
		RetryDelay: retryDelay,
		Task:       task,
		Payload:    payload,
		Logger:     logger,
		stopChan:   make(chan struct{}),
	}
}

// --- State Management Methods ---

// Start initiates the periodic execution of the task
// It runs in a separate goroutine
func (taskScheduler *TaskScheduler) Start() error {
	taskScheduler.mutex.Lock()
	defer taskScheduler.mutex.Unlock()

	if taskScheduler.isRunning {
		return fmt.Errorf("scheduler is already running")
	}
	if taskScheduler.Task == nil {
		return fmt.Errorf("task function is nil")
	}

	taskScheduler.isRunning = true

	if taskScheduler.Interval <= 0 {
		err := fmt.Errorf("invalid task scheduler interval %s", taskScheduler.Interval.String())
		taskScheduler.Logger.Error(err, "Error while start scheduler")
		return err
	}

	taskScheduler.ticker = time.NewTicker(taskScheduler.Interval)

	go taskScheduler.runLoop()

	taskScheduler.Logger.Info("TaskScheduler started",
		"interval", taskScheduler.Interval,
		"max_retries", taskScheduler.MaxRetries,
		"retry_delay", taskScheduler.RetryDelay)

	return nil
}

// Stop gracefully stops the periodic scheduling
func (taskScheduler *TaskScheduler) Stop() {
	taskScheduler.mutex.Lock()
	defer taskScheduler.mutex.Unlock()

	if !taskScheduler.isRunning {
		return
	}

	taskScheduler.Logger.Info("TaskScheduler received stop signal.")

	// Stop the ticker
	if taskScheduler.ticker != nil {
		taskScheduler.ticker.Stop()
	}

	// Abort current task before stopping the loop, if any is running
	if taskScheduler.currentTaskCancel != nil {
		taskScheduler.currentTaskCancel()
		taskScheduler.currentTaskCancel = nil
		taskScheduler.Logger.Warn("TaskScheduler aborted currently running task during stop.")
	}

	// Signal the runLoop to exit
	close(taskScheduler.stopChan)
	taskScheduler.isRunning = false
	taskScheduler.stopChan = make(chan struct{}) // Re-initialize for potential future Start
}

// DisableNextExecution prevents the next scheduled run
func (taskScheduler *TaskScheduler) DisableNextExecution() {
	taskScheduler.mutex.Lock()
	defer taskScheduler.mutex.Unlock()
	taskScheduler.isDisabled = true
	taskScheduler.Logger.Info("TaskScheduler: Next execution disabled.")
}

// EnableExecution re-enables the scheduled run
func (taskScheduler *TaskScheduler) EnableExecution() {
	taskScheduler.mutex.Lock()
	defer taskScheduler.mutex.Unlock()
	taskScheduler.isDisabled = false
	taskScheduler.Logger.Info("TaskScheduler: Execution re-enabled.")
}

// AbortCurrentExecution attempts to cancel the currently running task
func (taskScheduler *TaskScheduler) AbortCurrentExecution() {
	taskScheduler.mutex.Lock()
	defer taskScheduler.mutex.Unlock()

	if taskScheduler.currentTaskCancel != nil {
		taskScheduler.Logger.Warn("TaskScheduler: Aborting current task...", "task_id", taskScheduler.currentTaskID)
		taskScheduler.currentTaskCancel()
		// taskID will be cleared by the task goroutine's defer
	} else {
		taskScheduler.Logger.Debug("TaskScheduler: No current task to abort.")
	}
}

// --- Execution Logic ---

// runLoop is the main goroutine that manages the periodic scheduling
func (taskScheduler *TaskScheduler) runLoop() {
	taskScheduler.Logger.Info("TaskScheduler: Run loop started.")
	for {
		select {
		case <-taskScheduler.stopChan:
			taskScheduler.Logger.Info("TaskScheduler: Run loop gracefully stopped.")
			return
		case <-taskScheduler.ticker.C:
			taskScheduler.mutex.Lock()
			isDisabled := taskScheduler.isDisabled
			// Reset disable flag immediately after checking to ensure it only affects one run
			taskScheduler.isDisabled = false
			taskScheduler.mutex.Unlock()

			if isDisabled {
				taskScheduler.Logger.Info("TaskScheduler: Execution skipped due to DisableNextExecution flag.")
				continue
			}

			// Generate a unique ID for this task cycle
			newTaskID := atomic.AddUint64(&taskScheduler.taskIDCounter, 1)

			taskCtx, taskCancel := context.WithCancel(context.Background())

			// Store the cancel function AND the task ID in the struct
			taskScheduler.mutex.Lock()
			taskScheduler.currentTaskCancel = taskCancel
			taskScheduler.currentTaskID = newTaskID
			taskScheduler.mutex.Unlock()

			go taskScheduler.executeTaskWithRetries(taskCtx, taskCancel, newTaskID) // Pass ID to task
		}
	}
}

// executeTaskWithRetries runs the task function with retry logic
func (taskScheduler *TaskScheduler) executeTaskWithRetries(ctx context.Context, cancelFunc context.CancelFunc, taskID uint64) {
	// Ensure the cancel function is cleared when this execution finishes, regardless of how it exits
	defer func() {
		cancelFunc() // Always call cancel to release context resources
		taskScheduler.mutex.Lock()
		// Only clear the reference if it is still pointing to *this* task's cancel function
		if taskScheduler.currentTaskID == taskID {
			taskScheduler.currentTaskCancel = nil
			taskScheduler.currentTaskID = 0 // Clear the ID as well
		}
		taskScheduler.mutex.Unlock()
	}()

	taskScheduler.Logger.Debug("Task: Execution cycle started.")

	for attempt := 0; attempt <= taskScheduler.MaxRetries; attempt++ {
		// Check for context cancellation (e.g., from AbortCurrentExecution or Stop)
		if ctx.Err() != nil {
			taskScheduler.Logger.Warn("Task: Aborted due to context cancellation",
				"attempt", attempt+1,
				"error", ctx.Err())
			return
		}

		err := taskScheduler.Task(ctx, taskScheduler.Payload)

		if err == nil {
			taskScheduler.Logger.Info("Task: Completed successfully.")
			return
		}

		taskScheduler.Logger.Error(err, "Task: Failed and requires retry",
			"attempt", attempt+1,
			"max_attempts", taskScheduler.MaxRetries+1,
			"error", err)

		if attempt < taskScheduler.MaxRetries {
			// Wait for retry delay or be canceled
			select {
			case <-time.After(taskScheduler.RetryDelay):
				// Wait finished, proceed to next retry
			case <-ctx.Done():
				taskScheduler.Logger.Warn("Task: Aborted during retry delay wait",
					"error", ctx.Err())
				return
			}
		}
	}

	taskScheduler.Logger.Error(fmt.Errorf("task: Failed permanently after all attempts"), "Scheduler task failed",
		"max_attempts", taskScheduler.MaxRetries+1)
}