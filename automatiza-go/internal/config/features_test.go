package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFeatureFlags_Defaults(t *testing.T) {
	// Reset global state before test
	ResetFeatureFlags()

	// Clear any existing env vars
	os.Unsetenv("FEATURE_USE_GO_DISPATCH_LOGS")
	os.Unsetenv("FEATURE_USE_GO_RECONCILIATION")

	flags := LoadFeatureFlags()

	// Default values should be true
	if !flags.UseGoDispatchLogs {
		t.Error("UseGoDispatchLogs should default to true")
	}

	if !flags.UseGoReconciliation {
		t.Error("UseGoReconciliation should default to true")
	}
}

func TestLoadFeatureFlags_FromEnv(t *testing.T) {
	testCases := []struct {
		name                   string
		envDispatchLogs        string
		envReconciliation      string
		expectedDispatchLogs   bool
		expectedReconciliation bool
	}{
		{
			name:                   "both false",
			envDispatchLogs:        "false",
			envReconciliation:      "false",
			expectedDispatchLogs:   false,
			expectedReconciliation: false,
		},
		{
			name:                   "both true",
			envDispatchLogs:        "true",
			envReconciliation:      "true",
			expectedDispatchLogs:   true,
			expectedReconciliation: true,
		},
		{
			name:                   "dispatch logs false, reconciliation true",
			envDispatchLogs:        "false",
			envReconciliation:      "true",
			expectedDispatchLogs:   false,
			expectedReconciliation: true,
		},
		{
			name:                   "accepts 0 as false",
			envDispatchLogs:        "0",
			envReconciliation:      "0",
			expectedDispatchLogs:   false,
			expectedReconciliation: false,
		},
		{
			name:                   "accepts 1 as true",
			envDispatchLogs:        "1",
			envReconciliation:      "1",
			expectedDispatchLogs:   true,
			expectedReconciliation: true,
		},
		{
			name:                   "invalid value defaults to true",
			envDispatchLogs:        "invalid",
			envReconciliation:      "not-a-bool",
			expectedDispatchLogs:   true,
			expectedReconciliation: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset global state before each test
			ResetFeatureFlags()

			// Set environment variables
			os.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", tc.envDispatchLogs)
			os.Setenv("FEATURE_USE_GO_RECONCILIATION", tc.envReconciliation)

			flags := LoadFeatureFlags()

			if flags.UseGoDispatchLogs != tc.expectedDispatchLogs {
				t.Errorf("UseGoDispatchLogs: got %v, want %v", flags.UseGoDispatchLogs, tc.expectedDispatchLogs)
			}

			if flags.UseGoReconciliation != tc.expectedReconciliation {
				t.Errorf("UseGoReconciliation: got %v, want %v", flags.UseGoReconciliation, tc.expectedReconciliation)
			}
		})
	}

	// Cleanup
	os.Unsetenv("FEATURE_USE_GO_DISPATCH_LOGS")
	os.Unsetenv("FEATURE_USE_GO_RECONCILIATION")
}

func TestFeatureFlags_ShouldCreateDispatchLog(t *testing.T) {
	flags := &FeatureFlags{UseGoDispatchLogs: true}
	if !flags.ShouldCreateDispatchLog() {
		t.Error("ShouldCreateDispatchLog should return true when UseGoDispatchLogs is true")
	}

	flags.UseGoDispatchLogs = false
	if flags.ShouldCreateDispatchLog() {
		t.Error("ShouldCreateDispatchLog should return false when UseGoDispatchLogs is false")
	}
}

func TestFeatureFlags_ShouldReconcile(t *testing.T) {
	flags := &FeatureFlags{UseGoReconciliation: true}
	if !flags.ShouldReconcile() {
		t.Error("ShouldReconcile should return true when UseGoReconciliation is true")
	}

	flags.UseGoReconciliation = false
	if flags.ShouldReconcile() {
		t.Error("ShouldReconcile should return false when UseGoReconciliation is false")
	}
}

func TestGetFeatureFlags_Singleton(t *testing.T) {
	// Reset global state
	ResetFeatureFlags()

	os.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "false")

	// First call should load and return the flags
	flags1 := GetFeatureFlags()
	if flags1.UseGoDispatchLogs != false {
		t.Error("First call should return loaded flags with UseGoDispatchLogs=false")
	}

	// Change env var (should not affect cached value)
	os.Setenv("FEATURE_USE_GO_DISPATCH_LOGS", "true")

	// Second call should return the same cached instance
	flags2 := GetFeatureFlags()
	if flags2.UseGoDispatchLogs != false {
		t.Error("Second call should return cached flags with UseGoDispatchLogs=false")
	}

	// Verify it's the same instance
	if flags1 != flags2 {
		t.Error("GetFeatureFlags should return the same instance")
	}

	// Cleanup
	os.Unsetenv("FEATURE_USE_GO_DISPATCH_LOGS")
}

// =============================================================================
// GROUP_MEMBER_TRACKING_ENABLED
// =============================================================================

func TestGroupMemberTrackingEnabled(t *testing.T) {
	cases := []struct {
		name  string
		value string // "" = variável não definida
		set   bool
		want  bool
	}{
		{name: "não definida = desligada", set: false, want: false},
		{name: "false explícito", value: "false", set: true, want: false},
		{name: "true explícito", value: "true", set: true, want: true},
		{name: "1 liga", value: "1", set: true, want: true},
		{name: "0 desliga", value: "0", set: true, want: false},
		// Valor inválido cai no default seguro (desligado): uma variável mal
		// digitada não pode ligar escrita nova em produção sem querer.
		{name: "inválido cai no default", value: "talvez", set: true, want: false},
		{name: "string vazia cai no default", value: "", set: true, want: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.set {
				t.Setenv("GROUP_MEMBER_TRACKING_ENABLED", c.value)
			} else {
				t.Setenv("GROUP_MEMBER_TRACKING_ENABLED", "")
			}
			ResetFeatureFlags()
			t.Cleanup(ResetFeatureFlags)

			assert.Equal(t, c.want, GroupMemberTrackingEnabled())
		})
	}
}

// ResetFeatureFlags precisa re-ler o ambiente: sem isso um teste que liga a
// flag contaminaria todos os seguintes (a flag é sync.OnceValue).
func TestGroupMemberTracking_ResetFeatureFlagsRereadsEnv(t *testing.T) {
	t.Setenv("GROUP_MEMBER_TRACKING_ENABLED", "true")
	ResetFeatureFlags()
	require.True(t, GroupMemberTrackingEnabled())

	t.Setenv("GROUP_MEMBER_TRACKING_ENABLED", "false")
	ResetFeatureFlags()
	t.Cleanup(ResetFeatureFlags)
	assert.False(t, GroupMemberTrackingEnabled())
}
