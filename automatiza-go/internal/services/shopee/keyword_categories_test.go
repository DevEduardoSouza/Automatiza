package shopee

import (
	"reflect"
	"testing"
)

func TestCategoryLeafKeyword(t *testing.T) {
	cases := map[string]string{
		"Confeitaria e Doces - Ingredientes (Chocolates)": "Chocolates",
		"Casamento - Vestuário da Noiva (Véu)":            "Véu",
		"Confeitaria e Doces - Chocolates":                "Chocolates",          // sem parênteses
		"Confeitaria e Doces":                             "Confeitaria e Doces", // nicho-pai
		"Eletrônicos - Celulares (Smartphones)":           "Smartphones",
		"":                                                "",
	}
	for in, want := range cases {
		if got := categoryLeafKeyword(in); got != want {
			t.Errorf("categoryLeafKeyword(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestKeywordCategories(t *testing.T) {
	s := &shopeeService{categoryMap: initCategoryMapping()}

	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"sentinela ofertas gerais", []string{"Ofertas gerais"}, nil},
		{"sentinela sex shop", []string{"Sex Shop"}, nil},
		{"nicho com catId", []string{"Eletrônicos"}, nil},
		{"subnicho existente com catId", []string{"Eletrônicos - Celulares (Smartphones)"}, nil},
		{"subnicho novo sem catId", []string{"Confeitaria e Doces - Ingredientes (Chocolates)"}, []string{"Chocolates"}},
		{
			"misto: catId + keyword + sentinela",
			[]string{"Eletrônicos", "Confeitaria e Doces - Ingredientes (Chocolates)", "Ofertas gerais"},
			[]string{"Chocolates"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := s.keywordCategories(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("keywordCategories(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
