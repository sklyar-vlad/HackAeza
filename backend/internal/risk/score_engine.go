package risk

import (
	"time"

	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
)

// ScoreEngine assembles the existing features and scoring functions. Financial
// estimates remain explicitly unavailable until the financial engine is added.
type ScoreEngine struct{}

func (ScoreEngine) Calculate(input Input, settings entity.ModelSettings, asOf time.Time) (entity.CustomerDetail, error) {
	features, serviceFeatures, err := Features(input, settings.HorizonDays, asOf)
	if err != nil {
		return entity.CustomerDetail{}, err
	}
	detail := entity.CustomerDetail{Customer: entity.Customer{Account: input.Account,
		ServicesCount: int64(len(input.Services)), Loss: unavailableLoss(settings.HorizonDays)},
		Services: make([]entity.ServiceAssessment, len(input.Services))}
	detail.Risk, detail.Features, err = CalculateScore(features, settings)
	if err != nil {
		return entity.CustomerDetail{}, err
	}
	for i, service := range input.Services {
		assessment, features, err := CalculateScore(serviceFeatures[i], settings)
		if err != nil {
			return entity.CustomerDetail{}, err
		}
		detail.Services[i] = entity.ServiceAssessment{Service: service,
			Risk: assessment, Features: features, Loss: unavailableLoss(settings.HorizonDays)}
	}
	return detail, nil
}

func unavailableLoss(horizon int64) entity.LossEstimate {
	return entity.LossEstimate{Currency: entity.CurrencyEUR, HorizonDays: horizon, IsPartial: true,
		Explanation: "Денежная оценка недоступна: финансовый расчёт ещё не подключён."}
}

var _ Engine = ScoreEngine{}
