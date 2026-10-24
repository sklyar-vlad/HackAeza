package calculation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
	"github.com/sklyar-vlad/HackAeza/backend/internal/jobs"
	"github.com/sklyar-vlad/HackAeza/backend/internal/risk"
	"go.uber.org/zap"
)

type timeQueueFunc func(context.Context) (*jobs.Accepted, error)

func (f timeQueueFunc) ScheduleTime(ctx context.Context) (*jobs.Accepted, error) { return f(ctx) }

func TestSchedulerRunsImmediatelyAndSurvivesDatabaseError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	scheduler := NewTimeScheduler(timeQueueFunc(func(ctx context.Context) (*jobs.Accepted, error) {
		if _, ok := ctx.Deadline(); !ok {
			t.Error("database call must have a timeout")
		}
		calls++
		if calls == 1 {
			return nil, errors.New("database temporarily unavailable")
		}
		cancel()
		return &jobs.Accepted{}, nil
	}), zap.NewNop())
	scheduler.run(ctx, time.Millisecond)
	if calls != 2 {
		t.Fatalf("got %d ticks, want recovery after the first failed tick", calls)
	}
}

func TestSchedulerDoesNotScheduleAfterShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	scheduler := NewTimeScheduler(timeQueueFunc(func(context.Context) (*jobs.Accepted, error) {
		t.Fatal("scheduler called after shutdown")
		return nil, nil
	}), zap.NewNop())
	scheduler.Run(ctx)
}

func TestCurrentFeaturesChangeWithoutNewImport(t *testing.T) {
	asOf := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	input := risk.Input{Account: entity.Account{ID: 1}, Services: []entity.Service{{
		ID: 1, OwnerID: 1, PaymentTerm: entity.PaymentHour, Status: entity.ServiceActive,
		Price: 100, ExpiresAt: entity.SourceTimestamp{Time: asOf.Add(24*time.Hour + time.Second)},
	}}}
	settings := entity.ModelSettings{Coefficients: entity.Coefficients{
		entity.FeatureRenewalWithoutAutoProlong: .4,
		entity.FeatureInsufficientRenewalFunds:  .35,
		entity.FeaturePaymentWaiting:            .25,
	}, Thresholds: entity.RiskThresholds{MediumFrom: 35, HighFrom: 70}}
	scores := make([]float64, 0, 2)
	for _, now := range []time.Time{asOf, asOf.Add(2 * time.Second)} {
		features, _, err := risk.Features(input, 1, now)
		if err != nil {
			t.Fatal(err)
		}
		assessment, _, err := risk.CalculateScore(features, settings)
		if err != nil || assessment.Score == nil {
			t.Fatalf("score: %v, %v", assessment, err)
		}
		scores = append(scores, *assessment.Score)
	}
	if scores[0] >= scores[1] {
		t.Fatalf("time crossing the horizon did not update risk: %v", scores)
	}
}
