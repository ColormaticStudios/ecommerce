// Package telemetry owns metrics, tracing, health endpoints, and the
// dedicated observability listener.
package telemetry

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"ecommerce/models"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/gorm"
)

type Metrics struct {
	registry           *prometheus.Registry
	labels             prometheus.Labels
	httpRequests       *prometheus.CounterVec
	httpDuration       *prometheus.HistogramVec
	dbQueries          *prometheus.CounterVec
	dbDuration         *prometheus.HistogramVec
	jobClaims          *prometheus.CounterVec
	jobAttempts        *prometheus.CounterVec
	jobAttemptDuration *prometheus.HistogramVec
	jobWorkers         prometheus.Gauge
	providerCalls      *prometheus.CounterVec
	providerDuration   *prometheus.HistogramVec
}

func NewMetrics(service, environment, owner string) *Metrics {
	registry := prometheus.NewRegistry()
	registerer := prometheus.WrapRegistererWith(prometheus.Labels{
		"service": service, "deployment_environment": environment, "owner": normalizedOwner(owner),
	}, registry)
	labels := prometheus.Labels{"service": service, "deployment_environment": environment, "owner": normalizedOwner(owner)}
	metrics := &Metrics{
		registry: registry,
		labels:   labels,
		httpRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "ecommerce", Subsystem: "http", Name: "requests_total",
			Help: "Completed API requests.",
		}, []string{"method", "route", "status_class"}),
		httpDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "ecommerce", Subsystem: "http", Name: "request_duration_seconds",
			Help:    "End-to-end API request duration.",
			Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		}, []string{"method", "route", "status_class"}),
		dbQueries: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "ecommerce", Subsystem: "db", Name: "queries_total",
			Help: "Database queries observed by the GORM logger.",
		}, []string{"outcome"}),
		dbDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "ecommerce", Subsystem: "db", Name: "query_duration_seconds",
			Help:    "Database query duration observed by the GORM logger.",
			Buckets: []float64{0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		}, []string{"outcome"}),
		jobClaims: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "ecommerce", Subsystem: "jobs", Name: "claims_total",
			Help: "Successfully claimed durable jobs.",
		}, []string{"job_type"}),
		jobAttempts: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "ecommerce", Subsystem: "jobs", Name: "attempts_total",
			Help: "Completed durable-job attempts.",
		}, []string{"job_type", "outcome", "error_class"}),
		jobAttemptDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "ecommerce", Subsystem: "jobs", Name: "attempt_duration_seconds",
			Help:    "Durable-job attempt duration.",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 15, 30, 60, 300},
		}, []string{"job_type", "outcome"}),
		jobWorkers: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "ecommerce", Subsystem: "jobs", Name: "workers",
			Help: "Currently running durable-job workers.",
		}),
		providerCalls: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "ecommerce", Subsystem: "provider", Name: "calls_total",
			Help: "Completed provider calls.",
		}, []string{"provider_type", "operation", "outcome"}),
		providerDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "ecommerce", Subsystem: "provider", Name: "call_duration_seconds",
			Help:    "Provider call duration.",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 15, 30, 60},
		}, []string{"provider_type", "operation", "outcome"}),
	}
	registerer.MustRegister(
		metrics.httpRequests, metrics.httpDuration, metrics.dbQueries, metrics.dbDuration,
		metrics.jobClaims, metrics.jobAttempts, metrics.jobAttemptDuration, metrics.jobWorkers,
		metrics.providerCalls, metrics.providerDuration,
	)
	registry.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return metrics
}

func (m *Metrics) ProviderCallCompleted(providerType, operation, outcome string, duration time.Duration) {
	if m == nil {
		return
	}
	providerType = boundedProviderType(providerType)
	operation = boundedProviderOperation(operation)
	if outcome != "succeeded" && outcome != "failed" {
		outcome = "unknown"
	}
	m.providerCalls.WithLabelValues(providerType, operation, outcome).Inc()
	m.providerDuration.WithLabelValues(providerType, operation, outcome).Observe(duration.Seconds())
}

func boundedProviderType(value string) string {
	switch value {
	case "payment", "shipping", "tax":
		return value
	default:
		return "unknown"
	}
}

func boundedProviderOperation(value string) string {
	switch value {
	case "authorize", "capture", "void", "refund", "get_operation_outcome", "get_transaction", "quote_rates", "buy_label", "cancel_label", "get_shipment", "quote_tax", "finalize_tax", "cancel_finalization", "export_report":
		return value
	default:
		return "unknown"
	}
}

func normalizedOwner(owner string) string {
	if owner == "" {
		return "unassigned"
	}
	return owner
}

func (m *Metrics) RegisterDatabase(db *gorm.DB, pool *sql.DB) error {
	if m == nil {
		return nil
	}
	return m.registry.Register(newDatabaseCollector(db, pool, m.labels))
}

func (m *Metrics) Handler() http.Handler {
	if m == nil {
		return http.NotFoundHandler()
	}
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{EnableOpenMetrics: true})
}

func (m *Metrics) ObserveHTTPRequest(method, route string, status int, duration time.Duration) {
	if m == nil {
		return
	}
	statusClass := strconv.Itoa(status/100) + "xx"
	m.httpRequests.WithLabelValues(method, route, statusClass).Inc()
	m.httpDuration.WithLabelValues(method, route, statusClass).Observe(duration.Seconds())
}

func (m *Metrics) ObserveDBQuery(duration time.Duration, err error) {
	if m == nil {
		return
	}
	outcome := "success"
	if err != nil && err != gorm.ErrRecordNotFound {
		outcome = "error"
	}
	m.dbQueries.WithLabelValues(outcome).Inc()
	m.dbDuration.WithLabelValues(outcome).Observe(duration.Seconds())
}

func (m *Metrics) WorkerStarted() {
	if m != nil {
		m.jobWorkers.Inc()
	}
}
func (m *Metrics) WorkerStopped() {
	if m != nil {
		m.jobWorkers.Dec()
	}
}

func (m *Metrics) JobClaimed(jobType string) {
	if m != nil {
		m.jobClaims.WithLabelValues(jobType).Inc()
	}
}

func (m *Metrics) AttemptCompleted(jobType, outcome, errorClass string, duration time.Duration) {
	if m == nil {
		return
	}
	m.jobAttempts.WithLabelValues(jobType, outcome, errorClass).Inc()
	m.jobAttemptDuration.WithLabelValues(jobType, outcome).Observe(duration.Seconds())
}

type databaseCollector struct {
	db                *gorm.DB
	pool              *sql.DB
	queueDepth        *prometheus.Desc
	oldestRunnableAge *prometheus.Desc
	deadLetters       *prometheus.Desc
	collectorSuccess  *prometheus.Desc
	poolMaxOpen       *prometheus.Desc
	poolOpen          *prometheus.Desc
	poolInUse         *prometheus.Desc
	poolIdle          *prometheus.Desc
	poolWaitTotal     *prometheus.Desc
	poolWaitSeconds   *prometheus.Desc
}

func newDatabaseCollector(db *gorm.DB, pool *sql.DB, labels prometheus.Labels) *databaseCollector {
	return &databaseCollector{
		db: db, pool: pool,
		queueDepth:        prometheus.NewDesc("ecommerce_jobs_queue_depth", "Durable jobs by type and state.", []string{"job_type", "status"}, labels),
		oldestRunnableAge: prometheus.NewDesc("ecommerce_jobs_oldest_runnable_age_seconds", "Age of the oldest due runnable job.", []string{"job_type"}, labels),
		deadLetters:       prometheus.NewDesc("ecommerce_jobs_dead_letters", "Durable dead-letter records by job type.", []string{"job_type"}, labels),
		collectorSuccess:  prometheus.NewDesc("ecommerce_jobs_collector_success", "Whether the most recent durable-job collection succeeded.", nil, labels),
		poolMaxOpen:       prometheus.NewDesc("ecommerce_db_pool_max_open_connections", "Maximum configured open database connections.", nil, labels),
		poolOpen:          prometheus.NewDesc("ecommerce_db_pool_open_connections", "Open database connections.", nil, labels),
		poolInUse:         prometheus.NewDesc("ecommerce_db_pool_in_use_connections", "Database connections currently in use.", nil, labels),
		poolIdle:          prometheus.NewDesc("ecommerce_db_pool_idle_connections", "Idle database connections.", nil, labels),
		poolWaitTotal:     prometheus.NewDesc("ecommerce_db_pool_wait_total", "Connection-pool waits.", nil, labels),
		poolWaitSeconds:   prometheus.NewDesc("ecommerce_db_pool_wait_duration_seconds_total", "Total time blocked waiting for a connection.", nil, labels),
	}
}

func (c *databaseCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range []*prometheus.Desc{c.queueDepth, c.oldestRunnableAge, c.deadLetters, c.collectorSuccess, c.poolMaxOpen, c.poolOpen, c.poolInUse, c.poolIdle, c.poolWaitTotal, c.poolWaitSeconds} {
		ch <- desc
	}
}

func (c *databaseCollector) Collect(ch chan<- prometheus.Metric) {
	stats := c.pool.Stats()
	ch <- prometheus.MustNewConstMetric(c.poolMaxOpen, prometheus.GaugeValue, float64(stats.MaxOpenConnections))
	ch <- prometheus.MustNewConstMetric(c.poolOpen, prometheus.GaugeValue, float64(stats.OpenConnections))
	ch <- prometheus.MustNewConstMetric(c.poolInUse, prometheus.GaugeValue, float64(stats.InUse))
	ch <- prometheus.MustNewConstMetric(c.poolIdle, prometheus.GaugeValue, float64(stats.Idle))
	ch <- prometheus.MustNewConstMetric(c.poolWaitTotal, prometheus.CounterValue, float64(stats.WaitCount))
	ch <- prometheus.MustNewConstMetric(c.poolWaitSeconds, prometheus.CounterValue, stats.WaitDuration.Seconds())

	type countRow struct {
		JobType, Status string
		Count           int64
	}
	now := time.Now().UTC()
	var queue []countRow
	if err := c.db.Model(&models.JobQueue{}).Select("job_type, status, COUNT(*) AS count").Group("job_type, status").Scan(&queue).Error; err != nil {
		ch <- prometheus.MustNewConstMetric(c.collectorSuccess, prometheus.GaugeValue, 0)
		return
	}

	type runnableRow struct {
		JobType        string
		Status         string
		RunAt          time.Time
		LeaseExpiresAt *time.Time
	}
	var runnable []runnableRow
	if err := c.db.Model(&models.JobQueue{}).
		Select("job_type, status, run_at, lease_expires_at").
		Where(
			"(status IN ? AND run_at <= ?) OR (status = ? AND lease_expires_at IS NOT NULL AND lease_expires_at <= ?)",
			[]string{models.JobStatusPending, models.JobStatusRetryScheduled}, now, models.JobStatusRunning, now,
		).
		Scan(&runnable).Error; err != nil {
		ch <- prometheus.MustNewConstMetric(c.collectorSuccess, prometheus.GaugeValue, 0)
		return
	}
	expiredRunning := make(map[string]int64)
	oldest := make(map[string]time.Time)
	for _, row := range runnable {
		runnableAt := row.RunAt
		if row.Status == models.JobStatusRunning && row.LeaseExpiresAt != nil {
			runnableAt = *row.LeaseExpiresAt
			expiredRunning[row.JobType]++
		}
		if current, exists := oldest[row.JobType]; !exists || runnableAt.Before(current) {
			oldest[row.JobType] = runnableAt
		}
	}
	for _, row := range queue {
		count := row.Count
		if row.Status == models.JobStatusRunning {
			count -= expiredRunning[row.JobType]
		}
		if count > 0 {
			ch <- prometheus.MustNewConstMetric(c.queueDepth, prometheus.GaugeValue, float64(count), row.JobType, row.Status)
		}
	}
	for jobType, count := range expiredRunning {
		ch <- prometheus.MustNewConstMetric(c.queueDepth, prometheus.GaugeValue, float64(count), jobType, "expired_running")
	}
	for jobType, runAt := range oldest {
		age := now.Sub(runAt).Seconds()
		if age < 0 {
			age = 0
		}
		ch <- prometheus.MustNewConstMetric(c.oldestRunnableAge, prometheus.GaugeValue, age, jobType)
	}

	var deadLetters []countRow
	if err := c.db.Model(&models.JobDeadLetter{}).Select("job_type, COUNT(*) AS count").Group("job_type").Scan(&deadLetters).Error; err != nil {
		ch <- prometheus.MustNewConstMetric(c.collectorSuccess, prometheus.GaugeValue, 0)
		return
	}
	for _, row := range deadLetters {
		ch <- prometheus.MustNewConstMetric(c.deadLetters, prometheus.GaugeValue, float64(row.Count), row.JobType)
	}
	ch <- prometheus.MustNewConstMetric(c.collectorSuccess, prometheus.GaugeValue, 1)
}
