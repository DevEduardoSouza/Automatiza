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

  it('should substitute all 7 standard VARIABLE_MAP placeholders', () => {
    const template = Object.values(VARIABLE_MAP).join(' | ');

    const vars: Record<string, string> = {
      nome_produto: 'Fone BT',
      preco: '49.90',
      preco_formatado: 'R$ 49,90',
      preco_original: '99.90',
      preco_original_formatado: 'R$ 99,90',
      link_afiliado: 'https://shope.ee/abc',
      aviso_preco: 'Preco pode variar',
    };

    const result = renderTemplate(template, vars);

    expect(result).toBe(
      'Fone BT | 49.90 | R$ 49,90 | 99.90 | R$ 99,90 | https://shope.ee/abc | Preco pode variar'
    );
  });
});
