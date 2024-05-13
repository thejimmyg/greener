package greener

import (
	"testing"
)

// TestLogger implements Logger.
type TestLogger struct {
	logf func(string, ...interface{})
}

func (cl *TestLogger) Logf(m string, a ...interface{}) {
	cl.logf(m, a...)
}

func (cl *TestLogger) Errorf(m string, a ...interface{}) {
	cl.logf("ERROR: "+m, a...)
}

func NewTestLogger(t *testing.T) *TestLogger {
	return &TestLogger{logf: t.Logf}
}
