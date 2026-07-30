// Popup: mostra o estado da sessão e entrega o código de conexão.
//
// O cookie NUNCA é exibido nem logado. Ele sai da máquina só dentro do código
// que o próprio usuário copia e cola no Automatiza — um ato explícito dele.
//
// A montagem do token vive no service worker (background.js), não aqui: é o
// mesmo caminho usado pelo painel via externally_connectable, então os dois
// fluxos não podem divergir.

function render(state, text) {
	const el = document.getElementById("status");
	el.className = `status ${state}`;
	document.getElementById("text").textContent = text;
}

let tokenCache = "";

async function refresh() {
	const resp = await chrome.runtime.sendMessage({ action: "getMLToken" });

	if (!resp?.ok) {
		// Mensagens do background já são escritas para o usuário final: dizem o que
		// fazer ("faça login"), não o que falhou tecnicamente.
		render("bad", resp?.message || "Não foi possível ler sua sessão.");
		return;
	}

	tokenCache = resp.token;
	document.getElementById("copy").disabled = false;

	if (resp.needsTagChoice) {
		// Várias etiquetas: o Automatiza precisa perguntar qual usar. Escolher aqui
		// creditaria comissão na etiqueta errada em silêncio.
		render(
			"ok",
			`Conectado — ${resp.tags.length} etiquetas. Escolha qual usar no Automatiza.`,
		);
		return;
	}

	render(
		"ok",
		resp.tags?.length === 1
			? `Conectado — etiqueta ${resp.tags[0]}.`
			: "Conectado ao Mercado Livre.",
	);
}

document.getElementById("copy").addEventListener("click", async () => {
	if (!tokenCache) return;
	const btn = document.getElementById("copy");
	try {
		await navigator.clipboard.writeText(tokenCache);
		btn.textContent = "Código copiado!";
	} catch (_e) {
		btn.textContent = "Não deu para copiar";
	}
	setTimeout(() => {
		btn.textContent = "Copiar código de conexão";
	}, 2000);
});

refresh();
