package observability

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRuntimeCallerImpl_ReturnsCallerInfo(t *testing.T) {
	pc, file, line, ok := runtimeCallerImpl(0)
	assert.True(t, ok, "runtimeCallerImpl must succeed")
	assert.NotEmpty(t, file, "file must be non-empty")
	assert.Greater(t, line, 0, "line number must be positive")
	assert.NotZero(t, pc, "program counter must be non-zero")
}

func TestRuntimeCallerImpl_SkipZero_ReturnsRuntimeGoFrame(t *testing.T) {
	// skip=0 → runtime.Caller(0) returns the frame of runtimeCallerImpl in runtime.go
	_, file, _, ok := runtimeCallerImpl(0)
	assert.True(t, ok)
	assert.Contains(t, file, "runtime.go")
}

func TestRuntimeCallerImpl_SkipOne_ReturnsCallerFrame(t *testing.T) {
	// skip=1 means skip runtimeCallerImpl itself and return its direct caller
	_, _, _, ok := runtimeCallerImpl(1)
	assert.True(t, ok)
}
