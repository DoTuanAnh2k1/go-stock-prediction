package cron

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// Common cron schedule constants - No more headaches remembering syntax!
const (
	// Every X seconds
	EverySecond    = "* * * * * *"
	Every5Seconds  = "*/5 * * * * *"
	Every10Seconds = "*/10 * * * * *"
	Every15Seconds = "*/15 * * * * *"
	Every30Seconds = "*/30 * * * * *"

	// Every X minutes
	EveryMinute    = "0 * * * * *"
	Every2Minutes  = "0 */2 * * * *"
	Every5Minutes  = "0 */5 * * * *"
	Every10Minutes = "0 */10 * * * *"
	Every15Minutes = "0 */15 * * * *"
	Every30Minutes = "0 */30 * * * *"

	// Every X hours
	EveryHour    = "0 0 * * * *"
	Every2Hours  = "0 0 */2 * * *"
	Every3Hours  = "0 0 */3 * * *"
	Every6Hours  = "0 0 */6 * * *"
	Every12Hours = "0 0 */12 * * *"

	// Daily at specific times
	Daily6AM      = "0 0 6 * * *"
	Daily9AM      = "0 0 9 * * *"
	Daily10AM     = "0 0 10 * * *"
	Daily12PM     = "0 0 12 * * *"
	Daily6PM      = "0 0 18 * * *"
	Daily9PM      = "0 0 21 * * *"
	DailyMidnight = "0 0 0 * * *"

	// Weekly schedules
	WeeklyMondayAM = "0 0 9 * * MON"
	WeeklyFridayPM = "0 0 17 * * FRI"
	WeeklySundayAM = "0 0 9 * * SUN"

	// Monthly schedules
	MonthlyFirst = "0 0 9 1 * *"     // First day of month at 9AM
	MonthlyLast  = "0 0 9 28-31 * *" // Last few days of month at 9AM

	// Special occasions
	NewYearMidnight = "0 0 0 1 1 *" // New Year's Day midnight
)

// JobFunc represents the function type that will be executed by the cron job
type JobFunc func() error

// Job contains information about a scheduled job
type Job struct {
	ID       string
	Schedule string
	Func     JobFunc
	entryID  cron.EntryID
}

// CronManager manages all cron jobs
type CronManager struct {
	cron    *cron.Cron
	jobs    map[string]*Job
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
	running bool
}

// Global instance
var defaultManager *CronManager

// init initializes the default cron manager
func init() {
	defaultManager = NewCronManager()
}

// NewCronManager creates a new CronManager instance
func NewCronManager() *CronManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &CronManager{
		cron:   cron.New(cron.WithSeconds()),
		jobs:   make(map[string]*Job),
		ctx:    ctx,
		cancel: cancel,
	}
}

// AddJob adds a new job to the default cron scheduler
func AddJob(id, schedule string, jobFunc JobFunc) error {
	return defaultManager.AddJob(id, schedule, jobFunc)
}

// RemoveJob removes a job from the default cron scheduler
func RemoveJob(id string) error {
	return defaultManager.RemoveJob(id)
}

// Start begins executing all scheduled jobs
func Start() error {
	return defaultManager.Start()
}

// Stop halts all running jobs
func Stop() {
	defaultManager.Stop()
}

// GetJobs returns a list of all registered jobs
func GetJobs() []Job {
	return defaultManager.GetJobs()
}

// GetNextRun returns the next execution time for a specific job
func GetNextRun(id string) (time.Time, error) {
	return defaultManager.GetNextRun(id)
}

// IsRunning returns whether the cron manager is currently running
func IsRunning() bool {
	return defaultManager.IsRunning()
}

// AddJob adds a new job to the cron scheduler
func (cm *CronManager) AddJob(id, schedule string, jobFunc JobFunc) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, exists := cm.jobs[id]; exists {
		return fmt.Errorf("job with ID '%s' already exists", id)
	}

	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.DowOptional)
	if _, err := parser.Parse(schedule); err != nil {
		return fmt.Errorf("invalid schedule '%s': %v", schedule, err)
	}

	job := &Job{
		ID:       id,
		Schedule: schedule,
		Func:     jobFunc,
	}

	if cm.running {
		entryID, err := cm.cron.AddFunc(schedule, cm.wrapJobFunc(job))
		if err != nil {
			return fmt.Errorf("failed to add job '%s': %v", id, err)
		}
		job.entryID = entryID
	}

	cm.jobs[id] = job
	return nil
}

// RemoveJob removes a job from the cron scheduler
func (cm *CronManager) RemoveJob(id string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	job, exists := cm.jobs[id]
	if !exists {
		return fmt.Errorf("job '%s' does not exist", id)
	}

	if cm.running && job.entryID != 0 {
		cm.cron.Remove(job.entryID)
	}

	delete(cm.jobs, id)
	return nil
}

// Start begins executing all scheduled jobs
func (cm *CronManager) Start() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.running {
		return fmt.Errorf("cron manager is already running")
	}

	for _, job := range cm.jobs {
		entryID, err := cm.cron.AddFunc(job.Schedule, cm.wrapJobFunc(job))
		if err != nil {
			return fmt.Errorf("failed to start job '%s': %v", job.ID, err)
		}
		job.entryID = entryID
	}

	cm.cron.Start()
	cm.running = true
	return nil
}

// Stop halts all running jobs
func (cm *CronManager) Stop() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if !cm.running {
		return
	}

	ctx := cm.cron.Stop()
	<-ctx.Done()

	cm.running = false
	cm.cancel()
}

// GetJobs returns a list of all registered jobs
func (cm *CronManager) GetJobs() []Job {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	jobs := make([]Job, 0, len(cm.jobs))
	for _, job := range cm.jobs {
		jobs = append(jobs, *job)
	}

	return jobs
}

// GetNextRun returns the next execution time for a specific job
func (cm *CronManager) GetNextRun(id string) (time.Time, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	job, exists := cm.jobs[id]
	if !exists {
		return time.Time{}, fmt.Errorf("job '%s' does not exist", id)
	}

	if !cm.running || job.entryID == 0 {
		return time.Time{}, fmt.Errorf("job '%s' is not started", id)
	}

	entry := cm.cron.Entry(job.entryID)
	return entry.Next, nil
}

// wrapJobFunc wraps the job function with context support
func (cm *CronManager) wrapJobFunc(job *Job) func() {
	return func() {
		select {
		case <-cm.ctx.Done():
			return
		default:
		}

		job.Func()
	}
}

// IsRunning returns whether the cron manager is currently running
func (cm *CronManager) IsRunning() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.running
}
