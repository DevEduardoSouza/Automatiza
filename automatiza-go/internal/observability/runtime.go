package observability

import "runtime"

// runtimeCallerImpl wraps runtime.Caller to enable test overrides via runtimeCaller var.
func runtimeCallerImpl(skip int) (pc uintptr, file string, line int, ok bool) {
	return runtime.Caller(skip)
}
