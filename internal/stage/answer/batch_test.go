package answer

import "testing"

func TestNormalizeForbiddenToolNamesReturnsEmptySliceForNilInput(t *testing.T) {
	got := normalizeForbiddenToolNames(nil)

	if got == nil {
		t.Fatalf("expected empty slice, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %v", got)
	}
}

func TestNormalizeForbiddenToolNamesReturnsEmptySliceWhenValuesNormalizeAway(t *testing.T) {
	got := normalizeForbiddenToolNames([]string{"", "   "})

	if got == nil {
		t.Fatalf("expected empty slice, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %v", got)
	}
}
