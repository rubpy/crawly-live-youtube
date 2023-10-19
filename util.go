package youtube

import (
	"fmt"
	"math/rand"
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

// URL query param key used in conjuction with value returned by generateNonce.
var nonceKey = "_h"

// Generates a time-based unique string, generally used for signing HTTP
// requests (to bypass caching mechanisms).
func generateNonce() string {
	return uniqueHex()
}

func uniqueHex() string {
	return fmt.Sprintf("%016x", uniqueUint64())
}

func uniqueUint64() uint64 {
	v := uint64(time.Now().UnixMilli())
	r := uint64(rand.Uint32()) & 0x3fffff
	v = (v << 22) | r

	return v
}
