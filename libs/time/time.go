package time

import (
	"fmt"
	"strings"
	"time"
)

// ToYYYYMM converts "YYYY-MM" -> 202410.
func ToYYYYMM(period string) (int64, error) {
	s := strings.TrimSpace(period)
	t, err := time.Parse("2006-01", s) // strict: requires zero-padded month
	if err != nil {
		return 0, fmt.Errorf("invalid period %q (want YYYY-MM): %w", period, err)
	}
	y, m, _ := t.Date()
	return int64(y)*100 + int64(m), nil
}
