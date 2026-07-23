package handlers

import (
	"io"
	"net/http"
	"time"

	"automatiza-go/internal/services/instagram"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// signedRequestMaxBody limita o corpo dos callbacks da Meta. Sem teto, um POST
// gigante num endpoint público e não autenticado é DoS de graça.
const signedRequestMaxBody = 64 << 10

// InstagramPublicHandler serves the unauthenticated Instagram endpoints: the
// OAuth callback (no session — validated by the signed state), the ephemeral
// story-media fetch (unguessable id, so no auth), e os callbacks obrigatórios
// da Meta de desautorização e exclusão de dados (autenticados por signed_request).
type InstagramPublicHandler struct {
	svc        *instagram.Service
	mediaStore *instagram.MediaStore
	frontend   string // frontend base URL for post-OAuth redirects
	logger     *zap.Logger
}

// NewInstagramPublicHandler builds the handler from resolved deps.
//
// frontendBaseURL vem da configuração e serve APENAS aos redirects de OAuth.
// Nenhuma base de URL é lida de header ou query: aceitar isso seria redirect
// aberto no meio de um fluxo de credencial.
func NewInstagramPublicHandler(svc *instagram.Service, mediaStore *instagram.MediaStore, frontendBaseURL string, logger *zap.Logger) *InstagramPublicHandler {
	return &InstagramPublicHandler{svc: svc, mediaStore: mediaStore, frontend: frontendBaseURL, logger: logger}
}

// OAuthCallback handles GET /api/v1/instagram/oauth/callback — Instagram
// redirects here with ?code & ?state. It always ends in a redirect back to the
// frontend /instagram page carrying a connected/error flag.
//
// Note que NÃO há ramo próprio para ?error=: mesmo a negação de consentimento
// passa por HandleCallback, para que o state seja validado e CONSUMIDO. Antes,
// o ramo de erro respondia sem olhar o state — um callback público e repetível.
func (h *InstagramPublicHandler) OAuthCallback(c *gin.Context) {
	redirect := func(path string) {
		c.Header("Cache-Control", "no-store")
		c.Redirect(http.StatusFound, h.frontend+path)
	}
	if h.svc == nil {
		redirect("/instagram?error=disabled")
		return
	}
	path, err := h.svc.HandleCallback(c.Request.Context(),
		c.Query("code"), c.Query("state"), c.Query("error"))
	if err != nil {
		// Só a existência da falha; a mensagem pode carregar code/state.
		h.logger.Warn("instagram oauth callback failed")
	}
	redirect(path)
}

// ServeStoryMedia handles GET /api/v1/public/story-media/:id — returns the
// ephemeral JPEG so the Instagram Graph API can fetch it from a public URL.
//
// Content-Type é FIXO em image/jpeg (o store só aceita JPEG); devolver um mime
// vindo do armazenamento deixaria o conteúdo escolher como é interpretado.
func (h *InstagramPublicHandler) ServeStoryMedia(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	if h.mediaStore == nil {
		c.Status(http.StatusNotFound)
		return
	}
	// O id é validado dentro do store antes de qualquer ida ao Redis.
	data, err := h.mediaStore.Get(c.Request.Context(), c.Param("id"))
	if err != nil || len(data) == 0 {
		// Resposta uniforme: id inválido, ausente e expirado são o mesmo 404,
		// e o id nunca é refletido em corpo ou log.
		c.Status(http.StatusNotFound)
		return
	}
	c.Data(http.StatusOK, "image/jpeg", data)
}

// Deauthorize handles POST /api/v1/instagram/deauthorize — a Meta chama quando
// o usuário remove o app. Apaga a credencial daquela conta.
//
// Funciona mesmo com a publicação desligada: é contrato de plataforma, não
// feature do produto.
func (h *InstagramPublicHandler) Deauthorize(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if h.svc == nil || !h.svc.DeauthorizeConfigured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "unavailable"})
		return
	}
	signed, ok := h.readSignedRequest(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := h.svc.Deauthorize(c.Request.Context(), signed, time.Now()); err != nil {
		h.logger.Warn("instagram deauthorize rejected")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	// Repetição / conta desconhecida também respondem ok: a Meta só quer saber
	// que a revogação foi aceita, e diferenciar revelaria quem existe aqui.
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DataDeletion handles POST /api/v1/instagram/data-deletion — apaga credencial
// e ledger do sujeito numa transação e devolve o comprovante exigido pela Meta.
func (h *InstagramPublicHandler) DataDeletion(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if h.svc == nil || !h.svc.DataDeletionConfigured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "unavailable"})
		return
	}
	signed, ok := h.readSignedRequest(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	resp, err := h.svc.RequestDataDeletion(c.Request.Context(), signed, time.Now())
	if err != nil {
		h.logger.Warn("instagram data deletion rejected")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// DataDeletionStatus handles GET /api/v1/instagram/data-deletion/status/:code —
// a página que a Meta (e o usuário) abre para conferir a exclusão. HTML
// estático, sem PII e sem nada vindo do banco além do "existe/não existe".
func (h *InstagramPublicHandler) DataDeletionStatus(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	if h.svc == nil {
		c.Status(http.StatusNotFound)
		return
	}
	found, err := h.svc.DataDeletionStatus(c.Request.Context(), c.Param("code"), time.Now())
	if err != nil || !found {
		c.Status(http.StatusNotFound)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8",
		[]byte(`<!doctype html><html lang="pt-BR"><head><meta charset="utf-8">`+
			`<title>Exclusao de dados</title></head><body>`+
			`<h1>Solicitacao concluida</h1>`+
			`<p>Os dados associados a esta solicitacao foram excluidos.</p>`+
			`</body></html>`))
}

// readSignedRequest lê e valida a FORMA do corpo (não a assinatura, que é do
// service). Corpo limitado e exatamente um campo signed_request — repetir o
// campo é uma tentativa clássica de fazer parsers diferentes lerem valores
// diferentes.
func (h *InstagramPublicHandler) readSignedRequest(c *gin.Context) (string, bool) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, signedRequestMaxBody)
	if err := c.Request.ParseForm(); err != nil {
		return "", false
	}
	values := c.Request.PostForm["signed_request"]
	if len(values) != 1 || values[0] == "" {
		return "", false
	}
	return values[0], true
}

// webhookMaxBody limita o corpo do webhook. A Meta manda lotes pequenos; um
// corpo gigante num endpoint público e não autenticado é DoS de graça.
const webhookMaxBody = 1 << 20 // 1 MiB

// WebhookVerify handles GET /api/v1/instagram/webhook/:token — o handshake que
// a Meta faz ao salvar a URL no painel do cliente.
//
// Responde o hub.challenge em TEXTO PURO: a Meta compara o corpo byte a byte,
// e devolver JSON faz a verificação falhar sem explicação no painel dela.
func (h *InstagramPublicHandler) WebhookVerify(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if h.svc == nil {
		c.Status(http.StatusNotFound)
		return
	}
	challenge, err := h.svc.VerifyWebhookChallenge(
		c.Request.Context(),
		c.Param("token"),
		c.Query("hub.mode"),
		c.Query("hub.verify_token"),
		c.Query("hub.challenge"),
	)
	if err != nil {
		// 403 é o que a Meta espera quando o verify_token não bate; token de
		// cliente inexistente responde igual, para não revelar quais existem.
		c.Status(http.StatusForbidden)
		return
	}
	c.String(http.StatusOK, challenge)
}

// WebhookReceive handles POST /api/v1/instagram/webhook/:token.
//
// Lê o corpo CRU antes de qualquer decodificação: a assinatura da Meta é
// calculada sobre os bytes exatos, e reserializar o JSON quebraria a conferência.
func (h *InstagramPublicHandler) WebhookReceive(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if h.svc == nil {
		c.Status(http.StatusNotFound)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, webhookMaxBody))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	if _, err := h.svc.HandleWebhook(
		c.Request.Context(),
		c.Param("token"),
		body,
		c.GetHeader("X-Hub-Signature-256"),
	); err != nil {
		// Assinatura inválida ou token desconhecido: mesma resposta, sem
		// detalhe — o endpoint é público e não deve virar oráculo.
		h.logger.Warn("instagram webhook rejected")
		c.Status(http.StatusForbidden)
		return
	}
	// 200 rápido: a Meta trata não-200 como falha e reenvia o lote.
	c.Status(http.StatusOK)
}
