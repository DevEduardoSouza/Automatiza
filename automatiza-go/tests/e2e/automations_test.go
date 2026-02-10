package e2e

import (
	"net/http"
	"testing"

	"automatiza-go/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Automations Tests
// =============================================================================

func TestAutomations_List(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	tests := []struct {
		name           string
		automations    []models.Automation
		expectedCount  int
		expectedStatus int
	}{
		{
			name:           "empty list",
			automations:    []models.Automation{},
			expectedCount:  0,
			expectedStatus: http.StatusOK,
		},
		{
			name: "single automation",
			automations: []models.Automation{
				{
					ID:     "auto-1",
					UserID: "user-123",
					Name:   "Test Automation",
					Status: models.AutomationStatusActive,
				},
			},
			expectedCount:  1,
			expectedStatus: http.StatusOK,
		},
		{
			name: "multiple automations",
			automations: []models.Automation{
				{ID: "auto-1", UserID: "user-123", Name: "Auto 1", Status: models.AutomationStatusActive},
				{ID: "auto-2", UserID: "user-123", Name: "Auto 2", Status: models.AutomationStatusInactive},
				{ID: "auto-3", UserID: "user-123", Name: "Auto 3", Status: models.AutomationStatusPaused},
			},
			expectedCount:  3,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t)
			defer env.cleanup(t)

			// Setup: Create user and automations
			profileRepo := env.deps.ProfileRepo.(*mockProfileRepo)
			automationRepo := env.deps.AutomationRepo.(*mockAutomationRepo)

			profileRepo.AddProfile(&models.Profile{
				ID:    "user-123",
				Email: "test@example.com",
				Plan:  models.PlanPro,
			})

			for _, auto := range tc.automations {
				autoCopy := auto
				automationRepo.AddAutomation(&autoCopy)
			}

			client := newHTTPClient(t, env.router)
			client.setAuthToken("user-123")

			w := client.get("/api/v1/automations/")
			assert.Equal(t, tc.expectedStatus, w.Code)

			if tc.expectedStatus == http.StatusOK {
				type listResponse struct {
					Success bool `json:"success"`
					Data    struct {
						Automations []models.Automation `json:"automations"`
						Total       int                 `json:"total"`
					} `json:"data"`
				}
				response := parseResponse[listResponse](t, w)
				assert.True(t, response.Success)
				assert.Equal(t, tc.expectedCount, response.Data.Total)
				assert.Len(t, response.Data.Automations, tc.expectedCount)
			}
		})
	}
}

func TestAutomations_Create(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	tests := []struct {
		name           string
		request        models.CreateAutomationRequest
		expectedStatus int
		expectError    bool
	}{
		{
			name: "valid automation",
			request: models.CreateAutomationRequest{
				Name:         "New Automation",
				GroupIDs:     []string{"group-1", "group-2"},
				Status:       models.AutomationStatusActive,
				IncludeImage: true,
			},
			expectedStatus: http.StatusCreated,
			expectError:    false,
		},
		{
			name: "automation with search type",
			request: models.CreateAutomationRequest{
				Name:        "Search Automation",
				GroupIDs:    []string{"group-1"},
				Type:        models.AutomationTypeSearch,
				Stores:      []string{"shopee", "amazon"},
				Categories:  []string{"electronics", "games"},
				MinDiscount: floatPtr(10.0),
				MaxPrice:    floatPtr(500.0),
			},
			expectedStatus: http.StatusCreated,
			expectError:    false,
		},
		{
			name: "automation with monitoring type",
			request: models.CreateAutomationRequest{
				Name:            "Monitor Automation",
				Type:            models.AutomationTypeMonitoring,
				MonitorGroupIDs: []string{"monitor-group-1"},
				SendGroups:      []string{"send-group-1"},
				DelaySeconds:    intPtr(5),
				DedupeHours:     intPtr(24),
			},
			expectedStatus: http.StatusCreated,
			expectError:    false,
		},
		{
			name: "automation with schedule",
			request: models.CreateAutomationRequest{
				Name:         "Scheduled Automation",
				GroupIDs:     []string{"group-1"},
				ScheduleTime: stringPtr("09:00"),
				ScheduleDays: []string{"monday", "wednesday", "friday"},
			},
			expectedStatus: http.StatusCreated,
			expectError:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t)
			defer env.cleanup(t)

			// Setup: Create user
			profileRepo := env.deps.ProfileRepo.(*mockProfileRepo)
			profileRepo.AddProfile(&models.Profile{
				ID:    "user-123",
				Email: "test@example.com",
				Plan:  models.PlanPro,
			})

			client := newHTTPClient(t, env.router)
			client.setAuthToken("user-123")

			w := client.post("/api/v1/automations/", tc.request)
			assert.Equal(t, tc.expectedStatus, w.Code)

			if !tc.expectError {
				type createResponse struct {
					Success bool              `json:"success"`
					Data    models.Automation `json:"data"`
				}
				response := parseResponse[createResponse](t, w)
				assert.True(t, response.Success)
				assert.NotEmpty(t, response.Data.ID)
				assert.Equal(t, tc.request.Name, response.Data.Name)
			}
		})
	}
}

func TestAutomations_Update(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	tests := []struct {
		name           string
		updates        models.UpdateAutomationRequest
		expectedStatus int
	}{
		{
			name: "update name",
			updates: models.UpdateAutomationRequest{
				Name: stringPtr("Updated Name"),
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "update status",
			updates: models.UpdateAutomationRequest{
				Status: stringPtr(models.AutomationStatusActive),
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "update multiple fields",
			updates: models.UpdateAutomationRequest{
				Name:         stringPtr("New Name"),
				Status:       stringPtr(models.AutomationStatusPaused),
				IncludeImage: boolPtr(true),
				DelaySeconds: intPtr(10),
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t)
			defer env.cleanup(t)

			// Setup
			profileRepo := env.deps.ProfileRepo.(*mockProfileRepo)
			automationRepo := env.deps.AutomationRepo.(*mockAutomationRepo)

			profileRepo.AddProfile(&models.Profile{
				ID:    "user-123",
				Email: "test@example.com",
				Plan:  models.PlanPro,
			})

			automationRepo.AddAutomation(&models.Automation{
				ID:     "auto-123",
				UserID: "user-123",
				Name:   "Original Name",
				Status: models.AutomationStatusInactive,
			})

			client := newHTTPClient(t, env.router)
			client.setAuthToken("user-123")

			w := client.put("/api/v1/automations/auto-123", tc.updates)
			assert.Equal(t, tc.expectedStatus, w.Code)
		})
	}
}

func TestAutomations_Delete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	tests := []struct {
		name           string
		automationID   string
		ownerID        string
		requesterID    string
		expectedStatus int
	}{
		{
			name:           "owner can delete",
			automationID:   "auto-123",
			ownerID:        "user-123",
			requesterID:    "user-123",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "non-owner cannot delete",
			automationID:   "auto-123",
			ownerID:        "user-123",
			requesterID:    "other-user",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "non-existent automation",
			automationID:   "non-existent",
			ownerID:        "user-123",
			requesterID:    "user-123",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t)
			defer env.cleanup(t)

			// Setup
			profileRepo := env.deps.ProfileRepo.(*mockProfileRepo)
			automationRepo := env.deps.AutomationRepo.(*mockAutomationRepo)

			profileRepo.AddProfile(&models.Profile{
				ID:    tc.ownerID,
				Email: "owner@example.com",
				Plan:  models.PlanPro,
			})

			if tc.requesterID != tc.ownerID {
				profileRepo.AddProfile(&models.Profile{
					ID:    tc.requesterID,
					Email: "other@example.com",
					Plan:  models.PlanPro,
				})
			}

			// Only add automation for tests that expect it to exist
			if tc.automationID != "non-existent" {
				automationRepo.AddAutomation(&models.Automation{
					ID:     tc.automationID,
					UserID: tc.ownerID,
					Name:   "Test Automation",
					Status: models.AutomationStatusActive,
				})
			}

			client := newHTTPClient(t, env.router)
			client.setAuthToken(tc.requesterID)

			w := client.delete("/api/v1/automations/" + tc.automationID)
			assert.Equal(t, tc.expectedStatus, w.Code)

			if tc.expectedStatus == http.StatusOK {
				type deleteResponse struct {
					Success bool `json:"success"`
					Data    struct {
						Deleted bool `json:"deleted"`
					} `json:"data"`
				}
				response := parseResponse[deleteResponse](t, w)
				assert.True(t, response.Success)
				assert.True(t, response.Data.Deleted)
			}
		})
	}
}

func TestAutomations_Diagnose(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	tests := []struct {
		name           string
		automation     models.Automation
		expectedStatus int
		expectedResult string
	}{
		{
			name: "healthy automation",
			automation: models.Automation{
				ID:       "auto-1",
				UserID:   "user-123",
				Name:     "Active Automation",
				Status:   models.AutomationStatusActive,
				GroupIDs: []string{"group-1", "group-2"},
			},
			expectedStatus: http.StatusOK,
			expectedResult: "ok",
		},
		{
			name: "automation without groups",
			automation: models.Automation{
				ID:       "auto-2",
				UserID:   "user-123",
				Name:     "No Groups",
				Status:   models.AutomationStatusActive,
				GroupIDs: []string{},
			},
			expectedStatus: http.StatusOK,
			expectedResult: "warning",
		},
		{
			name: "inactive automation",
			automation: models.Automation{
				ID:       "auto-3",
				UserID:   "user-123",
				Name:     "Inactive",
				Status:   models.AutomationStatusInactive,
				GroupIDs: []string{"group-1"},
			},
			expectedStatus: http.StatusOK,
			expectedResult: "ok", // Still ok, but with warning check
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t)
			defer env.cleanup(t)

			// Setup
			profileRepo := env.deps.ProfileRepo.(*mockProfileRepo)
			automationRepo := env.deps.AutomationRepo.(*mockAutomationRepo)

			profileRepo.AddProfile(&models.Profile{
				ID:    "user-123",
				Email: "test@example.com",
				Plan:  models.PlanPro,
			})

			automationRepo.AddAutomation(&tc.automation)

			client := newHTTPClient(t, env.router)
			client.setAuthToken("user-123")

			reqBody := models.DiagnoseAutomationRequest{
				AutomationID: tc.automation.ID,
			}

			w := client.post("/api/v1/automations/diagnose", reqBody)
			assert.Equal(t, tc.expectedStatus, w.Code)

			if tc.expectedStatus == http.StatusOK {
				type diagnoseResponse struct {
					Success bool                  `json:"success"`
					Data    models.DiagnoseResult `json:"data"`
				}
				response := parseResponse[diagnoseResponse](t, w)
				assert.True(t, response.Success)
				assert.Equal(t, tc.expectedResult, response.Data.Status)
			}
		})
	}
}

func TestAutomations_Run(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	tests := []struct {
		name           string
		automationID   string
		ownerID        string
		requesterID    string
		expectedStatus int
	}{
		{
			name:           "owner can run",
			automationID:   "auto-123",
			ownerID:        "user-123",
			requesterID:    "user-123",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "non-owner cannot run",
			automationID:   "auto-123",
			ownerID:        "user-123",
			requesterID:    "other-user",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "non-existent automation",
			automationID:   "non-existent",
			ownerID:        "user-123",
			requesterID:    "user-123",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t)
			defer env.cleanup(t)

			// Setup
			profileRepo := env.deps.ProfileRepo.(*mockProfileRepo)
			automationRepo := env.deps.AutomationRepo.(*mockAutomationRepo)

			profileRepo.AddProfile(&models.Profile{
				ID:    tc.ownerID,
				Email: "owner@example.com",
				Plan:  models.PlanPro,
			})

			if tc.requesterID != tc.ownerID {
				profileRepo.AddProfile(&models.Profile{
					ID:    tc.requesterID,
					Email: "other@example.com",
					Plan:  models.PlanPro,
				})
			}

			// Only add automation for tests that expect it to exist
			if tc.automationID != "non-existent" {
				automationType := models.AutomationTypeSearch
				automationRepo.AddAutomation(&models.Automation{
					ID:       tc.automationID,
					UserID:   tc.ownerID,
					Name:     "Test Automation",
					Status:   models.AutomationStatusActive,
					Type:     &automationType,
					GroupIDs: []string{"group-1"},
				})
			}

			client := newHTTPClient(t, env.router)
			client.setAuthToken(tc.requesterID)

			reqBody := models.RunAutomationRequest{
				AutomationID: tc.automationID,
			}

			w := client.post("/api/v1/automations/run", reqBody)
			assert.Equal(t, tc.expectedStatus, w.Code)

			if tc.expectedStatus == http.StatusOK {
				type runResponse struct {
					Success bool                       `json:"success"`
					Data    models.RunAutomationResult `json:"data"`
				}
				response := parseResponse[runResponse](t, w)
				assert.True(t, response.Success)
				assert.True(t, response.Data.Success)
			}
		})
	}
}

func TestAutomations_Get(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	tests := []struct {
		name           string
		automationID   string
		ownerID        string
		requesterID    string
		expectedStatus int
	}{
		{
			name:           "owner can view",
			automationID:   "auto-123",
			ownerID:        "user-123",
			requesterID:    "user-123",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "non-owner cannot view",
			automationID:   "auto-123",
			ownerID:        "user-123",
			requesterID:    "other-user",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "non-existent automation",
			automationID:   "non-existent",
			ownerID:        "user-123",
			requesterID:    "user-123",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t)
			defer env.cleanup(t)

			// Setup
			profileRepo := env.deps.ProfileRepo.(*mockProfileRepo)
			automationRepo := env.deps.AutomationRepo.(*mockAutomationRepo)

			profileRepo.AddProfile(&models.Profile{
				ID:    tc.ownerID,
				Email: "owner@example.com",
				Plan:  models.PlanPro,
			})

			if tc.requesterID != tc.ownerID {
				profileRepo.AddProfile(&models.Profile{
					ID:    tc.requesterID,
					Email: "other@example.com",
					Plan:  models.PlanPro,
				})
			}

			// Only add automation for tests that expect it to exist
			if tc.automationID != "non-existent" {
				automationRepo.AddAutomation(&models.Automation{
					ID:     tc.automationID,
					UserID: tc.ownerID,
					Name:   "Test Automation",
					Status: models.AutomationStatusActive,
				})
			}

			client := newHTTPClient(t, env.router)
			client.setAuthToken(tc.requesterID)

			w := client.get("/api/v1/automations/" + tc.automationID)
			assert.Equal(t, tc.expectedStatus, w.Code)

			if tc.expectedStatus == http.StatusOK {
				type getResponse struct {
					Success bool              `json:"success"`
					Data    models.Automation `json:"data"`
				}
				response := parseResponse[getResponse](t, w)
				assert.True(t, response.Success)
				assert.Equal(t, tc.automationID, response.Data.ID)
			}
		})
	}
}

func TestAutomations_ResetHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	// Setup
	profileRepo := env.deps.ProfileRepo.(*mockProfileRepo)
	automationRepo := env.deps.AutomationRepo.(*mockAutomationRepo)

	profileRepo.AddProfile(&models.Profile{
		ID:    "user-123",
		Email: "test@example.com",
		Plan:  models.PlanPro,
	})

	automationRepo.AddAutomation(&models.Automation{
		ID:     "auto-123",
		UserID: "user-123",
		Name:   "Test Automation",
		Status: models.AutomationStatusActive,
	})

	client := newHTTPClient(t, env.router)
	client.setAuthToken("user-123")

	reqBody := models.ResetHistoryRequest{
		AutomationID: "auto-123",
	}

	w := client.post("/api/v1/automations/reset-history", reqBody)
	require.Equal(t, http.StatusOK, w.Code)

	type resetResponse struct {
		Success bool `json:"success"`
		Data    struct {
			Reset bool `json:"reset"`
		} `json:"data"`
	}
	response := parseResponse[resetResponse](t, w)
	assert.True(t, response.Success)
	assert.True(t, response.Data.Reset)
}

func TestAutomations_RequiresAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	env := newTestEnv(t)
	defer env.cleanup(t)

	client := newHTTPClient(t, env.router)

	endpoints := []struct {
		method string
		path   string
		body   interface{}
	}{
		{"GET", "/api/v1/automations/", nil},
		{"GET", "/api/v1/automations/auto-1", nil},
		{"POST", "/api/v1/automations/", models.CreateAutomationRequest{Name: "Test"}},
		{"PUT", "/api/v1/automations/auto-1", models.UpdateAutomationRequest{Name: stringPtr("Test")}},
		{"DELETE", "/api/v1/automations/auto-1", nil},
		{"POST", "/api/v1/automations/diagnose", models.DiagnoseAutomationRequest{AutomationID: "auto-1"}},
		{"POST", "/api/v1/automations/run", models.RunAutomationRequest{AutomationID: "auto-1"}},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			w := client.doRequest(ep.method, ep.path, ep.body)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

func floatPtr(f float64) *float64 {
	return &f
}

func boolPtr(b bool) *bool {
	return &b
}
