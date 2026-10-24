package source

import (
	"fmt"
	"net/mail"
	"regexp"
	"slices"
	"time"

	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
)

type ValidationError struct {
	Details []entity.FieldError
}

func (e *ValidationError) Error() string {
	return "snapshot validation failed"
}

const maxValidationErrors = 100

type validationErrors []entity.FieldError

func (v *validationErrors) add(path, message string) {
	if len(*v) < maxValidationErrors {
		*v = append(*v, entity.FieldError{Path: path, Message: message})
	}
}

func (v *validationErrors) check(valid bool, path, message string) bool {
	if !valid {
		v.add(path, message)
	}
	return valid
}

func (v validationErrors) err() error {
	if len(v) == 0 {
		return nil
	}
	return &ValidationError{Details: []entity.FieldError(v)}
}

var (
	serviceName  = regexp.MustCompile(`^[A-Za-z0-9-]+$`)
	quotedEmail  = regexp.MustCompile(`^"(?:[^"\\\r\n]|\\[^\r\n])*"@[^()\s<>]+$`)
	paymentTerms = []entity.PaymentTerm{
		entity.PaymentHour, entity.PaymentWeek, entity.PaymentMonth,
		entity.PaymentQuarterYear, entity.PaymentHalfYear, entity.PaymentYear,
	}
	serviceStatuses = []entity.ServiceStatus{
		entity.ServiceActive, entity.ServiceSuspended, entity.ServiceProlongWait,
		entity.ServiceDeleted, entity.ServiceBlocked,
	}
)

func ValidateSnapshot(snapshot entity.SourceSnapshot, asOf time.Time) ([]entity.ImportWarning, error) {
	var issues validationErrors
	warnings := make([]entity.ImportWarning, 0)
	if snapshot.CapturedAt != nil {
		if _, err := snapshot.CapturedAt.MarshalJSON(); err != nil {
			issues.add("capturedAt", "Ожидается представимая дата RFC 3339 с часовым поясом.")
		}
	}
	issues.check(snapshot.Accounts != nil, "accounts", "Обязателен массив аккаунтов; пустой массив допустим.")
	issues.check(snapshot.Services != nil, "services", "Обязателен массив услуг; пустой массив допустим.")
	issues.check(snapshot.Transactions != nil, "transactions", "Обязателен массив транзакций; пустой массив допустим.")

	accounts := make(map[entity.ID]int, len(snapshot.Accounts))
	emails := make(map[string]int, len(snapshot.Accounts))
	for i, account := range snapshot.Accounts {
		path := fmt.Sprintf("accounts[%d]", i)
		checkID(&issues, accounts, account.ID, path+".id", i)
		address, err := mail.ParseAddress(account.Email)
		issues.check(err == nil && address.Name == "" && (address.Address == account.Email || quotedEmail.MatchString(account.Email)),
			path+".email", "Ожидается email без имени получателя и пробелов по краям.")
		if first, exists := emails[account.Email]; exists {
			issues.add(path+".email", fmt.Sprintf("Email уже указан в accounts[%d].email (регистр учитывается).", first))
		} else {
			emails[account.Email] = i
		}
		for _, money := range []struct {
			name  string
			value entity.MoneyCents
		}{
			{"balance", account.Balance}, {"withdrawBalance", account.WithdrawBalance},
			{"bonusBalance", account.BonusBalance}, {"totalReplenished", account.TotalReplenished},
		} {
			issues.check(money.value >= 0, path+"."+money.name, "Сумма в евроцентах должна быть неотрицательной.")
		}
		issues.check(account.BonusState == entity.BonusLocked || account.BonusState == entity.BonusUnlocked, path+".bonusState", "Допустимы locked и unlocked.")
	}

	services := make(map[entity.ID]int, len(snapshot.Services))
	for i, service := range snapshot.Services {
		path := fmt.Sprintf("services[%d]", i)
		checkID(&issues, services, service.ID, path+".id", i)
		_, ownerExists := accounts[service.OwnerID]
		issues.check(ownerExists, path+".ownerId", "Аккаунт с таким ID отсутствует в снимке.")
		issues.check(serviceName.MatchString(service.Name), path+".name", "Название должно содержать только латинские буквы, цифры и дефис.")
		issues.check(slices.Contains(paymentTerms, service.PaymentTerm), path+".paymentTerm", "Неизвестный период оплаты.")
		issues.check(slices.Contains(serviceStatuses, service.Status), path+".status", "Неизвестный статус услуги.")
		createdOK := checkTimestamp(&issues, service.CreatedAt, path+".createdAt")
		expiresOK := checkTimestamp(&issues, service.ExpiresAt, path+".expiresAt")
		if createdOK && expiresOK && service.ExpiresAt.Before(service.CreatedAt.Time) {
			issues.add(path+".expiresAt", "Окончание услуги не может быть раньше её создания.")
		}
		issues.check(service.Price >= 0, path+".price", "Цена в евроцентах должна быть неотрицательной.")
		if expiresOK && service.Status == entity.ServiceProlongWait && !asOf.IsZero() && asOf.Sub(service.ExpiresAt.Time) > 5*24*time.Hour {
			warnings = append(warnings, entity.ImportWarning{
				Code: "STALE_PROLONG_WAIT", Path: path + ".status",
				Message: "Услуга ожидает оплаты более пяти дней; исходный статус сохранён.",
			})
		}
	}

	transactions := make(map[entity.ID]int, len(snapshot.Transactions))
	for i, transaction := range snapshot.Transactions {
		path := fmt.Sprintf("transactions[%d]", i)
		checkID(&issues, transactions, transaction.ID, path+".id", i)
		_, serviceExists := services[transaction.ServiceID]
		issues.check(serviceExists, path+".serviceId", "Услуга с таким ID отсутствует в снимке.")
		issues.check(transaction.Amount >= 0 && transaction.Amount <= 1000, path+".amount", "Сумма транзакции должна быть целым числом от 0 до 1000 евроцентов.")
		issues.check(transaction.Type == entity.TransactionOrder || transaction.Type == entity.TransactionProlong, path+".type", "Допустимы order и prolong.")
		createdOK := checkTimestamp(&issues, transaction.CreatedAt, path+".createdAt")
		performedOK := false
		if transaction.PerformedAt != nil {
			performedOK = checkTimestamp(&issues, *transaction.PerformedAt, path+".performedAt")
		}
		switch transaction.Status {
		case entity.TransactionPerformed:
			if transaction.PerformedAt == nil {
				issues.add(path+".performedAt", "Выполненная транзакция должна иметь дату выполнения.")
			} else if createdOK && performedOK && transaction.PerformedAt.Before(transaction.CreatedAt.Time) {
				issues.add(path+".performedAt", "Выполнение не может быть раньше создания транзакции.")
			}
		case entity.TransactionNotPerformed:
			issues.check(transaction.PerformedAt == nil, path+".performedAt", "Для невыполненной транзакции требуется null.")
		default:
			issues.add(path+".status", "Допустимы performed и not_performed.")
		}
	}
	return warnings, issues.err()
}

func checkID(issues *validationErrors, seen map[entity.ID]int, id entity.ID, path string, index int) {
	if id <= 0 {
		issues.add(path, "ID должен быть положительным целым int64.")
		return
	}
	if first, exists := seen[id]; exists {
		issues.add(path, fmt.Sprintf("ID уже указан в записи с индексом %d этого массива.", first))
	} else {
		seen[id] = index
	}
}

func checkTimestamp(issues *validationErrors, timestamp entity.SourceTimestamp, path string) bool {
	_, err := timestamp.MarshalJSON()
	return issues.check(err == nil, path, "Ожидается представимая дата UTC с точностью до миллисекунд.")
}
