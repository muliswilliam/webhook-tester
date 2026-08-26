package main

import (
	"testing"

	"github.com/robfig/cron"
	"github.com/stretchr/testify/require"
)

func TestScheduleCleanup(t *testing.T) {
	c := cron.New()

	require.NotPanics(t, func() {
		scheduleCleanup(nil, c)
	})

	require.Len(t, c.Entries(), 1)
}
