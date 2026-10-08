package bridge

import (
	"strings"
	"testing"
)

// TestInterceptMediaPayloadRoundTrip drives a live string across the
// CGo -> Rust staticlib boundary and back, exercising CString ownership
// on both sides of the seam: Go allocates the input, Rust reclaims the
// output via free_rust_string, and no panic may escape the FFI frame.
func TestInterceptMediaPayloadRoundTrip(t *testing.T) {
	const input = `E:\rips\structural-sample.vob`

	out, err := InterceptMediaPayload(input)
	if err != nil {
		t.Fatalf("boundary call failed: %v", err)
	}
	if out == "" {
		t.Fatal("boundary returned an empty payload")
	}
	if !strings.Contains(out, "Rust Processing Validated") {
		t.Fatalf("payload missing validation marker: %q", out)
	}
	if !strings.Contains(out, "structural-sample.vob") {
		t.Fatalf("input did not cross the boundary intact: %q", out)
	}
}

// TestInterceptMediaPayloadEmptyInput verifies the empty-string case does
// not panic and still returns a well-formed payload across the seam.
func TestInterceptMediaPayloadEmptyInput(t *testing.T) {
	out, err := InterceptMediaPayload("")
	if err != nil {
		t.Fatalf("boundary call failed on empty input: %v", err)
	}
	if out == "" {
		t.Fatal("expected a validation marker even for empty input")
	}
}
