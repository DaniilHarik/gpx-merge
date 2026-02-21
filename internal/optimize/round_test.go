package optimize

import "testing"

func TestRoundToPrecision(t *testing.T) {
	t.Parallel()

	got := RoundToPrecision(58.123456789, 6)
	if got != 58.123457 {
		t.Fatalf("RoundToPrecision() = %v", got)
	}

	got = RoundToPrecision(-123.0000004, 6)
	if got != -123 {
		t.Fatalf("RoundToPrecision() negative = %v", got)
	}
}

func TestFormatCoordinate(t *testing.T) {
	t.Parallel()

	if got := FormatCoordinate(24.1000001, 6); got != "24.1" {
		t.Fatalf("FormatCoordinate() = %q", got)
	}
	if got := FormatCoordinate(-0.00000001, 6); got != "0" {
		t.Fatalf("FormatCoordinate(-0) = %q", got)
	}
	if got := FormatCoordinate(-58.1234, 3); got != "-58.123" {
		t.Fatalf("FormatCoordinate(negative) = %q", got)
	}
}
