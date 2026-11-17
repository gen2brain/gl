// Package gltest creates headless OpenGL contexts for tests.
package gltest

// Config describes the requested context.
type Config struct {
	ES     bool
	Major  int
	Minor  int
	Compat bool
	Debug  bool

	Backend string
}
