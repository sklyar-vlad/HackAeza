package risk

import (
	"time"

	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
)

type ScoreEngine struct{}

func (ScoreEngine) Calculate(input Input, settings entity.ModelSettings, asOf time.Time) (entity.CustomerDetail, error) {
	features, serviceFeatures, err := Features(input, settings.HorizonDays, asOf)
	if err != nil {
		return entity.CustomerDetail{}, err
	}
	detail := entity.CustomerDetail{Customer: entity.Customer{Account: input.Account,
		ServicesCount: int64(len(input.Services))},
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
			Risk: assessment, Features: features}
	}
	loss, serviceLosses, err := CalculateLoss(detail.Services, settings.HorizonDays, asOf)
	if err != nil {
		return entity.CustomerDetail{}, err
	}
	detail.Loss = loss
	for i := range detail.Services {
		detail.Services[i].Loss = serviceLosses[i]
	}
	return detail, nil
}

var _ Engine = ScoreEngine{}
