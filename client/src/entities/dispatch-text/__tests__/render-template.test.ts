import { describe, expect, it } from 'vitest';
import { VARIABLE_MAP, renderTemplate } from '../model/types';

describe('renderTemplate', () => {
  // ── Basic substitution ──────────────────────────────────────

  it('should replace a single variable', () => {
    const result = renderTemplate('Hello {name}', { name: 'World' });
    expect(result).toBe('Hello World');
  });

  it('should replace multiple different variables', () => {
    const result = renderTemplate('{greeting} {name}!', {
      greeting: 'Hello',
      name: 'World',
    });
    expect(result).toBe('Hello World!');
  });

  it('should replace the same variable appearing multiple times', () => {
    const result = renderTemplate('{x} and {x} again', { x: 'foo' });
    expect(result).toBe('foo and foo again');
  });

  // ── Missing variables ───────────────────────────────────────

  it('should replace missing variables with empty string', () => {
    const result = renderTemplate('Hello {missing}!', {});
    expect(result).toBe('Hello !');
  });

  it('should replace some variables and remove missing ones', () => {
    const result = renderTemplate('{a} {b} {c}', { a: 'A', c: 'C' });
    expect(result).toBe('A  C');
  });

  // ── Edge cases ──────────────────────────────────────────────

  it('should return empty string for null template', () => {
    const result = renderTemplate(null as unknown as string, { x: 'y' });
    expect(result).toBe('');
  });

  it('should return empty string for undefined template', () => {
    const result = renderTemplate(undefined as unknown as string, { x: 'y' });
    expect(result).toBe('');
  });

  it('should return empty string for empty template', () => {
    const result = renderTemplate('', { x: 'y' });
    expect(result).toBe('');
  });

  it('should handle whitespace inside braces', () => {
    const result = renderTemplate('{ nome_produto }', { nome_produto: 'Fone' });
    expect(result).toBe('Fone');
  });

  it('should return template as-is when no placeholders exist', () => {
    const result = renderTemplate('No variables here', { x: 'y' });
    expect(result).toBe('No variables here');
  });

  it('should keep empty braces {} unchanged', () => {
    const result = renderTemplate('Empty {} braces', { '': 'nope' });
    expect(result).toBe('Empty {} braces');
  });

  // ── Integration with VARIABLE_MAP ───────────────────────────

  // Deriva os valores do próprio VARIABLE_MAP: antes a lista era escrita à
  // mão e ficava para trás a cada variável nova (cta, cupom_bloco e info já
  // estavam de fora), deixando o teste vermelho sem apontar bug nenhum.
  it('should substitute every VARIABLE_MAP placeholder', () => {
    const template = Object.values(VARIABLE_MAP).join(' | ');

    const vars = Object.fromEntries(
      Object.values(VARIABLE_MAP).map((token) => [
        token.slice(1, -1),
        `valor:${token.slice(1, -1)}`,
      ])
    );

    const result = renderTemplate(template, vars);

    expect(result).toBe(
      Object.values(VARIABLE_MAP)
        .map((token) => `valor:${token.slice(1, -1)}`)
        .join(' | ')
    );
    expect(result).not.toContain('{');
  });

  // ── Melhoria 10: desconto ───────────────────────────────────

  it('should map the discount chip to the formatted placeholder', () => {
    expect(VARIABLE_MAP.discount).toBe('{desconto_formatado}');
  });

  // Paridade com o preview do backend: as duas variáveis de desconto são
  // substituídas pelo mesmo mecanismo das demais.
  it('should render both discount variables', () => {
    const result = renderTemplate('{desconto} | {desconto_formatado}', {
      desconto: '25',
      desconto_formatado: '25% OFF',
    });
    expect(result).toBe('25 | 25% OFF');
  });
});
