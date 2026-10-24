// Package entity contains shared data contracts, with no infrastructure dependencies.
package entity

import "time"

type ID int64

// MoneyCents is an integer amount in eurocents. Validation enforces nonnegative values.
type MoneyCents int64

type BonusState string

const (
	BonusLocked   BonusState = "locked"
	BonusUnlocked BonusState = "unlocked"
)

type Account struct {
	ID               ID         `json:"id"`
	Email            string     `json:"email"`
	Balance          MoneyCents `json:"balance"`
	WithdrawBalance  MoneyCents `json:"withdrawBalance"`
	BonusBalance     MoneyCents `json:"bonusBalance"`
	BonusState       BonusState `json:"bonusState"`
	TotalReplenished MoneyCents `json:"totalReplenished"`
}

type ServiceStatus string

const (
	ServiceActive      ServiceStatus = "active"
	ServiceSuspended   ServiceStatus = "suspended"
	ServiceProlongWait ServiceStatus = "prolong_wait"
	ServiceDeleted     ServiceStatus = "deleted"
	ServiceBlocked     ServiceStatus = "blocked"
)

type PaymentTerm string

const (
	PaymentHour        PaymentTerm = "hour"
	PaymentWeek        PaymentTerm = "week"
	PaymentMonth       PaymentTerm = "month"
	PaymentQuarterYear PaymentTerm = "quarter_year"
	PaymentHalfYear    PaymentTerm = "half_year"
	PaymentYear        PaymentTerm = "year"
)

// Service corresponds to SourceService in OpenAPI. Status does not imply a deletion date.
type Service struct {
	ID          ID              `json:"id"`
	OwnerID     ID              `json:"ownerId"`
	Name        string          `json:"name"`
	PaymentTerm PaymentTerm     `json:"paymentTerm"`
	AutoProlong bool            `json:"autoProlong"`
	CreatedAt   SourceTimestamp `json:"createdAt"`
	ExpiresAt   SourceTimestamp `json:"expiresAt"`
	Status      ServiceStatus   `json:"status"`
	Price       MoneyCents      `json:"price"`
}

type TransactionStatus string

const (
	TransactionPerformed    TransactionStatus = "performed"
	TransactionNotPerformed TransactionStatus = "not_performed"
)

type TransactionType string

const (
	TransactionOrder   TransactionType = "order"
	TransactionProlong TransactionType = "prolong"
)

// Transaction corresponds to SourceTransaction, not a balance replenishment.
type Transaction struct {
	ID          ID                `json:"id"`
	ServiceID   ID                `json:"serviceId"`
	Amount      int32             `json:"amount"` // Eurocents, 0..1000 in the source contract.
	Status      TransactionStatus `json:"status"`
	Type        TransactionType   `json:"type"`
	CreatedAt   SourceTimestamp   `json:"createdAt"`
	PerformedAt *SourceTimestamp  `json:"performedAt"` // Required JSON null for not_performed.
}

// SourceSnapshot is a full replacement. All three arrays must be present, including when empty.
// Required fields, ranges and relationships are validated by the source module, not by these structs.
type SourceSnapshot struct {
	Accounts     []Account     `json:"accounts"`
	Services     []Service     `json:"services"`
	Transactions []Transaction `json:"transactions"`
	CapturedAt   *time.Time    `json:"capturedAt,omitempty"`
}

type SourceCounts struct {
	Accounts     int64 `json:"accounts"`
	Services     int64 `json:"services"`
	Transactions int64 `json:"transactions"`
}

type ImportWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path"`
}

type SnapshotResult struct {
	Counts      SourceCounts    `json:"counts"`
	Warnings    []ImportWarning `json:"warnings"`
	Calculation Calculation     `json:"calculation"`
}

type SourceStatus struct {
	SnapshotPresent bool         `json:"snapshotPresent"`
	Counts          SourceCounts `json:"counts"`
	ReceivedAt      *time.Time   `json:"receivedAt"`
	CapturedAt      *time.Time   `json:"capturedAt"`
	Calculation     Calculation  `json:"calculation"`
	PendingJobID    *JobID       `json:"pendingJobId"`
}
