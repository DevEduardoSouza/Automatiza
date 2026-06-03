package reconciliation

import (
	"context"
	"errors"
	"testing"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"

	"go.uber.org/zap"
)

type fakeSessionLister struct {
	sessions []whatsapp.SessionInfo
	err      error
}

func (f *fakeSessionLister) ListAllSessions(_ context.Context) ([]whatsapp.SessionInfo, error) {
	return f.sessions, f.err
}

type fakeInstanceStore struct {
	byName  map[string]*models.Instance
	updates map[int]string // instanceID -> novo status
}

func newFakeStore() *fakeInstanceStore {
	return &fakeInstanceStore{byName: map[string]*models.Instance{}, updates: map[int]string{}}
}

func (f *fakeInstanceStore) GetInstanceByName(_ context.Context, name string) (*models.Instance, error) {
	if inst, ok := f.byName[name]; ok {
		return inst, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeInstanceStore) UpdateInstanceStatus(_ context.Context, id int, status string) error {
	f.updates[id] = status
	if inst := findByID(f.byName, id); inst != nil {
		inst.Status = status
	}
	return nil
}

func findByID(m map[string]*models.Instance, id int) *models.Instance {
	for _, inst := range m {
		if inst.ID == id {
			return inst
		}
	}
	return nil
}

func conn(name string) whatsapp.SessionInfo {
	return whatsapp.SessionInfo{Name: name, Status: whatsapp.SessionStatusConnected}
}

func TestReconcile_PromovePendingTravadoParaConnected(t *testing.T) {
	store := newFakeStore()
	store.byName["session_abc"] = &models.Instance{ID: 2311, InstanceID: "session_abc", Status: models.InstanceStatusPending}
	store.byName["session_def"] = &models.Instance{ID: 99, InstanceID: "session_def", Status: models.InstanceStatusDisconnected}

	lister := &fakeSessionLister{sessions: []whatsapp.SessionInfo{conn("session_abc"), conn("session_def")}}
	r := NewInstanceStatusReconciler(store, lister, zap.NewNop())

	stats, err := r.Reconcile(context.Background())
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if stats.Fixed != 2 {
		t.Errorf("Fixed = %d, want 2", stats.Fixed)
	}
	if store.updates[2311] != models.InstanceStatusConnected || store.updates[99] != models.InstanceStatusConnected {
		t.Errorf("instâncias não foram promovidas: %+v", store.updates)
	}
}

func TestReconcile_NaoMexeQuandoJaConnected(t *testing.T) {
	store := newFakeStore()
	store.byName["session_ok"] = &models.Instance{ID: 1, InstanceID: "session_ok", Status: models.InstanceStatusConnected}

	lister := &fakeSessionLister{sessions: []whatsapp.SessionInfo{conn("session_ok")}}
	r := NewInstanceStatusReconciler(store, lister, zap.NewNop())

	stats, _ := r.Reconcile(context.Background())
	if stats.Fixed != 0 {
		t.Errorf("Fixed = %d, want 0 (já estava connected)", stats.Fixed)
	}
	if len(store.updates) != 0 {
		t.Errorf("não deveria atualizar nada: %+v", store.updates)
	}
}

func TestReconcile_IgnoraSessaoNaoConnectedEOrfa(t *testing.T) {
	store := newFakeStore()
	// instância existe mas no banco está connected; a sessão WAHA está FAILED → não rebaixa
	store.byName["session_failed"] = &models.Instance{ID: 5, InstanceID: "session_failed", Status: models.InstanceStatusConnected}

	lister := &fakeSessionLister{sessions: []whatsapp.SessionInfo{
		{Name: "session_failed", Status: whatsapp.SessionStatusFailed}, // não-connected → ignora (conservador)
		{Name: "session_orfa", Status: whatsapp.SessionStatusConnected}, // sem instância no banco → ignora
	}}
	r := NewInstanceStatusReconciler(store, lister, zap.NewNop())

	stats, _ := r.Reconcile(context.Background())
	if stats.Fixed != 0 {
		t.Errorf("Fixed = %d, want 0", stats.Fixed)
	}
	if len(store.updates) != 0 {
		t.Errorf("não deveria rebaixar nem tocar órfã: %+v", store.updates)
	}
}

func TestReconcile_ErroAoListarSessoes(t *testing.T) {
	r := NewInstanceStatusReconciler(newFakeStore(), &fakeSessionLister{err: errors.New("waha down")}, zap.NewNop())
	if _, err := r.Reconcile(context.Background()); err == nil {
		t.Error("esperava erro quando ListAllSessions falha")
	}
}
