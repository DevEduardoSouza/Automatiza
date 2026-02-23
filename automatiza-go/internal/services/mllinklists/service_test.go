package mllinklists

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	apperrors "automatiza-go/internal/errors"
	"automatiza-go/internal/models"

	"go.uber.org/zap"
)

// ============================================================================
// Mock repository
// ============================================================================

type mockRepository struct {
	lists           map[string]*models.MlLinkList
	items           map[string]*models.MlLinkListItem
	listsByUser     map[string][]models.MlLinkList
	itemsByList     map[string][]models.MlLinkListItem
	maxOrderIndex   map[string]int
	createListErr   error
	getListErr      error
	updateListErr   error
	deleteListErr   error
	createItemErr   error
	getItemErr      error
	updateItemErr   error
	deleteItemErr   error
	bulkCreateErr   error
	resetErr        error
	reorderErr      error
	markConsumedErr error
	maxOrderErr     error

	// Per-call error injection for simulating race conditions
	markConsumedCallCount atomic.Int32
	markConsumedErrOnCall int // 0 = disabled; N = fail on Nth call (1-based)

	// Separate error for re-fetch after update (GetListByIDAndUser/GetItemByID second call)
	getListCallCount   atomic.Int32
	getListErrOnCall   int // fail on Nth call (1-based)
	getItemCallCount   atomic.Int32
	getItemErrOnCall   int // fail on Nth call (1-based)
	getItemErrOnCallFn func() error

	// Track paginated calls
	paginatedErr error
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		lists:         make(map[string]*models.MlLinkList),
		items:         make(map[string]*models.MlLinkListItem),
		listsByUser:   make(map[string][]models.MlLinkList),
		itemsByList:   make(map[string][]models.MlLinkListItem),
		maxOrderIndex: make(map[string]int),
	}
}

func (m *mockRepository) GetListsByUserID(ctx context.Context, userID string) ([]models.MlLinkList, error) {
	if m.getListErr != nil {
		return nil, m.getListErr
	}
	return m.listsByUser[userID], nil
}

func (m *mockRepository) GetListByID(ctx context.Context, listID string) (*models.MlLinkList, error) {
	if m.getListErr != nil {
		return nil, m.getListErr
	}
	list, ok := m.lists[listID]
	if !ok {
		return nil, nil
	}
	return list, nil
}

func (m *mockRepository) GetListByIDAndUser(ctx context.Context, listID, userID string) (*models.MlLinkList, error) {
	callNum := int(m.getListCallCount.Add(1))
	if m.getListErrOnCall > 0 && callNum == m.getListErrOnCall {
		return nil, errors.New("repo error on nth call")
	}
	if m.getListErr != nil {
		return nil, m.getListErr
	}
	list, ok := m.lists[listID]
	if !ok {
		return nil, nil
	}
	if list.UserID != userID {
		return nil, nil
	}
	return list, nil
}

func (m *mockRepository) CreateList(ctx context.Context, userID string, list *models.MlLinkList) (*models.MlLinkList, error) {
	if m.createListErr != nil {
		return nil, m.createListErr
	}
	list.ID = "list-" + time.Now().Format("20060102150405")
	list.UserID = userID
	list.CreatedAt = time.Now().Format(time.RFC3339)
	list.UpdatedAt = list.CreatedAt
	m.lists[list.ID] = list
	m.listsByUser[userID] = append(m.listsByUser[userID], *list)
	return list, nil
}

func (m *mockRepository) UpdateList(ctx context.Context, listID string, updates map[string]interface{}) error {
	if m.updateListErr != nil {
		return m.updateListErr
	}
	list, ok := m.lists[listID]
	if !ok {
		return nil
	}
	if name, ok := updates["name"].(string); ok {
		list.Name = name
	}
	if desc, ok := updates["description"].(string); ok {
		list.Description = &desc
	}
	if active, ok := updates["is_active"].(bool); ok {
		list.IsActive = active
	}
	list.UpdatedAt = time.Now().Format(time.RFC3339)
	return nil
}

func (m *mockRepository) DeleteList(ctx context.Context, listID, userID string) error {
	if m.deleteListErr != nil {
		return m.deleteListErr
	}
	delete(m.lists, listID)
	delete(m.itemsByList, listID)
	return nil
}

func (m *mockRepository) GetItemsByListID(ctx context.Context, listID string) ([]models.MlLinkListItem, error) {
	if m.getItemErr != nil {
		return nil, m.getItemErr
	}
	return m.itemsByList[listID], nil
}

func (m *mockRepository) GetItemsByListIDPaginated(ctx context.Context, listID string, pageSize int, cursor string) ([]models.MlLinkListItem, string, int, error) {
	if m.paginatedErr != nil {
		return nil, "", 0, m.paginatedErr
	}
	if m.getItemErr != nil {
		return nil, "", 0, m.getItemErr
	}
	items := m.itemsByList[listID]
	total := len(items)
	if pageSize > len(items) {
		pageSize = len(items)
	}
	nextCursor := ""
	if pageSize < total {
		nextCursor = "next-cursor"
	}
	return items[:pageSize], nextCursor, total, nil
}

func (m *mockRepository) GetItemByID(ctx context.Context, itemID string) (*models.MlLinkListItem, error) {
	callNum := int(m.getItemCallCount.Add(1))
	if m.getItemErrOnCall > 0 && callNum == m.getItemErrOnCall {
		if m.getItemErrOnCallFn != nil {
			return nil, m.getItemErrOnCallFn()
		}
		return nil, errors.New("repo error on nth item call")
	}
	if m.getItemErr != nil {
		return nil, m.getItemErr
	}
	item, ok := m.items[itemID]
	if !ok {
		return nil, nil
	}
	return item, nil
}

func (m *mockRepository) CreateItem(ctx context.Context, item *models.MlLinkListItem) (*models.MlLinkListItem, error) {
	if m.createItemErr != nil {
		return nil, m.createItemErr
	}
	item.ID = "item-" + time.Now().Format("20060102150405.000000000")
	item.CreatedAt = time.Now().Format(time.RFC3339)
	item.UpdatedAt = item.CreatedAt
	m.items[item.ID] = item
	m.itemsByList[item.ListID] = append(m.itemsByList[item.ListID], *item)
	return item, nil
}

func (m *mockRepository) BulkCreateItems(ctx context.Context, items []models.MlLinkListItem) ([]models.MlLinkListItem, error) {
	if m.bulkCreateErr != nil {
		return nil, m.bulkCreateErr
	}
	created := make([]models.MlLinkListItem, 0, len(items))
	for i := range items {
		item := items[i]
		item.ID = fmt.Sprintf("item-%d-%s", i, time.Now().Format("20060102150405"))
		item.CreatedAt = time.Now().Format(time.RFC3339)
		item.UpdatedAt = item.CreatedAt
		m.items[item.ID] = &item
		m.itemsByList[item.ListID] = append(m.itemsByList[item.ListID], item)
		created = append(created, item)
	}
	return created, nil
}

func (m *mockRepository) UpdateItem(ctx context.Context, itemID string, updates map[string]interface{}) error {
	if m.updateItemErr != nil {
		return m.updateItemErr
	}
	item, ok := m.items[itemID]
	if !ok {
		return nil
	}
	if active, ok := updates["is_active"].(bool); ok {
		item.IsActive = active
	}
	if order, ok := updates["order_index"].(int32); ok {
		item.OrderIndex = int(order)
	}
	item.UpdatedAt = time.Now().Format(time.RFC3339)
	return nil
}

func (m *mockRepository) DeleteItem(ctx context.Context, itemID string) error {
	if m.deleteItemErr != nil {
		return m.deleteItemErr
	}
	delete(m.items, itemID)
	return nil
}

func (m *mockRepository) GetMaxOrderIndex(ctx context.Context, listID string) (int, error) {
	if m.maxOrderErr != nil {
		return 0, m.maxOrderErr
	}
	return m.maxOrderIndex[listID], nil
}

func (m *mockRepository) ResetConsumedItems(ctx context.Context, listID string) (int64, error) {
	if m.resetErr != nil {
		return 0, m.resetErr
	}
	count := int64(0)
	for _, item := range m.itemsByList[listID] {
		if item.ConsumedAt != nil {
			item.ConsumedAt = nil
			item.ConsumedByAutomationID = nil
			item.ConsumedByGroupID = nil
			count++
		}
	}
	return count, nil
}

func (m *mockRepository) ReorderItems(ctx context.Context, listID string, orderMap map[string]int32) error {
	if m.reorderErr != nil {
		return m.reorderErr
	}
	for itemID, order := range orderMap {
		if item, ok := m.items[itemID]; ok {
			item.OrderIndex = int(order)
		}
	}
	return nil
}

func (m *mockRepository) GetUnconsumedItems(ctx context.Context, listID string, limit int) ([]models.MlLinkListItem, error) {
	if m.getItemErr != nil {
		return nil, m.getItemErr
	}
	var unconsumed []models.MlLinkListItem
	for _, item := range m.itemsByList[listID] {
		if item.ConsumedAt == nil && item.IsActive {
			unconsumed = append(unconsumed, item)
			if len(unconsumed) >= limit {
				break
			}
		}
	}
	return unconsumed, nil
}

func (m *mockRepository) MarkConsumed(ctx context.Context, itemID, automationID string, consumedAt time.Time) error {
	callNum := int(m.markConsumedCallCount.Add(1))
	if m.markConsumedErrOnCall > 0 && callNum == m.markConsumedErrOnCall {
		return errors.New("item already consumed")
	}
	if m.markConsumedErr != nil {
		return m.markConsumedErr
	}
	if item, ok := m.items[itemID]; ok {
		ts := consumedAt.Format(time.RFC3339)
		item.ConsumedAt = &ts
		item.ConsumedByAutomationID = &automationID
	}
	return nil
}

// ============================================================================
// Mock history repository (audit trail)
// ============================================================================

type mockHistoryRepository struct {
	entries   []models.ActionHistory
	createErr error
}

func (m *mockHistoryRepository) Create(ctx context.Context, userID, actionType, description string, details map[string]interface{}) (*models.ActionHistory, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	entry := models.ActionHistory{
		ID:          fmt.Sprintf("ah-%d", len(m.entries)+1),
		UserID:      userID,
		ActionType:  actionType,
		Description: description,
		Details:     details,
		Status:      "completed",
	}
	m.entries = append(m.entries, entry)
	return &entry, nil
}

func (m *mockHistoryRepository) Update(_ context.Context, _ string, _ map[string]interface{}) error {
	return nil
}

func (m *mockHistoryRepository) UpdateStatus(_ context.Context, _, _, _ string) error {
	return nil
}

func (m *mockHistoryRepository) GetByID(_ context.Context, _ string) (*models.ActionHistory, error) {
	return nil, nil
}

func (m *mockHistoryRepository) GetUserHistory(_ context.Context, _ string, _, _ int) ([]models.ActionHistory, error) {
	return nil, nil
}

func (m *mockHistoryRepository) GetByUserID(_ context.Context, _ string, _, _ int) ([]models.ActionHistory, error) {
	return nil, nil
}

func (m *mockHistoryRepository) GetByActionType(_ context.Context, _, _ string, _ int) ([]models.ActionHistory, error) {
	return nil, nil
}

func (m *mockHistoryRepository) GetUserHistoryFiltered(_ context.Context, _ models.HistoryListRequest, _ string) ([]models.ActionHistory, error) {
	return nil, nil
}

func (m *mockHistoryRepository) CountUserHistoryFiltered(_ context.Context, _ models.HistoryListRequest, _ string) (models.HistoryStatsResult, error) {
	return models.HistoryStatsResult{}, nil
}

func (m *mockHistoryRepository) MarkCompleted(_ context.Context, _ string) error {
	return nil
}

func (m *mockHistoryRepository) MarkFailed(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockHistoryRepository) MarkRunning(_ context.Context, _ string) error {
	return nil
}

func (m *mockHistoryRepository) IncrementSuccessCount(_ context.Context, _ string) error {
	return nil
}

func (m *mockHistoryRepository) IncrementErrorCount(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockHistoryRepository) UpdateCounts(_ context.Context, _ string, _, _ int32) error {
	return nil
}

func (m *mockHistoryRepository) DeleteUserHistory(_ context.Context, _ string, _ *time.Time, _ []string) (int64, error) {
	return 0, nil
}

// ============================================================================
// Test helpers
// ============================================================================

func setupService() (Service, *mockRepository) {
	repo := newMockRepository()
	logger := zap.NewNop()
	svc := NewService(repo, logger)
	return svc, repo
}

func setupServiceWithAudit() (Service, *mockRepository, *mockHistoryRepository) {
	repo := newMockRepository()
	historyRepo := &mockHistoryRepository{}
	logger := zap.NewNop()
	svc := NewServiceWithAudit(repo, historyRepo, logger)
	return svc, repo, historyRepo
}

func seedList(repo *mockRepository, listID, userID, name string) *models.MlLinkList {
	list := &models.MlLinkList{
		ID:       listID,
		UserID:   userID,
		Name:     name,
		IsActive: true,
	}
	repo.lists[listID] = list
	repo.listsByUser[userID] = append(repo.listsByUser[userID], *list)
	return list
}

func seedItem(repo *mockRepository, itemID, listID, url string, orderIndex int, active bool) *models.MlLinkListItem {
	item := &models.MlLinkListItem{
		ID:         itemID,
		ListID:     listID,
		OriginURL:  url,
		OrderIndex: orderIndex,
		IsActive:   active,
		CreatedAt:  time.Now().Format(time.RFC3339),
		UpdatedAt:  time.Now().Format(time.RFC3339),
	}
	repo.items[itemID] = item
	repo.itemsByList[listID] = append(repo.itemsByList[listID], *item)
	return item
}

func ptr[T any](v T) *T {
	return &v
}

// ============================================================================
// Constructor tests
// ============================================================================

func TestNewService(t *testing.T) {
	t.Parallel()
	repo := newMockRepository()
	logger := zap.NewNop()
	svc := NewService(repo, logger)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestNewServiceWithAudit(t *testing.T) {
	t.Parallel()
	repo := newMockRepository()
	historyRepo := &mockHistoryRepository{}
	logger := zap.NewNop()
	svc := NewServiceWithAudit(repo, historyRepo, logger)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

// ============================================================================
// logAudit tests
// ============================================================================

func TestLogAudit_NilHistoryRepo(t *testing.T) {
	t.Parallel()
	svc, _ := setupService()
	ctx := context.Background()

	// Should not panic with nil historyRepo
	list, err := svc.CreateList(ctx, "user-1", CreateListInput{Name: "Test"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if list == nil {
		t.Fatal("expected list to be created")
	}
}

func TestLogAudit_WithHistoryRepo_Success(t *testing.T) {
	t.Parallel()
	svc, repo, historyRepo := setupServiceWithAudit()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")

	err := svc.DeleteList(ctx, "list-1", "user-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(historyRepo.entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(historyRepo.entries))
	}
	if historyRepo.entries[0].ActionType != models.ActionTypeMLLinkListDelete {
		t.Errorf("expected action type %s, got %s", models.ActionTypeMLLinkListDelete, historyRepo.entries[0].ActionType)
	}
}

func TestLogAudit_WithHistoryRepo_Error(t *testing.T) {
	t.Parallel()
	svc, _, historyRepo := setupServiceWithAudit()
	ctx := context.Background()

	historyRepo.createErr = errors.New("audit db error")

	// Should not fail the main operation even if audit fails
	list, err := svc.CreateList(ctx, "user-1", CreateListInput{Name: "Test"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if list == nil {
		t.Fatal("expected list to be created")
	}
}

// ============================================================================
// ListLists tests
// ============================================================================

func TestListLists_Success(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "List A")
	seedList(repo, "list-2", "user-1", "List B")

	lists, err := svc.ListLists(ctx, "user-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(lists) != 2 {
		t.Errorf("expected 2 lists, got %d", len(lists))
	}
}

func TestListLists_Empty(t *testing.T) {
	t.Parallel()
	svc, _ := setupService()
	ctx := context.Background()

	lists, err := svc.ListLists(ctx, "user-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if lists != nil && len(lists) != 0 {
		t.Errorf("expected empty list, got %d", len(lists))
	}
}

func TestListLists_RepoError(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()
	repo.getListErr = errors.New("db connection error")

	_, err := svc.ListLists(ctx, "user-1")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ============================================================================
// GetList tests
// ============================================================================

func TestGetList_Success(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "My List")

	list, err := svc.GetList(ctx, "list-1", "user-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if list.Name != "My List" {
		t.Errorf("expected 'My List', got %s", list.Name)
	}
}

func TestGetList_NotFound(t *testing.T) {
	t.Parallel()
	svc, _ := setupService()
	ctx := context.Background()

	_, err := svc.GetList(ctx, "nonexistent", "user-1")
	if !errors.Is(err, apperrors.ErrListNotFound) {
		t.Errorf("expected ErrListNotFound, got %v", err)
	}
}

func TestGetList_WrongUser(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "My List")

	_, err := svc.GetList(ctx, "list-1", "user-other")
	if !errors.Is(err, apperrors.ErrListNotFound) {
		t.Errorf("expected ErrListNotFound for wrong user, got %v", err)
	}
}

func TestGetList_RepoError(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()
	repo.getListErr = errors.New("db error")

	_, err := svc.GetList(ctx, "list-1", "user-1")
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, apperrors.ErrListNotFound) {
		t.Error("should be internal error, not ErrListNotFound")
	}
}

// ============================================================================
// CreateList tests
// ============================================================================

func TestCreateList_Success(t *testing.T) {
	t.Parallel()
	svc, _ := setupService()
	ctx := context.Background()

	list, err := svc.CreateList(ctx, "user-1", CreateListInput{Name: "Test List"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if list.Name != "Test List" {
		t.Errorf("expected name 'Test List', got %s", list.Name)
	}
	if !list.IsActive {
		t.Error("expected list to be active by default")
	}
}

func TestCreateList_EmptyName(t *testing.T) {
	t.Parallel()
	svc, _ := setupService()
	ctx := context.Background()

	_, err := svc.CreateList(ctx, "user-1", CreateListInput{Name: ""})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
	var mlErr *apperrors.MLLinksError
	if !errors.As(err, &mlErr) {
		t.Errorf("expected MLLinksError, got %T", err)
	}
}

func TestCreateList_WithDescription(t *testing.T) {
	t.Parallel()
	svc, _ := setupService()
	ctx := context.Background()

	desc := "A test list description"
	list, err := svc.CreateList(ctx, "user-1", CreateListInput{
		Name:        "Test List",
		Description: &desc,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if list.Description == nil || *list.Description != desc {
		t.Errorf("expected description %q, got %v", desc, list.Description)
	}
}

func TestCreateList_WithIsActiveFalse(t *testing.T) {
	t.Parallel()
	svc, _ := setupService()
	ctx := context.Background()

	list, err := svc.CreateList(ctx, "user-1", CreateListInput{
		Name:     "Inactive List",
		IsActive: ptr(false),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if list.IsActive {
		t.Error("expected list to be inactive")
	}
}

func TestCreateList_RepoError(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()
	repo.createListErr = errors.New("db insert error")

	_, err := svc.CreateList(ctx, "user-1", CreateListInput{Name: "Test"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateList_AuditTrail(t *testing.T) {
	t.Parallel()
	svc, _, historyRepo := setupServiceWithAudit()
	ctx := context.Background()

	_, err := svc.CreateList(ctx, "user-1", CreateListInput{Name: "Test"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(historyRepo.entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(historyRepo.entries))
	}
	if historyRepo.entries[0].ActionType != models.ActionTypeMLLinkListCreate {
		t.Errorf("expected %s, got %s", models.ActionTypeMLLinkListCreate, historyRepo.entries[0].ActionType)
	}
}

// ============================================================================
// UpdateList tests
// ============================================================================

func TestUpdateList_Success(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Old Name")

	updated, err := svc.UpdateList(ctx, "list-1", "user-1", UpdateListInput{
		Name:        ptr("New Name"),
		Description: ptr("New Desc"),
		IsActive:    ptr(false),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.Name != "New Name" {
		t.Errorf("expected 'New Name', got %s", updated.Name)
	}
}

func TestUpdateList_NotFound(t *testing.T) {
	t.Parallel()
	svc, _ := setupService()
	ctx := context.Background()

	_, err := svc.UpdateList(ctx, "nonexistent", "user-1", UpdateListInput{Name: ptr("X")})
	if !errors.Is(err, apperrors.ErrListNotFound) {
		t.Errorf("expected ErrListNotFound, got %v", err)
	}
}

func TestUpdateList_OwnershipCheckError(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()
	repo.getListErr = errors.New("db error")

	_, err := svc.UpdateList(ctx, "list-1", "user-1", UpdateListInput{Name: ptr("X")})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateList_NoUpdates(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Original")

	updated, err := svc.UpdateList(ctx, "list-1", "user-1", UpdateListInput{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.Name != "Original" {
		t.Errorf("expected name unchanged, got %s", updated.Name)
	}
}

func TestUpdateList_EmptyNameIgnored(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Original")

	emptyName := ""
	updated, err := svc.UpdateList(ctx, "list-1", "user-1", UpdateListInput{Name: &emptyName})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// Empty name should be ignored (not added to updates map)
	if updated.Name != "Original" {
		t.Errorf("expected name unchanged, got %s", updated.Name)
	}
}

func TestUpdateList_RepoUpdateError(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Original")
	repo.updateListErr = errors.New("db update error")

	_, err := svc.UpdateList(ctx, "list-1", "user-1", UpdateListInput{Name: ptr("New")})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateList_ReFetchError(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Original")
	// First call succeeds (ownership check), second call fails (re-fetch)
	repo.getListErrOnCall = 2

	_, err := svc.UpdateList(ctx, "list-1", "user-1", UpdateListInput{Name: ptr("New")})
	if err == nil {
		t.Fatal("expected error on re-fetch")
	}
}

func TestUpdateList_AuditTrail(t *testing.T) {
	t.Parallel()
	svc, repo, historyRepo := setupServiceWithAudit()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Original")

	_, err := svc.UpdateList(ctx, "list-1", "user-1", UpdateListInput{Name: ptr("Updated")})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(historyRepo.entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(historyRepo.entries))
	}
	if historyRepo.entries[0].ActionType != models.ActionTypeMLLinkListUpdate {
		t.Errorf("expected %s, got %s", models.ActionTypeMLLinkListUpdate, historyRepo.entries[0].ActionType)
	}
}

// ============================================================================
// DeleteList tests
// ============================================================================

func TestDeleteList_Success(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")

	err := svc.DeleteList(ctx, "list-1", "user-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if _, ok := repo.lists["list-1"]; ok {
		t.Error("expected list to be deleted")
	}
}

func TestDeleteList_RepoError(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()
	repo.deleteListErr = errors.New("db delete error")

	err := svc.DeleteList(ctx, "list-1", "user-1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDeleteList_AuditTrail(t *testing.T) {
	t.Parallel()
	svc, repo, historyRepo := setupServiceWithAudit()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")

	err := svc.DeleteList(ctx, "list-1", "user-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(historyRepo.entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(historyRepo.entries))
	}
	if historyRepo.entries[0].ActionType != models.ActionTypeMLLinkListDelete {
		t.Errorf("expected %s, got %s", models.ActionTypeMLLinkListDelete, historyRepo.entries[0].ActionType)
	}
}

// ============================================================================
// ListItems tests
// ============================================================================

func TestListItems_Success(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	seedItem(repo, "item-1", "list-1", "https://mercadolivre.com.br/p/1", 0, true)
	seedItem(repo, "item-2", "list-1", "https://mercadolivre.com.br/p/2", 1, true)

	items, err := svc.ListItems(ctx, "list-1", "user-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestListItems_ListNotFound(t *testing.T) {
	t.Parallel()
	svc, _ := setupService()
	ctx := context.Background()

	_, err := svc.ListItems(ctx, "nonexistent", "user-1")
	if !errors.Is(err, apperrors.ErrListNotFound) {
		t.Errorf("expected ErrListNotFound, got %v", err)
	}
}

func TestListItems_OwnershipCheckError(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()
	repo.getListErr = errors.New("db error")

	_, err := svc.ListItems(ctx, "list-1", "user-1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListItems_ItemsFetchError(t *testing.T) {
	t.Parallel()
	repo := newMockRepository()
	logger := zap.NewNop()
	svc := NewService(repo, logger)
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	// Set item error AFTER ownership check passes
	repo.getItemErr = errors.New("db error fetching items")

	// The ownership check uses getListErr, item fetch uses getItemErr
	// But GetListByIDAndUser uses getListErr, not getItemErr, so set it correctly
	// Actually, looking at the mock: GetListByIDAndUser uses getListErr, GetItemsByListID uses getItemErr
	// So we need to NOT set getListErr (so ownership passes) but set getItemErr (so items fetch fails)
	// This is already correct above

	_, err := svc.ListItems(ctx, "list-1", "user-1")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ============================================================================
// ListItemsPaginated tests
// ============================================================================

func TestListItemsPaginated_Success(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	seedItem(repo, "item-1", "list-1", "https://mercadolivre.com.br/p/1", 0, true)
	seedItem(repo, "item-2", "list-1", "https://mercadolivre.com.br/p/2", 1, true)

	result, err := svc.ListItemsPaginated(ctx, "list-1", "user-1", PaginationInput{PageSize: 10})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(result.Items))
	}
	if result.TotalCount != 2 {
		t.Errorf("expected total 2, got %d", result.TotalCount)
	}
}

func TestListItemsPaginated_DefaultPageSize(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	seedItem(repo, "item-1", "list-1", "https://mercadolivre.com.br/p/1", 0, true)

	result, err := svc.ListItemsPaginated(ctx, "list-1", "user-1", PaginationInput{PageSize: 0})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// Default page size 20 should be used; we have 1 item
	if len(result.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(result.Items))
	}
}

func TestListItemsPaginated_NegativePageSize(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	seedItem(repo, "item-1", "list-1", "https://mercadolivre.com.br/p/1", 0, true)

	result, err := svc.ListItemsPaginated(ctx, "list-1", "user-1", PaginationInput{PageSize: -5})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(result.Items))
	}
}

func TestListItemsPaginated_MaxPageSizeCap(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	seedItem(repo, "item-1", "list-1", "https://mercadolivre.com.br/p/1", 0, true)

	// PageSize > 100 should be capped to 100
	result, err := svc.ListItemsPaginated(ctx, "list-1", "user-1", PaginationInput{PageSize: 200})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(result.Items))
	}
}

func TestListItemsPaginated_HasMore(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	for i := 0; i < 5; i++ {
		seedItem(repo, fmt.Sprintf("item-%d", i), "list-1",
			fmt.Sprintf("https://mercadolivre.com.br/p/%d", i), i, true)
	}

	result, err := svc.ListItemsPaginated(ctx, "list-1", "user-1", PaginationInput{PageSize: 2})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(result.Items))
	}
	if !result.HasMore {
		t.Error("expected HasMore to be true")
	}
	if result.NextPageToken == "" {
		t.Error("expected non-empty NextPageToken")
	}
}

func TestListItemsPaginated_ListNotFound(t *testing.T) {
	t.Parallel()
	svc, _ := setupService()
	ctx := context.Background()

	_, err := svc.ListItemsPaginated(ctx, "nonexistent", "user-1", PaginationInput{PageSize: 10})
	if !errors.Is(err, apperrors.ErrListNotFound) {
		t.Errorf("expected ErrListNotFound, got %v", err)
	}
}

func TestListItemsPaginated_OwnershipCheckError(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()
	repo.getListErr = errors.New("db error")

	_, err := svc.ListItemsPaginated(ctx, "list-1", "user-1", PaginationInput{PageSize: 10})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListItemsPaginated_RepoError(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	repo.paginatedErr = errors.New("paginated query error")

	_, err := svc.ListItemsPaginated(ctx, "list-1", "user-1", PaginationInput{PageSize: 10})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListItemsPaginated_WithPageToken(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	seedItem(repo, "item-1", "list-1", "https://mercadolivre.com.br/p/1", 0, true)

	result, err := svc.ListItemsPaginated(ctx, "list-1", "user-1", PaginationInput{
		PageSize:  10,
		PageToken: "some-cursor",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// ============================================================================
// CreateItem tests
// ============================================================================

func TestCreateItem_Success(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")

	item, err := svc.CreateItem(ctx, "user-1", CreateItemInput{
		ListID:    "list-1",
		OriginURL: "https://www.mercadolivre.com.br/p/MLB12345",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if item.ListID != "list-1" {
		t.Errorf("expected list_id 'list-1', got %s", item.ListID)
	}
	if !item.IsActive {
		t.Error("expected item to be active by default")
	}
	// Auto-increment order index: maxOrderIndex[list-1] = 0, so item should get 1
	if item.OrderIndex != 1 {
		t.Errorf("expected order_index 1, got %d", item.OrderIndex)
	}
}

func TestCreateItem_WithOrderIndex(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")

	item, err := svc.CreateItem(ctx, "user-1", CreateItemInput{
		ListID:     "list-1",
		OriginURL:  "https://www.mercadolivre.com.br/p/MLB12345",
		OrderIndex: ptr(int32(42)),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if item.OrderIndex != 42 {
		t.Errorf("expected order_index 42, got %d", item.OrderIndex)
	}
}

func TestCreateItem_WithOptionalFields(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")

	item, err := svc.CreateItem(ctx, "user-1", CreateItemInput{
		ListID:          "list-1",
		OriginURL:       "https://www.mercadolivre.com.br/p/MLB12345",
		IsActive:        ptr(false),
		ProductTitle:    ptr("Test Product"),
		ProductPrice:    ptr(29.99),
		ProductImageURL: ptr("https://example.com/img.jpg"),
		Metadata:        ptr(`{"key":"value"}`),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if item.IsActive {
		t.Error("expected inactive item")
	}
	if item.ProductTitle == nil || *item.ProductTitle != "Test Product" {
		t.Errorf("expected product title 'Test Product', got %v", item.ProductTitle)
	}
	if item.ProductPrice == nil || *item.ProductPrice != 29.99 {
		t.Errorf("expected price 29.99, got %v", item.ProductPrice)
	}
	if item.ProductImageURL == nil || *item.ProductImageURL != "https://example.com/img.jpg" {
		t.Errorf("expected image URL, got %v", item.ProductImageURL)
	}
	if string(item.Metadata) != `{"key":"value"}` {
		t.Errorf("expected metadata, got %s", string(item.Metadata))
	}
}

func TestCreateItem_InvalidURL(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")

	_, err := svc.CreateItem(ctx, "user-1", CreateItemInput{
		ListID:    "list-1",
		OriginURL: "https://example.com/not-mercadolivre",
	})
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestCreateItem_ListNotFound(t *testing.T) {
	t.Parallel()
	svc, _ := setupService()
	ctx := context.Background()

	_, err := svc.CreateItem(ctx, "user-1", CreateItemInput{
		ListID:    "nonexistent",
		OriginURL: "https://www.mercadolivre.com.br/p/MLB12345",
	})
	if !errors.Is(err, apperrors.ErrListNotFound) {
		t.Errorf("expected ErrListNotFound, got %v", err)
	}
}

func TestCreateItem_OwnershipCheckError(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()
	repo.getListErr = errors.New("db error")

	_, err := svc.CreateItem(ctx, "user-1", CreateItemInput{
		ListID:    "list-1",
		OriginURL: "https://www.mercadolivre.com.br/p/MLB12345",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateItem_InvalidMetadataJSON(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")

	_, err := svc.CreateItem(ctx, "user-1", CreateItemInput{
		ListID:    "list-1",
		OriginURL: "https://www.mercadolivre.com.br/p/MLB12345",
		Metadata:  ptr("not valid json {{{"),
	})
	if err == nil {
		t.Fatal("expected error for invalid metadata JSON")
	}
}

func TestCreateItem_EmptyMetadataIgnored(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")

	item, err := svc.CreateItem(ctx, "user-1", CreateItemInput{
		ListID:    "list-1",
		OriginURL: "https://www.mercadolivre.com.br/p/MLB12345",
		Metadata:  ptr(""),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if item == nil {
		t.Fatal("expected item")
	}
}

func TestCreateItem_GetMaxOrderIndexError(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	repo.maxOrderErr = errors.New("max order error")

	_, err := svc.CreateItem(ctx, "user-1", CreateItemInput{
		ListID:    "list-1",
		OriginURL: "https://www.mercadolivre.com.br/p/MLB12345",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateItem_RepoCreateError(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	repo.createItemErr = errors.New("db insert error")

	_, err := svc.CreateItem(ctx, "user-1", CreateItemInput{
		ListID:    "list-1",
		OriginURL: "https://www.mercadolivre.com.br/p/MLB12345",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateItem_AuditTrail(t *testing.T) {
	t.Parallel()
	svc, repo, historyRepo := setupServiceWithAudit()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")

	_, err := svc.CreateItem(ctx, "user-1", CreateItemInput{
		ListID:    "list-1",
		OriginURL: "https://www.mercadolivre.com.br/p/MLB12345",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(historyRepo.entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(historyRepo.entries))
	}
	if historyRepo.entries[0].ActionType != models.ActionTypeMLLinkItemCreate {
		t.Errorf("expected %s, got %s", models.ActionTypeMLLinkItemCreate, historyRepo.entries[0].ActionType)
	}
}

// ============================================================================
// BulkCreateItems tests
// ============================================================================

func TestBulkCreateItems_Success(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")

	result, err := svc.BulkCreateItems(ctx, "user-1", BulkCreateItemsInput{
		ListID: "list-1",
		OriginURLs: []string{
			"https://www.mercadolivre.com.br/p/MLB12345",
			"https://www.mercadolivre.com.br/p/MLB67890",
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.CreatedCount != 2 {
		t.Errorf("expected 2 created, got %d", result.CreatedCount)
	}
	if result.SkippedCount != 0 {
		t.Errorf("expected 0 skipped, got %d", result.SkippedCount)
	}
}

func TestBulkCreateItems_WithInvalidURLs(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")

	result, err := svc.BulkCreateItems(ctx, "user-1", BulkCreateItemsInput{
		ListID: "list-1",
		OriginURLs: []string{
			"https://www.mercadolivre.com.br/p/MLB12345",
			"https://example.com/invalid",
			"not-a-url",
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.CreatedCount != 1 {
		t.Errorf("expected 1 created, got %d", result.CreatedCount)
	}
	if result.SkippedCount != 2 {
		t.Errorf("expected 2 skipped, got %d", result.SkippedCount)
	}
}

func TestBulkCreateItems_ListNotFound(t *testing.T) {
	t.Parallel()
	svc, _ := setupService()
	ctx := context.Background()

	_, err := svc.BulkCreateItems(ctx, "user-1", BulkCreateItemsInput{
		ListID:     "nonexistent",
		OriginURLs: []string{"https://www.mercadolivre.com.br/p/MLB12345"},
	})
	if !errors.Is(err, apperrors.ErrListNotFound) {
		t.Errorf("expected ErrListNotFound, got %v", err)
	}
}

func TestBulkCreateItems_OwnershipCheckError(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()
	repo.getListErr = errors.New("db error")

	_, err := svc.BulkCreateItems(ctx, "user-1", BulkCreateItemsInput{
		ListID:     "list-1",
		OriginURLs: []string{"https://www.mercadolivre.com.br/p/MLB12345"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBulkCreateItems_WithDuplicatesInBatch(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")

	result, err := svc.BulkCreateItems(ctx, "user-1", BulkCreateItemsInput{
		ListID: "list-1",
		OriginURLs: []string{
			"https://www.mercadolivre.com.br/p/MLB12345",
			"https://www.mercadolivre.com.br/p/MLB12345", // duplicate
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.CreatedCount != 1 {
		t.Errorf("expected 1 created (deduped), got %d", result.CreatedCount)
	}
}

func TestBulkCreateItems_WithExistingURLsInList(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	seedItem(repo, "existing-1", "list-1", "https://www.mercadolivre.com.br/p/MLB12345", 0, true)

	result, err := svc.BulkCreateItems(ctx, "user-1", BulkCreateItemsInput{
		ListID: "list-1",
		OriginURLs: []string{
			"https://www.mercadolivre.com.br/p/MLB12345", // already exists
			"https://www.mercadolivre.com.br/p/MLB67890", // new
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.CreatedCount != 1 {
		t.Errorf("expected 1 created, got %d", result.CreatedCount)
	}
	if result.SkippedCount != 1 {
		t.Errorf("expected 1 skipped (existing), got %d", result.SkippedCount)
	}
}

func TestBulkCreateItems_AllInvalidURLs(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")

	result, err := svc.BulkCreateItems(ctx, "user-1", BulkCreateItemsInput{
		ListID: "list-1",
		OriginURLs: []string{
			"https://example.com/invalid1",
			"not-a-url",
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.CreatedCount != 0 {
		t.Errorf("expected 0 created, got %d", result.CreatedCount)
	}
	if result.SkippedCount != 2 {
		t.Errorf("expected 2 skipped, got %d", result.SkippedCount)
	}
}

func TestBulkCreateItems_EmptyURLs(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")

	result, err := svc.BulkCreateItems(ctx, "user-1", BulkCreateItemsInput{
		ListID:     "list-1",
		OriginURLs: []string{},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.CreatedCount != 0 {
		t.Errorf("expected 0 created, got %d", result.CreatedCount)
	}
}

func TestBulkCreateItems_WithBaseOrderIndex(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")

	result, err := svc.BulkCreateItems(ctx, "user-1", BulkCreateItemsInput{
		ListID: "list-1",
		OriginURLs: []string{
			"https://www.mercadolivre.com.br/p/MLB12345",
			"https://www.mercadolivre.com.br/p/MLB67890",
		},
		BaseOrderIndex: ptr(int32(10)),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.CreatedCount != 2 {
		t.Errorf("expected 2 created, got %d", result.CreatedCount)
	}
}

func TestBulkCreateItems_GetExistingItemsError(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	repo.getItemErr = errors.New("db error")

	_, err := svc.BulkCreateItems(ctx, "user-1", BulkCreateItemsInput{
		ListID:     "list-1",
		OriginURLs: []string{"https://www.mercadolivre.com.br/p/MLB12345"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBulkCreateItems_GetMaxOrderIndexError(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	repo.maxOrderErr = errors.New("max order error")

	_, err := svc.BulkCreateItems(ctx, "user-1", BulkCreateItemsInput{
		ListID:     "list-1",
		OriginURLs: []string{"https://www.mercadolivre.com.br/p/MLB12345"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBulkCreateItems_RepoBulkCreateError(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	repo.bulkCreateErr = errors.New("db bulk insert error")

	_, err := svc.BulkCreateItems(ctx, "user-1", BulkCreateItemsInput{
		ListID:     "list-1",
		OriginURLs: []string{"https://www.mercadolivre.com.br/p/MLB12345"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBulkCreateItems_AuditTrail_CreatedItems(t *testing.T) {
	t.Parallel()
	svc, repo, historyRepo := setupServiceWithAudit()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")

	_, err := svc.BulkCreateItems(ctx, "user-1", BulkCreateItemsInput{
		ListID:     "list-1",
		OriginURLs: []string{"https://www.mercadolivre.com.br/p/MLB12345"},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(historyRepo.entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(historyRepo.entries))
	}
	if historyRepo.entries[0].ActionType != models.ActionTypeMLLinkItemBulkCreate {
		t.Errorf("expected %s, got %s", models.ActionTypeMLLinkItemBulkCreate, historyRepo.entries[0].ActionType)
	}
}

func TestBulkCreateItems_AuditTrail_NoItemsCreated(t *testing.T) {
	t.Parallel()
	svc, repo, historyRepo := setupServiceWithAudit()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")

	// All invalid → no items created → no audit
	_, err := svc.BulkCreateItems(ctx, "user-1", BulkCreateItemsInput{
		ListID:     "list-1",
		OriginURLs: []string{"https://example.com/invalid"},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(historyRepo.entries) != 0 {
		t.Errorf("expected 0 audit entries when no items created, got %d", len(historyRepo.entries))
	}
}

// ============================================================================
// UpdateItem tests
// ============================================================================

func TestUpdateItem_Success(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	seedItem(repo, "item-1", "list-1", "https://mercadolivre.com.br/p/1", 0, true)

	updated, err := svc.UpdateItem(ctx, "user-1", "item-1", UpdateItemInput{
		IsActive:   ptr(false),
		OrderIndex: ptr(int32(5)),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated == nil {
		t.Fatal("expected updated item")
	}
}

func TestUpdateItem_ItemNotFound(t *testing.T) {
	t.Parallel()
	svc, _ := setupService()
	ctx := context.Background()

	_, err := svc.UpdateItem(ctx, "user-1", "nonexistent", UpdateItemInput{IsActive: ptr(false)})
	if !errors.Is(err, apperrors.ErrItemNotFound) {
		t.Errorf("expected ErrItemNotFound, got %v", err)
	}
}

func TestUpdateItem_ListNotFound(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	// Item exists but its list doesn't belong to user
	item := &models.MlLinkListItem{
		ID:     "item-1",
		ListID: "list-1",
	}
	repo.items["item-1"] = item

	_, err := svc.UpdateItem(ctx, "user-1", "item-1", UpdateItemInput{IsActive: ptr(false)})
	if !errors.Is(err, apperrors.ErrListNotFound) {
		t.Errorf("expected ErrListNotFound, got %v", err)
	}
}

func TestUpdateItem_GetItemError(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()
	repo.getItemErr = errors.New("db error")

	_, err := svc.UpdateItem(ctx, "user-1", "item-1", UpdateItemInput{IsActive: ptr(false)})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateItem_OwnershipCheckError(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	item := &models.MlLinkListItem{
		ID:     "item-1",
		ListID: "list-1",
	}
	repo.items["item-1"] = item
	repo.getListErr = errors.New("db error")

	_, err := svc.UpdateItem(ctx, "user-1", "item-1", UpdateItemInput{IsActive: ptr(false)})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateItem_NoUpdates(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	seedItem(repo, "item-1", "list-1", "https://mercadolivre.com.br/p/1", 0, true)

	updated, err := svc.UpdateItem(ctx, "user-1", "item-1", UpdateItemInput{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated == nil {
		t.Fatal("expected item returned even with no updates")
	}
}

func TestUpdateItem_RepoUpdateError(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	seedItem(repo, "item-1", "list-1", "https://mercadolivre.com.br/p/1", 0, true)
	repo.updateItemErr = errors.New("db update error")

	_, err := svc.UpdateItem(ctx, "user-1", "item-1", UpdateItemInput{IsActive: ptr(false)})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateItem_ReFetchError(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	seedItem(repo, "item-1", "list-1", "https://mercadolivre.com.br/p/1", 0, true)
	// First call succeeds (get item), second call fails (re-fetch)
	repo.getItemErrOnCall = 2
	repo.getItemErrOnCallFn = func() error { return errors.New("re-fetch error") }

	_, err := svc.UpdateItem(ctx, "user-1", "item-1", UpdateItemInput{IsActive: ptr(false)})
	if err == nil {
		t.Fatal("expected error on re-fetch")
	}
}

func TestUpdateItem_AuditTrail(t *testing.T) {
	t.Parallel()
	svc, repo, historyRepo := setupServiceWithAudit()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	seedItem(repo, "item-1", "list-1", "https://mercadolivre.com.br/p/1", 0, true)

	_, err := svc.UpdateItem(ctx, "user-1", "item-1", UpdateItemInput{IsActive: ptr(false)})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(historyRepo.entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(historyRepo.entries))
	}
	if historyRepo.entries[0].ActionType != models.ActionTypeMLLinkItemUpdate {
		t.Errorf("expected %s, got %s", models.ActionTypeMLLinkItemUpdate, historyRepo.entries[0].ActionType)
	}
}

// ============================================================================
// DeleteItem tests
// ============================================================================

func TestDeleteItem_Success(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	seedItem(repo, "item-1", "list-1", "https://mercadolivre.com.br/p/1", 0, true)

	err := svc.DeleteItem(ctx, "user-1", "item-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if _, ok := repo.items["item-1"]; ok {
		t.Error("expected item to be deleted")
	}
}

func TestDeleteItem_ItemNotFound(t *testing.T) {
	t.Parallel()
	svc, _ := setupService()
	ctx := context.Background()

	err := svc.DeleteItem(ctx, "user-1", "nonexistent")
	if !errors.Is(err, apperrors.ErrItemNotFound) {
		t.Errorf("expected ErrItemNotFound, got %v", err)
	}
}

func TestDeleteItem_ListNotFound(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	item := &models.MlLinkListItem{
		ID:     "item-1",
		ListID: "list-1",
	}
	repo.items["item-1"] = item

	err := svc.DeleteItem(ctx, "user-1", "item-1")
	if !errors.Is(err, apperrors.ErrListNotFound) {
		t.Errorf("expected ErrListNotFound, got %v", err)
	}
}

func TestDeleteItem_GetItemError(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()
	repo.getItemErr = errors.New("db error")

	err := svc.DeleteItem(ctx, "user-1", "item-1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDeleteItem_OwnershipCheckError(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	item := &models.MlLinkListItem{
		ID:     "item-1",
		ListID: "list-1",
	}
	repo.items["item-1"] = item
	repo.getListErr = errors.New("db error")

	err := svc.DeleteItem(ctx, "user-1", "item-1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDeleteItem_RepoDeleteError(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	seedItem(repo, "item-1", "list-1", "https://mercadolivre.com.br/p/1", 0, true)
	repo.deleteItemErr = errors.New("db delete error")

	err := svc.DeleteItem(ctx, "user-1", "item-1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDeleteItem_AuditTrail(t *testing.T) {
	t.Parallel()
	svc, repo, historyRepo := setupServiceWithAudit()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	seedItem(repo, "item-1", "list-1", "https://mercadolivre.com.br/p/1", 0, true)

	err := svc.DeleteItem(ctx, "user-1", "item-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(historyRepo.entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(historyRepo.entries))
	}
	if historyRepo.entries[0].ActionType != models.ActionTypeMLLinkItemDelete {
		t.Errorf("expected %s, got %s", models.ActionTypeMLLinkItemDelete, historyRepo.entries[0].ActionType)
	}
}

// ============================================================================
// ResetConsumed tests
// ============================================================================

func TestResetConsumed_Success(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	consumedAt := time.Now().Format(time.RFC3339)
	item := models.MlLinkListItem{
		ID:         "item-1",
		ListID:     "list-1",
		OriginURL:  "https://mercadolivre.com.br/p/1",
		IsActive:   true,
		ConsumedAt: &consumedAt,
	}
	repo.items["item-1"] = &item
	repo.itemsByList["list-1"] = []models.MlLinkListItem{item}

	count, err := svc.ResetConsumed(ctx, "user-1", "list-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 item reset, got %d", count)
	}
}

func TestResetConsumed_ListNotFound(t *testing.T) {
	t.Parallel()
	svc, _ := setupService()
	ctx := context.Background()

	_, err := svc.ResetConsumed(ctx, "user-1", "nonexistent")
	if !errors.Is(err, apperrors.ErrListNotFound) {
		t.Errorf("expected ErrListNotFound, got %v", err)
	}
}

func TestResetConsumed_OwnershipCheckError(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()
	repo.getListErr = errors.New("db error")

	_, err := svc.ResetConsumed(ctx, "user-1", "list-1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResetConsumed_RepoResetError(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	repo.resetErr = errors.New("db reset error")

	_, err := svc.ResetConsumed(ctx, "user-1", "list-1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResetConsumed_AuditTrail(t *testing.T) {
	t.Parallel()
	svc, repo, historyRepo := setupServiceWithAudit()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")

	_, err := svc.ResetConsumed(ctx, "user-1", "list-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(historyRepo.entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(historyRepo.entries))
	}
	if historyRepo.entries[0].ActionType != models.ActionTypeMLLinkItemsReset {
		t.Errorf("expected %s, got %s", models.ActionTypeMLLinkItemsReset, historyRepo.entries[0].ActionType)
	}
}

// ============================================================================
// ReorderItems tests
// ============================================================================

func TestReorderItems_Success(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	seedItem(repo, "item-1", "list-1", "https://mercadolivre.com.br/p/1", 0, true)
	seedItem(repo, "item-2", "list-1", "https://mercadolivre.com.br/p/2", 1, true)

	err := svc.ReorderItems(ctx, "user-1", "list-1", map[string]int32{
		"item-1": 2,
		"item-2": 1,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if repo.items["item-1"].OrderIndex != 2 {
		t.Errorf("expected item-1 order 2, got %d", repo.items["item-1"].OrderIndex)
	}
	if repo.items["item-2"].OrderIndex != 1 {
		t.Errorf("expected item-2 order 1, got %d", repo.items["item-2"].OrderIndex)
	}
}

func TestReorderItems_ListNotFound(t *testing.T) {
	t.Parallel()
	svc, _ := setupService()
	ctx := context.Background()

	err := svc.ReorderItems(ctx, "user-1", "nonexistent", map[string]int32{"item-1": 1})
	if !errors.Is(err, apperrors.ErrListNotFound) {
		t.Errorf("expected ErrListNotFound, got %v", err)
	}
}

func TestReorderItems_OwnershipCheckError(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()
	repo.getListErr = errors.New("db error")

	err := svc.ReorderItems(ctx, "user-1", "list-1", map[string]int32{"item-1": 1})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReorderItems_RepoError(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	repo.reorderErr = errors.New("db reorder error")

	err := svc.ReorderItems(ctx, "user-1", "list-1", map[string]int32{"item-1": 1})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReorderItems_AuditTrail(t *testing.T) {
	t.Parallel()
	svc, repo, historyRepo := setupServiceWithAudit()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	seedItem(repo, "item-1", "list-1", "https://mercadolivre.com.br/p/1", 0, true)

	err := svc.ReorderItems(ctx, "user-1", "list-1", map[string]int32{"item-1": 5})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(historyRepo.entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(historyRepo.entries))
	}
	if historyRepo.entries[0].ActionType != models.ActionTypeMLLinkItemsReorder {
		t.Errorf("expected %s, got %s", models.ActionTypeMLLinkItemsReorder, historyRepo.entries[0].ActionType)
	}
}

// ============================================================================
// ConsumeNextItems tests
// ============================================================================

func TestConsumeNextItems_Success(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	seedItem(repo, "item-1", "list-1", "https://mercadolivre.com.br/p/1", 0, true)

	consumed, err := svc.ConsumeNextItems(ctx, "list-1", "automation-1", 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(consumed) != 1 {
		t.Errorf("expected 1 consumed, got %d", len(consumed))
	}
	if consumed[0].ConsumedAt == nil {
		t.Error("expected consumed_at to be set")
	}
	if consumed[0].ConsumedByAutomationID == nil || *consumed[0].ConsumedByAutomationID != "automation-1" {
		t.Error("expected consumed_by_automation_id to be set")
	}
}

func TestConsumeNextItems_MultipleItems(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	seedItem(repo, "item-1", "list-1", "https://mercadolivre.com.br/p/1", 0, true)
	seedItem(repo, "item-2", "list-1", "https://mercadolivre.com.br/p/2", 1, true)
	seedItem(repo, "item-3", "list-1", "https://mercadolivre.com.br/p/3", 2, true)

	consumed, err := svc.ConsumeNextItems(ctx, "list-1", "automation-1", 2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(consumed) != 2 {
		t.Errorf("expected 2 consumed, got %d", len(consumed))
	}
}

func TestConsumeNextItems_NoItemsAvailable(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	repo.itemsByList["list-1"] = []models.MlLinkListItem{}

	_, err := svc.ConsumeNextItems(ctx, "list-1", "automation-1", 1)
	if !errors.Is(err, apperrors.ErrNoItemsAvailable) {
		t.Errorf("expected ErrNoItemsAvailable, got %v", err)
	}
}

func TestConsumeNextItems_CountZeroDefaultsToOne(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	seedItem(repo, "item-1", "list-1", "https://mercadolivre.com.br/p/1", 0, true)
	seedItem(repo, "item-2", "list-1", "https://mercadolivre.com.br/p/2", 1, true)

	consumed, err := svc.ConsumeNextItems(ctx, "list-1", "automation-1", 0)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(consumed) != 1 {
		t.Errorf("expected 1 consumed (default count), got %d", len(consumed))
	}
}

func TestConsumeNextItems_NegativeCountDefaultsToOne(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	seedItem(repo, "item-1", "list-1", "https://mercadolivre.com.br/p/1", 0, true)

	consumed, err := svc.ConsumeNextItems(ctx, "list-1", "automation-1", -5)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(consumed) != 1 {
		t.Errorf("expected 1 consumed, got %d", len(consumed))
	}
}

func TestConsumeNextItems_RaceCondition_SomeConsumed(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	seedItem(repo, "item-1", "list-1", "https://mercadolivre.com.br/p/1", 0, true)
	seedItem(repo, "item-2", "list-1", "https://mercadolivre.com.br/p/2", 1, true)

	// First MarkConsumed call fails (simulates race condition), second succeeds
	repo.markConsumedErrOnCall = 1

	consumed, err := svc.ConsumeNextItems(ctx, "list-1", "automation-1", 2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(consumed) != 1 {
		t.Errorf("expected 1 consumed (one skipped due to race), got %d", len(consumed))
	}
}

func TestConsumeNextItems_RaceCondition_AllConsumed(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	seedItem(repo, "item-1", "list-1", "https://mercadolivre.com.br/p/1", 0, true)

	// All MarkConsumed calls fail
	repo.markConsumedErr = errors.New("item already consumed")

	_, err := svc.ConsumeNextItems(ctx, "list-1", "automation-1", 1)
	if !errors.Is(err, apperrors.ErrNoItemsAvailable) {
		t.Errorf("expected ErrNoItemsAvailable, got %v", err)
	}
}

func TestConsumeNextItems_GetUnconsumedError(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	repo.getItemErr = errors.New("db error")

	_, err := svc.ConsumeNextItems(ctx, "list-1", "automation-1", 1)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestConsumeNextItems_InactiveItemsSkipped(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	seedItem(repo, "item-1", "list-1", "https://mercadolivre.com.br/p/1", 0, false) // inactive
	seedItem(repo, "item-2", "list-1", "https://mercadolivre.com.br/p/2", 1, true)  // active

	consumed, err := svc.ConsumeNextItems(ctx, "list-1", "automation-1", 2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(consumed) != 1 {
		t.Errorf("expected 1 consumed (inactive skipped), got %d", len(consumed))
	}
}

func TestConsumeNextItems_AlreadyConsumedItemsSkipped(t *testing.T) {
	t.Parallel()
	svc, repo := setupService()
	ctx := context.Background()

	seedList(repo, "list-1", "user-1", "Test")
	// Item already consumed
	consumedAt := time.Now().Format(time.RFC3339)
	consumedItem := &models.MlLinkListItem{
		ID:         "item-1",
		ListID:     "list-1",
		OriginURL:  "https://mercadolivre.com.br/p/1",
		OrderIndex: 0,
		IsActive:   true,
		ConsumedAt: &consumedAt,
	}
	repo.items["item-1"] = consumedItem
	// Active unconsumed item
	seedItem(repo, "item-2", "list-1", "https://mercadolivre.com.br/p/2", 1, true)
	repo.itemsByList["list-1"] = []models.MlLinkListItem{*consumedItem, {
		ID:         "item-2",
		ListID:     "list-1",
		OriginURL:  "https://mercadolivre.com.br/p/2",
		OrderIndex: 1,
		IsActive:   true,
	}}

	consumed, err := svc.ConsumeNextItems(ctx, "list-1", "automation-1", 2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// Only the unconsumed item-2 should be consumed
	if len(consumed) != 1 {
		t.Errorf("expected 1 consumed, got %d", len(consumed))
	}
}

// ============================================================================
// validateMetadataJSON tests
// ============================================================================

func TestValidateMetadataJSON_EmptyString(t *testing.T) {
	t.Parallel()
	err := validateMetadataJSON("")
	if err != nil {
		t.Fatalf("expected no error for empty string, got %v", err)
	}
}

func TestValidateMetadataJSON_ValidJSON(t *testing.T) {
	t.Parallel()
	err := validateMetadataJSON(`{"key": "value", "nested": {"a": 1}}`)
	if err != nil {
		t.Fatalf("expected no error for valid JSON, got %v", err)
	}
}

func TestValidateMetadataJSON_InvalidJSON(t *testing.T) {
	t.Parallel()
	err := validateMetadataJSON("not valid json {{{")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	var mlErr *apperrors.MLLinksError
	if !errors.As(err, &mlErr) {
		t.Errorf("expected MLLinksError, got %T", err)
	}
}

func TestValidateMetadataJSON_ArrayNotObject(t *testing.T) {
	t.Parallel()
	err := validateMetadataJSON(`[1, 2, 3]`)
	if err == nil {
		t.Fatal("expected error for JSON array (not object)")
	}
}

func TestValidateMetadataJSON_PrimitiveValue(t *testing.T) {
	t.Parallel()
	err := validateMetadataJSON(`"just a string"`)
	if err == nil {
		t.Fatal("expected error for primitive JSON value")
	}
}
