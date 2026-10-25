package entity

import "time"

type FeatureKey string

const (
	FeatureRenewalWithoutAutoProlong FeatureKey = "renewalWithoutAutoProlong"
	FeatureInsufficientRenewalFunds  FeatureKey = "insufficientRenewalFunds"
	FeaturePaymentWaiting            FeatureKey = "paymentWaiting"
)

type Coefficients map[FeatureKey]float64

type RiskThresholds struct {
	MediumFrom float64 `json:"mediumFrom"`
	HighFrom   float64 `json:"highFrom"`
}

type ModelSettings struct {
	Version                int64          `json:"version"`
	UpdatedAt              time.Time      `json:"updatedAt"`
	Coefficients           Coefficients   `json:"coefficients"`
	Thresholds             RiskThresholds `json:"thresholds"`
	HorizonDays            int64          `json:"horizonDays"`
	RefreshIntervalSeconds int64          `json:"refreshIntervalSeconds"`
	ScoreFormula           string         `json:"scoreFormula"`
	LossFormula            string         `json:"lossFormula"`
}

type ModelSettingsPatch struct {
	ExpectedVersion        *int64          `json:"expectedVersion,omitempty"`
	Coefficients           Coefficients    `json:"coefficients,omitempty"`
	Thresholds             *RiskThresholds `json:"thresholds,omitempty"`
	HorizonDays            *int64          `json:"horizonDays,omitempty"`
	RefreshIntervalSeconds *int64          `json:"refreshIntervalSeconds,omitempty"`
}

type ModelSettingsResponse struct {
	Settings     ModelSettings `json:"settings"`
	Calculation  Calculation   `json:"calculation"`
	PendingJobID *JobID        `json:"pendingJobId"`
}

type FeatureDefinition struct {
	Key                FeatureKey `json:"key"`
	Name               string     `json:"name"`
	Description        string     `json:"description"`
	DefaultCoefficient float64    `json:"defaultCoefficient"`
	RequiredFields     []string   `json:"requiredFields"`
	MissingDataPolicy  string     `json:"missingDataPolicy"`
}
