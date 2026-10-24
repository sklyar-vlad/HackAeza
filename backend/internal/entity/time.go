package entity

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const sourceTimeLayout = "2006-01-02T15:04:05.000Z"

type SourceTimestamp struct {
	time.Time
}

func (t SourceTimestamp) MarshalJSON() ([]byte, error) {
	text := t.UTC().Format(sourceTimeLayout)
	if len(text) != len(sourceTimeLayout) || t.Nanosecond()%int(time.Millisecond) != 0 {
		return nil, errors.New("source timestamp cannot be represented as UTC milliseconds")
	}
	return json.Marshal(text)
}

func (t *SourceTimestamp) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return fmt.Errorf("decode source timestamp: %w", err)
	}
	parsed, err := time.Parse(sourceTimeLayout, text)
	if err != nil {
		return fmt.Errorf("source timestamp must use UTC milliseconds: %w", err)
	}
	if parsed.Format(sourceTimeLayout) != text {
		return errors.New("source timestamp must use UTC milliseconds with a decimal point")
	}
	t.Time = parsed
	return nil
}
