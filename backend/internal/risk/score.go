package risk

import (
	"fmt"
	"maps"
	"math"
	"slices"

	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
)

var reasonCodes = map[entity.FeatureKey]string{
	entity.FeatureRenewalWithoutAutoProlong: "AUTO_PROLONG_DISABLED",
	entity.FeatureInsufficientRenewalFunds:  "INSUFFICIENT_RENEWAL_FUNDS",
	entity.FeaturePaymentWaiting:            "PAYMENT_WAITING",
}

func CalculateScore(input []entity.FeatureResult, settings entity.ModelSettings) (entity.RiskAssessment, []entity.FeatureResult, error) {
	thresholds := settings.Thresholds
	if !finite(thresholds.MediumFrom) || !finite(thresholds.HighFrom) || thresholds.MediumFrom < 0 || thresholds.HighFrom > 100 || thresholds.MediumFrom >= thresholds.HighFrom {
		return entity.RiskAssessment{}, nil, fmt.Errorf("invalid risk thresholds")
	}
	keys := slices.Sorted(maps.Keys(settings.Coefficients))
	enabled := false
	for _, key := range keys {
		weight := settings.Coefficients[key]
		if key == "" || !finite(weight) || weight < 0 {
			return entity.RiskAssessment{}, nil, fmt.Errorf("invalid coefficient for %q", key)
		}
		enabled = enabled || weight > 0
	}
	if !enabled {
		return entity.RiskAssessment{}, nil, fmt.Errorf("at least one risk coefficient must be positive")
	}
	assessment := entity.RiskAssessment{
		Status: entity.AssessmentInsufficientData, Level: entity.RiskUnknown,
		Reasons: []entity.RiskReason{},
	}
	features := append([]entity.FeatureResult{}, input...)
	seen := make(map[entity.FeatureKey]bool, len(features))
	maxWeight := 0.0
	for i := range features {
		feature := &features[i]
		if feature.Key == "" || seen[feature.Key] {
			return entity.RiskAssessment{}, nil, fmt.Errorf("empty or duplicate feature key %q", feature.Key)
		}
		seen[feature.Key] = true
		feature.Coefficient = settings.Coefficients[feature.Key]
		feature.Contribution = nil
		feature.ServiceIDs = append([]entity.ID{}, feature.ServiceIDs...)
		if feature.Value != nil {
			value := *feature.Value
			if !finite(value) || value < 0 || value > 1 {
				return entity.RiskAssessment{}, nil, fmt.Errorf("invalid value for feature %q", feature.Key)
			}
			feature.Value = &value
		}
		if !feature.Applicable || feature.Coefficient == 0 {
			continue
		}
		maxWeight = math.Max(maxWeight, feature.Coefficient)
		if feature.Value == nil {
			assessment.Reasons = append(assessment.Reasons, entity.RiskReason{
				Code: "MISSING_FEATURE_DATA", FeatureKey: feature.Key, ServiceIDs: feature.ServiceIDs,
				Message: fmt.Sprintf("Нет данных для признака %s. %s", feature.Key, feature.Explanation),
			})
		}
	}
	for _, key := range keys {
		if settings.Coefficients[key] > 0 && !seen[key] {
			assessment.Reasons = append(assessment.Reasons, entity.RiskReason{
				Code: "MISSING_FEATURE_DATA", FeatureKey: key, ServiceIDs: []entity.ID{},
				Message: fmt.Sprintf("Не передан признак %s.", key),
			})
		}
	}
	if len(assessment.Reasons) > 0 {
		return assessment, features, nil
	}
	if maxWeight == 0 {
		assessment.Reasons = append(assessment.Reasons, entity.RiskReason{
			Code: "NO_APPLICABLE_FEATURES", Message: "Нет применимых включённых признаков для оценки риска.",
			ServiceIDs: []entity.ID{},
		})
		return assessment, features, nil
	}
	denominator := 0.0
	for _, feature := range features {
		if feature.Applicable {
			denominator += feature.Coefficient / maxWeight
		}
	}
	score := 0.0
	for i := range features {
		feature := &features[i]
		if !feature.Applicable || feature.Coefficient == 0 {
			continue
		}
		contribution := 100 * (feature.Coefficient / maxWeight) / denominator * *feature.Value
		feature.Contribution = &contribution
		score += contribution
		if *feature.Value > 0 {
			code := reasonCodes[feature.Key]
			if code == "" {
				code = string(feature.Key)
			}
			message := feature.Explanation
			if message == "" {
				message = fmt.Sprintf("Сработал признак %s.", feature.Key)
			}
			assessment.Reasons = append(assessment.Reasons, entity.RiskReason{
				Code: code, Message: message, FeatureKey: feature.Key, ServiceIDs: feature.ServiceIDs,
			})
		}
	}
	score = math.Min(math.Round(score*1e12)/1e12, 100)
	assessment.Status, assessment.Score, assessment.Level = entity.AssessmentAssessed, &score, entity.RiskLow
	if score >= thresholds.HighFrom {
		assessment.Level = entity.RiskHigh
	} else if score >= thresholds.MediumFrom {
		assessment.Level = entity.RiskMedium
	}
	return assessment, features, nil
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
