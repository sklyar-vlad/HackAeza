package source

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
)

var ErrInvalidJSON = errors.New("invalid snapshot JSON")

var capturedTime = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}[Tt]\d{2}:\d{2}:\d{2}(\.\d+)?([Zz]|[+-]([01]\d|2[0-3]):[0-5]\d)$`)

func DecodeSnapshot(reader io.Reader) (entity.SourceSnapshot, error) {
	decoder := json.NewDecoder(reader)
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return entity.SourceSnapshot{}, fmt.Errorf("%w: %w", ErrInvalidJSON, err)
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("expected exactly one JSON document")
		}
		return entity.SourceSnapshot{}, fmt.Errorf("%w: %w", ErrInvalidJSON, err)
	}
	var issues validationErrors
	object := readObject(raw, "$", []string{"accounts", "services", "transactions"}, &issues)
	allowed := map[string]bool{"accounts": true, "services": true, "transactions": true, "capturedAt": true}
	var unknown []string
	for name := range object {
		if !allowed[name] {
			unknown = append(unknown, name)
		}
	}
	sort.Strings(unknown)
	for _, name := range unknown {
		issues.add(name, "Неизвестное поле снимка.")
	}
	snapshot := entity.SourceSnapshot{
		Accounts: decodeRecords[entity.Account](object["accounts"], "accounts",
			[]string{"id", "email", "balance", "withdrawBalance", "bonusBalance", "bonusState", "totalReplenished"}, nil, &issues),
		Services: decodeRecords[entity.Service](object["services"], "services",
			[]string{"id", "ownerId", "name", "paymentTerm", "autoProlong", "createdAt", "expiresAt", "status", "price"},
			[]string{"createdAt", "expiresAt"}, &issues),
		Transactions: decodeRecords[entity.Transaction](object["transactions"], "transactions",
			[]string{"id", "serviceId", "amount", "status", "type", "createdAt", "performedAt"},
			[]string{"createdAt", "performedAt"}, &issues),
	}
	if captured, present := object["capturedAt"]; present {
		var text string
		if err := json.Unmarshal(captured, &text); err != nil || !capturedTime.MatchString(text) {
			issues.add("capturedAt", "Ожидается дата RFC 3339 с часовым поясом.")
		} else {
			timestamp, err := time.Parse(time.RFC3339Nano, strings.ToUpper(text))
			if err != nil {
				issues.add("capturedAt", "Ожидается дата RFC 3339 с часовым поясом.")
			} else {
				snapshot.CapturedAt = &timestamp
			}
		}
	}
	if err := issues.err(); err != nil {
		return entity.SourceSnapshot{}, err
	}
	return snapshot, nil
}

func decodeRecords[T any](raw json.RawMessage, name string, required, timestamps []string, issues *validationErrors) []T {
	if len(raw) == 0 || isNull(raw) {
		return nil
	}
	var records []json.RawMessage
	if err := json.Unmarshal(raw, &records); err != nil {
		issues.add(name, "Ожидается массив объектов.")
		return nil
	}
	result := make([]T, 0, len(records))
	for i, record := range records {
		path := fmt.Sprintf("%s[%d]", name, i)
		fields := readObject(record, path, required, issues)
		validDates := true
		for _, field := range timestamps {
			value, present := fields[field]
			if !present || isNull(value) {
				continue
			}
			var timestamp entity.SourceTimestamp
			if err := json.Unmarshal(value, &timestamp); err != nil {
				issues.add(path+"."+field, "Ожидается дата UTC вида 2026-10-19T15:00:00.000Z.")
				validDates = false
			}
		}
		if fields == nil || !validDates {
			continue
		}
		for field := range fields {
			if !slices.Contains(required, field) {
				delete(fields, field)
			}
		}
		canonical, err := json.Marshal(fields)
		if err != nil {
			issues.add(path, "Не удалось прочитать объект JSON.")
			continue
		}
		var value T
		if err := json.Unmarshal(canonical, &value); err != nil {
			var typeError *json.UnmarshalTypeError
			if errors.As(err, &typeError) && typeError.Field != "" {
				issues.add(path+"."+typeError.Field, "Значение имеет неверный тип или выходит за допустимый диапазон.")
			} else {
				issues.add(path, "Не удалось прочитать запись в формате источника.")
			}
			continue
		}
		result = append(result, value)
	}
	return result
}

func readObject(raw json.RawMessage, path string, required []string, issues *validationErrors) map[string]json.RawMessage {
	var object map[string]json.RawMessage
	if isNull(raw) || json.Unmarshal(raw, &object) != nil || object == nil {
		issues.add(path, "Ожидается объект JSON.")
		return nil
	}
	for _, name := range required {
		value, present := object[name]
		fieldPath := name
		if path != "$" {
			fieldPath = path + "." + name
		}
		if !present {
			issues.add(fieldPath, "Обязательное поле отсутствует.")
		} else if isNull(value) && name != "performedAt" {
			issues.add(fieldPath, "Обязательное поле не может быть null.")
		}
	}
	return object
}

func isNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}
