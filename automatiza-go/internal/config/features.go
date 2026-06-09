// Package config provides application configuration.
// This file implements feature flags for gradual migration control.
package config

import (
	"os"
	"strconv"
	"sync"

	"go.uber.org/zap"
)

// FeatureFlags holds all feature flag configurations for the application.
// These flags enable gradual migration from Node.js to Go and provide
// rollback capability if issues are detected.
type FeatureFlags struct {
	// UseGoDispatchLogs controls whether Go creates dispatch_logs in PostgreSQL.
	// When true (default), the Go backend creates dispatch logs.
	// When false, dispatch log creation is skipped.
	// Environment variable: FEATURE_USE_GO_DISPATCH_LOGS
	UseGoDispatchLogs bool

	// UseGoReconciliation controls whether Go performs message reconciliation.
	// When true, the Go reconciliation service creates dispatch logs for missed messages.
	// When false, reconciliation dispatch log creation is skipped.
	// Environment variable: FEATURE_USE_GO_RECONCILIATION
	UseGoReconciliation bool

	// UseMultiInstanceDispatch controls per-group routing in the dispatch layer.
	// When true, each message is sent through the WhatsApp instance that owns the
	// destination group (groups.instance_id), falling back to the user's first
	// connected instance only when the group's own chip is disconnected.
	// When false (default), legacy behavior is preserved: every message goes
	// through the user's first connected instance regardless of group ownership.
	// Defaulting to false provides a rollback path during the multi-instance
	// rollout.
	// Environment variable: FEATURE_USE_MULTI_INSTANCE_DISPATCH
	UseMultiInstanceDispatch bool

	// EnforceGroupLimits gates the per-chip group quota check inside an
	// automation's send_groups / monitor_group_ids lists. When true, the
	// handler rejects payloads where a single chip would host more than
	// quota.MaxGroupsPerInstance groups. When false (default), the check is
	// skipped and the user may pile any number of groups onto one chip — the
	// frontend is expected to honor the same flag so the UI matches.
	// Flip to true to re-enable the WhatsApp-ban-protection cap.
	// Environment variable: FEATURE_ENFORCE_GROUP_LIMITS
	EnforceGroupLimits bool

	// EnforcePlanGroupLimits gates the per-PLAN total group cap: a user may only
	// target up to their plan's group_limit distinct groups across all their
	// active automations (send_groups). When true, Create/Update automation is
	// rejected if it would push the distinct-group total over the limit. When
	// false (default), no plan limit is enforced — existing users are unaffected
	// (grandfathered), so this can ship dark and be flipped on after validation.
	// Environment variable: FEATURE_ENFORCE_PLAN_GROUP_LIMITS
	EnforcePlanGroupLimits bool
}

var (
	// globalFeatureFlags is the singleton instance of feature flags
	globalFeatureFlags *FeatureFlags
	featureFlagsOnce   sync.Once
	featureFlagsLogger *zap.Logger
)

// SetFeatureFlagsLogger sets the logger for feature flag operations.
// Should be called early in application startup.
func SetFeatureFlagsLogger(logger *zap.Logger) {
	featureFlagsLogger = logger
}

// LoadFeatureFlags loads feature flags from environment variables.
// This function is safe to call multiple times; it will only load once.
// Subsequent calls return the cached instance.
func LoadFeatureFlags() *FeatureFlags {
	featureFlagsOnce.Do(func() {
		globalFeatureFlags = &FeatureFlags{
			UseGoDispatchLogs:        getEnvBool("FEATURE_USE_GO_DISPATCH_LOGS", true),
			UseGoReconciliation:      getEnvBool("FEATURE_USE_GO_RECONCILIATION", true),
			UseMultiInstanceDispatch: getEnvBool("FEATURE_USE_MULTI_INSTANCE_DISPATCH", false),
			EnforceGroupLimits:       getEnvBool("FEATURE_ENFORCE_GROUP_LIMITS", false),
			EnforcePlanGroupLimits:   getEnvBool("FEATURE_ENFORCE_PLAN_GROUP_LIMITS", false),
		}

		// Log feature flag values on load
		if featureFlagsLogger != nil {
			featureFlagsLogger.Info("Feature flags loaded",
				zap.Bool("useGoDispatchLogs", globalFeatureFlags.UseGoDispatchLogs),
				zap.Bool("useGoReconciliation", globalFeatureFlags.UseGoReconciliation),
				zap.Bool("useMultiInstanceDispatch", globalFeatureFlags.UseMultiInstanceDispatch),
				zap.Bool("enforceGroupLimits", globalFeatureFlags.EnforceGroupLimits),
				zap.Bool("enforcePlanGroupLimits", globalFeatureFlags.EnforcePlanGroupLimits))
		}
	})

	return globalFeatureFlags
}

// GetFeatureFlags returns the current feature flags instance.
// If LoadFeatureFlags has not been called, it will load them first.
func GetFeatureFlags() *FeatureFlags {
	if globalFeatureFlags == nil {
		return LoadFeatureFlags()
	}
	return globalFeatureFlags
}

// ShouldCreateDispatchLog returns true if Go should create dispatch logs.
// This is a convenience method that checks the UseGoDispatchLogs flag.
func (f *FeatureFlags) ShouldCreateDispatchLog() bool {
	return f.UseGoDispatchLogs
}

// ShouldReconcile returns true if Go should perform reconciliation.
// This is a convenience method that checks the UseGoReconciliation flag.
func (f *FeatureFlags) ShouldReconcile() bool {
	return f.UseGoReconciliation
}

// getEnvBool reads a boolean value from an environment variable.
// Returns the default value if the variable is not set or cannot be parsed.
// Accepted values: "true", "false", "1", "0", "yes", "no" (case-insensitive)
func getEnvBool(key string, defaultVal bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}

	b, err := strconv.ParseBool(val)
	if err != nil {
		// Log warning if logger is available
		if featureFlagsLogger != nil {
			featureFlagsLogger.Warn("Invalid boolean value for feature flag, using default",
				zap.String("key", key),
				zap.String("value", val),
				zap.Bool("default", defaultVal))
		}
		return defaultVal
	}

	return b
}

// ResetFeatureFlags resets the global feature flags instance.
// This is primarily useful for testing.
func ResetFeatureFlags() {
	globalFeatureFlags = nil
	featureFlagsOnce = sync.Once{}
}
