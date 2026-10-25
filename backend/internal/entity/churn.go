package entity

import "time"

type ChurnPoint struct {
	Date                    string   `json:"date"`
	ObservedDeletedServices *int64   `json:"observedDeletedServices"`
	ForecastNonRenewals     *float64 `json:"forecastNonRenewals"`
	AtRiskServices          *int64   `json:"atRiskServices"`
}

type ChurnSeries struct {
	ObservedHistoryAvailable bool         `json:"observedHistoryAvailable"`
	ObservedFrom             *time.Time   `json:"observedFrom"`
	Points                   []ChurnPoint `json:"points"`
	Calculation              Calculation  `json:"calculation"`
}
