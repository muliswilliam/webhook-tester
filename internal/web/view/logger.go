package view

import "log"

// logger receives rendering errors. It defaults to the standard logger so
// the package works in tests; the server points it at its own logger.
var logger = log.Default()

// SetLogger sets the logger for rendering errors. Call it once at startup,
// before serving requests.
func SetLogger(l *log.Logger) {
	logger = l
}
