package risk

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
)

const paymentWaitLimit = 5 * 24 * time.Hour

type renewal struct {
	index   int
	service entity.Service
	due     time.Time
	missing entity.MoneyCents
}

func Features(input Input, horizonDays int64, asOf time.Time) ([]entity.FeatureResult, [][]entity.FeatureResult, error) {
	if horizonDays <= 0 {
		return nil, nil, fmt.Errorf("horizon must be positive")
	}
	account := input.Account
	horizon := asOf.Add(time.Duration(horizonDays) * 24 * time.Hour)
	available := account.Balance
	if account.BonusState == entity.BonusUnlocked {
		available += account.BonusBalance
	}

	var due []renewal
	var eligible bool
	for i, service := range input.Services {
		if service.OwnerID != account.ID {
			return nil, nil, fmt.Errorf("service %d does not belong to account %d", service.ID, account.ID)
		}
		if renewable(service.Status) {
			eligible = true
			if !service.ExpiresAt.After(horizon) {
				due = append(due, renewal{index: i, service: service, due: service.ExpiresAt.Time})
			}
		}
	}
	slices.SortFunc(due, func(a, b renewal) int {
		return cmp.Or(a.due.Compare(b.due), cmp.Compare(a.service.ID, b.service.ID))
	})
	var required, missing entity.MoneyCents
	left := available
	for i := range due {
		price := due[i].service.Price
		required += price
		paid := min(left, price)
		left -= paid
		due[i].missing = price - paid
		missing += due[i].missing
	}

	services := make([][]entity.FeatureResult, len(input.Services))
	for i, service := range input.Services {
		services[i] = []entity.FeatureResult{
			serviceRenewal(service, horizon, asOf),
			notApplicable(entity.FeatureInsufficientRenewalFunds, service.ID, "Ближайшей оплаты на горизонте нет."),
			serviceWaiting(service, asOf),
		}
		if !renewable(service.Status) {
			services[i][1].Explanation = notRenewable(service)
		}
	}
	for _, r := range due {
		services[r.index][1] = feature(entity.FeatureInsufficientRenewalFunds, share(r.missing, r.service.Price), []entity.ID{r.service.ID},
			fundsExplanation(r.missing, r.service.Price, r.due, available, required))
	}

	customer := []entity.FeatureResult{
		combine(entity.FeatureRenewalWithoutAutoProlong, services, 0, eligible,
			fmt.Sprintf("Без автопродления на горизонте %d дн.: услуги ", horizonDays), "Ближайших оплат без автопродления на горизонте нет."),
		notApplicable(entity.FeatureInsufficientRenewalFunds, 0, "Ближайших оплат на горизонте нет."),
		combine(entity.FeaturePaymentWaiting, services, 2, eligible,
			"Ожидают оплаты (prolong_wait) услуги ", "Услуг в ожидании оплаты нет."),
	}
	if len(due) > 0 {
		ids := make([]entity.ID, 0, len(due))
		for _, r := range due {
			if r.missing > 0 {
				ids = append(ids, r.service.ID)
			}
		}
		explanation := fmt.Sprintf("Доступно %d евроцентов, на ближайшие оплаты нужно %d.", available, required)
		if missing > 0 {
			explanation = fmt.Sprintf("Доступно %d евроцентов, на ближайшие оплаты нужно %d; не хватает %d.", available, required, missing)
		}
		customer[1] = feature(entity.FeatureInsufficientRenewalFunds, share(missing, required), ids, explanation)
	}
	if !eligible {
		for i := range customer {
			customer[i].Explanation = "Нет действующих услуг, которые можно продлить."
		}
	}
	return customer, services, nil
}

func renewable(status entity.ServiceStatus) bool {
	return status == entity.ServiceActive || status == entity.ServiceSuspended || status == entity.ServiceProlongWait
}

func serviceRenewal(service entity.Service, horizon, asOf time.Time) entity.FeatureResult {
	key := entity.FeatureRenewalWithoutAutoProlong
	ids := []entity.ID{service.ID}
	switch {
	case !renewable(service.Status):
		return notApplicable(key, service.ID, notRenewable(service))
	case service.ExpiresAt.After(horizon):
		return feature(key, 0, ids, fmt.Sprintf("Оплачено до %s, за пределами горизонта.", stamp(service.ExpiresAt.Time)))
	case service.AutoProlong:
		return feature(key, 0, ids, fmt.Sprintf("Оплата %s, автопродление включено.", dueText(service.ExpiresAt.Time, asOf)))
	default:
		return feature(key, 1, ids, fmt.Sprintf("Оплата %s, автопродление выключено.", dueText(service.ExpiresAt.Time, asOf)))
	}
}

func serviceWaiting(service entity.Service, asOf time.Time) entity.FeatureResult {
	key := entity.FeaturePaymentWaiting
	ids := []entity.ID{service.ID}
	switch {
	case !renewable(service.Status):
		return notApplicable(key, service.ID, notRenewable(service))
	case service.Status != entity.ServiceProlongWait:
		return feature(key, 0, ids, "Ожидания оплаты нет.")
	case asOf.After(service.ExpiresAt.Add(paymentWaitLimit)):
		return feature(key, 1, ids, fmt.Sprintf("Ожидает оплаты с %s, дольше пяти дней: статус мог устареть.", stamp(service.ExpiresAt.Time)))
	default:
		return feature(key, 1, ids, fmt.Sprintf("Ожидает оплаты с %s.", stamp(service.ExpiresAt.Time)))
	}
}

func combine(key entity.FeatureKey, services [][]entity.FeatureResult, position int, eligible bool, found, clear string) entity.FeatureResult {
	if !eligible {
		return notApplicable(key, 0, "")
	}
	ids := []entity.ID{}
	for _, features := range services {
		if f := features[position]; f.Value != nil && *f.Value > 0 {
			ids = append(ids, f.ServiceIDs...)
		}
	}
	if len(ids) == 0 {
		return feature(key, 0, ids, clear)
	}
	names := make([]string, len(ids))
	for i, id := range ids {
		names[i] = fmt.Sprint(id)
	}
	return feature(key, 1, ids, found+strings.Join(names, ", ")+".")
}

func fundsExplanation(missing, price entity.MoneyCents, due time.Time, available, required entity.MoneyCents) string {
	if missing == 0 {
		return fmt.Sprintf("Оплата %d евроцентов до %s покрыта балансом (доступно %d на все ближайшие оплаты, нужно %d).", price, stamp(due), available, required)
	}
	return fmt.Sprintf("На оплату %d евроцентов до %s не хватает %d (доступно %d на все ближайшие оплаты, нужно %d).", price, stamp(due), missing, available, required)
}

func notRenewable(service entity.Service) string {
	if service.Status == entity.ServiceDeleted {
		return "Услуга удалена и не продлевается."
	}
	return "Услуга заблокирована и не продлевается."
}

func feature(key entity.FeatureKey, value float64, ids []entity.ID, explanation string) entity.FeatureResult {
	return entity.FeatureResult{Key: key, Applicable: true, Value: &value, Explanation: explanation, ServiceIDs: ids}
}

func notApplicable(key entity.FeatureKey, id entity.ID, explanation string) entity.FeatureResult {
	ids := []entity.ID{}
	if id != 0 {
		ids = append(ids, id)
	}
	return entity.FeatureResult{Key: key, Explanation: explanation, ServiceIDs: ids}
}

func share(part, total entity.MoneyCents) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total)
}

func dueText(due, asOf time.Time) string {
	if due.Before(asOf) {
		return "просрочена с " + stamp(due)
	}
	return "до " + stamp(due)
}

func stamp(t time.Time) string {
	return t.UTC().Format("02.01.2006 15:04 UTC")
}
