// Package jobs provides the platform's durable background-job runtime.
package jobs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"ecommerce/internal/reliability"
	"ecommerce/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const maxPayloadBytes = 1024 * 1024

var (
	ErrNoRunnableJob       = errors.New("no runnable job")
	ErrIdempotencyConflict = errors.New("job idempotency conflict")
	ErrLeaseLost           = errors.New("job lease lost")
	ErrInvalidJob          = errors.New("invalid job")
	jobTypePattern         = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,127}$`)
)

type Handler func(context.Context, json.RawMessage) error

type DeadLetterHandler func(context.Context, *gorm.DB, json.RawMessage, error) error

type Registration struct {
	Handle       Handler
	OnDeadLetter DeadLetterHandler
}

// Observer receives bounded-cardinality runtime events for platform telemetry.
type Observer interface {
	WorkerStarted()
	WorkerStopped()
	JobClaimed(jobType string)
	AttemptCompleted(jobType, outcome, errorClass string, duration time.Duration)
}

type Config struct {
	WorkerConcurrency int
	PollInterval      time.Duration
	LeaseDuration     time.Duration
	MaxAttempts       int
	RetryBaseDelay    time.Duration
	RetryMaxDelay     time.Duration
	Logger            *slog.Logger
	Now               func() time.Time
	Jitter            func(time.Duration) time.Duration
	Observer          Observer
}

type EnqueueInput struct {
	JobType        string
	Payload        any
	IdempotencyKey string
	CorrelationID  string
	RunAt          time.Time
	MaxAttempts    int
}

type Runtime struct {
	db       *gorm.DB
	config   Config
	mu       sync.RWMutex
	claimMu  sync.Mutex
	handlers map[string]Registration
	wake     chan struct{}
	running  atomic.Int64
}

// Running reports whether at least one worker is currently polling or
// executing work. It is used by dependency-aware readiness checks.
func (r *Runtime) Running() bool {
	return r != nil && r.running.Load() > 0
}

func NewRuntime(db *gorm.DB, config Config) *Runtime {
	if config.WorkerConcurrency <= 0 {
		config.WorkerConcurrency = 4
	}
	if config.PollInterval <= 0 {
		config.PollInterval = time.Second
	}
	if config.LeaseDuration <= 0 {
		config.LeaseDuration = 2 * time.Minute
	}
	if config.MaxAttempts <= 0 {
		config.MaxAttempts = 5
	}
	if config.RetryBaseDelay <= 0 {
		config.RetryBaseDelay = 5 * time.Second
	}
	if config.RetryMaxDelay <= 0 {
		config.RetryMaxDelay = 15 * time.Minute
	}
	if config.RetryMaxDelay < config.RetryBaseDelay {
		config.RetryMaxDelay = config.RetryBaseDelay
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	if config.Now == nil {
		config.Now = func() time.Time { return time.Now().UTC() }
	}
	if config.Jitter == nil {
		config.Jitter = func(max time.Duration) time.Duration {
			if max <= 0 {
				return 0
			}
			return time.Duration(rand.Int64N(int64(max) + 1))
		}
	}
	return &Runtime{db: db, config: config, handlers: make(map[string]Registration), wake: make(chan struct{}, 1)}
}

func (r *Runtime) Register(jobType string, registration Registration) error {
	jobType = strings.TrimSpace(jobType)
	if !jobTypePattern.MatchString(jobType) || registration.Handle == nil {
		return fmt.Errorf("%w: valid job type and handler are required", ErrInvalidJob)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.handlers[jobType]; exists {
		return fmt.Errorf("%w: handler already registered for %s", ErrInvalidJob, jobType)
	}
	r.handlers[jobType] = registration
	return nil
}

func (r *Runtime) registration(jobType string) (Registration, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	registration, ok := r.handlers[jobType]
	return registration, ok
}

func (r *Runtime) Enqueue(ctx context.Context, input EnqueueInput) (models.JobQueue, error) {
	if r == nil || r.db == nil {
		return models.JobQueue{}, errors.New("job runtime database is required")
	}
	return r.EnqueueTx(ctx, r.db, input)
}

// EnqueueTx allows domain services to persist intent and the job in the same
// caller-owned transaction.
func (r *Runtime) EnqueueTx(ctx context.Context, tx *gorm.DB, input EnqueueInput) (models.JobQueue, error) {
	if r == nil || tx == nil {
		return models.JobQueue{}, errors.New("job runtime and transaction are required")
	}
	jobType := strings.TrimSpace(input.JobType)
	if !jobTypePattern.MatchString(jobType) {
		return models.JobQueue{}, fmt.Errorf("%w: invalid job type %q", ErrInvalidJob, input.JobType)
	}
	payload, err := canonicalPayload(input.Payload)
	if err != nil {
		return models.JobQueue{}, fmt.Errorf("%w: payload: %v", ErrInvalidJob, err)
	}
	if len(payload) > maxPayloadBytes {
		return models.JobQueue{}, fmt.Errorf("%w: payload exceeds %d bytes", ErrInvalidJob, maxPayloadBytes)
	}
	fingerprintBytes := sha256.Sum256(payload)
	fingerprint := hex.EncodeToString(fingerprintBytes[:])
	now := r.config.Now().UTC()
	runAt := input.RunAt.UTC()
	if input.RunAt.IsZero() {
		runAt = now
	}
	maxAttempts := input.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = r.config.MaxAttempts
	}
	if maxAttempts < 1 || maxAttempts > 100 {
		return models.JobQueue{}, fmt.Errorf("%w: max attempts must be between 1 and 100", ErrInvalidJob)
	}
	correlationID := strings.TrimSpace(input.CorrelationID)
	if correlationID == "" {
		correlationID = reliability.FromContext(ctx).CorrelationID
	}
	if correlationID != "" && reliability.ValidExternalID(correlationID) == "" {
		return models.JobQueue{}, fmt.Errorf("%w: invalid correlation ID", ErrInvalidJob)
	}

	var idempotencyKey *string
	if value := strings.TrimSpace(input.IdempotencyKey); value != "" {
		if len(value) > 255 {
			return models.JobQueue{}, fmt.Errorf("%w: idempotency key exceeds 255 bytes", ErrInvalidJob)
		}
		idempotencyKey = &value
		var existing models.JobQueue
		err := tx.WithContext(ctx).Where("job_type = ? AND idempotency_key = ?", jobType, value).First(&existing).Error
		if err == nil {
			return sameIdempotentJob(existing, fingerprint)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return models.JobQueue{}, err
		}
	}

	job := models.JobQueue{
		ID: uuid.NewString(), JobType: jobType, PayloadJSON: string(payload), PayloadFingerprint: fingerprint,
		Status: models.JobStatusPending, RunAt: runAt, IdempotencyKey: idempotencyKey,
		CorrelationID: correlationID, MaxAttempts: maxAttempts,
	}
	create := tx.WithContext(ctx)
	if idempotencyKey != nil {
		create = create.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "job_type"}, {Name: "idempotency_key"}},
			DoNothing: true,
		})
	}
	result := create.Create(&job)
	if result.Error != nil {
		return models.JobQueue{}, result.Error
	}
	if result.RowsAffected == 0 && idempotencyKey != nil {
		var existing models.JobQueue
		if err := tx.WithContext(ctx).Where("job_type = ? AND idempotency_key = ?", jobType, *idempotencyKey).First(&existing).Error; err != nil {
			return models.JobQueue{}, err
		}
		return sameIdempotentJob(existing, fingerprint)
	}
	r.notify()
	return job, nil
}

func sameIdempotentJob(existing models.JobQueue, fingerprint string) (models.JobQueue, error) {
	if existing.PayloadFingerprint != fingerprint {
		return models.JobQueue{}, fmt.Errorf("%w for %s/%s", ErrIdempotencyConflict, existing.JobType, valueOrEmpty(existing.IdempotencyKey))
	}
	return existing, nil
}

func canonicalPayload(payload any) ([]byte, error) {
	if payload == nil {
		return []byte("{}"), nil
	}
	if raw, ok := payload.(json.RawMessage); ok {
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		return json.Marshal(value)
	}
	return json.Marshal(payload)
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (r *Runtime) notify() {
	select {
	case r.wake <- struct{}{}:
	default:
	}
}
