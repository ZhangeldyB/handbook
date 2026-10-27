package validation

import (
	"testing"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type lineInput struct {
	SKU      string
	Quantity int64
}

// validateLines walks every field and never stops at the first problem.
func validateLines(customerID string, lines []lineInput) error {
	var v Violations
	if customerID == "" {
		v.Add("customer_id", Required, "customer_id is required")
	}
	if len(lines) == 0 {
		v.Add("lines", Required, "at least one line is required")
	}
	for i, line := range lines {
		if line.SKU == "" {
			v.Add(Index("lines", i)+".sku", Required, "sku is required")
		}
		if line.Quantity <= 0 {
			v.Add(Index("lines", i)+".quantity", OutOfRange, "quantity must be positive")
		}
	}
	return v.Err()
}

func TestErrCarriesEveryFieldViolation(t *testing.T) {
	err := validateLines("", []lineInput{{SKU: "a", Quantity: 1}, {SKU: "", Quantity: 0}})

	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("code = %v, want %v", st.Code(), codes.InvalidArgument)
	}
	got := map[string]string{}
	for _, detail := range st.Details() {
		if br, ok := detail.(*errdetails.BadRequest); ok {
			for _, fv := range br.GetFieldViolations() {
				got[fv.GetField()] = fv.GetReason()
			}
		}
	}
	want := map[string]string{
		"customer_id":       string(Required),
		"lines[1].sku":      string(Required),
		"lines[1].quantity": string(OutOfRange),
	}
	if len(got) != len(want) {
		t.Fatalf("violations = %v, want %v", got, want)
	}
	for field, reason := range want {
		if got[field] != reason {
			t.Errorf("violation %s = %q, want %q", field, got[field], reason)
		}
	}
}

func TestErrIsNilWithoutViolations(t *testing.T) {
	if err := validateLines("customer-1", []lineInput{{SKU: "a", Quantity: 1}}); err != nil {
		t.Fatalf("Err() = %v, want nil", err)
	}
}
