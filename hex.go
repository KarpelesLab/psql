package psql

import (
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"fmt"
)

type Hex []byte

func (h *Hex) Scan(src interface{}) error {
	var v []byte
	var err error

	switch s := src.(type) {
	case string:
		v, err = hex.DecodeString(s)
	case []byte:
		v, err = hex.DecodeString(string(s))
	case sql.RawBytes:
		v, err = hex.DecodeString(string(s))
	default:
		return fmt.Errorf("unsupported input format %T", src)
	}
	if err != nil {
		return err
	}
	*h = v
	return nil
}

func (h *Hex) Value() (driver.Value, error) {
	// encode to hex
	v := hex.EncodeToString(*h)

	return v, nil
}
