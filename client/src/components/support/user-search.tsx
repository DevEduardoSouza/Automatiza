import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import {
	MatchField,
	SearchUsersRequestSchema,
} from "@/gen/support/v1/support_pb";
import { searchUsers } from "@/gen/support/v1/support-SupportService_connectquery";
import { useDebouncedValue } from "@/hooks/use-debounced-value";
import { create } from "@bufbuild/protobuf";
import { useQuery } from "@connectrpc/connect-query";
import { Search, Smartphone, Zap } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";

/** Rotulo do campo que casou com a busca. */
const MATCH_LABEL: Record<number, string> = {
	[MatchField.EMAIL]: "e-mail",
	[MatchField.NAME]: "nome",
	[MatchField.PHONE]: "telefone",
	[MatchField.INSTANCE]: "chip",
};

/**
 * Busca de usuario do painel de suporte.
 *
 * Casa com e-mail, nome, telefone (com ou sem mascara) e chip. Mostrar POR QUE
 * cada resultado apareceu importa: quando o atendente busca um numero e o
 * resultado casou pelo chip (e nao pelo telefone da conta), isso ja e metade
 * do diagnostico.
 */
export function UserSearch() {
	const { t } = useTranslation();
	const navigate = useNavigate();
	const [term, setTerm] = useState("");
	// Sem debounce, cada tecla vira um ILIKE sobre profiles + instances.
	const debounced = useDebouncedValue(term, 350);

	const { data, isLoading, error } = useQuery(
		searchUsers,
		create(SearchUsersRequestSchema, { query: debounced }),
		{ enabled: debounced.trim().length >= 2 },
	);

	const users = data?.users ?? [];
	const showEmpty =
		debounced.trim().length >= 2 && !isLoading && users.length === 0;

	return (
		<div className="mx-auto w-full max-w-3xl space-y-4">
			<div>
				<h1 className="text-xl font-semibold tracking-tight sm:text-2xl">
					{t("pages.support.search.title")}
				</h1>
				<p className="text-sm text-muted-foreground">
					{t("pages.support.search.subtitle")}
				</p>
			</div>

			<div className="relative">
				<Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
				<Input
					value={term}
					onChange={(e) => setTerm(e.target.value)}
					placeholder={t("pages.support.search.placeholder")}
					className="pl-9"
					autoFocus
					aria-label={t("pages.support.search.title")}
				/>
			</div>

			{error && (
				<p className="rounded-lg border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive">
					{error.message}
				</p>
			)}

			{isLoading && debounced.trim().length >= 2 && (
				<div className="space-y-2">
					{[0, 1, 2].map((i) => (
						<Skeleton key={i} className="h-20 w-full rounded-xl" />
					))}
				</div>
			)}

			{showEmpty && (
				<p className="rounded-xl border border-dashed p-6 text-center text-sm text-muted-foreground">
					{t("pages.support.search.empty")}
				</p>
			)}

			<ul className="space-y-2">
				{users.map((u) => (
					<li key={u.id}>
						<button
							type="button"
							onClick={() => navigate(`/support/user/${u.id}`)}
							className="w-full cursor-pointer rounded-xl border bg-background p-4 text-left transition-all hover:-translate-y-0.5 hover:shadow-md"
						>
							<div className="flex flex-wrap items-center justify-between gap-2">
								<div className="min-w-0">
									<p className="truncate font-medium">{u.name || u.email}</p>
									<p className="truncate text-sm text-muted-foreground">
										{u.email}
									</p>
									{u.phoneNumber && (
										<p className="truncate text-xs text-muted-foreground">
											{u.phoneNumber}
										</p>
									)}
								</div>
								<div className="flex shrink-0 flex-wrap items-center gap-1.5">
									{!u.isActive && (
										<Badge variant="destructive">
											{t("pages.support.search.inactive")}
										</Badge>
									)}
									{/* Cobranca suspensa e a explicacao mais comum para "parou de
                      enviar", e e invisivel no resto da tela: sem este badge o
                      atendente abre o cliente e vai procurar defeito tecnico. */}
									{u.billingStatus && u.billingStatus !== "active" && (
										<Badge variant="destructive">{u.billingStatus}</Badge>
									)}
									{u.plan && <Badge variant="secondary">{u.plan}</Badge>}
								</div>
							</div>

							<div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
								<span className="inline-flex items-center gap-1">
									<Smartphone className="h-3.5 w-3.5" />
									{t("pages.support.search.chips", {
										connected: u.chipsConnected,
										total: u.chipsTotal,
									})}
								</span>
								<span className="inline-flex items-center gap-1">
									<Zap className="h-3.5 w-3.5" />
									{t("pages.support.search.automations", {
										active: u.automationsActive,
										total: u.automationsTotal,
									})}
								</span>
								{u.matchedOn.length > 0 && (
									<span>
										{t("pages.support.search.matchedOn", {
											fields: u.matchedOn
												.map((m) => MATCH_LABEL[m] ?? "")
												.filter(Boolean)
												.join(", "),
										})}
									</span>
								)}
							</div>
						</button>
					</li>
				))}
			</ul>

			{data?.hasMore && (
				<p className="text-center text-xs text-muted-foreground">
					{t("pages.support.search.hasMore")}
				</p>
			)}
		</div>
	);
}
