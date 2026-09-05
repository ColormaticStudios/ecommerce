package backups

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type ManifestCollector struct {
	store               Store
	labels              prometheus.Labels
	lastRun             *prometheus.Desc
	lastSuccess         *prometheus.Desc
	drillLastRun        *prometheus.Desc
	drillLastSuccess    *prometheus.Desc
	drillRPO            *prometheus.Desc
	drillRTO            *prometheus.Desc
	collectionSucceeded *prometheus.Desc
}

func NewMetricsHandler(store Store, service, environment, owner string) http.Handler {
	registry := prometheus.NewRegistry()
	registry.MustRegister(newManifestCollector(store, prometheus.Labels{
		"service": service, "deployment_environment": environment, "owner": owner,
	}))
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{EnableOpenMetrics: true})
}

func newManifestCollector(store Store, labels prometheus.Labels) *ManifestCollector {
	return &ManifestCollector{
		store: store, labels: labels,
		lastRun:             prometheus.NewDesc("ecommerce_backup_last_run_timestamp_seconds", "Completion time of the latest backup run.", []string{"outcome", "failure_stage"}, labels),
		lastSuccess:         prometheus.NewDesc("ecommerce_backup_last_success_timestamp_seconds", "Completion time of the latest successful backup.", nil, labels),
		drillLastRun:        prometheus.NewDesc("ecommerce_restore_drill_last_run_timestamp_seconds", "Completion time of the latest restore drill.", []string{"outcome", "failure_stage"}, labels),
		drillLastSuccess:    prometheus.NewDesc("ecommerce_restore_drill_last_success_timestamp_seconds", "Completion time of the latest successful restore drill.", nil, labels),
		drillRPO:            prometheus.NewDesc("ecommerce_restore_drill_rpo_seconds", "Measured recovery point objective of the latest successful restore drill.", nil, labels),
		drillRTO:            prometheus.NewDesc("ecommerce_restore_drill_rto_seconds", "Measured recovery time objective of the latest successful restore drill.", nil, labels),
		collectionSucceeded: prometheus.NewDesc("ecommerce_backup_manifest_collection_success", "Whether backup manifests were collected successfully.", nil, labels),
	}
}

func (c *ManifestCollector) Describe(channel chan<- *prometheus.Desc) {
	for _, desc := range []*prometheus.Desc{c.lastRun, c.lastSuccess, c.drillLastRun, c.drillLastSuccess, c.drillRPO, c.drillRTO, c.collectionSucceeded} {
		channel <- desc
	}
}

func (c *ManifestCollector) Collect(channel chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	statuses, err := statusManifests(ctx, c.store)
	if err != nil {
		channel <- prometheus.MustNewConstMetric(c.collectionSucceeded, prometheus.GaugeValue, 0)
		return
	}
	lastBackup, hasBackup := latestFromStatus(statuses, OperationBackup)
	lastSuccess, successErr := latestSuccessfulFromStatus(statuses, OperationBackup)
	if successErr != nil && hasBackup && lastBackup.Outcome == OutcomeSucceeded {
		lastSuccess = lastBackup
		successErr = nil
	}
	if hasBackup {
		channel <- prometheus.MustNewConstMetric(c.lastRun, prometheus.GaugeValue, float64(lastBackup.CompletedAt.Unix()), lastBackup.Outcome, lastBackup.FailureStage)
	}
	if successErr == nil {
		channel <- prometheus.MustNewConstMetric(c.lastSuccess, prometheus.GaugeValue, float64(lastSuccess.CompletedAt.Unix()))
	} else {
		channel <- prometheus.MustNewConstMetric(c.lastSuccess, prometheus.GaugeValue, 0)
	}

	lastDrill, hasDrill := latestFromStatus(statuses, OperationRestoreDrill)
	lastSuccessfulDrill, successfulDrillErr := latestSuccessfulFromStatus(statuses, OperationRestoreDrill)
	if successfulDrillErr != nil && hasDrill && lastDrill.Outcome == OutcomeSucceeded {
		lastSuccessfulDrill = lastDrill
		successfulDrillErr = nil
	}
	if hasDrill {
		channel <- prometheus.MustNewConstMetric(c.drillLastRun, prometheus.GaugeValue, float64(lastDrill.CompletedAt.Unix()), lastDrill.Outcome, lastDrill.FailureStage)
	}
	if successfulDrillErr == nil {
		channel <- prometheus.MustNewConstMetric(c.drillLastSuccess, prometheus.GaugeValue, float64(lastSuccessfulDrill.CompletedAt.Unix()))
		channel <- prometheus.MustNewConstMetric(c.drillRPO, prometheus.GaugeValue, lastSuccessfulDrill.RPOSeconds)
		channel <- prometheus.MustNewConstMetric(c.drillRTO, prometheus.GaugeValue, lastSuccessfulDrill.RTOSeconds)
	} else {
		channel <- prometheus.MustNewConstMetric(c.drillLastSuccess, prometheus.GaugeValue, 0)
	}
	channel <- prometheus.MustNewConstMetric(c.collectionSucceeded, prometheus.GaugeValue, 1)
}
