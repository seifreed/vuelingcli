package vueling

import "testing"

func TestExtraNames(t *testing.T) {
	if got, ok := SeatTypeName("space"); !ok || got != "Space (extra legroom)" {
		t.Fatalf("SeatTypeName(space) = %q, %v", got, ok)
	}
	if got, ok := SeatTypeName("hidden"); ok || got != "" {
		t.Fatalf("SeatTypeName(hidden) = %q, %v", got, ok)
	}
	if got := ServiceName("PETC"); got != "Pet in the cabin" {
		t.Fatalf("ServiceName(PETC) = %q", got)
	}
	if got := ServiceName("unknown"); got != "unknown" {
		t.Fatalf("ServiceName(unknown) = %q", got)
	}
}

func TestBundleName(t *testing.T) {
	if got, ok := BundleName("VYBA"); !ok || got != "Fly Light" {
		t.Fatalf("BundleName(VYBA) = %q, %v", got, ok)
	}
	if got, ok := BundleName("VYOP"); ok || got != "" {
		t.Fatalf("BundleName(VYOP) = %q, %v", got, ok)
	}
}
