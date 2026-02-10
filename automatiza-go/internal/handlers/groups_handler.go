package handlers

import (
	"automatiza-go/internal/middleware"
	"net/http"

	"github.com/gin-gonic/gin"
)

// GroupsHandler handles group operations
type GroupsHandler struct {
	deps *Dependencies
}

// NewGroupsHandler creates a new groups handler
func NewGroupsHandler(deps *Dependencies) *GroupsHandler {
	return &GroupsHandler{deps: deps}
}

// List returns all groups for the authenticated user
func (h *GroupsHandler) List(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)

	adminOnly := c.Query("adminOnly") == "true"

	groups, err := h.deps.GroupRepo.GetUserGroups(c.Request.Context(), user.ID, adminOnly)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao buscar grupos")
		return
	}

	middleware.RespondSuccess(c, map[string]interface{}{
		"groups": groups,
		"total":  len(groups),
	})
}

// Get returns a specific group
func (h *GroupsHandler) Get(c *gin.Context) {
	user := middleware.MustGetAuthenticatedUser(c)
	groupID := c.Param("id")

	group, err := h.deps.GroupRepo.GetGroupByID(c.Request.Context(), groupID)
	if err != nil {
		middleware.RespondError(c, http.StatusInternalServerError, "Falha ao buscar grupo")
		return
	}

	if group == nil || group.UserID != user.ID {
		middleware.RespondError(c, http.StatusNotFound, "Grupo não encontrado")
		return
	}

	middleware.RespondSuccess(c, group)
}
