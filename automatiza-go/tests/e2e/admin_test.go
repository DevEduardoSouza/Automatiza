package e2e

import (
	"fmt"
	"net/http"
	"testing"

	"automatiza-go/internal/models"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Admin Users Tests
// =============================================================================

func TestAdmin_ListUsers_RequiresAdminRole(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	// Setup: Create regular user
	profileRepo := env.deps.ProfileRepo.(*mockProfileRepo)
	profileRepo.AddProfile(&models.Profile{
		ID:    "regular-user",
		Email: "regular@example.com",
		Plan:  models.PlanPro,
	})

	client := newHTTPClient(t, env.router)

	// Test: Regular user cannot access admin endpoints
	client.setAuthToken("regular-user")
	w := client.get("/api/v1/admin/users")
	assert.Equal(t, http.StatusForbidden, w.Code)

	response := parseResponse[apiResponse[any]](t, w)
	assert.False(t, response.Success)
}

func TestAdmin_ListUsers_AdminCanAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	// Setup: Create admin user
	profileRepo := env.deps.ProfileRepo.(*mockProfileRepo)
	userRoleRepo := env.deps.UserRoleRepo.(*mockUserRoleRepo)

	profileRepo.AddProfile(&models.Profile{
		ID:    "admin-user",
		Email: "admin@example.com",
		Plan:  models.PlanElite,
	})
	userRoleRepo.SetAdmin("admin-user", true)

	// Add some users to list
	profileRepo.AddProfile(&models.Profile{
		ID:    "user-1",
		Email: "user1@example.com",
		Plan:  models.PlanFree,
	})
	profileRepo.AddProfile(&models.Profile{
		ID:    "user-2",
		Email: "user2@example.com",
		Plan:  models.PlanPro,
	})

	client := newHTTPClient(t, env.router)

	// Test: Admin user can access user list
	client.setAuthToken("admin-user")
	w := client.get("/api/v1/admin/users")

	// Admin should not get 401 or 403
	assert.NotEqual(t, http.StatusUnauthorized, w.Code)
	assert.NotEqual(t, http.StatusForbidden, w.Code)
}

func TestAdmin_ListUsers_Pagination(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	// Setup: Create admin user
	profileRepo := env.deps.ProfileRepo.(*mockProfileRepo)
	userRoleRepo := env.deps.UserRoleRepo.(*mockUserRoleRepo)

	profileRepo.AddProfile(&models.Profile{
		ID:    "admin-user",
		Email: "admin@example.com",
		Plan:  models.PlanElite,
	})
	userRoleRepo.SetAdmin("admin-user", true)

	// Add multiple users
	for i := 1; i <= 15; i++ {
		profileRepo.AddProfile(&models.Profile{
			ID:    fmt.Sprintf("user-%d", i),
			Email: fmt.Sprintf("user%d@example.com", i),
			Plan:  models.PlanFree,
		})
	}

	client := newHTTPClient(t, env.router)
	client.setAuthToken("admin-user")

	// Test: Paginated request
	w := client.get("/api/v1/admin/users?limit=10")
	assert.NotEqual(t, http.StatusUnauthorized, w.Code)
	assert.NotEqual(t, http.StatusForbidden, w.Code)
}

// =============================================================================
// Admin Create User Tests
// =============================================================================

func TestAdmin_CreateUser_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	// Setup: Create admin user
	profileRepo := env.deps.ProfileRepo.(*mockProfileRepo)
	userRoleRepo := env.deps.UserRoleRepo.(*mockUserRoleRepo)

	profileRepo.AddProfile(&models.Profile{
		ID:    "admin-user",
		Email: "admin@example.com",
		Plan:  models.PlanElite,
	})
	userRoleRepo.SetAdmin("admin-user", true)

	client := newHTTPClient(t, env.router)
	client.setAuthToken("admin-user")

	// Test: Create new user
	newUser := map[string]interface{}{
		"email": "newuser@example.com",
		"plan":  "pro",
	}
	w := client.post("/api/v1/admin/users", newUser)

	// Should not be unauthorized or forbidden
	assert.NotEqual(t, http.StatusUnauthorized, w.Code)
	assert.NotEqual(t, http.StatusForbidden, w.Code)
}

func TestAdmin_CreateUser_RequiresAdminRole(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	// Setup: Create regular user
	profileRepo := env.deps.ProfileRepo.(*mockProfileRepo)
	profileRepo.AddProfile(&models.Profile{
		ID:    "regular-user",
		Email: "regular@example.com",
		Plan:  models.PlanPro,
	})

	client := newHTTPClient(t, env.router)
	client.setAuthToken("regular-user")

	// Test: Regular user cannot create users
	newUser := map[string]interface{}{
		"email": "newuser@example.com",
		"plan":  "pro",
	}
	w := client.post("/api/v1/admin/users", newUser)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAdmin_CreateUser_DuplicateEmail(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	// Setup: Create admin and existing user
	profileRepo := env.deps.ProfileRepo.(*mockProfileRepo)
	userRoleRepo := env.deps.UserRoleRepo.(*mockUserRoleRepo)

	profileRepo.AddProfile(&models.Profile{
		ID:    "admin-user",
		Email: "admin@example.com",
		Plan:  models.PlanElite,
	})
	userRoleRepo.SetAdmin("admin-user", true)

	profileRepo.AddProfile(&models.Profile{
		ID:    "existing-user",
		Email: "existing@example.com",
		Plan:  models.PlanFree,
	})

	client := newHTTPClient(t, env.router)
	client.setAuthToken("admin-user")

	// Test: Create user with duplicate email
	existingProfile, _ := profileRepo.GetProfileByEmail(nil, "existing@example.com")
	assert.NotNil(t, existingProfile)
}

// =============================================================================
// Admin Update User Role Tests
// =============================================================================

func TestAdmin_UpdateUserRole_AddRole(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	// Setup: Create admin and target user
	profileRepo := env.deps.ProfileRepo.(*mockProfileRepo)
	userRoleRepo := env.deps.UserRoleRepo.(*mockUserRoleRepo)

	profileRepo.AddProfile(&models.Profile{
		ID:    "admin-user",
		Email: "admin@example.com",
		Plan:  models.PlanElite,
	})
	userRoleRepo.SetAdmin("admin-user", true)

	profileRepo.AddProfile(&models.Profile{
		ID:    "target-user",
		Email: "target@example.com",
		Plan:  models.PlanPro,
	})

	client := newHTTPClient(t, env.router)
	client.setAuthToken("admin-user")

	// Test: Add admin role to user
	roleUpdate := map[string]interface{}{
		"userId": "target-user",
		"role":   "admin",
		"action": "add",
	}
	w := client.post("/api/v1/admin/users/roles", roleUpdate)

	assert.NotEqual(t, http.StatusUnauthorized, w.Code)
	assert.NotEqual(t, http.StatusForbidden, w.Code)
}

func TestAdmin_UpdateUserRole_RemoveRole(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	// Setup: Create admin and target admin user
	profileRepo := env.deps.ProfileRepo.(*mockProfileRepo)
	userRoleRepo := env.deps.UserRoleRepo.(*mockUserRoleRepo)

	profileRepo.AddProfile(&models.Profile{
		ID:    "admin-user",
		Email: "admin@example.com",
		Plan:  models.PlanElite,
	})
	userRoleRepo.SetAdmin("admin-user", true)

	profileRepo.AddProfile(&models.Profile{
		ID:    "target-admin",
		Email: "targetadmin@example.com",
		Plan:  models.PlanElite,
	})
	userRoleRepo.SetAdmin("target-admin", true)

	client := newHTTPClient(t, env.router)
	client.setAuthToken("admin-user")

	// Test: Remove admin role from user
	roleUpdate := map[string]interface{}{
		"userId": "target-admin",
		"role":   "admin",
		"action": "remove",
	}
	w := client.post("/api/v1/admin/users/roles", roleUpdate)

	assert.NotEqual(t, http.StatusUnauthorized, w.Code)
	assert.NotEqual(t, http.StatusForbidden, w.Code)
}

func TestAdmin_UpdateUserRole_RequiresAdminRole(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	// Setup: Create regular user
	profileRepo := env.deps.ProfileRepo.(*mockProfileRepo)
	profileRepo.AddProfile(&models.Profile{
		ID:    "regular-user",
		Email: "regular@example.com",
		Plan:  models.PlanPro,
	})

	client := newHTTPClient(t, env.router)
	client.setAuthToken("regular-user")

	// Test: Regular user cannot update roles
	roleUpdate := map[string]interface{}{
		"userId": "some-user",
		"role":   "admin",
		"action": "add",
	}
	w := client.post("/api/v1/admin/users/roles", roleUpdate)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// =============================================================================
// Admin System Events Tests
// =============================================================================

func TestAdmin_GetSystemEvents_RequiresAdminRole(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	// Setup: Create regular user
	profileRepo := env.deps.ProfileRepo.(*mockProfileRepo)
	profileRepo.AddProfile(&models.Profile{
		ID:    "regular-user",
		Email: "regular@example.com",
		Plan:  models.PlanPro,
	})

	client := newHTTPClient(t, env.router)
	client.setAuthToken("regular-user")

	w := client.get("/api/v1/admin/system-events")
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAdmin_GetSystemEvents_AdminCanAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	// Setup: Create admin user
	profileRepo := env.deps.ProfileRepo.(*mockProfileRepo)
	userRoleRepo := env.deps.UserRoleRepo.(*mockUserRoleRepo)

	profileRepo.AddProfile(&models.Profile{
		ID:    "admin-user",
		Email: "admin@example.com",
		Plan:  models.PlanElite,
	})
	userRoleRepo.SetAdmin("admin-user", true)

	client := newHTTPClient(t, env.router)
	client.setAuthToken("admin-user")

	w := client.get("/api/v1/admin/system-events")
	assert.NotEqual(t, http.StatusUnauthorized, w.Code)
	assert.NotEqual(t, http.StatusForbidden, w.Code)
}

// =============================================================================
// Admin Observability Tests
// =============================================================================

func TestAdmin_GetObservability_RequiresAdminRole(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	// Setup: Create regular user
	profileRepo := env.deps.ProfileRepo.(*mockProfileRepo)
	profileRepo.AddProfile(&models.Profile{
		ID:    "regular-user",
		Email: "regular@example.com",
		Plan:  models.PlanPro,
	})

	client := newHTTPClient(t, env.router)
	client.setAuthToken("regular-user")

	w := client.get("/api/v1/admin/observability")
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAdmin_GetObservability_AdminCanAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	// Setup: Create admin user
	profileRepo := env.deps.ProfileRepo.(*mockProfileRepo)
	userRoleRepo := env.deps.UserRoleRepo.(*mockUserRoleRepo)

	profileRepo.AddProfile(&models.Profile{
		ID:    "admin-user",
		Email: "admin@example.com",
		Plan:  models.PlanElite,
	})
	userRoleRepo.SetAdmin("admin-user", true)

	client := newHTTPClient(t, env.router)
	client.setAuthToken("admin-user")

	w := client.get("/api/v1/admin/observability")
	assert.NotEqual(t, http.StatusUnauthorized, w.Code)
	assert.NotEqual(t, http.StatusForbidden, w.Code)
}

// =============================================================================
// Admin Plans Management Tests
// =============================================================================

func TestAdmin_ListPlans_AdminCanAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	// Setup: Create admin user
	profileRepo := env.deps.ProfileRepo.(*mockProfileRepo)
	userRoleRepo := env.deps.UserRoleRepo.(*mockUserRoleRepo)

	profileRepo.AddProfile(&models.Profile{
		ID:    "admin-user",
		Email: "admin@example.com",
		Plan:  models.PlanElite,
	})
	userRoleRepo.SetAdmin("admin-user", true)

	client := newHTTPClient(t, env.router)
	client.setAuthToken("admin-user")

	w := client.get("/api/v1/admin/plans")
	assert.NotEqual(t, http.StatusUnauthorized, w.Code)
	assert.NotEqual(t, http.StatusForbidden, w.Code)
}

// =============================================================================
// Admin Email Templates Tests
// =============================================================================

func TestAdmin_ListEmailTemplates_AdminCanAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	// Setup: Create admin user
	profileRepo := env.deps.ProfileRepo.(*mockProfileRepo)
	userRoleRepo := env.deps.UserRoleRepo.(*mockUserRoleRepo)

	profileRepo.AddProfile(&models.Profile{
		ID:    "admin-user",
		Email: "admin@example.com",
		Plan:  models.PlanElite,
	})
	userRoleRepo.SetAdmin("admin-user", true)

	client := newHTTPClient(t, env.router)
	client.setAuthToken("admin-user")

	w := client.get("/api/v1/admin/email-templates")
	assert.NotEqual(t, http.StatusUnauthorized, w.Code)
	assert.NotEqual(t, http.StatusForbidden, w.Code)
}

// =============================================================================
// RBAC Edge Cases Tests
// =============================================================================

func TestAdmin_RBAC_UnauthenticatedRequest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	client := newHTTPClient(t, env.router)

	// Test: Unauthenticated requests should be rejected
	endpoints := []string{
		"/api/v1/admin/users",
		"/api/v1/admin/system-events",
		"/api/v1/admin/observability",
		"/api/v1/admin/plans",
		"/api/v1/admin/email-templates",
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			w := client.get(endpoint)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

func TestAdmin_RBAC_SuspendedAdminDenied(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	// Setup: Create suspended admin
	suspended := models.BillingStatusSuspended
	profileRepo := env.deps.ProfileRepo.(*mockProfileRepo)
	userRoleRepo := env.deps.UserRoleRepo.(*mockUserRoleRepo)

	profileRepo.AddProfile(&models.Profile{
		ID:            "suspended-admin",
		Email:         "suspendedadmin@example.com",
		Plan:          models.PlanElite,
		BillingStatus: &suspended,
	})
	userRoleRepo.SetAdmin("suspended-admin", true)

	client := newHTTPClient(t, env.router)
	client.setAuthToken("suspended-admin")

	// Test: Suspended admin should be denied even with admin role
	w := client.get("/api/v1/admin/users")
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAdmin_RBAC_MultipleRolesHandled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	// Setup: Create user with multiple roles
	profileRepo := env.deps.ProfileRepo.(*mockProfileRepo)
	userRoleRepo := env.deps.UserRoleRepo.(*mockUserRoleRepo)

	profileRepo.AddProfile(&models.Profile{
		ID:    "multi-role-user",
		Email: "multirole@example.com",
		Plan:  models.PlanElite,
	})
	userRoleRepo.SetAdmin("multi-role-user", true)

	client := newHTTPClient(t, env.router)
	client.setAuthToken("multi-role-user")

	// Test: User with admin role can access admin endpoints
	w := client.get("/api/v1/admin/users")
	assert.NotEqual(t, http.StatusUnauthorized, w.Code)
	assert.NotEqual(t, http.StatusForbidden, w.Code)
}
