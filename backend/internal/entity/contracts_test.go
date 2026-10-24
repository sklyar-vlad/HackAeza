package entity_test

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
)

func TestOrganizerSnapshotRoundTrip(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/data.json")
	if err != nil {
		t.Fatal(err)
	}
	var snapshot entity.SourceSnapshot
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Accounts) != 10 || len(snapshot.Services) != 30 || len(snapshot.Transactions) != 100 {
		t.Fatal("organizer snapshot was not decoded completely")
	}
	if snapshot.CapturedAt != nil {
		t.Fatal("missing source capture time must not be invented")
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decodeJSON(t, data), decodeJSON(t, encoded)) {
		t.Fatal("source fields, nulls, dates or integer values changed on round trip")
	}
}

func TestMoneyPreservesInt64Precision(t *testing.T) {
	var account entity.Account
	if err := json.Unmarshal([]byte(`{"id":9223372036854775807,"balance":9223372036854775807}`), &account); err != nil {
		t.Fatal(err)
	}
	if account.ID != entity.ID(math.MaxInt64) || account.Balance != entity.MoneyCents(math.MaxInt64) {
		t.Fatal("ID or eurocents lost int64 precision")
	}
	encoded, err := json.Marshal(account)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encoded, []byte(`"balance":9223372036854775807`)) {
		t.Fatalf("eurocents changed: %s", encoded)
	}
}

func TestUnknownScoreDiffersFromZero(t *testing.T) {
	zero := 0.0
	for _, test := range []struct {
		name  string
		risk  entity.RiskAssessment
		score string
	}{
		{"unknown", entity.RiskAssessment{Status: entity.AssessmentInsufficientData, Level: entity.RiskUnknown, Reasons: []entity.RiskReason{}}, "null"},
		{"assessed_zero", entity.RiskAssessment{Status: entity.AssessmentAssessed, Score: &zero, Level: entity.RiskLow, Reasons: []entity.RiskReason{}}, "0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := json.Marshal(test.risk)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &fields); err != nil {
				t.Fatal(err)
			}
			if string(fields["score"]) != test.score || string(fields["reasons"]) != "[]" {
				t.Fatalf("unknown, zero and required empty arrays must stay distinct: %s", encoded)
			}
		})
	}
}

func TestPatchKeepsDisabledCoefficient(t *testing.T) {
	var patch entity.ModelSettingsPatch
	if err := json.Unmarshal([]byte(`{"coefficients":{"paymentWaiting":0}}`), &patch); err != nil {
		t.Fatal(err)
	}
	value, present := patch.Coefficients[entity.FeaturePaymentWaiting]
	if !present || value != 0 || patch.HorizonDays != nil || patch.Thresholds != nil {
		t.Fatal("disabled coefficient must survive without overwriting absent settings")
	}
	encoded, err := json.Marshal(patch)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"coefficients":{"paymentWaiting":0}}` {
		t.Fatalf("PATCH fields changed: %s", encoded)
	}
}

func TestJobResultVariants(t *testing.T) {
	now := time.Date(2026, time.October, 24, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name   string
		kind   entity.JobKind
		result entity.JobResult
		field  string
	}{
		{"model", entity.JobModelSettings, entity.ModelSettingsResponse{}, "settings"},
		{"snapshot", entity.JobSourceSnapshot, entity.SnapshotResult{Warnings: []entity.ImportWarning{}}, "counts"},
	} {
		t.Run(test.name, func(t *testing.T) {
			job := entity.CalculationJob{
				JobID: "8f24ad45-14ab-4df6-909b-8b5c6767b18e", Kind: test.kind,
				Status: entity.JobSucceeded, CreatedAt: now, StartedAt: &now, FinishedAt: &now,
				Attempt: 1, Result: test.result,
			}
			encoded, err := json.Marshal(job)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &fields); err != nil {
				t.Fatal(err)
			}
			var result map[string]json.RawMessage
			if err := json.Unmarshal(fields["result"], &result); err != nil {
				t.Fatal(err)
			}
			if _, ok := result[test.field]; !ok {
				t.Fatalf("wrong result variant: %s", encoded)
			}
			if _, ok := fields["error"]; ok {
				t.Fatal("success must not contain an error field")
			}
		})
	}
	queued, err := json.Marshal(entity.CalculationJob{
		JobID: "8f24ad45-14ab-4df6-909b-8b5c6767b18e", Kind: entity.JobSourceSnapshot,
		Status: entity.JobQueued, CreatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(queued, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"result", "error", "progress"} {
		if _, ok := fields[key]; ok {
			t.Fatalf("queued job must omit %s", key)
		}
	}
	if string(fields["startedAt"]) != "null" || string(fields["finishedAt"]) != "null" {
		t.Fatalf("queued timestamps must be explicit nulls: %s", queued)
	}
}

func TestSourceTimestampFormat(t *testing.T) {
	for _, text := range []string{
		`"2026-10-24T12:00:00Z"`,
		`"2026-10-24T12:00:00.00Z"`,
		`"2026-10-24T12:00:00.0000Z"`,
		`"2026-10-24T12:00:00.000+00:00"`,
		`"2026-10-24T12:00:00,000Z"`,
		`null`,
	} {
		t.Run(text, func(t *testing.T) {
			var timestamp entity.SourceTimestamp
			if err := json.Unmarshal([]byte(text), &timestamp); err == nil {
				t.Fatalf("accepted non-contract timestamp %s", text)
			}
		})
	}
	precise := entity.SourceTimestamp{Time: time.Date(2026, time.October, 24, 12, 0, 0, 1, time.UTC)}
	if _, err := json.Marshal(precise); err == nil {
		t.Fatal("must not silently truncate submillisecond source data")
	}
}

func decodeJSON(t *testing.T, data []byte) any {
	t.Helper()
	var value any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	return value
}
