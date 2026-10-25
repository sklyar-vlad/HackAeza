package risk

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"time"

	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
)

func CalculateLoss(services []entity.ServiceAssessment, horizonDays int64, asOf time.Time) (entity.LossEstimate, []entity.LossEstimate, error) {
	if horizonDays <= 0 {
		return entity.LossEstimate{}, nil, fmt.Errorf("horizon must be positive")
	}
	total := entity.LossEstimate{Currency: entity.CurrencyEUR, HorizonDays: horizonDays}
	results := make([]entity.LossEstimate, len(services))
	sum := new(big.Rat)
	assessed := false
	seen := make(map[entity.ID]bool, len(services))
	for i, service := range services {
		if service.ID <= 0 || seen[service.ID] || service.Price < 0 {
			return entity.LossEstimate{}, nil, fmt.Errorf("invalid or duplicate service %d", service.ID)
		}
		seen[service.ID] = true
		zero := entity.MoneyCents(0)
		results[i] = entity.LossEstimate{Currency: entity.CurrencyEUR, HorizonDays: horizonDays, AmountCents: &zero}
		result := &results[i]
		switch service.Status {
		case entity.ServiceDeleted, entity.ServiceBlocked:
			result.Explanation = notRenewable(service.Service)
			continue
		case entity.ServiceActive, entity.ServiceSuspended, entity.ServiceProlongWait:
		default:
			return entity.LossEstimate{}, nil, fmt.Errorf("unknown status for service %d", service.ID)
		}
		switch service.PaymentTerm {
		case entity.PaymentHour, entity.PaymentWeek, entity.PaymentMonth, entity.PaymentQuarterYear, entity.PaymentHalfYear, entity.PaymentYear:
		default:
			return entity.LossEstimate{}, nil, fmt.Errorf("unknown payment term for service %d", service.ID)
		}
		if service.ExpiresAt.Before(service.CreatedAt.Time) {
			return entity.LossEstimate{}, nil, fmt.Errorf("inconsistent dates for service %d", service.ID)
		}
		seconds := service.ExpiresAt.Unix() - asOf.Unix()
		if seconds/86400 > horizonDays || seconds/86400 == horizonDays && (seconds%86400 > 0 || service.ExpiresAt.Nanosecond() > asOf.Nanosecond()) {
			result.Explanation = "Ближайшая оплата за пределами горизонта; выручка под риском на нём равна нулю."
			continue
		}
		if service.Price > math.MaxInt64-total.RenewalRevenueCents {
			return entity.LossEstimate{}, nil, fmt.Errorf("renewal revenue exceeds int64")
		}
		result.RenewalRevenueCents = service.Price
		total.RenewalRevenueCents += service.Price
		if service.Price == 0 {
			result.Explanation = "Ближайшая оплата бесплатна; выручка под риском равна нулю."
			continue
		}
		if service.Risk.Status != entity.AssessmentAssessed || service.Risk.Score == nil {
			result.AmountCents, result.IsPartial = nil, true
			result.UnassessedRevenueCents = service.Price
			result.Explanation = fmt.Sprintf("Нет оценки риска для ближайшей оплаты %d евроцентов (%s); сумма под риском неизвестна.", service.Price, dueText(service.ExpiresAt.Time, asOf))
			total.IsPartial = true
			total.UnassessedRevenueCents += service.Price
			continue
		}
		score := *service.Risk.Score
		if !finite(score) || score < 0 || score > 100 {
			return entity.LossEstimate{}, nil, fmt.Errorf("invalid risk score for service %d", service.ID)
		}
		text := strconv.FormatFloat(score, 'f', -1, 64)
		value, _ := new(big.Rat).SetString(text)
		value.Mul(value, new(big.Rat).SetInt64(int64(service.Price)))
		value.Quo(value, big.NewRat(100, 1))
		amount := roundLoss(value)
		result.AmountCents = &amount
		result.Explanation = fmt.Sprintf("Одна ближайшая оплата %d евроцентов (%s, период %s) × индекс риска %s/100; округление до %d евроцентов. Сценарная оценка, не вероятность оттока.", service.Price, dueText(service.ExpiresAt.Time, asOf), service.PaymentTerm, text, amount)
		sum.Add(sum, value)
		assessed = true
	}
	if !total.IsPartial || assessed {
		amount := roundLoss(sum)
		total.AmountCents = &amount
		total.Explanation = fmt.Sprintf("Одна ближайшая оплата на услугу: %d евроцентов, неоценённая часть: %d. Сумма оценённых оплат × score/100 округлена один раз до %d евроцентов; половина цента округляется вверх. Баланс повторно не вычитается. Сценарная оценка, не вероятность оттока.", total.RenewalRevenueCents, total.UnassessedRevenueCents, amount)
	} else {
		total.Explanation = fmt.Sprintf("Нет оценки риска для ближайших оплат на %d евроцентов; денежная оценка неизвестна, а не равна нулю.", total.UnassessedRevenueCents)
	}
	return total, results, nil
}

func roundLoss(value *big.Rat) entity.MoneyCents {
	var amount, remainder big.Int
	amount.QuoRem(value.Num(), value.Denom(), &remainder)
	if remainder.Lsh(&remainder, 1).Cmp(value.Denom()) >= 0 {
		amount.Add(&amount, big.NewInt(1))
	}
	return entity.MoneyCents(amount.Int64())
}
