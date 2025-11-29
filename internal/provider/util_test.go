package provider

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestInterfaceSliceToStrSlice(t *testing.T) {
	interfaceSlice := []any{"foo", "bar"}

	stringSlice := interfaceSliceToStrSlice(interfaceSlice)

	if want, got := 2, len(stringSlice); want != got {
		t.Errorf("wanted length of %d, got %d", want, got)
	}

	want := []string{"foo", "bar"}
	if diff := cmp.Diff(want, stringSlice); diff != "" {
		t.Errorf("MakeGatewayInfo() mismatch (-want +got):\n%s", diff)
	}
}
