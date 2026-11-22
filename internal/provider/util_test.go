package provider

import (
	"slices"
	"testing"
)

func TestInterfaceSliceToStrSlice(t *testing.T) {
	want := []string{"foo", "bar"}

	got := interfaceSliceToStrSlice([]any{"foo", "bar"})

	if !slices.Equal(want, got) {
		t.Errorf("interfaceSliceToStrSlice() = %q, want %q", got, want)
	}
}
