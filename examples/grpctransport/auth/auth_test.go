package auth

import "testing"

func TestFromContext(t *testing.T) {
	ctx := WithPrincipal(t.Context(), Principal{TenantID: "tenant-1", Subject: "user-1"})

	p, ok := FromContext(ctx)

	if !ok || p.TenantID != "tenant-1" || p.Subject != "user-1" {
		t.Fatalf("FromContext() = %+v, %v; want tenant-1/user-1, true", p, ok)
	}
}

func TestFromContextRejectsMissingOrIncompletePrincipal(t *testing.T) {
	if _, ok := FromContext(t.Context()); ok {
		t.Error("FromContext(empty) ok = true, want false")
	}
	if _, ok := FromContext(WithPrincipal(t.Context(), Principal{Subject: "user-1"})); ok {
		t.Error("FromContext(no tenant) ok = true, want false")
	}
}
