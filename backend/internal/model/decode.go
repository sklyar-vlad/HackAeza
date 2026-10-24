package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
)

var ErrInvalidJSON = errors.New("invalid settings JSON")

func DecodePatch(data []byte) (entity.ModelSettingsPatch, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var fields map[string]json.RawMessage
	if err := decoder.Decode(&fields); err != nil || fields == nil {
		return entity.ModelSettingsPatch{}, ErrInvalidJSON
	}
	if err := decoder.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return entity.ModelSettingsPatch{}, ErrInvalidJSON
	}
	var details []entity.FieldError
	for name, raw := range fields {
		if !slices.Contains([]string{"expectedVersion", "coefficients", "thresholds", "horizonDays", "refreshIntervalSeconds"}, name) {
			return entity.ModelSettingsPatch{}, fmt.Errorf("%w: unknown field %s", ErrInvalidJSON, name)
		}
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			details = append(details, entity.FieldError{Path: name, Message: "null недопустим."})
		}
	}
	if raw, exists := fields["coefficients"]; exists {
		var coefficients map[string]json.RawMessage
		if err := json.Unmarshal(raw, &coefficients); err != nil {
			return entity.ModelSettingsPatch{}, fmt.Errorf("%w: coefficients must be an object", ErrInvalidJSON)
		}
		for name, value := range coefficients {
			if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				details = append(details, entity.FieldError{Path: "coefficients." + name, Message: "null недопустим."})
			}
		}
	}
	if raw, exists := fields["thresholds"]; exists {
		var thresholds map[string]json.RawMessage
		if err := json.Unmarshal(raw, &thresholds); err != nil {
			return entity.ModelSettingsPatch{}, fmt.Errorf("%w: thresholds must be an object", ErrInvalidJSON)
		}
		for name := range thresholds {
			if name != "mediumFrom" && name != "highFrom" {
				return entity.ModelSettingsPatch{}, fmt.Errorf("%w: unknown threshold %s", ErrInvalidJSON, name)
			}
		}
		for _, name := range []string{"mediumFrom", "highFrom"} {
			if raw, exists := thresholds[name]; !exists || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
				details = append(details, entity.FieldError{Path: "thresholds." + name, Message: "Обязателен числовой порог."})
			}
		}
	}
	if len(details) > 0 {
		return entity.ModelSettingsPatch{}, &ValidationError{Details: details}
	}
	var patch entity.ModelSettingsPatch
	if err := json.Unmarshal(data, &patch); err != nil {
		return patch, fmt.Errorf("%w: %w", ErrInvalidJSON, err)
	}
	return patch, nil
}
