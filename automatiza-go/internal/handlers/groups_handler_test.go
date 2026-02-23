package handlers

import (
	"automatiza-go/internal/models"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// GroupsHandler.List
// ---------------------------------------------------------------------------

func TestGroupsHandler_List_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewGroupsHandler(deps)

	mocks.GroupRepo.GetUserGroupsFunc = func(ctx context.Context, userID string, adminOnly bool) ([]models.Group, error) {
		return []models.Group{
			{ID: "g1", UserID: userID, Name: "Group 1"},
			{ID: "g2", UserID: userID, Name: "Group 2"},
		}, nil
	}

	w, c := newAuthContext(http.MethodGet, "/api/v1/groups/", "")
	handler.List(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := mustParseBody(w)
	data := body["data"].(map[string]interface{})
	groups := data["groups"].([]interface{})
	if len(groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(groups))
	}
}

func TestGroupsHandler_List_AdminOnly(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewGroupsHandler(deps)

	var capturedAdminOnly bool
	mocks.GroupRepo.GetUserGroupsFunc = func(ctx context.Context, userID string, adminOnly bool) ([]models.Group, error) {
		capturedAdminOnly = adminOnly
		return []models.Group{}, nil
	}

	w, c := newAuthContext(http.MethodGet, "/api/v1/groups/?adminOnly=true", "")
	handler.List(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !capturedAdminOnly {
		t.Error("expected adminOnly=true to be passed to repository")
	}
}

func TestGroupsHandler_List_RepoError(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewGroupsHandler(deps)

	mocks.GroupRepo.GetUserGroupsFunc = func(ctx context.Context, userID string, adminOnly bool) ([]models.Group, error) {
		return nil, errors.New("db error")
	}

	w, c := newAuthContext(http.MethodGet, "/api/v1/groups/", "")
	handler.List(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// GroupsHandler.Get
// ---------------------------------------------------------------------------

func TestGroupsHandler_Get_Success(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewGroupsHandler(deps)

	mocks.GroupRepo.GetGroupByIDFunc = func(ctx context.Context, groupID string) (*models.Group, error) {
		return &models.Group{ID: groupID, UserID: "user-123", Name: "Test Group"}, nil
	}

	w, c := newAuthContext(http.MethodGet, "/api/v1/groups/g1", "")
	c.Params = append(c.Params, gin.Param{Key: "id", Value: "g1"})
	handler.Get(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
}

func TestGroupsHandler_Get_NotFound(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewGroupsHandler(deps)

	mocks.GroupRepo.GetGroupByIDFunc = func(ctx context.Context, groupID string) (*models.Group, error) {
		return nil, nil
	}

	w, c := newAuthContext(http.MethodGet, "/api/v1/groups/g1", "")
	c.Params = append(c.Params, gin.Param{Key: "id", Value: "g1"})
	handler.Get(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGroupsHandler_Get_WrongUser(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewGroupsHandler(deps)

	mocks.GroupRepo.GetGroupByIDFunc = func(ctx context.Context, groupID string) (*models.Group, error) {
		return &models.Group{ID: groupID, UserID: "other-user", Name: "Test"}, nil
	}

	w, c := newAuthContext(http.MethodGet, "/api/v1/groups/g1", "")
	c.Params = append(c.Params, gin.Param{Key: "id", Value: "g1"})
	handler.Get(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for wrong user, got %d", w.Code)
	}
}

func TestGroupsHandler_Get_RepoError(t *testing.T) {
	t.Parallel()
	deps, mocks := newTestDeps()
	handler := NewGroupsHandler(deps)

	mocks.GroupRepo.GetGroupByIDFunc = func(ctx context.Context, groupID string) (*models.Group, error) {
		return nil, errors.New("db error")
	}

	w, c := newAuthContext(http.MethodGet, "/api/v1/groups/g1", "")
	c.Params = append(c.Params, gin.Param{Key: "id", Value: "g1"})
	handler.Get(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}
