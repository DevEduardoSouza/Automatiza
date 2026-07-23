package failover

import (
	"sort"

	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"
)

// EligibleItem é uma automação que a operação vai mexer.
type EligibleItem struct {
	AutomationID string
	Name         string
	Status       string
	Before       *models.RoutingSnapshot
	After        *models.RoutingSnapshot
	Reasons      []string
}

// EligibleSet é o conjunto completo de uma operação, mais o que bloqueia a
// conta inteira.
//
// Não existe "aplicar as boas e pular as ruins": qualquer bloqueio derruba a
// operação inteira antes da primeira alteração. Trocar metade das automações
// deixa a conta num estado que ninguém projetou e que a reversão não sabe
// desfazer.
type EligibleSet struct {
	Items []EligibleItem
	// BlockingReasons são códigos de nível de CONTA (ex.: automação com
	// roteamento implícito, configuração inválida).
	BlockingReasons []string
}

// Blocked reporta se o conjunto impede a operação.
func (s *EligibleSet) Blocked() bool { return len(s.BlockingReasons) > 0 }

// AutomationIDs devolve os IDs em ordem ascendente (a mesma ordem de lock).
func (s *EligibleSet) AutomationIDs() []string {
	out := make([]string, 0, len(s.Items))
	for _, it := range s.Items {
		out = append(out, it.AutomationID)
	}
	sort.Strings(out)
	return out
}

// BuildSwapSet monta o conjunto elegível de uma troca `from` → `to`.
//
// Varre TODAS as automações do usuário, em qualquer status: uma automação
// pausada hoje volta amanhã, e se ficar fora do snapshot volta apontando para o
// chip trocado.
//
// Duas classes de bloqueio de conta:
//
//   - roteamento IMPLÍCITO em qualquer automação (escalar nulo e lista vazia).
//     Essa automação cai no "primeiro chip conectado" do dispatcher, que pode
//     ser justamente a primária — ela continuaria servindo os grupos depois da
//     troca, fora de qualquer snapshot e sem aparecer no relatório.
//   - configuração inválida numa automação elegível. O failover não conserta
//     configuração por baixo; ele para e mostra.
func BuildSwapSet(rows []repository.FailoverAutomationRouting, from, to int) *EligibleSet {
	set := &EligibleSet{}
	var blocking []string

	for i := range rows {
		row := &rows[i]
		snap, reasons := models.NewRoutingSnapshot(&row.Automation)

		implicit := false
		for _, r := range reasons {
			if r == models.ReasonUnmanagedImplicit {
				implicit = true
			}
		}
		if implicit {
			blocking = append(blocking, models.ReasonUnmanagedImplicit)
			// Uma automação implícita não tem o que projetar — ela já está
			// bloqueando a conta. Segue para a próxima.
			continue
		}
		if !snap.UsesInstance(from) {
			continue
		}
		if len(reasons) > 0 {
			blocking = append(blocking, reasons...)
		}

		set.Items = append(set.Items, EligibleItem{
			AutomationID: row.Automation.ID,
			Name:         row.Name,
			Status:       row.Status,
			Before:       snap,
			After:        snap.ProjectSwap(from, to),
			Reasons:      reasons,
		})
	}

	sort.Slice(set.Items, func(i, j int) bool {
		return set.Items[i].AutomationID < set.Items[j].AutomationID
	})
	set.BlockingReasons = dedupeSorted(blocking)
	return set
}

// BuildRevertSet monta o conjunto de uma REVERSÃO a partir dos itens gravados
// no failover concluído.
//
// Não faz substituição textual inversa: o alvo é exatamente o snapshot_before
// da operação original. Substituir "reserva por primária" de volta produziria
// um resultado diferente sempre que a automação já tivesse a primária em outra
// posição da lista, ou tivesse mudado de forma no meio.
//
// Regras de divergência:
//   - automação apagada depois do switch → deleted_noop (nunca recriada);
//   - automação nova → não entra (ninguém prometeu nada sobre ela);
//   - automação existente com roteamento diferente do fingerprint_after do
//     switch → bloqueia a reversão INTEIRA. Não existe force.
func BuildRevertSet(
	items []models.InstanceFailoverOperationItem,
	current map[string]*repository.FailoverAutomationRouting,
) (*EligibleSet, error) {
	set := &EligibleSet{}
	var blocking []string

	for i := range items {
		it := &items[i]

		row, exists := current[it.AutomationIDSnapshot]
		if !exists || row == nil {
			// Some do conjunto: nada a reverter, e recriar seria inventar
			// configuração que o usuário apagou de propósito.
			continue
		}

		liveSnap, reasons := models.NewRoutingSnapshot(&row.Automation)
		if len(reasons) > 0 {
			blocking = append(blocking, reasons...)
			continue
		}
		if liveSnap.Fingerprint() != it.FingerprintAfter {
			blocking = append(blocking, models.ReasonRoutingDiverged)
			continue
		}

		before, err := models.UnmarshalRoutingSnapshot(it.SnapshotBefore)
		if err != nil {
			return nil, err
		}

		set.Items = append(set.Items, EligibleItem{
			AutomationID: it.AutomationIDSnapshot,
			Name:         row.Name,
			Status:       row.Status,
			Before:       liveSnap,
			After:        before,
		})
	}

	sort.Slice(set.Items, func(i, j int) bool {
		return set.Items[i].AutomationID < set.Items[j].AutomationID
	})
	set.BlockingReasons = dedupeSorted(blocking)
	return set, nil
}

// ToOperationItems converte o conjunto nos itens persistidos da operação.
func (s *EligibleSet) ToOperationItems(operationID string) ([]models.InstanceFailoverOperationItem, error) {
	out := make([]models.InstanceFailoverOperationItem, 0, len(s.Items))
	for i := range s.Items {
		it := &s.Items[i]
		before, err := it.Before.MarshalSnapshot()
		if err != nil {
			return nil, err
		}
		after, err := it.After.MarshalSnapshot()
		if err != nil {
			return nil, err
		}
		id := it.AutomationID
		out = append(out, models.InstanceFailoverOperationItem{
			OperationID:          operationID,
			AutomationID:         &id,
			AutomationIDSnapshot: id,
			FingerprintBefore:    it.Before.Fingerprint(),
			FingerprintAfter:     it.After.Fingerprint(),
			SnapshotBefore:       before,
			SnapshotAfter:        after,
			Result:               models.FailoverItemPending,
		})
	}
	return out, nil
}

func dedupeSorted(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
