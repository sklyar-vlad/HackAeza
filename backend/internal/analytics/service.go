package analytics

import (
	"context"

	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
)

type Service struct {
	repository *Repository
}

func NewService(repository *Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Summary(ctx context.Context) (entity.AnalyticsSummary, error) {
	summary, err := s.repository.Summary(ctx)
	if err != nil {
		return entity.AnalyticsSummary{}, err
	}
	summary.Loss.Currency = entity.CurrencyEUR
	if summary.Calculation.Version == 0 {
		summary.Loss.Explanation = "Снимок данных ещё не загружен."
	}
	return summary, nil
}
