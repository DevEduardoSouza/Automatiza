package automation

import (
	"context"
	"errors"
	"testing"

	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"go.uber.org/zap"
)

// categorySpyRuleRepo captura os argumentos que o CouponHandler entrega ao
// matcher. Só FindMatchingCoupon importa aqui; o resto do contrato existe
// para satisfazer a interface.
type categorySpyRuleRepo struct {
	called     bool
	lastCats   []int64
	returnErr  error
	returnCoup *models.Coupon
}

var _ repository.CouponRuleRepository = (*categorySpyRuleRepo)(nil)

func (m *categorySpyRuleRepo) GetUserCouponRules(context.Context, string) ([]models.CouponRule, error) {
	return nil, nil
}

func (m *categorySpyRuleRepo) GetCouponRuleByID(context.Context, string, string) (*models.CouponRule, error) {
	return nil, nil
}

func (m *categorySpyRuleRepo) CreateCouponRule(context.Context, string, *models.CouponRule) (*models.CouponRule, error) {
	return nil, nil
}

func (m *categorySpyRuleRepo) UpdateCouponRule(context.Context, string, string, map[string]interface{}) error {
	return nil
}

func (m *categorySpyRuleRepo) DeleteCouponRule(context.Context, string, string) error { return nil }

func (m *categorySpyRuleRepo) FindMatchingCoupon(
	_ context.Context,
	_, _ string,
	_ float64,
	_, _ bool,
	productCategoryIDs []int64,
) (*models.Coupon, error) {
	m.called = true
	m.lastCats = productCategoryIDs
	return m.returnCoup, m.returnErr
}

func categoryTestAutomation() *models.Automation {
	return &models.Automation{
		ID:               "auto-1",
		UserID:           "user-1",
		ApplyCouponRules: true,
	}
}

// newCategoryTestHandler constrói o handler DEPOIS do t.Setenv — o gate é
// parseado uma vez no construtor, então a ordem importa.
func newCategoryTestHandler(t *testing.T, spy *categorySpyRuleRepo) *CouponHandler {
	t.Helper()
	return NewCouponHandler(nil, spy, zap.NewNop())
}

// TestCouponHandlerCategoryMergeRefetch fixa a regra C7: no refetch de uma
// oferta da busca, lista nova substitui, lista vazia preserva a original e
// as duas nunca são combinadas.
func TestCouponHandlerCategoryMergeRefetch(t *testing.T) {
	original := []int64{100001}
	refreshed := []int64{100002, 100003}

	if got := mergeRefetchedCategoryIDs(original, refreshed); len(got) != 2 || got[0] != 100002 || got[1] != 100003 {
		t.Errorf("refetch não vazio deveria SUBSTITUIR; got %v", got)
	}
	if got := mergeRefetchedCategoryIDs(original, nil); len(got) != 1 || got[0] != 100001 {
		t.Errorf("refetch nil deveria PRESERVAR a original; got %v", got)
	}
	if got := mergeRefetchedCategoryIDs(original, []int64{}); len(got) != 1 || got[0] != 100001 {
		t.Errorf("refetch vazio deveria PRESERVAR a original; got %v", got)
	}
	if got := mergeRefetchedCategoryIDs(nil, nil); got != nil {
		t.Errorf("sem sinal em nenhum lado deveria ser nil; got %v", got)
	}
}

// TestCouponHandlerCategoryGate cobre o parse do gate de processo e o que
// chega ao matcher em cada combinação de gate × plataforma.
func TestCouponHandlerCategoryGate(t *testing.T) {
	cats := []int64{100001, 100002}

	t.Run("env ausente: matcher recebe nil", func(t *testing.T) {
		spy := &categorySpyRuleRepo{}
		h := newCategoryTestHandler(t, spy)
		_, err := h.ApplyCoupon(context.Background(), categoryTestAutomation(), 100, "shopee", true, cats)
		if err != nil {
			t.Fatalf("ApplyCoupon devolveu erro: %v", err)
		}
		if !spy.called {
			t.Fatal("matcher deveria ter sido consultado")
		}
		if spy.lastCats != nil {
			t.Errorf("categorias = %v, want nil (gate desligado por default)", spy.lastCats)
		}
	})

	t.Run("valores verdadeiros ligam o gate", func(t *testing.T) {
		for _, raw := range []string{"1", "true", "TRUE", " on ", "yes"} {
			t.Run(raw, func(t *testing.T) {
				t.Setenv("COUPON_CATEGORY_RULES_ENABLED", raw)
				spy := &categorySpyRuleRepo{}
				h := newCategoryTestHandler(t, spy)
				if _, err := h.ApplyCoupon(context.Background(), categoryTestAutomation(), 100, "shopee", true, cats); err != nil {
					t.Fatalf("ApplyCoupon devolveu erro: %v", err)
				}
				if len(spy.lastCats) != 2 {
					t.Errorf("categorias = %v, want %v (gate deveria estar ligado)", spy.lastCats, cats)
				}
			})
		}
	})

	t.Run("valor inválido mantém o gate desligado", func(t *testing.T) {
		t.Setenv("COUPON_CATEGORY_RULES_ENABLED", "talvez")
		spy := &categorySpyRuleRepo{}
		h := newCategoryTestHandler(t, spy)
		if _, err := h.ApplyCoupon(context.Background(), categoryTestAutomation(), 100, "shopee", true, cats); err != nil {
			t.Fatalf("ApplyCoupon devolveu erro: %v", err)
		}
		if spy.lastCats != nil {
			t.Errorf("categorias = %v, want nil", spy.lastCats)
		}
	})

	t.Run("gate ligado + Shopee: matcher recebe os IDs", func(t *testing.T) {
		t.Setenv("COUPON_CATEGORY_RULES_ENABLED", "true")
		spy := &categorySpyRuleRepo{}
		h := newCategoryTestHandler(t, spy)
		if _, err := h.ApplyCoupon(context.Background(), categoryTestAutomation(), 100, "shopee", true, cats); err != nil {
			t.Fatalf("ApplyCoupon devolveu erro: %v", err)
		}
		if len(spy.lastCats) != 2 || spy.lastCats[0] != 100001 || spy.lastCats[1] != 100002 {
			t.Errorf("categorias = %v, want [100001 100002]", spy.lastCats)
		}
	})

	t.Run("gate ligado + Mercado Livre: matcher recebe nil", func(t *testing.T) {
		t.Setenv("COUPON_CATEGORY_RULES_ENABLED", "true")
		for _, store := range []string{"ml", "mercado_livre", "mercadolivre"} {
			t.Run(store, func(t *testing.T) {
				spy := &categorySpyRuleRepo{}
				h := newCategoryTestHandler(t, spy)
				if _, err := h.ApplyCoupon(context.Background(), categoryTestAutomation(), 100, store, false, cats); err != nil {
					t.Fatalf("ApplyCoupon devolveu erro: %v", err)
				}
				if spy.lastCats != nil {
					t.Errorf("categorias = %v, want nil (categoria é sinal só da Shopee)", spy.lastCats)
				}
			})
		}
	})

	t.Run("erro do matcher mantém o fail-open", func(t *testing.T) {
		t.Setenv("COUPON_CATEGORY_RULES_ENABLED", "true")
		spy := &categorySpyRuleRepo{returnErr: errors.New("boom")}
		h := newCategoryTestHandler(t, spy)
		res, err := h.ApplyCoupon(context.Background(), categoryTestAutomation(), 100, "shopee", true, cats)
		if err != nil {
			t.Fatalf("erro do matcher NÃO pode vazar para o caminho de envio: %v", err)
		}
		if res == nil || res.Applied {
			t.Fatalf("resultado = %+v, want Applied=false", res)
		}
		if res.SkipReason != "coupon rule lookup failed" {
			t.Errorf("SkipReason = %q", res.SkipReason)
		}
	})

	t.Run("o slice do caller não é mutado", func(t *testing.T) {
		t.Setenv("COUPON_CATEGORY_RULES_ENABLED", "true")
		input := []int64{100002, 100001, 100002}
		snapshot := append([]int64(nil), input...)
		spy := &categorySpyRuleRepo{}
		h := newCategoryTestHandler(t, spy)
		if _, err := h.ApplyCoupon(context.Background(), categoryTestAutomation(), 100, "shopee", true, input); err != nil {
			t.Fatalf("ApplyCoupon devolveu erro: %v", err)
		}
		for i := range snapshot {
			if input[i] != snapshot[i] {
				t.Fatalf("input mutado: %v, want %v", input, snapshot)
			}
		}
	})
}
