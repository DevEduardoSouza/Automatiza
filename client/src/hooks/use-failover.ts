import {
  ConfirmFailoverRequestSchema,
  DisablePolicyRequestSchema,
  type DryRunReport,
  FailoverDirection,
  type FailoverEvent,
  FailoverMode,
  type FailoverPolicy,
  FailoverService,
  FailoverState,
  ListEventsRequestSchema,
  ListPoliciesRequestSchema,
  type OperationReport,
  RevertFailoverRequestSchema,
  RunDryRunRequestSchema,
  SetPolicyRequestSchema,
} from '@/gen/failover/v1/failover_pb';
import { create } from '@bufbuild/protobuf';
import { createClient } from '@connectrpc/connect';
import { useTransport } from '@connectrpc/connect-query';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useState } from 'react';

// ============================================================
// Contrato com o backend
// ============================================================

/**
 * Reason code estável de "feature desligada".
 *
 * É o ÚNICO sinal que esconde a seção inteira. Erro de rede, 500 ou qualquer
 * outra falha PRECISA aparecer para o usuário: tratar tudo como "desligado"
 * transformaria um backend fora do ar num recurso que "sumiu do painel", que é
 * o tipo de bug que ninguém reporta e ninguém acha.
 */
export const FAILOVER_DISABLED_REASON = 'INSTANCE_FAILOVER_DISABLED';

/** isFailoverDisabled reconhece o reason code no erro do Connect. */
export function isFailoverDisabled(error: unknown): boolean {
  if (!error) return false;
  const err = error as { message?: string; metadata?: Headers };
  if (err.metadata?.get?.('failover-reason') === FAILOVER_DISABLED_REASON) return true;
  return typeof err.message === 'string' && err.message.includes(FAILOVER_DISABLED_REASON);
}

/** Chave de idempotência: obrigatória em toda mutação. */
export function newIdempotencyKey(): string {
  // 36 bytes ASCII imprimíveis — dentro do contrato de 16..128 do backend.
  return crypto.randomUUID();
}

export interface SetFailoverPolicyParams {
  policyId?: bigint;
  primaryInstanceId: number;
  backupInstanceId: number;
  mode: FailoverMode;
  expectedVersion: bigint;
}

export interface ConfirmParams {
  policyId: bigint;
  expectedVersion: bigint;
  dryRunId: string;
  dryRunRef: string;
}

interface UseFailoverReturn {
  policies: FailoverPolicy[];
  loading: boolean;
  /** true SÓ quando o backend devolveu o reason code de feature desligada. */
  disabled: boolean;
  /** Erro real (rede/servidor) — nunca confundido com "desligado". */
  error: Error | null;
  refetch: () => Promise<void>;

  setPolicy: (params: SetFailoverPolicyParams) => Promise<FailoverPolicy | undefined>;
  disablePolicy: (policyId: bigint, expectedVersion: bigint) => Promise<void>;
  runDryRun: (policyId: bigint, direction: FailoverDirection) => Promise<DryRunReport | undefined>;
  confirmFailover: (params: ConfirmParams) => Promise<OperationReport | undefined>;
  revertFailover: (params: ConfirmParams) => Promise<OperationReport | undefined>;
  listEvents: (policyId: bigint, limit?: number) => Promise<FailoverEvent[]>;

  isMutating: boolean;
}

/**
 * useFailover — política de chip reserva.
 *
 * Polling adaptativo: 2s enquanto uma operação está em curso (SWITCHING /
 * REVERTING), 30s nos estados estáveis. Uma troca leva segundos e o usuário
 * está olhando; fora dela, bater no servidor a cada 2s é desperdício.
 */
export function useFailover(): UseFailoverReturn {
  const transport = useTransport();
  const queryClient = useQueryClient();
  const [isMutating, setIsMutating] = useState(false);

  const client = createClient(FailoverService, transport);

  const query = useQuery({
    queryKey: ['failover', 'policies'],
    queryFn: async () => {
      const res = await client.listPolicies(create(ListPoliciesRequestSchema, {}));
      return res.policies;
    },
    retry: false,
    refetchInterval: (q) => {
      const policies = (q.state.data ?? []) as FailoverPolicy[];
      const busy = policies.some(
        (p) => p.state === FailoverState.SWITCHING || p.state === FailoverState.REVERTING
      );
      return busy ? 2000 : 30000;
    },
  });

  const disabled = isFailoverDisabled(query.error);

  const invalidate = useCallback(async () => {
    await queryClient.invalidateQueries({ queryKey: ['failover'] });
  }, [queryClient]);

  const setPolicyMutation = useMutation({
    mutationFn: async (params: SetFailoverPolicyParams) => {
      const res = await client.setPolicy(
        create(SetPolicyRequestSchema, {
          policyId: params.policyId ?? 0n,
          primaryInstanceId: BigInt(params.primaryInstanceId),
          backupInstanceId: BigInt(params.backupInstanceId),
          mode: params.mode,
          expectedVersion: params.expectedVersion,
          idempotencyKey: newIdempotencyKey(),
        })
      );
      return res.policy;
    },
    onSuccess: invalidate,
  });

  const disablePolicyMutation = useMutation({
    mutationFn: async ({
      policyId,
      expectedVersion,
    }: { policyId: bigint; expectedVersion: bigint }) => {
      await client.disablePolicy(
        create(DisablePolicyRequestSchema, {
          policyId,
          expectedVersion,
          idempotencyKey: newIdempotencyKey(),
        })
      );
    },
    onSuccess: invalidate,
  });

  const dryRunMutation = useMutation({
    mutationFn: async ({
      policyId,
      direction,
    }: { policyId: bigint; direction: FailoverDirection }) => {
      const res = await client.runDryRun(
        create(RunDryRunRequestSchema, {
          policyId,
          direction,
          idempotencyKey: newIdempotencyKey(),
        })
      );
      return res.report;
    },
  });

  const confirmMutation = useMutation({
    mutationFn: async (params: ConfirmParams) => {
      const res = await client.confirmFailover(
        create(ConfirmFailoverRequestSchema, {
          policyId: params.policyId,
          expectedVersion: params.expectedVersion,
          dryRunId: params.dryRunId,
          dryRunRef: params.dryRunRef,
          idempotencyKey: newIdempotencyKey(),
        })
      );
      return res.operation;
    },
    onSuccess: invalidate,
  });

  const revertMutation = useMutation({
    mutationFn: async (params: ConfirmParams) => {
      const res = await client.revertFailover(
        create(RevertFailoverRequestSchema, {
          policyId: params.policyId,
          expectedVersion: params.expectedVersion,
          dryRunId: params.dryRunId,
          dryRunRef: params.dryRunRef,
          idempotencyKey: newIdempotencyKey(),
        })
      );
      return res.operation;
    },
    onSuccess: invalidate,
  });

  const withMutating = useCallback(async <T>(fn: () => Promise<T>): Promise<T> => {
    setIsMutating(true);
    try {
      return await fn();
    } finally {
      setIsMutating(false);
    }
  }, []);

  return {
    policies: query.data ?? [],
    loading: query.isLoading,
    disabled,
    // Erro de verdade só quando NÃO é o reason code de desligado.
    error: disabled ? null : ((query.error as Error) ?? null),
    refetch: async () => {
      await query.refetch();
    },

    setPolicy: (params) => withMutating(() => setPolicyMutation.mutateAsync(params)),
    disablePolicy: (policyId, expectedVersion) =>
      withMutating(() => disablePolicyMutation.mutateAsync({ policyId, expectedVersion })),
    runDryRun: (policyId, direction) =>
      withMutating(() => dryRunMutation.mutateAsync({ policyId, direction })),
    confirmFailover: (params) => withMutating(() => confirmMutation.mutateAsync(params)),
    revertFailover: (params) => withMutating(() => revertMutation.mutateAsync(params)),

    listEvents: async (policyId, limit = 20) => {
      const res = await client.listEvents(create(ListEventsRequestSchema, { policyId, limit }));
      return res.events;
    },

    isMutating,
  };
}
