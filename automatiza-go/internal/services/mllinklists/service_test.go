package mllinklists

import (
	"context"
	"errors"
	"testing"
	"time"

	apperrors "automatiza-go/internal/errors"
	"automatiza-go/internal/models"

	"go.uber.org/zap"
)

// mockRepository is a mock implementation of MLLinkListRepository for testing.
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
	if m.getItemErr != nil {
		return nil, "", 0, m.getItemErr
	}
	items := m.itemsByList[listID]
	total := len(items)
	if pageSize > len(items) {
		pageSize = len(items)
	}
	return items[:pageSize], "", total, nil
}

func (m *mockRepository) GetItemByID(ctx context.Context, itemID string) (*models.MlLinkListItem, error) {
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
		item.ID = "item-" + time.Now().Format("20060102150405.000000000")
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

// Test setup helper
func setupService() (Service, *mockRepository) {
	repo := newMockRepository()
	logger := zap.NewNop()
	svc := NewService(repo, logger)
	return svc, repo
}

// ============================================================================
// List tests
// ============================================================================

func TestCreateList_Success(t *testing.T) {
	svc, _ := setupService()
	ctx := context.Background()

	input := CreateListInput{
		Name: "Test List",
	}

	list, err := svc.CreateList(ctx, "user-123", input)
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
	svc, _ := setupService()
	ctx := context.Background()

	input := CreateListInput{
		Name: "",
	}

	_, err := svc.CreateList(ctx, "user-123", input)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestGetList_NotFound(t *testing.T) {
	svc, _ := setupService()
	ctx := context.Background()

	_, err := svc.GetList(ctx, "nonexistent", "user-123")
	if err == nil {
		t.Fatal("expected error for nonexistent list")
	}

	if !errors.Is(err, apperrors.ErrListNotFound) {
		t.Errorf("expected ErrListNotFound, got %v", err)
	}
}

func TestDeleteList_Success(t *testing.T) {
	svc, repo := setupService()
	ctx := context.Background()

	// Create a list first
	list := &models.MlLinkList{
		ID:       "list-123",
		UserID:   "user-123",
		Name:     "Test List",
		IsActive: true,
	}
	repo.lists["list-123"] = list

	err := svc.DeleteList(ctx, "list-123", "user-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify list was deleted
	if _, ok := repo.lists["list-123"]; ok {
		t.Error("expected list to be deleted")
	}
}

// ============================================================================
// Item tests
// ============================================================================

func TestCreateItem_Success(t *testing.T) {
	svc, repo := setupService()
	ctx := context.Background()

	// Create a list first
	list := &models.MlLinkList{
		ID:       "list-123",
		UserID:   "user-123",
		Name:     "Test List",
		IsActive: true,
	}
	repo.lists["list-123"] = list

	input := CreateItemInput{
		ListID:    "list-123",
		OriginURL: "https://www.mercadolivre.com.br/p/MLB12345",
	}

	item, err := svc.CreateItem(ctx, "user-123", input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if item.ListID != "list-123" {
		t.Errorf("expected list_id 'list-123', got %s", item.ListID)
	}

	if !item.IsActive {
		t.Error("expected item to be active by default")
	}
}

func TestCreateItem_InvalidURL(t *testing.T) {
	svc, repo := setupService()
	ctx := context.Background()

	// Create a list first
	list := &models.MlLinkList{
		ID:       "list-123",
		UserID:   "user-123",
		Name:     "Test List",
		IsActive: true,
	}
	repo.lists["list-123"] = list

	input := CreateItemInput{
		ListID:    "list-123",
		OriginURL: "https://example.com/not-mercadolivre",
	}

	_, err := svc.CreateItem(ctx, "user-123", input)
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestCreateItem_ListNotFound(t *testing.T) {
	svc, _ := setupService()
	ctx := context.Background()

	input := CreateItemInput{
		ListID:    "nonexistent",
		OriginURL: "https://www.mercadolivre.com.br/p/MLB12345",
	}

	_, err := svc.CreateItem(ctx, "user-123", input)
	if err == nil {
		t.Fatal("expected error for nonexistent list")
	}

	if !errors.Is(err, apperrors.ErrListNotFound) {
		t.Errorf("expected ErrListNotFound, got %v", err)
	}
}

// ============================================================================
// Bulk create tests
// ============================================================================

func TestBulkCreateItems_Success(t *testing.T) {
	svc, repo := setupService()
	ctx := context.Background()

	// Create a list first
	list := &models.MlLinkList{
		ID:       "list-123",
		UserID:   "user-123",
		Name:     "Test List",
		IsActive: true,
	}
	repo.lists["list-123"] = list

	input := BulkCreateItemsInput{
		ListID: "list-123",
		OriginURLs: []string{
			"https://www.mercadolivre.com.br/p/MLB12345",
			"https://www.mercadolivre.com.br/p/MLB67890",
		},
	}

	result, err := svc.BulkCreateItems(ctx, "user-123", input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.CreatedCount != 2 {
		t.Errorf("expected 2 items created, got %d", result.CreatedCount)
	}
}

func TestBulkCreateItems_WithInvalidURLs(t *testing.T) {
	svc, repo := setupService()
	ctx := context.Background()

	// Create a list first
	list := &models.MlLinkList{
		ID:       "list-123",
		UserID:   "user-123",
		Name:     "Test List",
		IsActive: true,
	}
	repo.lists["list-123"] = list

	input := BulkCreateItemsInput{
		ListID: "list-123",
		OriginURLs: []string{
			"https://www.mercadolivre.com.br/p/MLB12345",
			"https://example.com/invalid",
			"not-a-url",
		},
	}

	result, err := svc.BulkCreateItems(ctx, "user-123", input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.CreatedCount != 1 {
		t.Errorf("expected 1 item created, got %d", result.CreatedCount)
	}

	if result.SkippedCount != 2 {
		t.Errorf("expected 2 items skipped, got %d", result.SkippedCount)
	}
}

// ============================================================================
// Consume tests
// ============================================================================

func TestConsumeNextItems_Success(t *testing.T) {
	svc, repo := setupService()
	ctx := context.Background()

	// Create a list with items
	list := &models.MlLinkList{
		ID:       "list-123",
		UserID:   "user-123",
		Name:     "Test List",
		IsActive: true,
	}
	repo.lists["list-123"] = list

	item := models.MlLinkListItem{
		ID:         "item-1",
		ListID:     "list-123",
		OriginURL:  "https://www.mercadolivre.com.br/p/MLB12345",
		OrderIndex: 0,
		IsActive:   true,
	}
	repo.items["item-1"] = &item
	repo.itemsByList["list-123"] = []models.MlLinkListItem{item}

	consumed, err := svc.ConsumeNextItems(ctx, "list-123", "automation-1", 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(consumed) != 1 {
		t.Errorf("expected 1 item consumed, got %d", len(consumed))
	}

	if consumed[0].ConsumedAt == nil {
		t.Error("expected item to have consumed_at set")
	}
}

func TestConsumeNextItems_NoItemsAvailable(t *testing.T) {
	svc, repo := setupService()
	ctx := context.Background()

	// Create an empty list
	list := &models.MlLinkList{
		ID:       "list-123",
		UserID:   "user-123",
		Name:     "Test List",
		IsActive: true,
	}
	repo.lists["list-123"] = list
	repo.itemsByList["list-123"] = []models.MlLinkListItem{}

	_, err := svc.ConsumeNextItems(ctx, "list-123", "automation-1", 1)
	if err == nil {
		t.Fatal("expected error for no items available")
	}

	if !errors.Is(err, apperrors.ErrNoItemsAvailable) {
		t.Errorf("expected ErrNoItemsAvailable, got %v", err)
	}
}

// ============================================================================
// Reset consumed tests
// ============================================================================

func TestResetConsumed_Success(t *testing.T) {
	svc, repo := setupService()
	ctx := context.Background()

	// Create a list with consumed item
	list := &models.MlLinkList{
		ID:       "list-123",
		UserID:   "user-123",
		Name:     "Test List",
		IsActive: true,
	}
	repo.lists["list-123"] = list

	consumedAt := time.Now().Format(time.RFC3339)
	item := models.MlLinkListItem{
		ID:         "item-1",
		ListID:     "list-123",
		OriginURL:  "https://www.mercadolivre.com.br/p/MLB12345",
		OrderIndex: 0,
		IsActive:   true,
		ConsumedAt: &consumedAt,
	}
	repo.items["item-1"] = &item
	repo.itemsByList["list-123"] = []models.MlLinkListItem{item}

	count, err := svc.ResetConsumed(ctx, "user-123", "list-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 item reset, got %d", count)
	}
}

func TestResetConsumed_ListNotFound(t *testing.T) {
	svc, _ := setupService()
	ctx := context.Background()

	_, err := svc.ResetConsumed(ctx, "user-123", "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent list")
	}

	if !errors.Is(err, apperrors.ErrListNotFound) {
		t.Errorf("expected ErrListNotFound, got %v", err)
	}
}
