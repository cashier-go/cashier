package store

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// StringSlice is a []string which will be stored in a database as a JSON array.
type StringSlice []string

var _ driver.Valuer = (*StringSlice)(nil)

// Value implements the driver.Valuer interface, marshalling the raw value to
// a JSON array.
func (s StringSlice) Value() (driver.Value, error) {
	v, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return string(v), err
}

// Scan implements the sql.Scanner interface, unmarshalling the value coming
// off the wire and storing the result in the StringSlice.
func (s *StringSlice) Scan(value any) error {
	if value == nil {
		return nil
	}
	var b []byte
	// sqlite's driver returns a string for a varchar column, mysql's returns a []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return fmt.Errorf("unable to scan %T into a StringSlice", value)
	}
	if len(b) == 0 {
		return nil
	}
	return json.Unmarshal(b, s)
}
