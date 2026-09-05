package operability

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type IncidentRecord struct {
	SchemaVersion       int          `json:"schema_version" yaml:"schema_version"`
	ID                  string       `json:"id" yaml:"id"`
	Kind                string       `json:"kind" yaml:"kind"`
	Scenario            string       `json:"scenario,omitempty" yaml:"scenario,omitempty"`
	Title               string       `json:"title" yaml:"title"`
	Severity            string       `json:"severity" yaml:"severity"`
	Status              string       `json:"status" yaml:"status"`
	Environment         string       `json:"environment" yaml:"environment"`
	Services            []string     `json:"services" yaml:"services"`
	IncidentCommander   string       `json:"incident_commander" yaml:"incident_commander"`
	StartedAt           time.Time    `json:"started_at" yaml:"started_at"`
	DetectedAt          time.Time    `json:"detected_at" yaml:"detected_at"`
	RecoveredAt         time.Time    `json:"recovered_at" yaml:"recovered_at"`
	ClosedAt            time.Time    `json:"closed_at,omitempty" yaml:"closed_at,omitempty"`
	DetectionSource     string       `json:"detection_source" yaml:"detection_source"`
	CustomerImpact      string       `json:"customer_impact" yaml:"customer_impact"`
	RootCause           string       `json:"root_cause" yaml:"root_cause"`
	ContributingFactors []string     `json:"contributing_factors" yaml:"contributing_factors"`
	Lessons             []string     `json:"lessons" yaml:"lessons"`
	Timeline            []Timeline   `json:"timeline" yaml:"timeline"`
	Actions             []ActionItem `json:"actions" yaml:"actions"`
}

type Timeline struct {
	At    time.Time `json:"at" yaml:"at"`
	Event string    `json:"event" yaml:"event"`
}

type ActionItem struct {
	ID          string     `json:"id" yaml:"id"`
	Description string     `json:"description" yaml:"description"`
	Owner       string     `json:"owner" yaml:"owner"`
	DueDate     string     `json:"due_date" yaml:"due_date"`
	Status      string     `json:"status" yaml:"status"`
	CompletedAt *time.Time `json:"completed_at,omitempty" yaml:"completed_at,omitempty"`
}

func DecodeIncident(reader io.Reader) (IncidentRecord, error) {
	decoder := yaml.NewDecoder(reader)
	decoder.KnownFields(true)
	var record IncidentRecord
	if err := decoder.Decode(&record); err != nil {
		return IncidentRecord{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return IncidentRecord{}, errors.New("incident file must contain exactly one document")
		}
		return IncidentRecord{}, err
	}
	return record, record.Validate()
}

func (r IncidentRecord) Validate() error {
	if r.SchemaVersion != 1 {
		return errors.New("schema_version must be 1")
	}
	if !validBoundedName(r.ID) {
		return errors.New("id must use 1-64 lowercase letters, numbers, hyphens, or underscores")
	}
	if r.Kind != "incident" && r.Kind != "drill" {
		return errors.New("kind must be incident or drill")
	}
	if r.Kind == "drill" {
		if strings.EqualFold(r.Environment, "production") {
			return errors.New("drill records must not target production")
		}
		if !allowedValue(r.Scenario, "db_unavailable", "worker_crash_loop", "provider_timeout_storm", "other") {
			return errors.New("drill scenario is unsupported")
		}
	}
	if !allowedValue(r.Severity, "sev1", "sev2", "sev3", "sev4") || !allowedValue(r.Status, "recovered", "closed") {
		return errors.New("severity or status is invalid")
	}
	for name, value := range map[string]string{
		"title": r.Title, "environment": r.Environment, "incident_commander": r.IncidentCommander,
		"detection_source": r.DetectionSource, "customer_impact": r.CustomerImpact, "root_cause": r.RootCause,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	if len(r.Services) == 0 {
		return errors.New("at least one affected service is required")
	}
	if r.StartedAt.IsZero() || r.DetectedAt.Before(r.StartedAt) || r.RecoveredAt.Before(r.DetectedAt) {
		return errors.New("started_at, detected_at, and recovered_at must be chronological")
	}
	if r.Status == "closed" && (r.ClosedAt.IsZero() || r.ClosedAt.Before(r.RecoveredAt)) {
		return errors.New("closed records require closed_at at or after recovery")
	}
	if (r.Severity == "sev1" || r.Severity == "sev2") && (len(r.Lessons) == 0 || len(r.Actions) == 0) {
		return errors.New("sev1/sev2 records require lessons and corrective actions")
	}
	if len(r.Timeline) == 0 {
		return errors.New("timeline is required")
	}
	previous := r.StartedAt
	for _, entry := range r.Timeline {
		if entry.At.Before(previous) || strings.TrimSpace(entry.Event) == "" {
			return errors.New("timeline entries must be populated and chronological")
		}
		previous = entry.At
	}
	seenActions := make(map[string]struct{}, len(r.Actions))
	for _, action := range r.Actions {
		if !validBoundedName(action.ID) || strings.TrimSpace(action.Description) == "" || strings.TrimSpace(action.Owner) == "" {
			return errors.New("action id, description, and owner are required")
		}
		if _, duplicate := seenActions[action.ID]; duplicate {
			return fmt.Errorf("duplicate action ID %q", action.ID)
		}
		seenActions[action.ID] = struct{}{}
		dueDate, err := time.Parse("2006-01-02", action.DueDate)
		if err != nil {
			return fmt.Errorf("action %s due_date must use YYYY-MM-DD", action.ID)
		}
		if dueDate.Before(time.Date(r.StartedAt.Year(), r.StartedAt.Month(), r.StartedAt.Day(), 0, 0, 0, 0, time.UTC)) {
			return fmt.Errorf("action %s due_date must not precede the incident", action.ID)
		}
		if !allowedValue(action.Status, "open", "in_progress", "done") || (action.Status == "done") != (action.CompletedAt != nil) {
			return fmt.Errorf("action %s has inconsistent status/completed_at", action.ID)
		}
		if action.CompletedAt != nil && action.CompletedAt.Before(r.StartedAt) {
			return fmt.Errorf("action %s completed_at must not precede the incident", action.ID)
		}
	}
	return nil
}

type IncidentSummary struct {
	SchemaVersion   int             `json:"schema_version"`
	RecordCount     int             `json:"record_count"`
	OpenActionCount int             `json:"open_action_count"`
	MeanMTTDSeconds float64         `json:"mean_mttd_seconds"`
	MeanMTTRSeconds float64         `json:"mean_mttr_seconds"`
	Quarterly       []QuarterMetric `json:"quarterly"`
}

type QuarterMetric struct {
	Quarter         string  `json:"quarter"`
	RecordCount     int     `json:"record_count"`
	MeanMTTDSeconds float64 `json:"mean_mttd_seconds"`
	MeanMTTRSeconds float64 `json:"mean_mttr_seconds"`
}

func SummarizeIncidents(records []IncidentRecord) (IncidentSummary, error) {
	if err := ValidateIncidentSet(records); err != nil {
		return IncidentSummary{}, err
	}
	type aggregate struct {
		count      int
		mttd, mttr float64
	}
	quarters := make(map[string]*aggregate)
	summary := IncidentSummary{SchemaVersion: 1, RecordCount: len(records)}
	var totalMTTD, totalMTTR float64
	for _, record := range records {
		mttd := record.DetectedAt.Sub(record.StartedAt).Seconds()
		mttr := record.RecoveredAt.Sub(record.StartedAt).Seconds()
		totalMTTD += mttd
		totalMTTR += mttr
		quarter := fmt.Sprintf("%04d-Q%d", record.StartedAt.Year(), (int(record.StartedAt.Month())-1)/3+1)
		if quarters[quarter] == nil {
			quarters[quarter] = &aggregate{}
		}
		quarters[quarter].count++
		quarters[quarter].mttd += mttd
		quarters[quarter].mttr += mttr
		for _, action := range record.Actions {
			if action.Status != "done" {
				summary.OpenActionCount++
			}
		}
	}
	summary.MeanMTTDSeconds = totalMTTD / float64(len(records))
	summary.MeanMTTRSeconds = totalMTTR / float64(len(records))
	keys := make([]string, 0, len(quarters))
	for quarter := range quarters {
		keys = append(keys, quarter)
	}
	sort.Strings(keys)
	for _, quarter := range keys {
		value := quarters[quarter]
		summary.Quarterly = append(summary.Quarterly, QuarterMetric{
			Quarter: quarter, RecordCount: value.count,
			MeanMTTDSeconds: value.mttd / float64(value.count), MeanMTTRSeconds: value.mttr / float64(value.count),
		})
	}
	return summary, nil
}

func ValidateIncidentSet(records []IncidentRecord) error {
	if len(records) == 0 {
		return errors.New("at least one incident record is required")
	}
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		if err := record.Validate(); err != nil {
			return fmt.Errorf("record %s: %w", record.ID, err)
		}
		if _, duplicate := seen[record.ID]; duplicate {
			return fmt.Errorf("duplicate incident record ID %q", record.ID)
		}
		seen[record.ID] = struct{}{}
	}
	return nil
}

func allowedValue(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
