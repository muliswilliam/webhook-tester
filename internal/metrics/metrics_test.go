package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Each registry gets its own registration, so registering again - another
// server in the same process, or a test run with -count>1 - doesn't panic.
func TestRegister(t *testing.T) {
	for range 2 {
		reg := prometheus.NewRegistry()
		require.NotPanics(t, func() { Register(reg) })

		families, err := reg.Gather()
		require.NoError(t, err)
		var names []string
		for _, f := range families {
			names = append(names, f.GetName())
		}
		assert.Contains(t, names, "webhooks_created_total")
		assert.Contains(t, names, "webhook_delivery_duration_seconds")
	}
}

func TestRegister_TwiceInOneRegistryPanics(t *testing.T) {
	reg := prometheus.NewRegistry()
	Register(reg)
	assert.Panics(t, func() { Register(reg) })
}
