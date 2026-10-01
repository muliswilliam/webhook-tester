package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"webhook-tester/config"
)

func TestLoadEnv_NoDotEnvFile(t *testing.T) {
	t.Chdir(t.TempDir())

	require.NotPanics(t, func() {
		config.LoadEnv()
	})
}

func TestLoadEnv_WithDotEnvFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	envFile := filepath.Join(dir, ".env")
	require.NoError(t, os.WriteFile(envFile, []byte("FOO=bar\n"), 0o644))

	config.LoadEnv()

	require.Equal(t, "bar", os.Getenv("FOO"))
}

var forwardEnvVars = []string{"FORWARD_ALLOW_PRIVATE_NETWORKS", "FORWARD_TIMEOUT", "FORWARD_MAX_CONCURRENT"}

func clearForwardEnv(t *testing.T) {
	t.Helper()
	for _, k := range forwardEnvVars {
		t.Setenv(k, "")
	}
}

func TestForwardingFromEnv_Defaults(t *testing.T) {
	clearForwardEnv(t)

	f, err := config.ForwardingFromEnv()

	require.NoError(t, err)
	require.Equal(t, config.Forwarding{
		AllowPrivateNetworks: false,
		Timeout:              config.DefaultForwardTimeout,
		MaxConcurrent:        config.DefaultForwardMaxConcurrent,
	}, f)
}

func TestForwardingFromEnv_Set(t *testing.T) {
	t.Setenv("FORWARD_ALLOW_PRIVATE_NETWORKS", "true")
	t.Setenv("FORWARD_TIMEOUT", "2500ms")
	t.Setenv("FORWARD_MAX_CONCURRENT", "8")

	f, err := config.ForwardingFromEnv()

	require.NoError(t, err)
	require.Equal(t, config.Forwarding{AllowPrivateNetworks: true, Timeout: 2500 * time.Millisecond, MaxConcurrent: 8}, f)
}

func TestForwardingFromEnv_Invalid(t *testing.T) {
	cases := map[string][2]string{
		"allow private not a bool":    {"FORWARD_ALLOW_PRIVATE_NETWORKS", "yes please"},
		"timeout without a unit":      {"FORWARD_TIMEOUT", "10"},
		"timeout zero":                {"FORWARD_TIMEOUT", "0s"},
		"timeout negative":            {"FORWARD_TIMEOUT", "-1s"},
		"max concurrent not a number": {"FORWARD_MAX_CONCURRENT", "many"},
		"max concurrent zero":         {"FORWARD_MAX_CONCURRENT", "0"},
		"max concurrent negative":     {"FORWARD_MAX_CONCURRENT", "-3"},
	}
	for name, kv := range cases {
		t.Run(name, func(t *testing.T) {
			clearForwardEnv(t)
			t.Setenv(kv[0], kv[1])

			_, err := config.ForwardingFromEnv()

			require.ErrorContains(t, err, kv[0])
		})
	}
}
