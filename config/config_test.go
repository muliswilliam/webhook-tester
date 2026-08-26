package config_test

import (
	"os"
	"path/filepath"
	"testing"

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
