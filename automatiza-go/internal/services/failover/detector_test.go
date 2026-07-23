package failover

import (
	"context"
	"errors"
	"testing"
	"time"

	"automatiza-go/internal/adapters/whatsapp"
	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"go.uber.org/zap"
)

// fakeDetectorRepo grava as observações que o detector registrou.
type fakeDetectorRepo struct {
	repository.InstanceFailoverRepository

	policies []models.InstanceFailoverPolicy
	probes   []repository.FailoverProbeInput
	expired  int
}

func (f *fakeDetectorRepo) ListAssistedPolicies(context.Context) ([]models.InstanceFailoverPolicy, error) {
	return f.policies, nil
}

func (f *fakeDetectorRepo) ExpireProposals(context.Context, time.Time) (int, error) {
	f.expired++
	return 0, nil
}

func (f *fakeDetectorRepo) RecordProbe(_ context.Context, in repository.FailoverProbeInput) (*models.InstanceFailoverPolicy, *models.InstanceFailoverEvent, error) {
	f.probes = append(f.probes, in)
	return nil, nil, nil
}

type fakeInstanceRepo struct {
	repository.InstanceRepository
	instance *models.Instance
	err      error
}

func (f *fakeInstanceRepo) GetInstanceByID(context.Context, int) (*models.Instance, error) {
	return f.instance, f.err
}

func detectorPolicy() models.InstanceFailoverPolicy {
	return models.InstanceFailoverPolicy{
		ID:                        7,
		UserID:                    "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		PrimaryInstanceID:         10,
		BackupInstanceID:          20,
		Mode:                      models.FailoverModeAssisted,
		State:                     models.FailoverStateActive,
		UnhealthyAfterSeconds:     900,
		RequiredConsecutiveProbes: 3,
	}
}

func connectedInstance() *models.Instance {
	return &models.Instance{
		ID: 10, UserID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		InstanceID: "s1", Status: models.InstanceStatusConnected,
	}
}

func runDetector(t *testing.T, inst *models.Instance, instErr error, p *fakeProvider) *fakeDetectorRepo {
	t.Helper()
	repo := &fakeDetectorRepo{policies: []models.InstanceFailoverPolicy{detectorPolicy()}}
	d := NewDetector(repo, &fakeInstanceRepo{instance: inst, err: instErr}, p, func() bool { return true }, zap.NewNop())
	if err := d.Run(context.Background()); err != nil {
		t.Fatalf("detector run: %v", err)
	}
	if len(repo.probes) != 1 {
		t.Fatalf("expected exactly 1 probe, got %d", len(repo.probes))
	}
	return repo
}

// Feature desligada: o detector não faz NADA.
func TestDetector_DisabledIsNoop(t *testing.T) {
	repo := &fakeDetectorRepo{policies: []models.InstanceFailoverPolicy{detectorPolicy()}}
	d := NewDetector(repo, &fakeInstanceRepo{instance: connectedInstance()}, &fakeProvider{}, func() bool { return false }, zap.NewNop())
	if err := d.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.probes) != 0 || repo.expired != 0 {
		t.Fatal("disabled detector must not probe or expire anything")
	}
}

// WORKING + banco concordando = saudável, e zera a acumulação.
func TestDetector_HealthyProbe(t *testing.T) {
	repo := runDetector(t, connectedInstance(), nil, &fakeProvider{status: workingStatus()})
	probe := repo.probes[0]
	if !probe.Healthy || !probe.Valid {
		t.Fatalf("expected healthy+valid probe, got %+v", probe)
	}
}

// 13.6.1 — FAILED isolado é uma observação VÁLIDA de indisponibilidade (o que
// acumula), mas sozinho nunca propõe: quem propõe é o limiar de 15 minutos +
// contagem, aplicado no repositório.
func TestDetector_KnownBadStatusAccumulates(t *testing.T) {
	inst := connectedInstance()
	inst.Status = models.InstanceStatusDisconnected

	repo := runDetector(t, inst, nil, &fakeProvider{
		status: &whatsapp.FreshSessionStatus{RawStatus: whatsapp.WahaStatusFailed, Known: true},
	})
	probe := repo.probes[0]
	if probe.Healthy || !probe.Valid {
		t.Fatalf("known non-WORKING with agreeing DB must be a valid unhealthy probe, got %+v", probe)
	}
}

// 13.6.1 — erro/timeout do provider NÃO é evidência. Zera.
func TestDetector_ProviderErrorIsNotEvidence(t *testing.T) {
	repo := runDetector(t, connectedInstance(), nil, &fakeProvider{statusErr: errors.New("timeout")})
	probe := repo.probes[0]
	if probe.Valid {
		t.Fatalf("provider error must be an invalid probe, got %+v", probe)
	}
	if probe.Status != "provider_error" {
		t.Fatalf("status label: %s", probe.Status)
	}
}

// Status FORA do enum documentado é indeterminado, não evidência.
//
// É por isso que o detector lê o status CRU: parseWahaStatus colapsaria
// PASSKEY_REQUIRED em DISCONNECTED e o tornaria indistinguível de um FAILED.
func TestDetector_UnknownStatusIsNotEvidence(t *testing.T) {
	inst := connectedInstance()
	inst.Status = models.InstanceStatusDisconnected

	repo := runDetector(t, inst, nil, &fakeProvider{
		status: &whatsapp.FreshSessionStatus{RawStatus: "SOMETHING_NEW", Known: false},
	})
	if repo.probes[0].Valid {
		t.Fatal("unknown provider status must never count as evidence")
	}
}

// Divergência banco × WAHA: um dos dois está velho. Não conta.
func TestDetector_DBProviderDisagreementIsNotEvidence(t *testing.T) {
	repo := runDetector(t, connectedInstance(), nil, &fakeProvider{
		status: &whatsapp.FreshSessionStatus{RawStatus: whatsapp.WahaStatusFailed, Known: true},
	})
	if repo.probes[0].Valid {
		t.Fatal("DB/WAHA disagreement must not count as evidence")
	}
}

// Instância sem linha local / sem sessão: indeterminado.
func TestDetector_MissingInstanceIsNotEvidence(t *testing.T) {
	repo := runDetector(t, nil, nil, &fakeProvider{})
	if repo.probes[0].Valid {
		t.Fatal("missing instance row must be an invalid probe")
	}
}

// Instância de OUTRO usuário nunca é sondada como se fosse da política.
func TestDetector_ForeignInstanceIsNotEvidence(t *testing.T) {
	inst := connectedInstance()
	inst.UserID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	repo := runDetector(t, inst, nil, &fakeProvider{})
	if repo.probes[0].Valid {
		t.Fatal("instance owned by another user must be an invalid probe")
	}
}

// 13.6.4 — o try-lock por política é pedido em TODA observação: é ele que faz
// duas réplicas produzirem um único evento.
func TestDetector_AlwaysRequestsPerPolicyLock(t *testing.T) {
	for _, tc := range []struct {
		name string
		inst *models.Instance
		prov *fakeProvider
	}{
		{"saudável", connectedInstance(), &fakeProvider{status: workingStatus()}},
		{"erro de provider", connectedInstance(), &fakeProvider{statusErr: errors.New("x")}},
		{"instância ausente", nil, &fakeProvider{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := runDetector(t, tc.inst, nil, tc.prov)
			if !repo.probes[0].RequireDetectorLock {
				t.Fatal("every probe must take the per-policy advisory lock")
			}
		})
	}
}

// 13.6.7/13.6.8 — o detector NUNCA lê dispatch_logs nem chama dry-run/confirm.
// A prova é estrutural: o fake do repositório só implementa três métodos, e o
// embed da interface faria qualquer outro estourar.
func TestDetector_TouchesOnlyProbeSurface(t *testing.T) {
	repo := &fakeDetectorRepo{policies: []models.InstanceFailoverPolicy{detectorPolicy()}}
	d := NewDetector(repo, &fakeInstanceRepo{instance: connectedInstance()},
		&fakeProvider{status: workingStatus()}, func() bool { return true }, zap.NewNop())

	if err := d.Run(context.Background()); err != nil {
		t.Fatalf("detector must not call anything beyond list/expire/probe: %v", err)
	}
	if repo.expired != 1 {
		t.Fatalf("expected one expiry sweep per tick, got %d", repo.expired)
	}
}

// Uma política com problema não pode parar as outras.
func TestDetector_OnePolicyFailureDoesNotStopTheRest(t *testing.T) {
	repo := &fakeDetectorRepo{policies: []models.InstanceFailoverPolicy{detectorPolicy(), detectorPolicy()}}
	d := NewDetector(repo, &fakeInstanceRepo{err: errors.New("db down")},
		&fakeProvider{}, func() bool { return true }, zap.NewNop())

	if err := d.Run(context.Background()); err != nil {
		t.Fatalf("per-policy failures must be swallowed: %v", err)
	}
}
