/**
 * Returns the user-set friendly name for a chip, or null when it is the
 * raw session id placeholder.
 *
 * Background: o backend antigo (pré-2026-05-14) defaultava `friendly_name`
 * ao próprio `instance_id` (`session_01krk9...`) na criação da instância.
 * Como toda UI prioriza friendly_name, isso fazia o chip aparecer com o
 * id ilegível em vez do telefone. Este helper trata esse caso como
 * "sem nome", deixando o caller cair pro telefone (ou outro fallback).
 *
 * O patch do backend já não preenche mais friendly_name; este helper é a
 * rede de segurança para instâncias antigas no banco e para qualquer
 * outro caminho de criação que ainda o defaulte.
 */
export function effectiveFriendlyName(inst: {
  friendly_name?: string | null;
  instance_id: string;
}): string | null {
  const trimmed = inst.friendly_name?.trim();
  if (!trimmed) return null;
  if (trimmed === inst.instance_id) return null;
  return trimmed;
}
