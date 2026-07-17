import { diagnose } from "@/components/support/diagnostics";
import { DiagnosticsPanel } from "@/components/support/diagnostics-panel";
import { DispatchStatusBadge } from "@/components/support/dispatch-status-badge";
import { GroupChatDialog } from "@/components/support/group-chat-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
	GetUserActivityRequestSchema,
	GetUserOverviewRequestSchema,
	type SupportAutomationActivity,
} from "@/gen/support/v1/support_pb";
import {
	getUserActivity,
	getUserOverview,
} from "@/gen/support/v1/support-SupportService_connectquery";
import { create } from "@bufbuild/protobuf";
import { useQuery } from "@connectrpc/connect-query";
import { ArrowLeft, MessageSquare } from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams } from "react-router";

/** De quanto em quanto tempo a atividade e re-medida. */
const ACTIVITY_POLL_MS = 15_000;

/** Tipos de automacao que o produto chama de monitoramento e espelhamento. */
const TYPE_LABEL: Record<string, string> = {
	search: "Busca",
	list: "Lista",
	monitoring: "Monitoramento",
	offer_generate: "Espelhamento",
};

export function UserDetail() {
	const { t } = useTranslation();
	const navigate = useNavigate();
	const { userId = "" } = useParams();
	const [chat, setChat] = useState<{
		groupId: string;
		name?: string;
		chip?: string;
	} | null>(null);

	const {
		data: overview,
		isLoading,
		error,
	} = useQuery(
		getUserOverview,
		create(GetUserOverviewRequestSchema, { userId }),
		{
			enabled: !!userId,
		},
	);

	// Atividade e a unica coisa que muda sozinha, entao so ela faz polling.
	const { data: activity, dataUpdatedAt } = useQuery(
		getUserActivity,
		create(GetUserActivityRequestSchema, { userId, windowMinutes: 5 }),
		{ enabled: !!userId, refetchInterval: ACTIVITY_POLL_MS, staleTime: 0 },
	);

	const activityByAutomation = new Map<string, SupportAutomationActivity>(
		(activity?.automations ?? []).map((a) => [a.automationId, a]),
	);

	// Diagnóstico recomputa quando o overview OU a atividade mudam (a atividade
	// faz polling, então backlog que escoa e erro que some saem do painel).
	const diagnoses = useMemo(
		() => diagnose(overview, activity),
		[overview, activity],
	);

	if (isLoading) {
		return (
			<div className="mx-auto w-full max-w-5xl space-y-4">
				<Skeleton className="h-9 w-32" />
				<Skeleton className="h-28 w-full rounded-xl" />
				<Skeleton className="h-64 w-full rounded-xl" />
			</div>
		);
	}

	if (error) {
		return (
			<div className="mx-auto w-full max-w-5xl">
				<Button
					variant="ghost"
					size="sm"
					onClick={() => navigate("/support")}
					className="mb-4"
				>
					<ArrowLeft className="mr-2 h-4 w-4" />
					{t("pages.support.detail.back")}
				</Button>
				<p className="rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive">
					{error.message}
				</p>
			</div>
		);
	}

	const profile = overview?.profile;
	const chips = overview?.chips ?? [];
	const automations = overview?.automations ?? [];
	const linkLists = overview?.linkLists ?? [];
	const groups = overview?.groups ?? [];

	const monitors = automations.filter((a) => a.type === "monitoring");
	const mirrors = automations.filter((a) => a.type === "offer_generate");
	const others = automations.filter(
		(a) => a.type !== "monitoring" && a.type !== "offer_generate",
	);

	return (
		<div className="mx-auto w-full max-w-5xl space-y-4">
			<Button variant="ghost" size="sm" onClick={() => navigate("/support")}>
				<ArrowLeft className="mr-2 h-4 w-4" />
				{t("pages.support.detail.back")}
			</Button>

			{/* Cabecalho do cliente */}
			<div className="rounded-xl border bg-background p-4 sm:p-5">
				<div className="flex flex-wrap items-start justify-between gap-3">
					<div className="min-w-0">
						<h1 className="truncate text-lg font-semibold sm:text-xl">
							{profile?.name || profile?.email}
						</h1>
						<p className="truncate text-sm text-muted-foreground">
							{profile?.email}
						</p>
						{profile?.phoneNumber && (
							<p className="text-xs text-muted-foreground">
								{profile.phoneNumber}
							</p>
						)}
					</div>
					<div className="flex flex-wrap gap-1.5">
						{profile && !profile.isActive && (
							<Badge variant="destructive">
								{t("pages.support.detail.inactive")}
							</Badge>
						)}
						{profile?.plan && <Badge variant="secondary">{profile.plan}</Badge>}
						{profile?.billingStatus && profile.billingStatus !== "active" && (
							<Badge variant="destructive">{profile.billingStatus}</Badge>
						)}
					</div>
				</div>
				{profile?.billingSuspensionReason && (
					<p className="mt-2 rounded-lg bg-destructive/5 p-2 text-xs text-destructive">
						{profile.billingSuspensionReason}
					</p>
				)}

				{/* Contexto rápido da conta: quem está atendendo entende em 1 olhada
            se o cliente usa o produto, há quanto tempo e se bateu no teto do
            plano. */}
				<div className="mt-3 flex flex-wrap gap-x-6 gap-y-1 border-t pt-3 text-xs text-muted-foreground">
					<span>
						{t("pages.support.detail.lastLogin")}:{" "}
						<span className="text-foreground">
							{relativeTime(profile?.lastLoginAt, t)}
						</span>
					</span>
					{profile?.createdAt && (
						<span>
							{t("pages.support.detail.customerSince")}:{" "}
							<span className="text-foreground">
								{formatDate(profile.createdAt)}
							</span>
						</span>
					)}
					<span>
						{t("pages.support.detail.planUsage")}:{" "}
						<span className={usageClass(chips.length, profile?.maxInstances)}>
							{chips.length}/{profile?.maxInstances ?? 0}{" "}
							{t("pages.support.detail.chipsWord")}
						</span>
						{" · "}
						<span
							className={usageClass(
								automations.length,
								profile?.maxAutomations,
							)}
						>
							{automations.length}/{profile?.maxAutomations ?? 0}{" "}
							{t("pages.support.detail.autosWord")}
						</span>
					</span>
				</div>
			</div>

			{/* Diagnóstico — a primeira leitura do atendente. */}
			<DiagnosticsPanel diagnoses={diagnoses} loading={isLoading} />

			{/* Atividade ao vivo */}
			<div className="rounded-xl border bg-background p-4 sm:p-5">
				<div className="mb-3 flex flex-wrap items-center justify-between gap-2">
					<h2 className="text-sm font-semibold">
						{t("pages.support.detail.activity")}
					</h2>
					<span className="text-xs text-muted-foreground">
						{dataUpdatedAt
							? t("pages.support.detail.updatedAt", {
									time: new Date(dataUpdatedAt).toLocaleTimeString("pt-BR"),
								})
							: ""}
					</span>
				</div>

				{/* Sem "velocidade geral": a velocidade agora é POR automação de
            busca/lista (real x configurada), mostrada em cada linha da aba
            "Buscas e listas". Aqui ficam só os totais de contexto. */}
				<div className="grid grid-cols-3 gap-3">
					<Stat
						label={t("pages.support.detail.sent")}
						value={activity ? String(activity.sentInWindow) : "—"}
						hint={t("pages.support.detail.inWindow", {
							window: activity?.windowMinutes ?? 5,
						})}
					/>
					<Stat
						label={t("pages.support.detail.failed")}
						value={activity ? String(activity.failedInWindow) : "—"}
						hint={t("pages.support.detail.inWindow", {
							window: activity?.windowMinutes ?? 5,
						})}
						danger={(activity?.failedInWindow ?? 0) > 0}
					/>
					<Stat
						label={t("pages.support.detail.queued")}
						value={activity ? String(activity.queuedNow) : "—"}
						hint={t("pages.support.detail.queuedHint")}
						danger={(activity?.queuedNow ?? 0) > 50}
					/>
				</div>

				{/* Filas globais: contexto de plataforma, NAO do cliente. O rotulo diz
            isso explicitamente para ninguem ler "5k pendentes" como backlog
            deste usuario. */}
				{(activity?.queues ?? []).length > 0 && (
					<div className="mt-4 border-t pt-3">
						<p className="mb-2 text-xs font-medium text-muted-foreground">
							{t("pages.support.detail.globalQueues")}
						</p>
						<div className="flex flex-wrap gap-2">
							{activity?.queues.map((q) => (
								<Badge
									key={q.queue}
									variant="outline"
									className="font-mono text-[11px]"
								>
									{q.queue}: {q.pending}p / {q.active}a
								</Badge>
							))}
						</div>
					</div>
				)}
			</div>

			<Tabs defaultValue="chips">
				<TabsList className="flex-wrap">
					<TabsTrigger value="chips">
						{t("pages.support.detail.tabs.chips")} ({chips.length})
					</TabsTrigger>
					<TabsTrigger value="monitors">
						{t("pages.support.detail.tabs.monitors")} ({monitors.length})
					</TabsTrigger>
					<TabsTrigger value="mirrors">
						{t("pages.support.detail.tabs.mirrors")} ({mirrors.length})
					</TabsTrigger>
					<TabsTrigger value="others">
						{t("pages.support.detail.tabs.others")} ({others.length})
					</TabsTrigger>
					<TabsTrigger value="lists">
						{t("pages.support.detail.tabs.lists")} ({linkLists.length})
					</TabsTrigger>
					<TabsTrigger value="groups">
						{t("pages.support.detail.tabs.groups")} ({groups.length})
					</TabsTrigger>
				</TabsList>

				<TabsContent value="chips" className="mt-3 space-y-2">
					{chips.length === 0 && (
						<Empty text={t("pages.support.detail.noChips")} />
					)}
					{chips.map((c) => (
						<div
							key={c.instanceId}
							className="flex flex-wrap items-center justify-between gap-2 rounded-xl border bg-background p-3"
						>
							<div className="min-w-0">
								<p className="truncate text-sm font-medium">
									{c.friendlyName || c.instanceId}
								</p>
								<p className="truncate font-mono text-xs text-muted-foreground">
									{c.instanceId}
								</p>
								{c.phoneNumber && (
									<p className="text-xs text-muted-foreground">
										{c.phoneNumber}
									</p>
								)}
							</div>
							<div className="flex items-center gap-2">
								<span className="text-xs text-muted-foreground">
									{t("pages.support.detail.groupsCount", {
										count: c.groupsCount,
									})}
								</span>
								<Badge
									variant={c.status === "connected" ? "default" : "destructive"}
								>
									{c.status}
								</Badge>
							</div>
						</div>
					))}
				</TabsContent>

				{[
					{ key: "monitors", list: monitors },
					{ key: "mirrors", list: mirrors },
					{ key: "others", list: others },
				].map(({ key, list }) => (
					<TabsContent key={key} value={key} className="mt-3 space-y-2">
						{list.length === 0 && (
							<Empty text={t("pages.support.detail.noAutomations")} />
						)}
						{list.map((a) => {
							const act = activityByAutomation.get(a.id);
							return (
								<div key={a.id} className="rounded-xl border bg-background p-3">
									<div className="flex flex-wrap items-start justify-between gap-2">
										<div className="min-w-0">
											<p className="truncate text-sm font-medium">{a.name}</p>
											<p className="text-xs text-muted-foreground">
												{TYPE_LABEL[a.type] ?? a.type}
												{a.instanceIds.length > 0 &&
													` · ${a.instanceIds.join(", ")}`}
												{a.intervalMinutes ? ` · ${a.intervalMinutes}min` : ""}
											</p>
										</div>
										<div className="flex flex-wrap items-center gap-1.5">
											<Badge
												variant={
													a.status === "active" ? "secondary" : "outline"
												}
											>
												{a.status}
											</Badge>
											<DispatchStatusBadge
												isDispatching={act?.isDispatching ?? false}
												signal={act?.signal ?? ""}
											/>
										</div>
									</div>

									<div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
										<span>
											{t("pages.support.detail.sentShort", {
												n: act?.sentInWindow ?? 0,
											})}
										</span>
										{(act?.failedInWindow ?? 0) > 0 && (
											<span className="text-destructive">
												{t("pages.support.detail.failedShort", {
													n: act?.failedInWindow,
												})}
											</span>
										)}
										{(act?.queuedNow ?? 0) > 0 && (
											<span>
												{t("pages.support.detail.queuedShort", {
													n: act?.queuedNow,
												})}
											</span>
										)}
									</div>

									{a.monitorGroups.length > 0 && (
										<GroupLine
											label={t("pages.support.detail.monitoring")}
											groups={a.monitorGroups}
										/>
									)}
									{a.sendGroups.length > 0 && (
										<GroupLine
											label={t("pages.support.detail.sendingTo")}
											groups={a.sendGroups}
										/>
									)}
									{(a.type === "search" || a.type === "list") && (
										<VelocityLine
											measured={
												act && activity
													? act.sentInWindow /
														Math.max(activity.windowMinutes, 1)
													: 0
											}
											configured={activity?.configuredPerMinute ?? 0}
											queued={act?.queuedNow ?? 0}
											t={t}
										/>
									)}
									{a.lastError && (
										<p className="mt-2 rounded bg-destructive/5 p-2 font-mono text-[11px] text-destructive">
											{a.lastError}
										</p>
									)}
								</div>
							);
						})}
					</TabsContent>
				))}

				<TabsContent value="lists" className="mt-3 space-y-2">
					{linkLists.length === 0 && (
						<Empty text={t("pages.support.detail.noLists")} />
					)}
					{linkLists.map((l) => (
						<div key={l.id} className="rounded-xl border bg-background p-3">
							<div className="flex flex-wrap items-center justify-between gap-2">
								<div className="min-w-0">
									<p className="truncate text-sm font-medium">{l.name}</p>
									{l.description && (
										<p className="truncate text-xs text-muted-foreground">
											{l.description}
										</p>
									)}
								</div>
								<Badge variant={l.isActive ? "secondary" : "outline"}>
									{l.isActive
										? t("pages.support.detail.active")
										: t("pages.support.detail.inactive")}
								</Badge>
							</div>
							<div className="mt-2 flex flex-wrap gap-x-4 text-xs text-muted-foreground">
								<span>
									{t("pages.support.detail.itemsTotal", { n: l.itemsTotal })}
								</span>
								<span>
									{t("pages.support.detail.itemsConsumed", {
										n: l.itemsConsumed,
									})}
								</span>
								<span>
									{t("pages.support.detail.itemsPending", {
										n: l.itemsPending,
									})}
								</span>
							</div>
						</div>
					))}
				</TabsContent>

				<TabsContent value="groups" className="mt-3 space-y-2">
					{groups.length === 0 && (
						<Empty text={t("pages.support.detail.noGroups")} />
					)}
					{groups.map((g) => (
						<div
							key={g.waGroupId}
							className="flex flex-wrap items-center justify-between gap-2 rounded-xl border bg-background p-3"
						>
							<div className="min-w-0">
								<p className="truncate text-sm font-medium">
									{g.name || g.waGroupId}
								</p>
								<p className="truncate font-mono text-[11px] text-muted-foreground">
									{g.waGroupId}
								</p>
								{g.participatingInstanceIds.length > 0 && (
									<p className="truncate text-xs text-muted-foreground">
										{g.participatingInstanceIds.join(", ")}
									</p>
								)}
							</div>
							<div className="flex items-center gap-2">
								{g.isAdmin && <Badge variant="secondary">admin</Badge>}
								<Button
									variant="outline"
									size="sm"
									onClick={() =>
										setChat({
											groupId: g.waGroupId,
											name: g.name,
											chip: g.participatingInstanceIds[0],
										})
									}
									disabled={g.participatingInstanceIds.length === 0}
								>
									<MessageSquare className="mr-1.5 h-3.5 w-3.5" />
									{t("pages.support.detail.openChat")}
								</Button>
							</div>
						</div>
					))}
				</TabsContent>
			</Tabs>

			{chat && (
				<GroupChatDialog
					open={!!chat}
					onOpenChange={(o) => !o && setChat(null)}
					userId={userId}
					groupId={chat.groupId}
					groupName={chat.name}
					instanceId={chat.chip}
				/>
			)}
		</div>
	);
}

function Stat({
	label,
	value,
	hint,
	danger,
}: {
	label: string;
	value: string;
	hint?: string;
	danger?: boolean;
}) {
	return (
		<div className="rounded-lg border p-3">
			<p className="text-xs text-muted-foreground">{label}</p>
			<p
				className={`text-xl font-semibold ${danger ? "text-destructive" : ""}`}
			>
				{value}
			</p>
			{hint && <p className="text-[10px] text-muted-foreground">{hint}</p>}
		</div>
	);
}

function Empty({ text }: { text: string }) {
	return (
		<p className="rounded-xl border border-dashed p-6 text-center text-sm text-muted-foreground">
			{text}
		</p>
	);
}

type TFn = (key: string, opts?: Record<string, unknown>) => string;

/** "há 5 minutos" / "ontem" via Intl; "nunca" quando o cliente jamais logou. */
function relativeTime(iso: string | undefined, t: TFn): string {
	if (!iso) return t("pages.support.detail.never");
	const ts = Date.parse(iso);
	if (Number.isNaN(ts)) return t("pages.support.detail.never");
	const diffMs = ts - Date.now();
	const rtf = new Intl.RelativeTimeFormat("pt-BR", { numeric: "auto" });
	const min = Math.round(diffMs / 60_000);
	if (Math.abs(min) < 60) return rtf.format(min, "minute");
	const hr = Math.round(diffMs / 3_600_000);
	if (Math.abs(hr) < 24) return rtf.format(hr, "hour");
	return rtf.format(Math.round(diffMs / 86_400_000), "day");
}

function formatDate(iso: string): string {
	const ts = Date.parse(iso);
	return Number.isNaN(ts) ? "—" : new Date(ts).toLocaleDateString("pt-BR");
}

/** Destaca em âmbar quando o cliente bateu no teto do plano. */
function usageClass(used: number, max: number | undefined): string {
	return max && max > 0 && used >= max
		? "font-medium text-amber-600"
		: "text-foreground";
}

/** Linha "Monitorando / Enviando para: grupo1, grupo2". */
function GroupLine({
  label,
  groups,
}: {
  label: string;
  groups: { waGroupId: string; name?: string }[];
}) {
  return (
    <p className="mt-1 text-xs text-muted-foreground">
      <span className="font-medium">{label}: </span>
      {groups.map((g) => g.name || g.waGroupId).join(', ')}
    </p>
  );
}

/**
 * Velocidade de uma automação de busca/lista: real x configurada.
 * O atendente precisa saber num relance: está fluindo perto do teto (ok), ou
 * tem fila acumulada e quase nada saindo (travado)?
 */
function VelocityLine({
  measured,
  configured,
  queued,
  t,
}: {
  measured: number;
  configured: number;
  queued: number;
  t: TFn;
}) {
  const stalled = queued > 0 && measured < 0.5;
  const color = stalled
    ? 'text-red-600'
    : measured > 0
      ? 'text-emerald-600'
      : 'text-muted-foreground';
  return (
    <p className="mt-1 text-xs">
      <span className="font-medium text-muted-foreground">
        {t('pages.support.detail.velocity')}:{' '}
      </span>
      <span className={color}>
        {measured.toFixed(1)} {t('pages.support.detail.perMin')}
      </span>
      <span className="text-muted-foreground">
        {' '}
        {t('pages.support.detail.ofConfigured', { n: configured.toFixed(0) })}
      </span>
      {stalled && (
        <span className="ml-1 font-medium text-red-600">{t('pages.support.detail.stalled')}</span>
      )}
    </p>
  );
}
