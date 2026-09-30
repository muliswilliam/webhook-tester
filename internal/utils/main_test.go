package utils

import (
	"os"
	"testing"
)

// TestMain runs the tests from the repo root, where the server itself runs,
// so templates can resolve assets under static/.
func TestMain(m *testing.M) {
	if err := os.Chdir("../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}
