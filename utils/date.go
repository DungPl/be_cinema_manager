// utils/custom_date.go
package utils

import (
	"database/sql/driver"
	"fmt"
	"time"
)

// CustomDate chỉ lưu ngày (không giờ)
type CustomDate struct {
	time.Time
}

// === JSON: Nhận và trả về "YYYY-MM-DD" ===
func (d *CustomDate) UnmarshalJSON(data []byte) error {
	if string(data) == `null` {
		*d = CustomDate{time.Time{}}
		return nil
	}

	str := string(data)
	if len(str) >= 2 && str[0] == '"' && str[len(str)-1] == '"' {
		str = str[1 : len(str)-1]
	}

	t, err := time.Parse("2006-01-02", str)
	if err != nil {
		return fmt.Errorf("invalid date format: %s", str)
	}
	*d = CustomDate{t}
	return nil
}

func (d CustomDate) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte(`null`), nil
	}
	return []byte(`"` + d.Format("2006-01-02") + `"`), nil
}

// === DB: Ghi và đọc từ MySQL/PostgreSQL ===
func (d CustomDate) Value() (driver.Value, error) {
	if d.IsZero() {
		return nil, nil // NULL
	}
	return d.Time.Format("2006-01-02"), nil
}

func (d *CustomDate) Scan(value interface{}) error {
	if value == nil {
		*d = CustomDate{time.Time{}}
		return nil
	}

	switch v := value.(type) {
	case time.Time:
		*d = CustomDate{v}
		return nil
	case string:
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			return fmt.Errorf("cannot parse date string: %v", err)
		}
		*d = CustomDate{t}
		return nil
	case []byte:
		t, err := time.Parse("2006-01-02", string(v))
		if err != nil {
			return fmt.Errorf("cannot parse date bytes: %v", err)
		}
		*d = CustomDate{t}
		return nil
	default:
		return fmt.Errorf("unsupported scan type for CustomDate: %T", value)
	}
}

// === Helper ===
func (d CustomDate) String() string {
	if d.IsZero() {
		return ""
	}
	return d.Format("2006-01-02")
}
