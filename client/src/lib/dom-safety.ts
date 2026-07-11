// Tradutores de página (Google Translate embrulha text nodes em <font>) e
// extensões de navegador mexem no DOM por fora do React. Quando o React
// re-renderiza, removeChild/insertBefore recebem um nó que já não é filho do
// pai esperado e o DOM lança NotFoundError ("Failed to execute 'removeChild'
// on 'Node'"), derrubando a árvore inteira — visto em produção no
// /offer-generate, cuja tabela de logs re-renderiza texto com frequência.
// Workaround canônico de facebook/react#11538: degrada essas duas chamadas
// para no-op APENAS no caso exato que lançaria, preservando todo o resto.
export function installDomSafetyNet(): void {
  if (typeof Node !== 'function' || !Node.prototype) return;

  const originalRemoveChild = Node.prototype.removeChild;
  Node.prototype.removeChild = function <T extends Node>(this: Node, child: T): T {
    if (child.parentNode !== this) {
      console.warn(
        'removeChild ignorado: o nó não é filho do pai esperado (DOM alterado por tradutor/extensão)',
        child
      );
      return child;
    }
    return originalRemoveChild.call(this, child) as T;
  };

  const originalInsertBefore = Node.prototype.insertBefore;
  Node.prototype.insertBefore = function <T extends Node>(
    this: Node,
    newNode: T,
    referenceNode: Node | null
  ): T {
    if (referenceNode && referenceNode.parentNode !== this) {
      console.warn(
        'insertBefore ignorado: a referência não é filho do pai esperado (DOM alterado por tradutor/extensão)',
        referenceNode
      );
      return newNode;
    }
    return originalInsertBefore.call(this, newNode, referenceNode) as T;
  };
}
