package model

import (
	"context"
	"maps"
	"math"

	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
	"github.com/sklyar-vlad/HackAeza/backend/internal/jobs"
)

type ValidationError struct{ Details []entity.FieldError }

func (e *ValidationError) Error() string { return "invalid model settings" }

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) Settings(ctx context.Context) (entity.ModelSettingsResponse, error) {
	return s.repository.Published(ctx)
}

func (s *Service) Update(ctx context.Context, patch entity.ModelSettingsPatch, submission jobs.Submission) (jobs.Accepted, error) {
	if accepted, err := s.repository.Lookup(ctx, entity.JobModelSettings, submission); err != nil || accepted != nil {
		if err != nil {
			return jobs.Accepted{}, err
		}
		return *accepted, nil
	}
	published, err := s.repository.Published(ctx)
	if err != nil {
		return jobs.Accepted{}, err
	}
	if patch.ExpectedVersion != nil {
		if *patch.ExpectedVersion < 1 {
			return jobs.Accepted{}, &ValidationError{Details: []entity.FieldError{{Path: "expectedVersion", Message: "Версия должна быть положительной."}}}
		}
		if *patch.ExpectedVersion != published.Settings.Version {
			return jobs.Accepted{}, entity.ErrVersionConflict
		}
	}
	settings := published.Settings
	settings.Coefficients = maps.Clone(settings.Coefficients)
	if settings.Coefficients == nil {
		settings.Coefficients = entity.Coefficients{}
	}
	var details []entity.FieldError
	if patch.Coefficients == nil && patch.Thresholds == nil && patch.HorizonDays == nil && patch.RefreshIntervalSeconds == nil {
		details = append(details, entity.FieldError{Path: "$", Message: "Нужно изменить хотя бы одно поле настроек."})
	}
	known := map[entity.FeatureKey]bool{}
	for _, feature := range Features() {
		known[feature.Key] = true
	}
	for key, weight := range patch.Coefficients {
		if !known[key] || math.IsNaN(weight) || math.IsInf(weight, 0) || weight < 0 {
			details = append(details, entity.FieldError{Path: "coefficients." + string(key), Message: "Неизвестный признак или недопустимый вес."})
		}
		settings.Coefficients[key] = weight
	}
	if patch.Coefficients != nil && len(patch.Coefficients) == 0 {
		details = append(details, entity.FieldError{Path: "coefficients", Message: "Нужен хотя бы один коэффициент."})
	}
	positive := false
	for _, weight := range settings.Coefficients {
		positive = positive || weight > 0
	}
	if !positive {
		details = append(details, entity.FieldError{Path: "coefficients", Message: "Хотя бы один вес должен быть положительным."})
	}
	if patch.Thresholds != nil {
		settings.Thresholds = *patch.Thresholds
	}
	t := settings.Thresholds
	if math.IsNaN(t.MediumFrom) || math.IsNaN(t.HighFrom) || math.IsInf(t.MediumFrom, 0) || math.IsInf(t.HighFrom, 0) || t.MediumFrom < 0 || t.HighFrom > 100 || t.MediumFrom >= t.HighFrom {
		details = append(details, entity.FieldError{Path: "thresholds", Message: "Требуется 0 ≤ mediumFrom < highFrom ≤ 100."})
	}
	if patch.HorizonDays != nil {
		settings.HorizonDays = *patch.HorizonDays
	}
	if patch.RefreshIntervalSeconds != nil {
		settings.RefreshIntervalSeconds = *patch.RefreshIntervalSeconds
	}
	if settings.HorizonDays < 1 {
		details = append(details, entity.FieldError{Path: "horizonDays", Message: "Горизонт должен быть положительным."})
	}
	if settings.RefreshIntervalSeconds < 1 {
		details = append(details, entity.FieldError{Path: "refreshIntervalSeconds", Message: "Интервал должен быть положительным."})
	}
	if len(details) > 0 {
		return jobs.Accepted{}, &ValidationError{Details: details}
	}
	return s.repository.AcceptSettings(ctx, Candidate{BaseVersion: published.Settings.Version, Settings: settings}, submission)
}
