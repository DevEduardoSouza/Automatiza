package automation

import (
	"context"
	"errors"
	"testing"

	"automatiza-go/internal/lib/commerceurl"
	"automatiza-go/internal/models"
	"automatiza-go/internal/services/affiliatelink"
)

type failingConverter struct{ err error }

func (f failingConverter) Convert(_ context.Context, _ commerceurl.URL, _ string) (string, error) {
	return "", f.err
}

type okConverter struct{ out string }

func (o okConverter) Convert(_ context.Context, _ commerceurl.URL, _ string) (string, error) {
	return o.out, nil
}

// O modo LIST é passthrough por projeto: o usuário cola o PRÓPRIO link, já
// afiliado. Uma conversão indisponível não pode impedir o envio — a versão
// anterior deste código pulava o item E o consumia (sem retry), o que pararia
// listas que hoje funcionam.
func TestListRegistryEhFailOpen(t *testing.T) {
	const pasted = "https://www.amazon.com.br/dp/B09B8V1LZ3?tag=do-cliente-20"

	t.Run("sem credencial, o link colado segue", func(t *testing.T) {
		reg := affiliatelink.New(map[string]affiliatelink.Converter{
			models.MonitorStoreAmazon: failingConverter{err: affiliatelink.ErrNoCredentials},
		})
		converted, ok, err := reg.Convert(context.Background(), pasted, "user-1")

		// O contrato que o process_list consome: erro presente, sem conversão.
		if ok || converted != "" {
			t.Fatalf("(%q, %v) — não deveria converter", converted, ok)
		}
		if !errors.Is(err, affiliatelink.ErrNoCredentials) {
			t.Fatalf("erro = %v", err)
		}
		// process_list mantém affiliateURL = originURL nesse caso; o teste de
		// integração vive no comportamento do ramo (ver process_list.go).
	})

	t.Run("com credencial, o link é reatribuído", func(t *testing.T) {
		reg := affiliatelink.New(map[string]affiliatelink.Converter{
			models.MonitorStoreAmazon: okConverter{out: "https://www.amazon.com.br/dp/B09B8V1LZ3?tag=dono-20"},
		})
		converted, ok, err := reg.Convert(context.Background(), pasted, "user-1")
		if err != nil || !ok {
			t.Fatalf("(%q, %v, %v)", converted, ok, err)
		}
		if converted == pasted {
			t.Error("a tag do link colado deveria ter sido substituída")
		}
	})
}
