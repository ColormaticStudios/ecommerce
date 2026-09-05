package operability

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validIncident = `
schema_version: 1
id: inc-2026-001
kind: incident
title: Database outage
severity: sev2
status: closed
environment: production
services: [ecommerce-api]
incident_commander: commerce-on-call
started_at: 2026-09-05T01:00:00Z
detected_at: 2026-09-05T01:02:00Z
recovered_at: 2026-09-05T01:12:00Z
closed_at: 2026-09-05T03:00:00Z
detection_source: EcommerceDatabaseErrors
customer_impact: Checkout requests failed.
root_cause: Database networking failed.
contributing_factors: [No redundant network path.]
lessons: [Exercise dependency failover quarterly.]
timeline:
  - at: 2026-09-05T01:00:00Z
    event: Failures began.
  - at: 2026-09-05T01:12:00Z
    event: Service recovered.
actions:
  - id: action-1
    description: Add redundant routing.
    owner: platform
    due_date: 2026-09-30
    status: open
`

func TestIncidentDecodeValidationAndSummary(t *testing.T) {
	record, err := DecodeIncident(strings.NewReader(validIncident))
	require.NoError(t, err)
	summary, err := SummarizeIncidents([]IncidentRecord{record})
	require.NoError(t, err)
	assert.Equal(t, float64(120), summary.MeanMTTDSeconds)
	assert.Equal(t, float64(720), summary.MeanMTTRSeconds)
	assert.Equal(t, 1, summary.OpenActionCount)
	require.Len(t, summary.Quarterly, 1)
	assert.Equal(t, "2026-Q3", summary.Quarterly[0].Quarter)
}

func TestIncidentRejectsProductionDrillAndIncompleteSev2(t *testing.T) {
	record, err := DecodeIncident(strings.NewReader(validIncident))
	require.NoError(t, err)
	record.Kind = "drill"
	record.Scenario = "db_unavailable"
	require.ErrorContains(t, record.Validate(), "must not target production")
	record.Kind = "incident"
	record.Lessons = nil
	require.ErrorContains(t, record.Validate(), "require lessons")
}

func TestIncidentRequiresChronologyAndCompletedActionEvidence(t *testing.T) {
	record, err := DecodeIncident(strings.NewReader(validIncident))
	require.NoError(t, err)
	record.DetectedAt = record.StartedAt.Add(-time.Second)
	require.ErrorContains(t, record.Validate(), "chronological")
	record.DetectedAt = record.StartedAt
	record.Actions[0].Status = "done"
	require.ErrorContains(t, record.Validate(), "inconsistent")
}

func TestIncidentSetRejectsDuplicateIDs(t *testing.T) {
	record, err := DecodeIncident(strings.NewReader(validIncident))
	require.NoError(t, err)
	require.ErrorContains(t, ValidateIncidentSet([]IncidentRecord{record, record}), "duplicate incident record ID")
}
