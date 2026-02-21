package optimize

import (
	"math"
	"strconv"
	"strings"
)

func RoundToPrecision(v float64, digits int) float64 {
	if digits < 0 {
		return v
	}
	pow := math.Pow(10, float64(digits))
	return math.Round(v*pow) / pow
}

func FormatCoordinate(v float64, digits int) string {
	rounded := RoundToPrecision(v, digits)
	s := strconv.FormatFloat(rounded, 'f', digits, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "-0" || s == "" {
		return "0"
	}
	return s
}
