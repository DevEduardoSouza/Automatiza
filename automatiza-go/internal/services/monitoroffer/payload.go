package monitoroffer

import (
	"encoding/json"
	"errors"
	"fmt"

	"automatiza-go/internal/lib/crypto"
	"automatiza-go/internal/models"
)

// O payload existe para que um retry consiga refazer classificação e render
// sem ter guardado a mensagem em claro no banco.
//
// O que NÃO entra é tão importante quanto o que entra (§7): sender, push name,
// telefone, MediaURL, base64 da mídia, título/descrição de preview e token
// ficam de fora. O corpo cifrado conserva info/cta/desc porque o renderer do
// retry precisa delas — elas só somem quando o corpo público é construído.

// ErrPayloadTooLarge sinaliza estouro do teto ANTES de cifrar.
var ErrPayloadTooLarge = errors.New(models.ReasonPayloadTooLarge)

// EncodePayload serializa e cifra o payload mínimo.
//
// Estourar o teto NÃO trunca. Um payload truncado reclassificaria links
// diferentes dos que a mensagem realmente tinha — o retry emitiria uma oferta
// de outro produto. Melhor bloquear o que dependeria de retry e deixar sair só
// o que já está seguro.
func EncodePayload(p models.MonitorOfferPayload, keyBase64 string) (string, error) {
	p.Version = models.MonitorOfferPayloadVersion

	plain, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("monitor_offer: marshal do payload: %w", err)
	}
	if len(plain) > models.MaxPayloadPlaintextBytes {
		return "", ErrPayloadTooLarge
	}

	blob, err := crypto.EncryptJSONV1(p, keyBase64)
	if err != nil {
		// Erro de cifra é blocked/payload_encrypt_failed: nunca cair para
		// "grava em claro" nem para "manda sem retry silenciosamente".
		return "", fmt.Errorf("%s: %w", models.ReasonPayloadEncryptFail, err)
	}
	out, err := json.Marshal(blob)
	if err != nil {
		return "", fmt.Errorf("%s: %w", models.ReasonPayloadEncryptFail, err)
	}
	return string(out), nil
}

// DecodePayload decifra e valida a versão.
//
// Versão desconhecida é recusada em vez de interpretada: um formato futuro
// lido com o parser antigo produziria URLs erradas, e URL errada aqui vira
// mensagem errada no grupo do cliente.
func DecodePayload(ciphertext, keyBase64 string) (models.MonitorOfferPayload, error) {
	var out models.MonitorOfferPayload
	if ciphertext == "" {
		return out, errors.New(models.ReasonPayloadExpired)
	}

	var blob crypto.EncryptedBlobV1
	if err := json.Unmarshal([]byte(ciphertext), &blob); err != nil {
		return out, fmt.Errorf("%s: %w", models.ReasonPayloadDecryptFail, err)
	}
	if err := crypto.DecryptJSONV1(&blob, keyBase64, &out); err != nil {
		return out, fmt.Errorf("%s: %w", models.ReasonPayloadDecryptFail, err)
	}
	if out.Version != models.MonitorOfferPayloadVersion {
		return out, fmt.Errorf("%s: versão %d desconhecida", models.ReasonPayloadDecryptFail, out.Version)
	}
	return out, nil
}
