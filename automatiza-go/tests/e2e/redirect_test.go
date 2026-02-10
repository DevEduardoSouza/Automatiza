package e2e

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"automatiza-go/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Redirect Links Tests
// =============================================================================

// redirectWrappedResponse is used for successful redirect responses which are
// wrapped in ApiResponse format: { "success": true, "data": { ... } }
type redirectWrappedResponse struct {
	Success bool                          `json:"success"`
	Data    models.PublicRedirectResponse `json:"data"`
}

// parseSuccessRedirectResponse parses a successful redirect response (wrapped in ApiResponse)
func parseSuccessRedirectResponse(t *testing.T, w *httptest.ResponseRecorder) models.PublicRedirectResponse {
	t.Helper()
	wrapped := parseResponse[redirectWrappedResponse](t, w)
	return wrapped.Data
}

func TestRedirect_ProcessValidSlug(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	// Setup: Create user, instance, group, and redirect link
	profileRepo := env.deps.ProfileRepo.(*mockProfileRepo)
	instanceRepo := env.deps.InstanceRepo.(*mockInstanceRepo)
	groupRepo := env.deps.GroupRepo.(*mockGroupRepo)
	redirectLinkRepo := env.redirectLinkRepo

	profileRepo.AddProfile(&models.Profile{
		ID:    "user-123",
		Email: "test@example.com",
		Plan:  models.PlanPro,
	})

	instanceRepo.AddInstance(&models.Instance{
		ID:         1,
		UserID:     "user-123",
		InstanceID: "test-instance",
		Status:     models.InstanceStatusConnected,
	})

	groupRepo.AddGroup(&models.Group{
		ID:           "group-123",
		UserID:       "user-123",
		Name:         "Test Group",
		WAGroupID:    "123456789@g.us",
		InstanceID:   "1",
		MembersCount: 50,
		MemberLimit:  200,
		IsAdmin:      true,
		Status:       models.GroupStatusOpen,
	})

	// Configure WAHA adapter to return invite code
	env.wahaAdapter.SetInviteCode("test-instance", "123456789@g.us", "AbCdEfGhIjK")

	redirectLinkRepo.AddLink(&models.RedirectLink{
		ID:       "link-123",
		UserID:   "user-123",
		Slug:     "test-redirect",
		Name:     "Test Link",
		Strategy: models.RedirectStrategyRoundRobin,
		TargetGroups: []models.GroupPriority{
			{
				GroupID:   "group-123",
				GroupName: "Test Group",
				Priority:  1,
				IsActive:  true,
			},
		},
		IsActive:    true,
		ClickCount:  0,
		GroupClicks: make(map[string]int),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	})

	client := newHTTPClient(t, env.router)

	// Test: Process redirect with valid slug
	reqBody := models.PublicRedirectRequest{
		Slug: "test-redirect",
	}

	w := client.post("/api/v1/redirect", reqBody)
	require.Equal(t, http.StatusOK, w.Code)

	response := parseSuccessRedirectResponse(t, w)
	assert.True(t, response.Success)
	assert.Contains(t, response.RedirectURL, "chat.whatsapp.com")
	assert.Equal(t, "Test Group", response.GroupName)
	assert.Equal(t, "group-123", response.GroupID)
}

func TestRedirect_ProcessInvalidSlug(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	client := newHTTPClient(t, env.router)

	// Test: Process redirect with non-existent slug
	reqBody := models.PublicRedirectRequest{
		Slug: "non-existent-slug",
	}

	w := client.post("/api/v1/redirect", reqBody)
	require.Equal(t, http.StatusOK, w.Code) // Returns 200 with success=false

	response := parseResponse[models.PublicRedirectResponse](t, w)
	assert.False(t, response.Success)
	assert.Contains(t, response.Error, "not found")
}

func TestRedirect_ProcessInactiveLink(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	// Setup: Create inactive redirect link
	redirectLinkRepo := env.redirectLinkRepo

	redirectLinkRepo.AddLink(&models.RedirectLink{
		ID:       "link-inactive",
		UserID:   "user-123",
		Slug:     "inactive-link",
		Name:     "Inactive Link",
		Strategy: models.RedirectStrategyRoundRobin,
		TargetGroups: []models.GroupPriority{
			{GroupID: "group-1", Priority: 1, IsActive: true},
		},
		IsActive:  false, // Link is inactive
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	client := newHTTPClient(t, env.router)

	reqBody := models.PublicRedirectRequest{
		Slug: "inactive-link",
	}

	w := client.post("/api/v1/redirect", reqBody)
	require.Equal(t, http.StatusOK, w.Code)

	response := parseResponse[models.PublicRedirectResponse](t, w)
	assert.False(t, response.Success)
	assert.Contains(t, response.Error, "inactive")
}

func TestRedirect_MemberLimitStrategy(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	tests := []struct {
		name   string
		groups []struct {
			id          string
			memberCount int
			memberLimit int
		}
		expectedGroup string
		expectError   bool
	}{
		{
			name: "skips full groups",
			groups: []struct {
				id          string
				memberCount int
				memberLimit int
			}{
				{id: "group-1", memberCount: 200, memberLimit: 200}, // Full
				{id: "group-2", memberCount: 50, memberLimit: 200},  // Available
			},
			expectedGroup: "group-2",
			expectError:   false,
		},
		{
			name: "all groups full",
			groups: []struct {
				id          string
				memberCount int
				memberLimit int
			}{
				{id: "group-1", memberCount: 200, memberLimit: 200},
				{id: "group-2", memberCount: 200, memberLimit: 200},
			},
			expectedGroup: "",
			expectError:   true,
		},
		{
			name: "selects by priority",
			groups: []struct {
				id          string
				memberCount int
				memberLimit int
			}{
				{id: "group-1", memberCount: 50, memberLimit: 200}, // Priority 1
				{id: "group-2", memberCount: 10, memberLimit: 200}, // Priority 2
			},
			expectedGroup: "group-1", // Higher priority (lower number)
			expectError:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t)
			defer env.cleanup(t)

			// Setup
			profileRepo := env.deps.ProfileRepo.(*mockProfileRepo)
			instanceRepo := env.deps.InstanceRepo.(*mockInstanceRepo)
			groupRepo := env.deps.GroupRepo.(*mockGroupRepo)
			redirectLinkRepo := env.redirectLinkRepo

			profileRepo.AddProfile(&models.Profile{
				ID:    "user-123",
				Email: "test@example.com",
				Plan:  models.PlanPro,
			})

			instanceRepo.AddInstance(&models.Instance{
				ID:         1,
				UserID:     "user-123",
				InstanceID: "test-instance",
				Status:     models.InstanceStatusConnected,
			})

			var targetGroups []models.GroupPriority
			for i, g := range tc.groups {
				groupRepo.AddGroup(&models.Group{
					ID:           g.id,
					UserID:       "user-123",
					Name:         "Group " + g.id,
					WAGroupID:    g.id + "@g.us",
					InstanceID:   "1",
					MembersCount: g.memberCount,
					MemberLimit:  g.memberLimit,
					IsAdmin:      true,
				})

				env.wahaAdapter.SetInviteCode("test-instance", g.id+"@g.us", "invite-"+g.id)

				targetGroups = append(targetGroups, models.GroupPriority{
					GroupID:     g.id,
					GroupName:   "Group " + g.id,
					Priority:    i + 1,
					MemberLimit: g.memberLimit,
					IsActive:    true,
				})
			}

			redirectLinkRepo.AddLink(&models.RedirectLink{
				ID:           "link-member-limit",
				UserID:       "user-123",
				Slug:         "member-limit-test",
				Name:         "Member Limit Test",
				Strategy:     models.RedirectStrategyMemberLimit,
				TargetGroups: targetGroups,
				IsActive:     true,
				GroupClicks:  make(map[string]int),
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			})

			client := newHTTPClient(t, env.router)

			reqBody := models.PublicRedirectRequest{
				Slug: "member-limit-test",
			}

			w := client.post("/api/v1/redirect", reqBody)
			require.Equal(t, http.StatusOK, w.Code)

			if tc.expectError {
				// Error responses are not wrapped
				response := parseResponse[models.PublicRedirectResponse](t, w)
				assert.False(t, response.Success)
			} else {
				// Success responses are wrapped in ApiResponse
				response := parseSuccessRedirectResponse(t, w)
				assert.True(t, response.Success)
				assert.Equal(t, tc.expectedGroup, response.GroupID)
			}
		})
	}
}

func TestRedirect_ClickLimitStrategy(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	tests := []struct {
		name          string
		groupClicks   map[string]int
		clickLimits   map[string]int
		expectedGroup string
		expectError   bool
	}{
		{
			name: "skips groups at click limit",
			groupClicks: map[string]int{
				"group-1": 100, // At limit
				"group-2": 50,  // Under limit
			},
			clickLimits: map[string]int{
				"group-1": 100,
				"group-2": 100,
			},
			expectedGroup: "group-2",
			expectError:   false,
		},
		{
			name: "all groups at limit",
			groupClicks: map[string]int{
				"group-1": 100,
				"group-2": 100,
			},
			clickLimits: map[string]int{
				"group-1": 100,
				"group-2": 100,
			},
			expectedGroup: "",
			expectError:   true,
		},
		{
			name:        "fresh link no clicks",
			groupClicks: map[string]int{},
			clickLimits: map[string]int{
				"group-1": 100,
				"group-2": 100,
			},
			expectedGroup: "group-1",
			expectError:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t)
			defer env.cleanup(t)

			// Setup
			profileRepo := env.deps.ProfileRepo.(*mockProfileRepo)
			instanceRepo := env.deps.InstanceRepo.(*mockInstanceRepo)
			groupRepo := env.deps.GroupRepo.(*mockGroupRepo)
			redirectLinkRepo := env.redirectLinkRepo

			profileRepo.AddProfile(&models.Profile{
				ID:    "user-123",
				Email: "test@example.com",
				Plan:  models.PlanPro,
			})

			instanceRepo.AddInstance(&models.Instance{
				ID:         1,
				UserID:     "user-123",
				InstanceID: "test-instance",
				Status:     models.InstanceStatusConnected,
			})

			var targetGroups []models.GroupPriority
			priority := 1
			for groupID, limit := range tc.clickLimits {
				groupRepo.AddGroup(&models.Group{
					ID:           groupID,
					UserID:       "user-123",
					Name:         "Group " + groupID,
					WAGroupID:    groupID + "@g.us",
					InstanceID:   "1",
					MembersCount: 50,
					IsAdmin:      true,
				})

				env.wahaAdapter.SetInviteCode("test-instance", groupID+"@g.us", "invite-"+groupID)

				targetGroups = append(targetGroups, models.GroupPriority{
					GroupID:    groupID,
					GroupName:  "Group " + groupID,
					Priority:   priority,
					ClickLimit: limit,
					IsActive:   true,
				})
				priority++
			}

			redirectLinkRepo.AddLink(&models.RedirectLink{
				ID:           "link-click-limit",
				UserID:       "user-123",
				Slug:         "click-limit-test",
				Name:         "Click Limit Test",
				Strategy:     models.RedirectStrategyClickLimit,
				TargetGroups: targetGroups,
				IsActive:     true,
				GroupClicks:  tc.groupClicks,
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			})

			client := newHTTPClient(t, env.router)

			reqBody := models.PublicRedirectRequest{
				Slug: "click-limit-test",
			}

			w := client.post("/api/v1/redirect", reqBody)
			require.Equal(t, http.StatusOK, w.Code)

			if tc.expectError {
				// Error responses are not wrapped
				response := parseResponse[models.PublicRedirectResponse](t, w)
				assert.False(t, response.Success)
			} else {
				// Success responses are wrapped in ApiResponse
				response := parseSuccessRedirectResponse(t, w)
				assert.True(t, response.Success)
				assert.Equal(t, tc.expectedGroup, response.GroupID)
			}
		})
	}
}

func TestRedirect_RoundRobinStrategy(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	// Setup
	profileRepo := env.deps.ProfileRepo.(*mockProfileRepo)
	instanceRepo := env.deps.InstanceRepo.(*mockInstanceRepo)
	groupRepo := env.deps.GroupRepo.(*mockGroupRepo)
	redirectLinkRepo := env.redirectLinkRepo

	profileRepo.AddProfile(&models.Profile{
		ID:    "user-123",
		Email: "test@example.com",
		Plan:  models.PlanPro,
	})

	instanceRepo.AddInstance(&models.Instance{
		ID:         1,
		UserID:     "user-123",
		InstanceID: "test-instance",
		Status:     models.InstanceStatusConnected,
	})

	groups := []string{"group-1", "group-2", "group-3"}
	var targetGroups []models.GroupPriority

	for i, groupID := range groups {
		groupRepo.AddGroup(&models.Group{
			ID:           groupID,
			UserID:       "user-123",
			Name:         "Group " + groupID,
			WAGroupID:    groupID + "@g.us",
			InstanceID:   "1",
			MembersCount: 50,
			IsAdmin:      true,
		})

		env.wahaAdapter.SetInviteCode("test-instance", groupID+"@g.us", "invite-"+groupID)

		targetGroups = append(targetGroups, models.GroupPriority{
			GroupID:   groupID,
			GroupName: "Group " + groupID,
			Priority:  i + 1,
			IsActive:  true,
		})
	}

	redirectLinkRepo.AddLink(&models.RedirectLink{
		ID:           "link-round-robin",
		UserID:       "user-123",
		Slug:         "round-robin-test",
		Name:         "Round Robin Test",
		Strategy:     models.RedirectStrategyRoundRobin,
		TargetGroups: targetGroups,
		IsActive:     true,
		GroupClicks:  make(map[string]int),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	})

	client := newHTTPClient(t, env.router)

	// Test: Multiple requests should distribute across groups
	clickCounts := make(map[string]int)

	for i := 0; i < 6; i++ {
		reqBody := models.PublicRedirectRequest{
			Slug: "round-robin-test",
		}

		w := client.post("/api/v1/redirect", reqBody)
		require.Equal(t, http.StatusOK, w.Code)

		response := parseSuccessRedirectResponse(t, w)
		require.True(t, response.Success)

		clickCounts[response.GroupID]++
	}

	// Each group should have received 2 clicks (6 total / 3 groups)
	for _, groupID := range groups {
		assert.Equal(t, 2, clickCounts[groupID], "Group %s should have 2 clicks", groupID)
	}
}

func TestRedirect_IncrementClickCounter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	// Setup
	profileRepo := env.deps.ProfileRepo.(*mockProfileRepo)
	instanceRepo := env.deps.InstanceRepo.(*mockInstanceRepo)
	groupRepo := env.deps.GroupRepo.(*mockGroupRepo)
	redirectLinkRepo := env.redirectLinkRepo

	profileRepo.AddProfile(&models.Profile{
		ID:    "user-123",
		Email: "test@example.com",
		Plan:  models.PlanPro,
	})

	instanceRepo.AddInstance(&models.Instance{
		ID:         1,
		UserID:     "user-123",
		InstanceID: "test-instance",
		Status:     models.InstanceStatusConnected,
	})

	groupRepo.AddGroup(&models.Group{
		ID:           "group-1",
		UserID:       "user-123",
		Name:         "Test Group",
		WAGroupID:    "group-1@g.us",
		InstanceID:   "1",
		MembersCount: 50,
		IsAdmin:      true,
	})

	env.wahaAdapter.SetInviteCode("test-instance", "group-1@g.us", "invite-code")

	link := &models.RedirectLink{
		ID:       "link-counter",
		UserID:   "user-123",
		Slug:     "counter-test",
		Name:     "Counter Test",
		Strategy: models.RedirectStrategyRoundRobin,
		TargetGroups: []models.GroupPriority{
			{GroupID: "group-1", GroupName: "Test Group", Priority: 1, IsActive: true},
		},
		IsActive:    true,
		ClickCount:  0,
		GroupClicks: make(map[string]int),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	redirectLinkRepo.AddLink(link)

	client := newHTTPClient(t, env.router)

	// Make 5 requests
	for i := 0; i < 5; i++ {
		reqBody := models.PublicRedirectRequest{
			Slug: "counter-test",
		}

		w := client.post("/api/v1/redirect", reqBody)
		require.Equal(t, http.StatusOK, w.Code)

		response := parseSuccessRedirectResponse(t, w)
		require.True(t, response.Success)
	}

	// Verify click count was incremented
	updatedLink, err := redirectLinkRepo.GetBySlug(nil, "counter-test")
	require.NoError(t, err)
	assert.Equal(t, 5, updatedLink.ClickCount)
	assert.Equal(t, 5, updatedLink.GroupClicks["group-1"])
}

func TestRedirect_MissingSlugAndLinkID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	client := newHTTPClient(t, env.router)

	// Test: Both slug and linkId are missing
	reqBody := models.PublicRedirectRequest{}

	w := client.post("/api/v1/redirect", reqBody)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRedirect_ByLinkID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	// Setup
	profileRepo := env.deps.ProfileRepo.(*mockProfileRepo)
	instanceRepo := env.deps.InstanceRepo.(*mockInstanceRepo)
	groupRepo := env.deps.GroupRepo.(*mockGroupRepo)
	redirectLinkRepo := env.redirectLinkRepo

	profileRepo.AddProfile(&models.Profile{
		ID:    "user-123",
		Email: "test@example.com",
		Plan:  models.PlanPro,
	})

	instanceRepo.AddInstance(&models.Instance{
		ID:         1,
		UserID:     "user-123",
		InstanceID: "test-instance",
		Status:     models.InstanceStatusConnected,
	})

	groupRepo.AddGroup(&models.Group{
		ID:           "group-1",
		UserID:       "user-123",
		Name:         "Test Group",
		WAGroupID:    "group-1@g.us",
		InstanceID:   "1",
		MembersCount: 50,
		IsAdmin:      true,
	})

	env.wahaAdapter.SetInviteCode("test-instance", "group-1@g.us", "invite-code")

	redirectLinkRepo.AddLink(&models.RedirectLink{
		ID:       "link-by-id",
		UserID:   "user-123",
		Slug:     "slug-for-link",
		Name:     "Link By ID",
		Strategy: models.RedirectStrategyRoundRobin,
		TargetGroups: []models.GroupPriority{
			{GroupID: "group-1", GroupName: "Test Group", Priority: 1, IsActive: true},
		},
		IsActive:    true,
		GroupClicks: make(map[string]int),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	})

	client := newHTTPClient(t, env.router)

	// Test: Process by link ID instead of slug
	reqBody := models.PublicRedirectRequest{
		LinkID: "link-by-id",
	}

	w := client.post("/api/v1/redirect", reqBody)
	require.Equal(t, http.StatusOK, w.Code)

	response := parseSuccessRedirectResponse(t, w)
	assert.True(t, response.Success)
	assert.Contains(t, response.RedirectURL, "chat.whatsapp.com")
}

func TestRedirect_NoTargetGroups(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	// Setup
	redirectLinkRepo := env.redirectLinkRepo

	redirectLinkRepo.AddLink(&models.RedirectLink{
		ID:           "link-no-groups",
		UserID:       "user-123",
		Slug:         "no-groups",
		Name:         "No Groups Link",
		Strategy:     models.RedirectStrategyRoundRobin,
		TargetGroups: []models.GroupPriority{}, // No groups
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	})

	client := newHTTPClient(t, env.router)

	reqBody := models.PublicRedirectRequest{
		Slug: "no-groups",
	}

	w := client.post("/api/v1/redirect", reqBody)
	require.Equal(t, http.StatusOK, w.Code)

	response := parseResponse[models.PublicRedirectResponse](t, w)
	assert.False(t, response.Success)
	assert.Contains(t, response.Error, "no target groups")
}

func TestRedirect_InactiveGroup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	// Setup
	redirectLinkRepo := env.redirectLinkRepo

	redirectLinkRepo.AddLink(&models.RedirectLink{
		ID:       "link-inactive-group",
		UserID:   "user-123",
		Slug:     "inactive-group",
		Name:     "Inactive Group Link",
		Strategy: models.RedirectStrategyRoundRobin,
		TargetGroups: []models.GroupPriority{
			{GroupID: "group-1", GroupName: "Group 1", Priority: 1, IsActive: false}, // Inactive
		},
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	client := newHTTPClient(t, env.router)

	reqBody := models.PublicRedirectRequest{
		Slug: "inactive-group",
	}

	w := client.post("/api/v1/redirect", reqBody)
	require.Equal(t, http.StatusOK, w.Code)

	response := parseResponse[models.PublicRedirectResponse](t, w)
	assert.False(t, response.Success)
	assert.Contains(t, response.Error, "no available groups")
}
