import { describe, expect, it } from 'vitest';

import { isInstanceMissingError } from '@/types/dispatch-log';

describe('isInstanceMissingError', () => {
  describe('Portuguese (pt-BR) patterns', () => {
    it('returns true for exact "instância não encontrada"', () => {
      expect(isInstanceMissingError('instância não encontrada')).toBe(true);
    });

    it('returns true case-insensitively for Portuguese pattern', () => {
      expect(isInstanceMissingError('Instância Não Encontrada')).toBe(true);
      expect(isInstanceMissingError('INSTÂNCIA NÃO ENCONTRADA')).toBe(true);
    });

    it('returns true when Portuguese pattern is embedded in a longer message', () => {
      expect(
        isInstanceMissingError(
          'Erro ao enviar mensagem: instância não encontrada no servidor',
        ),
      ).toBe(true);
    });
  });

  describe('English patterns', () => {
    it('returns true for "instance not found"', () => {
      expect(isInstanceMissingError('instance not found')).toBe(true);
    });

    it('returns true for "instance not found" case-insensitively', () => {
      expect(isInstanceMissingError('Instance Not Found')).toBe(true);
      expect(isInstanceMissingError('INSTANCE NOT FOUND')).toBe(true);
    });

    it('returns true for "instancename"', () => {
      expect(isInstanceMissingError('instancename')).toBe(true);
      expect(isInstanceMissingError('Error: instanceName is required')).toBe(
        true,
      );
    });

    it('returns true for "not connected"', () => {
      expect(isInstanceMissingError('not connected')).toBe(true);
      expect(
        isInstanceMissingError('WhatsApp session is not connected'),
      ).toBe(true);
    });
  });

  describe('negative cases', () => {
    it('returns false for undefined', () => {
      expect(isInstanceMissingError(undefined)).toBe(false);
    });

    it('returns false for empty string', () => {
      expect(isInstanceMissingError('')).toBe(false);
    });

    it('returns false for unrelated error messages', () => {
      expect(isInstanceMissingError('timeout error')).toBe(false);
      expect(isInstanceMissingError('network request failed')).toBe(false);
      expect(isInstanceMissingError('internal server error')).toBe(false);
    });
  });

  describe('edge cases', () => {
    it('returns false when only "instância" is present without "não encontrada"', () => {
      expect(isInstanceMissingError('instância ativa')).toBe(false);
      expect(isInstanceMissingError('instância conectada com sucesso')).toBe(
        false,
      );
    });

    it('returns false when only "não encontrada" is present without "instância"', () => {
      expect(isInstanceMissingError('sessão não encontrada')).toBe(false);
    });
  });
});
