package parse

import (
	"strconv"
	"strings"
)

// Helper functions để parse data
func ParseFloat(s string) float64 {
	// Loại bỏ dấu phẩy và ký tự đặc biệt trong số Việt Nam
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, " ", "")

	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return val
}

func ParseInt64(s string) int64 {
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, " ", "")

	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return val
}
