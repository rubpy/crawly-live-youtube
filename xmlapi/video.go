package xmlapi

//////////////////////////////////////////////////

// Checks (roughly) if the given string is a valid YouTube video ID.
func IsValidVideoID(s string) bool {
	n := len(s)
	if n < 6 || n > 48 {
		return false
	}

	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')) {
			if r == '-' || r == '_' {
				continue
			}

			return false
		}
	}

	return true
}
