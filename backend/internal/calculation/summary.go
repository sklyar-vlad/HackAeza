package calculation

import (
	"fmt"
	"math"

	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
)

func newSummary(run Run) entity.AnalyticsSummary {
	zero := entity.MoneyCents(0)
	return entity.AnalyticsSummary{Calculation: run.Target, Loss: entity.LossEstimate{
		AmountCents: &zero, Currency: entity.CurrencyEUR, HorizonDays: run.Settings.HorizonDays,
		Explanation: "Сумма денежных оценок клиентов; выручка без оценки показана отдельно.",
	}}
}

func addCustomer(summary *entity.AnalyticsSummary, customer entity.CustomerDetail) error {
	summary.TotalCustomers++
	summary.TotalServices += int64(len(customer.Services))
	switch customer.Risk.Level {
	case entity.RiskLow:
		summary.CustomersByRisk.Low++
	case entity.RiskMedium:
		summary.CustomersByRisk.Medium++
	case entity.RiskHigh:
		summary.CustomersByRisk.High++
	case entity.RiskUnknown:
		summary.CustomersByRisk.Unknown++
	default:
		return fmt.Errorf("invalid customer risk level %q", customer.Risk.Level)
	}
	if customer.Risk.Status == entity.AssessmentAssessed {
		summary.AssessedCustomers++
	}
	for _, service := range customer.Services {
		if service.Risk.Level == entity.RiskMedium || service.Risk.Level == entity.RiskHigh {
			summary.AtRiskServices++
		}
	}
	for _, pair := range []struct {
		target *entity.MoneyCents
		value  entity.MoneyCents
	}{
		{&summary.Loss.RenewalRevenueCents, customer.Loss.RenewalRevenueCents},
		{&summary.Loss.UnassessedRevenueCents, customer.Loss.UnassessedRevenueCents},
	} {
		if pair.value < 0 || pair.value > entity.MoneyCents(math.MaxInt64)-*pair.target {
			return fmt.Errorf("analytics money total overflows int64")
		}
		*pair.target += pair.value
	}
	if amount := customer.Loss.AmountCents; amount != nil {
		if *amount < 0 || *amount > entity.MoneyCents(math.MaxInt64)-*summary.Loss.AmountCents {
			return fmt.Errorf("analytics loss total overflows int64")
		}
		*summary.Loss.AmountCents += *amount
	}
	summary.Loss.IsPartial = summary.Loss.IsPartial || customer.Loss.IsPartial
	return nil
}
