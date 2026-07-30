// Status do popup: diz ao usuário, em uma linha, se a conexão está pronta.
//
// Não expõe nem loga o cookie: o popup só olha se EXISTE sessão e se o painel
// de afiliados responde. O valor do cookie nunca sai da máquina a não ser
// quando o próprio usuário clica em conectar no Automatiza.

const ML_DOMAIN = 'mercadolivre.com.br';
const TAGS_ENDPOINT = 'https://www.mercadolivre.com.br/affiliate-program/api/v2/stripe/user/tags';

function render(state, text) {
  const el = document.getElementById('status');
  el.className = `status ${state}`;
  document.getElementById('text').textContent = text;
}

async function check() {
  const cookies = await chrome.cookies.getAll({ domain: ML_DOMAIN });
  if (!cookies || cookies.length === 0) {
    render('bad', 'Você não está logado no Mercado Livre.');
    return;
  }

  try {
    const resp = await fetch(TAGS_ENDPOINT, {
      credentials: 'include',
      headers: { Accept: 'application/json' },
    });

    if (resp.status === 401 || resp.status === 403) {
      render('bad', 'Sessão expirada — faça login de novo.');
      return;
    }
    if (!resp.ok) {
      // Não acusar o usuário por instabilidade do ML: "indeterminado" é a
      // informação honesta aqui.
      render('', 'Não deu para verificar agora. Tente mais tarde.');
      return;
    }

    const data = await resp.json();
    const items = Array.isArray(data) ? data : data?.tags || data?.data || [];
    const n = items.length;
    render(
      'ok',
      n === 1
        ? 'Conectado ao Mercado Livre.'
        : n > 1
          ? `Conectado — ${n} etiquetas encontradas.`
          : 'Conectado ao Mercado Livre.'
    );
  } catch (_e) {
    render('', 'Não deu para verificar agora. Tente mais tarde.');
  }
}

check();
