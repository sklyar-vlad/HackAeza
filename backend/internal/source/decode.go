package source

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
)

var ErrInvalidJSON = errors.New("invalid snapshot JSON")

var capturedTime = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}[Tt]\d{2}:\d{2}:\d{2}(\.\d+)?([Zz]|[+-]([01]\d|2[0-3]):[0-5]\d)$`)

func DecodeSnapshot(reader io.Reader) (entity.SourceSnapshot, error) {
	decoder := json.NewDecoder(reader)
	token, err := decoder.Token()
	if err != nil {
		return entity.SourceSnapshot{}, fmt.Errorf("%w: %w", ErrInvalidJSON, err)
	}
	if token != json.Delim('{') {
		return entity.SourceSnapshot{}, &ValidationError{Details: []entity.FieldError{{Path: "$", Message: "Ожидается объект JSON."}}}
	}
	var snapshot entity.SourceSnapshot
	var issues validationErrors
	seen := make(map[string]bool)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return snapshot, fmt.Errorf("%w: %w", ErrInvalidJSON, err)
		}
		key := token.(string)
		seen[key] = true
		switch key {
		case "accounts":
			snapshot.Accounts, err = decodeArray[entity.Account](decoder, key, []string{"id", "email", "balance", "withdrawBalance", "bonusBalance", "bonusState", "totalReplenished"}, nil, &issues)
		case "services":
			snapshot.Services, err = decodeArray[entity.Service](decoder, key, []string{"id", "ownerId", "name", "paymentTerm", "autoProlong", "createdAt", "expiresAt", "status", "price"}, []string{"createdAt", "expiresAt"}, &issues)
		case "transactions":
			snapshot.Transactions, err = decodeArray[entity.Transaction](decoder, key, []string{"id", "serviceId", "amount", "status", "type", "createdAt", "performedAt"}, []string{"createdAt", "performedAt"}, &issues)
		default:
			var raw json.RawMessage
			if err = decoder.Decode(&raw); err == nil {
				if key != "capturedAt" {
					issues.add(key, "Неизвестное поле снимка.")
				} else {
					var text string
					if json.Unmarshal(raw, &text) != nil || !capturedTime.MatchString(text) {
						issues.add(key, "Ожидается дата RFC 3339 с часовым поясом.")
					} else {
						value, parseErr := time.Parse(time.RFC3339Nano, strings.ToUpper(text))
						if parseErr != nil {
							issues.add(key, "Ожидается дата RFC 3339 с часовым поясом.")
						} else {
							snapshot.CapturedAt = &value
						}
					}
				}
			}
		}
		if err != nil {
			return snapshot, err
		}
	}
	if _, err := decoder.Token(); err != nil {
		return snapshot, fmt.Errorf("%w: %w", ErrInvalidJSON, err)
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return snapshot, ErrInvalidJSON
	}
	for _, key := range []string{"accounts", "services", "transactions"} {
		if !seen[key] {
			issues.add(key, "Обязательное поле отсутствует.")
		}
	}
	return snapshot, issues.err()
}

func decodeArray[T any](decoder *json.Decoder, name string, required, timestamps []string, issues *validationErrors) ([]T, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidJSON, err)
	}
	if token != json.Delim('[') {
		return nil, &ValidationError{Details: []entity.FieldError{{Path: name, Message: "Ожидается массив объектов."}}}
	}
	values := make([]T, 0)
	for index := 0; decoder.More(); index++ {
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidJSON, err)
		}
		var recordIssues validationErrors
		wrapped := make([]byte, 0, len(raw)+2)
		wrapped = append(wrapped, '[')
		wrapped = append(wrapped, raw...)
		wrapped = append(wrapped, ']')
		records := decodeRecords[T](wrapped, name, required, timestamps, &recordIssues)
		for _, issue := range recordIssues {
			issue.Path = strings.Replace(issue.Path, name+"[0]", fmt.Sprintf("%s[%d]", name, index), 1)
			issues.add(issue.Path, issue.Message)
		}
		values = append(values, records...)
	}
	if _, err := decoder.Token(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidJSON, err)
	}
	return values, nil
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
