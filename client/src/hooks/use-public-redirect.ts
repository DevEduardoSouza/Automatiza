import { captureHookError } from "@/lib/sentry";
import { redirectService } from "@/services/api.service";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useLocation, useParams } from "react-router";

interface RedirectResponse {
	success: boolean;
	group_name?: string;
	redirect_url?: string;
	error?: string;
}

type RedirectErrorType =
	| "groups_full"
	| "temporary"
	| "not_found"
	| "generic"
	| null;

interface UsePublicRedirectReturn {
	loading: boolean;
	error: string | null;
	errorType: RedirectErrorType;
	groupName: string;
	redirectUrl: string;
	actionHint: string;
	isInAppBrowser: boolean;
	inviteCode: string;
	isIOS: boolean;
	isAndroid: boolean;
	tryOpenWhatsApp: () => void;
	copyInviteLink: () => Promise<void>;
	openInviteLink: () => void;
}

/**
 * Closed set of error codes the backend puts in `/join/<slug>?error=<code>`
 * (see redirectBusinessErrorCodes in redirect_public_handler.go). Anything not
 * listed here renders as generic — the query string is user-controllable, so
 * it may only ever select an existing translation, never supply text.
 */
const ERROR_CODE_TO_I18N_KEY: Record<string, string> = {
	not_found: "pages.publicRedirect.error.invalidLink",
	inactive: "pages.publicRedirect.error.linkInactive",
	no_groups: "pages.publicRedirect.error.noGroups",
	groups_full: "pages.publicRedirect.error.groupsFull",
	group_not_found: "pages.publicRedirect.error.groupNotFound",
	instance_not_found: "pages.publicRedirect.error.instanceNotFound",
	not_admin: "pages.publicRedirect.error.notAdmin",
	temporary: "pages.publicRedirect.error.temporarilyUnavailable",
	invite_unavailable: "pages.publicRedirect.error.inviteCodeFailed",
	provider_unavailable: "pages.publicRedirect.error.providerNotConfigured",
};

const ERROR_CODE_TO_TYPE: Record<string, RedirectErrorType> = {
	groups_full: "groups_full",
	temporary: "temporary",
	not_found: "not_found",
	group_not_found: "not_found",
	instance_not_found: "not_found",
};

/**
 * Hook for public redirect page logic
 * Manages WhatsApp group redirect with in-app browser detection
 *
 * Two routes render this hook:
 * - `/r/:slug` — legacy SPA path, kept for old bundles and for anyone whose
 *   service worker still serves the app shell for /r. Auto-redirects.
 * - `/join/:slug` — fallback the backend 302s to. ALWAYS manual: the backend
 *   only sends clients here when a direct redirect would not work, so
 *   auto-redirecting would undo the very decision that routed them here.
 */
export function usePublicRedirect(): UsePublicRedirectReturn {
	const { t } = useTranslation();
	const { slug } = useParams<{ slug: string }>();
	const location = useLocation();
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [errorType, setErrorType] = useState<RedirectErrorType>(null);
	const [groupName, setGroupName] = useState<string>("");
	const [redirectUrl, setRedirectUrl] = useState<string>("");
	const [actionHint, setActionHint] = useState<string>("");

	const ua = useMemo(
		() => (typeof navigator !== "undefined" ? navigator.userAgent || "" : ""),
		[],
	);

	const referrer = useMemo(
		() => (typeof document !== "undefined" ? document.referrer || "" : ""),
		[],
	);

	const isInAppBrowser = useMemo(() => {
		const byUa =
			/TikTok|Musical\.ly|TTWebView|ByteDance|Aweme|trill|Instagram|FBAN|FBAV|FB_IAB|Line|Twitter|Snapchat/i.test(
				ua,
			);
		const byReferrer = /tiktok\.com|tiktoklinksafety\.com/i.test(referrer);
		return byUa || byReferrer;
	}, [ua, referrer]);

	const inviteCode = useMemo(() => {
		if (!redirectUrl) return "";
		// Same character class the backend validates against: codes do contain
		// `_` and `-`, and truncating at the first one produced a dead deep link.
		const match = redirectUrl.match(/chat\.whatsapp\.com\/([A-Za-z0-9_-]+)/i);
		return match?.[1] || "";
	}, [redirectUrl]);

	// `/join/:slug` is the backend's fallback destination; `/r/:slug` is the
	// legacy SPA route.
	const isJoinRoute = useMemo(
		() => location.pathname.startsWith("/join/"),
		[location.pathname],
	);

	const errorCode = useMemo(
		() => new URLSearchParams(location.search).get("error") || "",
		[location.search],
	);

	const isPreviewMode = useMemo(
		() => new URLSearchParams(location.search).get("mode") === "preview",
		[location.search],
	);

	// Drop our own query params from the address bar so the page's own
	// "try again" (a plain reload) starts a CLEAN attempt instead of replaying
	// the error/preview state forever.
	const stripRedirectQuery = useCallback(() => {
		if (typeof window === "undefined" || !window.history?.replaceState) return;
		window.history.replaceState(null, "", location.pathname);
	}, [location.pathname]);

	const isIOS = useMemo(() => /iPhone|iPad|iPod/i.test(ua), [ua]);
	const isAndroid = useMemo(() => /Android/i.test(ua), [ua]);

	// Translate backend error messages to user-friendly messages
	const translateError = useCallback(
		(backendError: string): string => {
			const errorMap: Record<string, string> = {
				"link not found": t("pages.publicRedirect.error.invalidLink"),
				"link is inactive": t("pages.publicRedirect.error.linkInactive"),
				"no target groups configured": t("pages.publicRedirect.error.noGroups"),
				"no available groups (all groups are full or inactive)": t(
					"pages.publicRedirect.error.groupsFull",
				),
				"group not found": t("pages.publicRedirect.error.groupNotFound"),
				"instance not found": t("pages.publicRedirect.error.instanceNotFound"),
				"bot is not admin in group": t("pages.publicRedirect.error.notAdmin"),
				"failed to get invite code": t(
					"pages.publicRedirect.error.inviteCodeFailed",
				),
				"service temporarily unavailable": t(
					"pages.publicRedirect.error.temporarilyUnavailable",
				),
				"invite code not available": t(
					"pages.publicRedirect.error.inviteCodeNotAvailable",
				),
				"WhatsApp provider not configured": t(
					"pages.publicRedirect.error.providerNotConfigured",
				),
			};
			return errorMap[backendError] || backendError;
		},
		[t],
	);

	const handleRedirect = useCallback(async () => {
		try {
			const data: RedirectResponse =
				await redirectService.processRedirectBySlug(slug as string);

			if (!data?.success) {
				const backendError = data?.error || "";

				// Classify error type BEFORE translation
				let type: RedirectErrorType = "generic";
				if (backendError.includes("all groups are full")) type = "groups_full";
				else if (backendError.includes("temporarily unavailable"))
					type = "temporary";
				else if (backendError.includes("not found")) type = "not_found";

				setErrorType(type);
				throw new Error(
					translateError(backendError) ||
						t("pages.publicRedirect.error.genericError"),
				);
			}

			setGroupName(data.group_name || "");
			setRedirectUrl(data.redirect_url || "");

			// /join is manual by definition, whatever the UA says: the backend
			// already decided a direct redirect would not work for this client.
			if (!isJoinRoute && !isInAppBrowser && data.redirect_url) {
				window.location.href = data.redirect_url;
				setTimeout(() => setLoading(false), 2500);
				return;
			}

			setLoading(false);
		} catch (err: unknown) {
			captureHookError(err, "usePublicRedirect", "handleRedirect");
			const errorMessage =
				err instanceof Error
					? err.message
					: t("pages.publicRedirect.error.genericError");
			setError(errorMessage);
			setLoading(false);
		}
	}, [slug, t, isInAppBrowser, isJoinRoute, translateError]);

	// Slug already attempted in this mount. A ref (not state) and marked BEFORE
	// the call, so a rerender — e.g. i18next swapping the `t` identity — cannot
	// fire a second POST for the same slug. A genuinely different slug is a new
	// attempt and is allowed through.
	const processedSlugRef = useRef<string | null>(null);

	useEffect(() => {
		if (!slug) {
			setError(t("pages.publicRedirect.error.invalidLink"));
			setLoading(false);
			return;
		}

		// The direct GET already ran the whole thing and told us how it failed.
		// Render that verdict; do NOT ask the API again — a second call would be a
		// second attempt, on top of the one that just failed.
		if (errorCode) {
			// Object.hasOwn, não indexação direta: `errorCode` vem da URL, e chaves
			// como `constructor`/`toString` resolvem no PROTÓTIPO — devolvendo uma
			// função em vez de undefined, o `??` nunca dispararia e t() receberia um
			// objeto, deixando a tela em branco.
			const key = Object.hasOwn(ERROR_CODE_TO_I18N_KEY, errorCode)
				? ERROR_CODE_TO_I18N_KEY[errorCode]
				: "pages.publicRedirect.error.genericError";
			setErrorType(
				Object.hasOwn(ERROR_CODE_TO_TYPE, errorCode)
					? ERROR_CODE_TO_TYPE[errorCode]
					: "generic",
			);
			setError(t(key));
			setLoading(false);
			stripRedirectQuery();
			return;
		}

		if (isPreviewMode) {
			const doc = typeof document !== "undefined" ? document : undefined;
			const isPrerendering = (
				doc as (Document & { prerendering?: boolean }) | undefined
			)?.prerendering;

			// A crawler (or a speculative prefetch) is looking at the page. Counting
			// a click here would credit a group with a visitor that never existed.
			if (isPrerendering !== true || !doc) {
				// A real user can still land here — the UA heuristic has false
				// positives (CUBOT, say). Give them the retryable error state, whose
				// button reloads the now-clean URL into a normal attempt.
				setErrorType("temporary");
				setError(t("pages.publicRedirect.error.temporarilyUnavailable"));
				setLoading(false);
				stripRedirectQuery();
				return;
			}

			// Prerendered document: only when the user actually activates it does
			// this become a real visit worth counting.
			const onActivate = () => {
				stripRedirectQuery();
				if (processedSlugRef.current === slug) return;
				processedSlugRef.current = slug;
				handleRedirect();
			};
			doc.addEventListener("prerenderingchange", onActivate, { once: true });
			return () => doc.removeEventListener("prerenderingchange", onActivate);
		}

		if (processedSlugRef.current === slug) return;
		processedSlugRef.current = slug;
		handleRedirect();
	}, [slug, t, handleRedirect, errorCode, isPreviewMode, stripRedirectQuery]);

	const tryOpenWhatsApp = useCallback(() => {
		if (!redirectUrl) return;

		setActionHint("");

		const trySchemeWithoutNavigating = (url: string) => {
			try {
				const iframe = document.createElement("iframe");
				iframe.style.display = "none";
				iframe.src = url;
				document.body.appendChild(iframe);
				setTimeout(() => {
					try {
						document.body.removeChild(iframe);
					} catch {
						// Expected fallback: iframe may already be removed
					}
				}, 1500);
			} catch {
				// Expected fallback: iframe scheme navigation not supported
			}
		};

		if (inviteCode) {
			trySchemeWithoutNavigating(`whatsapp://chat?code=${inviteCode}`);
			trySchemeWithoutNavigating(`whatsapp://invite?code=${inviteCode}`);
			trySchemeWithoutNavigating(`whatsapp://invite/${inviteCode}`);

			if (isAndroid) {
				try {
					window.location.href = `intent://chat.whatsapp.com/${inviteCode}#Intent;scheme=https;package=com.whatsapp;end`;
					return;
				} catch {
					// Expected fallback: Android intent not supported
				}
			}
		}

		setTimeout(() => {
			if (document.visibilityState === "visible") {
				setActionHint(
					isIOS
						? t("pages.publicRedirect.manual.iosHint")
						: t("pages.publicRedirect.manual.androidHint"),
				);
			}
		}, 1200);
	}, [redirectUrl, inviteCode, isAndroid, isIOS, t]);

	const copyInviteLink = useCallback(async () => {
		if (!redirectUrl) return;
		try {
			await navigator.clipboard.writeText(redirectUrl);
		} catch {
			// Expected fallback: clipboard API unavailable, use execCommand
			const ta = document.createElement("textarea");
			ta.value = redirectUrl;
			ta.style.position = "fixed";
			ta.style.left = "-9999px";
			document.body.appendChild(ta);
			ta.select();
			document.execCommand("copy");
			document.body.removeChild(ta);
		}
	}, [redirectUrl]);

	const openInviteLink = useCallback(() => {
		if (!redirectUrl) return;
		try {
			const win = window.open(redirectUrl, "_blank", "noopener,noreferrer");
			if (!win) window.location.href = redirectUrl;
		} catch {
			// Expected fallback: window.open blocked, use location.href
			window.location.href = redirectUrl;
		}
	}, [redirectUrl]);

	return {
		loading,
		error,
		errorType,
		groupName,
		redirectUrl,
		actionHint,
		isInAppBrowser,
		inviteCode,
		isIOS,
		isAndroid,
		tryOpenWhatsApp,
		copyInviteLink,
		openInviteLink,
	};
}
