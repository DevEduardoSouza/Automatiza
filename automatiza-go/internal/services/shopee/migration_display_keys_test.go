package shopee

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Termos guarda-chuva: descrevem uma gaveta, não um produto. Como o texto entre
// parênteses do display_key é a keyword ENVIADA à Shopee (e, medido em 30/07/2026,
// o único filtro que sobra — ver o comentário de keywordCategories), um termo desses
// devolve qualquer coisa. Comprovado: keyword "Acessórios" trouxe lupa de celular e
// carregador portátil mesmo com o productCatId de Animais Domésticos.
var genericLeafTerms = map[string]bool{
	"acessórios": true, "acessorios": true, "kit": true, "kits": true,
	"outros": true, "geral": true, "gerais": true, "diversos": true,
	"peças": true, "pecas": true, "produtos": true, "itens": true,
	"básicos": true, "basicos": true, "variados": true, "complementos": true,
}

// Seeds anteriores à medição de 30/07/2026, quando a regra do termo autossuficiente
// ainda não existia: 405 nichos de Confeitaria/Casamento/Bebê, dos quais 155 têm
// termo de uma palavra só. Ficam de fora do gate porque JÁ estão no banco de
// produção; a curadoria deles é tarefa própria, não pode travar migração nova.
var preFase0Seeds = map[string]bool{
	"082_confeitaria_casamento_bebe_niches.up.sql": true,
}

var seedLineRe = regexp.MustCompile(`(?m)^SELECT c\.id, (.+)$`)
var quotedRe = regexp.MustCompile(`'((?:[^']|'')*)'`)
var leafRe = regexp.MustCompile(`\(([^)]*)\)\s*$`)

// TestSeedKeywordsAreSelfSufficient trava a armadilha de autoria do buscador: um
// display_key SEM productCatId próprio vira busca por palavra-chave, e a palavra é
// o texto entre parênteses. Se esse texto não descreve o produto sozinho, o cliente
// recebe oferta fora do nicho — silenciosamente, sem erro em log nenhum.
func TestSeedKeywordsAreSelfSufficient(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "scripts", "db", "migrations")
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("nenhuma migração encontrada em %s (err=%v)", dir, err)
	}

	categoryMap := initCategoryMapping()
	checked := 0

	for _, file := range files {
		base := filepath.Base(file)
		if preFase0Seeds[base] {
			continue
		}
		body, readErr := os.ReadFile(file)
		if readErr != nil {
			t.Fatalf("lendo %s: %v", file, readErr)
		}
		if !strings.Contains(string(body), "INSERT INTO subcategories") {
			continue
		}

		for _, m := range seedLineRe.FindAllStringSubmatch(string(body), -1) {
			quoted := quotedRe.FindAllStringSubmatch(m[1], -1)
			// SELECT c.id, 'slug', 'name', 'display_key', 'group_name', N, true
			if len(quoted) < 4 {
				continue
			}
			displayKey := strings.ReplaceAll(quoted[2][1], "''", "'")
			groupName := strings.ReplaceAll(quoted[3][1], "''", "'")

			// Tem productCatId próprio: filtra pela categoria da Shopee e nunca
			// chega no caminho de keyword — o texto do parêntese é só rótulo.
			if ids, ok := categoryMap[displayKey]; ok && len(ids) > 0 {
				continue
			}
			checked++

			leaf := leafRe.FindStringSubmatch(displayKey)
			if leaf == nil {
				t.Errorf("%s: display_key %q não tem catId próprio e não termina em "+
					"'(termo de busca)' — a keyword sairia do texto errado", base, displayKey)
				continue
			}
			term := strings.TrimSpace(leaf[1])

			if genericLeafTerms[strings.ToLower(term)] {
				t.Errorf("%s: display_key %q usa o termo guarda-chuva %q como keyword de "+
					"busca — troque por algo que descreva o produto sozinho "+
					"(ex.: 'Ração para cachorro', não 'Ração')", base, displayKey, term)
			}
			if strings.EqualFold(term, groupName) {
				t.Errorf("%s: display_key %q repete o nome do grupo %q como keyword — "+
					"o termo precisa nomear o produto, não a gaveta", base, displayKey, groupName)
			}
		}
	}

	if checked == 0 {
		t.Fatal("nenhum display_key keyword-driven encontrado — o parser quebrou")
	}
	t.Logf("%d display_keys keyword-driven verificados", checked)
}
