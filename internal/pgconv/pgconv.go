// Package pgconv converts between pgx/pgtype values and the plain string /
// protobuf types used across the service boundary.
package pgconv

import (
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// UUIDString renders a pgtype.UUID as its canonical string, or "" when null.
func UUIDString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	v, err := u.Value()
	if err != nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

// ParseUUID parses a canonical UUID string into a pgtype.UUID. An empty string
// yields a null (invalid) UUID with no error.
func ParseUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	if s == "" {
		return u, nil
	}
	if err := u.Scan(s); err != nil {
		return u, err
	}
	return u, nil
}

// PbTime converts a pgtype.Timestamptz to a protobuf timestamp, or nil if null.
func PbTime(t pgtype.Timestamptz) *timestamppb.Timestamp {
	if !t.Valid {
		return nil
	}
	return timestamppb.New(t.Time)
}

// Timestamptz converts a protobuf timestamp to a pgtype.Timestamptz; nil yields
// a null value.
func Timestamptz(ts *timestamppb.Timestamp) pgtype.Timestamptz {
	if ts == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: ts.AsTime(), Valid: true}
}
