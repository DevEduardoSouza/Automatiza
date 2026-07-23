package handlers

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// redirectClientClass is how GET /r/:slug decides what to do with a caller.
//
// It is a HEURISTIC over the User-Agent and a few request headers, never an
// identity: a bot that presents itself as Chrome is classified as normal and
// will consume one attempt. Rate limiting bounds abuse; classification does not.
type redirectClientClass string

const (
	// redirectClientNormal is a real browser navigation: it gets the 302
	// straight to WhatsApp and it is the ONLY class that touches the service.
	redirectClientNormal redirectClientClass = "normal"

	// redirectClientInApp is a known in-app webview (TikTok, Instagram, ...)
	// where a direct 302 to chat.whatsapp.com dies inside the embedded browser.
	// It is bounced to /join/<slug>, which always shows the manual fallback.
	redirectClientInApp redirectClientClass = "in_app"

	// redirectClientPreview is a crawler, link unfurler, HEAD probe or
	// prefetch/prerender. It must never select a group nor increment a click.
	redirectClientPreview redirectClientClass = "preview"
)

// Headers browsers/proxies use to announce a speculative (non-user) fetch.
var redirectPurposeHeaders = []string{"Sec-Purpose", "Purpose", "X-Purpose", "X-Moz"}

// Bots, crawlers and link unfurlers. Matched BEFORE the in-app list so that
// `Twitterbot` (a crawler) wins over `Twitter` (the in-app browser).
//
// `bot` is deliberately a substring match: it catches Slackbot/TelegramBot/
// Discordbot/LinkedInBot and every unknown `...bot/1.0` in one rule. The known
// cost is a false positive on the CUBOT phone brand — a real user on a CUBOT
// device gets the manual fallback instead of the direct 302, which is degraded
// but functional. Narrowing this would let unknown crawlers count clicks.
var redirectBotUARe = regexp.MustCompile(`(?i)bot|crawler|spider|facebookexternalhit|facebot|WhatsApp/|Slackbot|Slack-ImgProxy|TelegramBot|Twitterbot|Discordbot|LinkedInBot|Pinterest|curl|wget|python|Go-http-client|HeadlessChrome|SkypeUriPreview|Iframely|Embedly|preview`)

// Known in-app browsers. Mirrors the frontend list in use-public-redirect.ts.
var redirectInAppUARe = regexp.MustCompile(`(?i)TikTok|Musical\.ly|TTWebView|ByteDance|Aweme|trill|Instagram|FBAN|FBAV|FB_IAB|Line/|Twitter|Snapchat`)

// Hosts whose referrer means the click came out of an in-app browser even when
// the UA is scrubbed. Matched on the parsed hostname, never as a substring:
// `tiktok.com.evil.test` must NOT match.
var redirectInAppRefererHosts = []string{"tiktok.com", "tiktoklinksafety.com"}

// classifyRedirectClient decides how to serve GET/HEAD /r/:slug.
//
// Pure function: no I/O, no state. The first rule that matches wins, in the
// documented order — the order IS the contract (see the plan's decision table).
func classifyRedirectClient(method, ua string, header http.Header) redirectClientClass {
	// 1. HEAD is never a user navigation.
	if method == http.MethodHead {
		return redirectClientPreview
	}

	// 2. Speculative fetch announced by the client.
	if hasPreviewPurpose(header) {
		return redirectClientPreview
	}

	ua = strings.TrimSpace(ua)

	// 3. No UA at all: scripts and probes, not browsers.
	if ua == "" {
		return redirectClientPreview
	}

	// 4. Known crawler/unfurler.
	if redirectBotUARe.MatchString(ua) {
		return redirectClientPreview
	}

	// 5. Referrer from an in-app host (UA may be scrubbed).
	if header != nil && isInAppReferer(header.Get("Referer")) {
		return redirectClientInApp
	}

	// 6. Known in-app browser by UA.
	if redirectInAppUARe.MatchString(ua) {
		return redirectClientInApp
	}

	// 7. Android WebView: the `; wv)` token in the platform section.
	if strings.Contains(ua, "Android") && strings.Contains(ua, "; wv)") {
		return redirectClientInApp
	}

	// 8. iOS WKWebView: an iOS UA with `Mobile/` but no `Safari` token. Real
	// Safari, and CriOS/FxiOS/EdgiOS, all carry `Safari`.
	if isIOSUA(ua) && strings.Contains(ua, "Mobile/") && !strings.Contains(ua, "Safari") {
		return redirectClientInApp
	}

	// 9. Everything else is a normal browser.
	return redirectClientNormal
}

// hasPreviewPurpose reports whether any purpose header announces a speculative
// fetch. Tokens are separated by space, comma or `;` (Sec-Purpose sends
// `prefetch;prerender`).
func hasPreviewPurpose(header http.Header) bool {
	if header == nil {
		return false
	}
	isSep := func(r rune) bool {
		return r == ' ' || r == ',' || r == ';' || r == '\t'
	}
	for _, name := range redirectPurposeHeaders {
		for _, raw := range header.Values(name) {
			for _, tok := range strings.FieldsFunc(raw, isSep) {
				switch strings.ToLower(tok) {
				case "prefetch", "prerender", "preview":
					return true
				}
			}
		}
	}
	return false
}

// isInAppReferer matches the referrer's HOSTNAME exactly or as a subdomain.
func isInAppReferer(referer string) bool {
	if referer == "" {
		return false
	}
	u, err := url.Parse(referer)
	if err != nil || u.Host == "" {
		return false
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	for _, domain := range redirectInAppRefererHosts {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

func isIOSUA(ua string) bool {
	return strings.Contains(ua, "iPhone") || strings.Contains(ua, "iPad") || strings.Contains(ua, "iPod")
}
