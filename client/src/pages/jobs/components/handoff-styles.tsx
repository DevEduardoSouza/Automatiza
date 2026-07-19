/**
 * Estilos de :hover do handoff (paleta vinho). Injetado uma única vez dentro do
 * modal — os hovers exatos do protótipo não são expressáveis por inline style e
 * usar valores dinâmicos em classes Tailwind quebraria o JIT. Escopo por prefixo
 * `hf-` para não vazar para o resto do app.
 */
export function HandoffStyles() {
  return (
    <style>{`
      .hf-close:hover { background:#FBF5F7; }
      .hf-btn-wine:hover { background:#FBF5F7; }
      .hf-btn-neutral:hover { background:#F7F1F4; }
      .hf-chip:hover { border-color:#C89AAD; }
      .hf-row:hover { background:#FBF5F7 !important; }
      .hf-addtime:hover { background:#FBF5F7; }
      .hf-timecell:hover { background:#FBF5F7; }
      .hf-nav:hover { background:#FBF5F7; }
      .hf-day:hover { background:#FBF5F7; }
      .hf-yeartitle:hover { background:#FBF5F7; }
      .hf-cta:not(:disabled):hover { background:#6B1C3D; }
      .hf-search:focus-within { border-color:#7D2248; }
    `}</style>
  );
}
