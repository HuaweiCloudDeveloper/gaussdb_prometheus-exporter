// Copyright 2021 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tool

import (
	"bytes"
	"database/sql/driver"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/HuaweiCloudDeveloper/gaussdb_prometheus-exporter/oid"
)

type parameterStatus struct {
	// server version in the same format as server_version_num, or 0 if
	// unavailable
	serverVersion int
}

func encode(parameterStatus *parameterStatus, x interface{}, gstypOid oid.Oid) []byte {
	switch v := x.(type) {
	case int64:
		return strconv.AppendInt(nil, v, 10)
	case float64:
		return strconv.AppendFloat(nil, v, 'f', -1, 64)
	case []byte:
		if gstypOid == oid.T_bytea {
			return encodeBytea(parameterStatus.serverVersion, v)
		}

		return v
	case string:
		if gstypOid == oid.T_bytea {
			return encodeBytea(parameterStatus.serverVersion, []byte(v))
		}

		return []byte(v)
	case bool:
		return strconv.AppendBool(nil, v)
	case time.Time:
		return formatTs(v)

	default:
		errorf("encode: unknown type for %T", v)
	}

	panic("not reached")
}

var errInvalidTimestamp = errors.New("invalid timestamp")

type timestampParser struct {
	err error
}

// The location cache caches the time zones typically used by the client.
type locationCache struct {
	cache map[int]*time.Location
	lock  sync.Mutex
}

var infinityTsEnabled = false
var infinityTsNegative time.Time
var infinityTsPositive time.Time

const (
	infinityTsEnabledAlready        = "pq: infinity timestamp enabled already"
	infinityTsNegativeMustBeSmaller = "pq: infinity timestamp: negative value must be smaller (before) than positive"
)

// EnableInfinityTs controls the handling of GaussDB' "-infinity" and
// "infinity" "timestamp"s.
//
// If EnableInfinityTs is not called, "-infinity" and "infinity" will return
// []byte("-infinity") and []byte("infinity") respectively, and potentially
// cause error "sql: Scan error on column index 0: unsupported driver -> Scan
// pair: []uint8 -> *time.Time", when scanning into a time.Time value.
//
// Once EnableInfinityTs has been called, all connections created using this
// driver will decode GaussDB' "-infinity" and "infinity" for "timestamp",
// "timestamp with time zone" and "date" types to the predefined minimum and
// maximum times, respectively.  When encoding time.Time values, any time which
// equals or precedes the predefined minimum time will be encoded to
// "-infinity".  Any values at or past the maximum time will similarly be
// encoded to "infinity".
//
// If EnableInfinityTs is called with negative >= positive, it will panic.
// Calling EnableInfinityTs after a connection has been established results in
// undefined behavior.  If EnableInfinityTs is called more than once, it will
// panic.
func EnableInfinityTs(negative time.Time, positive time.Time) {
	if infinityTsEnabled {
		panic(infinityTsEnabledAlready)
	}
	if !negative.Before(positive) {
		panic(infinityTsNegativeMustBeSmaller)
	}
	infinityTsEnabled = true
	infinityTsNegative = negative
	infinityTsPositive = positive
}

// formatTs formats t into a format GaussDB understands.
func formatTs(t time.Time) []byte {
	if infinityTsEnabled {
		// t <= -infinity : ! (t > -infinity)
		if !t.After(infinityTsNegative) {
			return []byte("-infinity")
		}
		// t >= infinity : ! (!t < infinity)
		if !t.Before(infinityTsPositive) {
			return []byte("infinity")
		}
	}
	return FormatTimestamp(t)
}

// FormatTimestamp formats t into GaussDB' text format for timestamps.
func FormatTimestamp(t time.Time) []byte {
	// Need to send dates before 0001 A.D. with " BC" suffix, instead of the
	// minus sign preferred by Go.
	// Beware, "0000" in ISO is "1 BC", "-0001" is "2 BC" and so on
	bc := false
	if t.Year() <= 0 {
		// flip year sign, and add 1, e.g: "0" will be "1", and "-10" will be "11"
		t = t.AddDate((-t.Year())*2+1, 0, 0)
		bc = true
	}
	b := []byte(t.Format("2006-01-02 15:04:05.999999999Z07:00"))

	_, offset := t.Zone()
	offset %= 60
	if offset != 0 {
		// RFC3339Nano already printed the minus sign
		if offset < 0 {
			offset = -offset
		}

		b = append(b, ':')
		if offset < 10 {
			b = append(b, '0')
		}
		b = strconv.AppendInt(b, int64(offset), 10)
	}

	if bc {
		b = append(b, " BC"...)
	}
	return b
}

// Parse a bytea value received from the server.  Both "hex" and the legacy
// "escape" format are supported.
func parseBytea(s []byte) (result []byte, err error) {
	if len(s) >= 2 && bytes.Equal(s[:2], []byte("\\x")) {
		// bytea_output = hex
		s = s[2:] // trim off leading "\\x"
		result = make([]byte, hex.DecodedLen(len(s)))
		_, err := hex.Decode(result, s)
		if err != nil {
			return nil, err
		}
	} else {
		// bytea_output = escape
		for len(s) > 0 {
			if s[0] == '\\' {
				// escaped '\\'
				if len(s) >= 2 && s[1] == '\\' {
					result = append(result, '\\')
					s = s[2:]
					continue
				}

				// '\\' followed by an octal number
				if len(s) < 4 {
					return nil, fmt.Errorf("invalid bytea sequence %v", s)
				}
				r, err := strconv.ParseUint(string(s[1:4]), 8, 8)
				if err != nil {
					return nil, fmt.Errorf("could not parse bytea value: %s", err.Error())
				}
				result = append(result, byte(r))
				s = s[4:]
			} else {
				// We hit an unescaped, raw byte.  Try to read in as many as
				// possible in one go.
				i := bytes.IndexByte(s, '\\')
				if i == -1 {
					result = append(result, s...)
					break
				}
				result = append(result, s[:i]...)
				s = s[i:]
			}
		}
	}

	return result, nil
}

func encodeBytea(serverVersion int, v []byte) (result []byte) {
	if serverVersion >= 90000 {
		// Use the hex format if we know that the server supports it
		result = make([]byte, 2+hex.EncodedLen(len(v)))
		result[0] = '\\'
		result[1] = 'x'
		hex.Encode(result[2:], v)
	} else {
		// .. or resort to "escape"
		for _, b := range v {
			if b == '\\' {
				result = append(result, '\\', '\\')
			} else if b < 0x20 || b > 0x7e {
				result = append(result, []byte(fmt.Sprintf("\\%03o", b))...)
			} else {
				result = append(result, b)
			}
		}
	}

	return result
}

// NullTime represents a time.Time that may be null. NullTime implements the
// sql.Scanner interface so it can be used as a scan destination, similar to
// sql.NullString.
type NullTime struct {
	Time  time.Time
	Valid bool // Valid is true if Time is not NULL
}

// Scan implements the Scanner interface.
func (nt *NullTime) Scan(value interface{}) error {
	nt.Time, nt.Valid = value.(time.Time)
	return nil
}

// Value implements the driver Valuer interface.
func (nt NullTime) Value() (driver.Value, error) {
	if !nt.Valid {
		return nil, nil
	}
	return nt.Time, nil
}
