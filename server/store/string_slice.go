package store

import (
	"database/sql/driver"
	"encoding/json"
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
func (s *StringSlice) Scan(value interface{}) error {
	if value == nil {
		s = &StringSlice{}
		return nil
	}
	var err error
	if v, err := driver.String.ConvertValue(value); err == nil {
		if v, ok := v.([]byte); ok {
			err = json.Unmarshal(v, s)
		}
	}
	return err
}
