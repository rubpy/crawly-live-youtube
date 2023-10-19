package xmlapi

import (
	"net/url"
	"time"
)

//////////////////////////////////////////////////

func isValidURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}

	if u.Scheme == "" {
		return false
	}

	return true
}

// Parses a date-and-time string (RFC3389) as formatted by YouTube at various
// endpoints, and returns the corresponding timestamp (in milliseconds).
//
// On parsing failure, ok is false.
func parseDate(datetime string) (timestamp int64, ok bool) {
	t, err := time.Parse(time.RFC3339, datetime)
	if err != nil {
		return 0, false
	}

	return t.UnixMilli(), true
}
