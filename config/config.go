package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

func LoadEnv() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env found - using defaults")
	}
}

// Defaults for the forwarding settings.
const (
	DefaultForwardTimeout       = 10 * time.Second
	DefaultForwardMaxConcurrent = 32
)

// Forwarding holds the settings for relaying captured requests to their
// webhook's forward URL.
type Forwarding struct {
	// AllowPrivateNetworks lets forwards reach private, loopback, link-local
	// and unspecified addresses. Keep it off on a public instance.
	AllowPrivateNetworks bool
	// Timeout bounds a single forward, from dialing to reading the response.
	Timeout time.Duration
	// MaxConcurrent bounds the automatic forwards in flight at once.
	MaxConcurrent int
}

// ForwardingFromEnv reads FORWARD_ALLOW_PRIVATE_NETWORKS, FORWARD_TIMEOUT (a
// Go duration such as "10s") and FORWARD_MAX_CONCURRENT, using the defaults
// for unset or empty ones. It returns an error naming the first invalid one.
func ForwardingFromEnv() (Forwarding, error) {
	f := Forwarding{Timeout: DefaultForwardTimeout, MaxConcurrent: DefaultForwardMaxConcurrent}

	if v := os.Getenv("FORWARD_ALLOW_PRIVATE_NETWORKS"); v != "" {
		allow, err := strconv.ParseBool(v)
		if err != nil {
			return Forwarding{}, fmt.Errorf("FORWARD_ALLOW_PRIVATE_NETWORKS must be true or false, got %q", v)
		}
		f.AllowPrivateNetworks = allow
	}
	if v := os.Getenv("FORWARD_TIMEOUT"); v != "" {
		timeout, err := time.ParseDuration(v)
		if err != nil || timeout <= 0 {
			return Forwarding{}, fmt.Errorf("FORWARD_TIMEOUT must be a positive duration such as 10s, got %q", v)
		}
		f.Timeout = timeout
	}
	if v := os.Getenv("FORWARD_MAX_CONCURRENT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return Forwarding{}, fmt.Errorf("FORWARD_MAX_CONCURRENT must be a whole number of at least 1, got %q", v)
		}
		f.MaxConcurrent = n
	}
	return f, nil
}
