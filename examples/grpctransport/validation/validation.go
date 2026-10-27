// Package validation collects request field violations and converts them into
// a gRPC InvalidArgument status carrying google.rpc.BadRequest details, so
// clients can correct every field in one round trip.
package validation

import (
	"fmt"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Reason is a stable, closed classification of a violation. Clients and tests
// branch on it; the description is only for humans.
type Reason string

// Violation reasons.
const (
	Required     Reason = "REQUIRED"
	Invalid      Reason = "INVALID"
	OutOfRange   Reason = "OUT_OF_RANGE"
	TooLong      Reason = "TOO_LONG"
	Duplicate    Reason = "DUPLICATE"
	NotSupported Reason = "NOT_SUPPORTED"
)

// Violation is one invalid field.
type Violation struct {
	// Field is the path into the request, for example "lines[2].quantity".
	Field  string
	Reason Reason
	// Description is safe remediation text; never echo secrets or raw input.
	Description string
}

// Violations accumulates independent field problems. The zero value is ready
// to use.
type Violations []Violation

// Add records a violation.
func (v *Violations) Add(field string, reason Reason, description string) {
	*v = append(*v, Violation{Field: field, Reason: reason, Description: description})
}

// Index returns the path of element i of a repeated field.
func Index(field string, i int) string { return fmt.Sprintf("%s[%d]", field, i) }

// Err returns nil when there are no violations, otherwise an InvalidArgument
// status with one BadRequest field violation per entry.
func (v Violations) Err() error {
	if len(v) == 0 {
		return nil
	}
	details := &errdetails.BadRequest{}
	for _, violation := range v {
		details.FieldViolations = append(details.FieldViolations, &errdetails.BadRequest_FieldViolation{
			Field:       violation.Field,
			Reason:      string(violation.Reason),
			Description: violation.Description,
		})
	}
	st, err := status.New(codes.InvalidArgument, "request has invalid fields").WithDetails(details)
	if err != nil {
		return status.Error(codes.InvalidArgument, "request has invalid fields")
	}
	return st.Err()
}
