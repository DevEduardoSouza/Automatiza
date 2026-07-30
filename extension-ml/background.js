// Conector Mercado Livre — service worker.
//
// POR QUE ESTA EXTENSÃO EXISTE
// O cookie de sessão do Mercado Livre é HttpOnly: nenhum código de página
// consegue lê-lo (bookmarklet, script no console do painel — nada). Até aqui o
// cliente precisava abrir o DevTools, achar o cookie e colar no Automatiza. Ele
// expirava em silêncio e ninguém percebia: as credenciais de produção ficaram
// entre 111 e 144 dias mortas, gerando ZERO links.
//
// A extensão é o único caminho técnico para ler esse cookie automaticamente.
//
// DESENHO: QUEM CHAMA QUEM
// É o PAINEL do Automatiza que chama a extensão (via externally_connectable),
// nunca o contrário. A extensão devolve apenas o token e o painel segue no
// fluxo normal dele — mesma sessão, mesmo CSRF, mesmo RPC de sempre.
// Consequência: zero configuração de CORS, zero exceção de CSRF e ZERO
// mudança no backend. A extensão não tem endpoint, não guarda segredo e não
// fala com servidor nenhum além do próprio Mercado Livre.
//
// LIMITES DE PERMISSÃO (deliberados)
// - `host_permissions` cobre SÓ www.mercadolivre.com.br. A extensão é
//   tecnicamente incapaz de tocar qualquer outro site.
// - Nenhum content script: ela não lê nem altera o conteúdo de página alguma.
// - Só dois endpoints do ML são chamados, os dois do painel de afiliados.
// Isso importa para os T&C do programa (cláusula 1.9): a extensão gera LINKS,
// nunca cliques, e não interfere na navegação de ninguém.

const ML_DOMAIN = "mercadolivre.com.br";
const TAGS_ENDPOINT =
	"https://www.mercadolivre.com.br/affiliate-program/api/v2/stripe/user/tags";
const LINKBUILDER_URL = "https://www.mercadolivre.com.br/afiliados/linkbuilder";

// base64url sem padding — o painel decodifica com base64UrlDecode().
function base64UrlEncode(str) {
	const bytes = new TextEncoder().encode(str);
	let bin = "";
	for (const b of bytes) bin += String.fromCharCode(b);
	return btoa(bin).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

// Monta o header Cookie a partir de TODOS os cookies do domínio.
//
// Deliberadamente não filtra por nome: os cookies de sessão do ML já mudaram de
// nome mais de uma vez, e uma allowlist desatualizada produz o pior sintoma
// possível — token que parece válido e falha só na hora de gerar o link.
async function readMlCookieHeader() {
	const cookies = await chrome.cookies.getAll({ domain: ML_DOMAIN });
	if (!cookies || cookies.length === 0) return "";
	return cookies.map((c) => `${c.name}=${c.value}`).join("; ");
}

// Descobre a etiqueta de afiliado. Elimina o campo manual — digitar a etiqueta
// errada gerava links que creditavam comissão a ninguém, sem nenhum sintoma.
//
// Devolve [] em qualquer falha: o painel então pede a etiqueta ao usuário, que
// é o fluxo de hoje. Chutar aqui seria pior que não descobrir.
async function discoverTags() {
	try {
		const resp = await fetch(TAGS_ENDPOINT, {
			credentials: "include",
			headers: { Accept: "application/json, text/plain, */*" },
		});
		if (!resp.ok) return { tags: [], status: resp.status };
		const data = await resp.json();
		const items = Array.isArray(data) ? data : data?.tags || data?.data || [];
		const tags = [];
		for (const it of items) {
			const tag =
				typeof it === "string" ? it : it?.tag || it?.name || it?.id || "";
			if (tag && !tags.includes(tag)) tags.push(tag);
		}
		return { tags, status: resp.status };
	} catch (_e) {
		return { tags: [], status: 0 };
	}
}

// Raspa o CSRF do HTML do linkbuilder. É OPCIONAL: o backend sabe obtê-lo
// sozinho a partir do cookie. Mandar quando é barato só poupa um round-trip.
async function readCsrfToken() {
	try {
		const resp = await fetch(LINKBUILDER_URL, { credentials: "include" });
		if (!resp.ok) return "";
		const html = await resp.text();
		const patterns = [
			/<meta[^>]+name=["']csrf-token["'][^>]+content=["']([^"']+)["']/i,
			/["']csrf[_-]?token["']\s*:\s*["']([^"']+)["']/i,
			/window\.__CSRF__\s*=\s*["']([^"']+)["']/i,
		];
		for (const re of patterns) {
			const m = html.match(re);
			if (m && m[1]) return m[1];
		}
		return "";
	} catch (_e) {
		return "";
	}
}

async function buildConnectionToken() {
	const cookie = await readMlCookieHeader();
	if (!cookie) {
		return {
			ok: false,
			reason: "not_logged_in",
			message:
				"Você não está logado no Mercado Livre neste navegador. Abra mercadolivre.com.br, faça login e tente de novo.",
		};
	}

	const { tags, status } = await discoverTags();

	// 401/403 = os cookies existem mas a sessão não vale mais. Avisar AGORA é o
	// ponto inteiro desta extensão: salvar um cookie morto só adia a descoberta.
	if (status === 401 || status === 403) {
		return {
			ok: false,
			reason: "session_expired",
			message:
				"Sua sessão do Mercado Livre expirou. Faça login novamente em mercadolivre.com.br e tente de novo.",
		};
	}

	const csrfToken = await readCsrfToken();

	// Uma etiqueta só → preenche. Várias → devolve a lista e o painel PERGUNTA;
	// escolher por conta própria creditaria comissão na etiqueta errada em
	// silêncio, que é o tipo de erro que só aparece no extrato.
	const payload = {
		tag: tags.length === 1 ? tags[0] : "",
		cookie,
		csrfToken: csrfToken || undefined,
	};

	return {
		ok: true,
		token: `MLCRED_V1:${base64UrlEncode(JSON.stringify(payload))}`,
		tags,
		needsTagChoice: tags.length > 1,
	};
}

// Ponte com o painel. `return true` mantém o canal aberto para a resposta
// assíncrona — sem ele o painel recebe undefined silenciosamente.
chrome.runtime.onMessageExternal.addListener((msg, _sender, sendResponse) => {
	if (msg?.action === "ping") {
		sendResponse({ ok: true, version: chrome.runtime.getManifest().version });
		return false;
	}
	if (msg?.action === "getMLToken") {
		buildConnectionToken().then(sendResponse);
		return true;
	}
	sendResponse({ ok: false, reason: "unknown_action" });
	return false;
});

// Mesma ponte, para o POPUP da própria extensão.
//
// onMessageExternal só entrega mensagens de páginas web (o painel); mensagens
// do popup chegam por onMessage. São listeners distintos de propósito no
// Chrome, e usar só o primeiro deixaria o botão do popup sem resposta.
// Os dois chamam buildConnectionToken() para que os dois caminhos não possam
// divergir.
chrome.runtime.onMessage.addListener((msg, _sender, sendResponse) => {
	if (msg?.action === "getMLToken") {
		buildConnectionToken().then(sendResponse);
		return true;
	}
	sendResponse({ ok: false, reason: "unknown_action" });
	return false;
});
