package entity

import "time"

// Calculation identifies one published generation. CalculatedAt is null before the first run.
type Calculation struct {
	Version       int64      `json:"version"`
	SourceVersion int64      `json:"sourceVersion"`
	ModelVersion  int64      `json:"modelVersion"`
	CalculatedAt  *time.Time `json:"calculatedAt"`
}

type RiskLevel string

const (
	RiskLow     RiskLevel = "low"
	RiskMedium  RiskLevel = "medium"
	RiskHigh    RiskLevel = "high"
	RiskUnknown RiskLevel = "unknown"
)

type AssessmentStatus string

const (
	AssessmentAssessed         AssessmentStatus = "assessed"
	AssessmentInsufficientData AssessmentStatus = "insufficient_data"
)

type RiskReason struct {
	Code       string     `json:"code"`
	Message    string     `json:"message"`
	FeatureKey FeatureKey `json:"featureKey"`
	ServiceIDs []ID       `json:"serviceIds"`
}

// Score is a heuristic index, not a probability. Unknown is represented by nil, not zero.
type RiskAssessment struct {
	Status  AssessmentStatus `json:"status"`
	Score   *float64         `json:"score"`
	Level   RiskLevel        `json:"level"`
	Reasons []RiskReason     `json:"reasons"`
}

type FeatureResult struct {
	Key          FeatureKey `json:"key"`
	Applicable   bool       `json:"applicable"`
	Value        *float64   `json:"value"`
	Coefficient  float64    `json:"coefficient"`
	Contribution *float64   `json:"contribution"`
	Explanation  string     `json:"explanation"`
	ServiceIDs   []ID       `json:"serviceIds"`
}

type Currency string

const CurrencyEUR Currency = "EUR"

// LossEstimate covers one nearest renewal per eligible service on the horizon.
// AmountCents is nil when unassessable. IsPartial and UnassessedRevenueCents expose gaps.
type LossEstimate struct {
	AmountCents            *MoneyCents `json:"amountCents"`
	Currency               Currency    `json:"currency"`
	HorizonDays            int64       `json:"horizonDays"`
	RenewalRevenueCents    MoneyCents  `json:"renewalRevenueCents"`
	UnassessedRevenueCents MoneyCents  `json:"unassessedRevenueCents"`
	IsPartial              bool        `json:"isPartial"`
	Explanation            string      `json:"explanation"`
}

type Customer struct {
	Account
	ServicesCount int64          `json:"servicesCount"`
	Risk          RiskAssessment `json:"risk"`
	Loss          LossEstimate   `json:"loss"`
}

type ServiceAssessment struct {
	Service
	Risk     RiskAssessment  `json:"risk"`
	Features []FeatureResult `json:"features"`
	Loss     LossEstimate    `json:"loss"`
}

// CustomerDetail is the common engine output and published customer read model.
// Required array fields must be initialized to empty slices, rather than nil.
type CustomerDetail struct {
	Customer
	Services []ServiceAssessment `json:"services"`
	Features []FeatureResult     `json:"features"`
}

type RiskCounts struct {
	Low     int64 `json:"low"`
	Medium  int64 `json:"medium"`
	High    int64 `json:"high"`
	Unknown int64 `json:"unknown"`
}

type AnalyticsSummary struct {
	TotalCustomers    int64        `json:"totalCustomers"`
	AssessedCustomers int64        `json:"assessedCustomers"`
	CustomersByRisk   RiskCounts   `json:"customersByRisk"`
	TotalServices     int64        `json:"totalServices"`
	AtRiskServices    int64        `json:"atRiskServices"`
	Loss              LossEstimate `json:"loss"`
	Calculation       Calculation  `json:"calculation"`
}
