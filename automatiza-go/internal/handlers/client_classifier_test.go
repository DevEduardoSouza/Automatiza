package handlers

import (
	"net/http"
	"testing"
)

// Real-world User-Agent strings, kept verbatim so the table doubles as
// documentation of what we actually see in the access log.
const (
	uaChromeDesktop  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
	uaSafariMacOS    = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Safari/605.1.15"
	uaFirefoxDesktop = "Mozilla/5.0 (X11; Linux x86_64; rv:127.0) Gecko/20100101 Firefox/127.0"
	uaChromeAndroid  = "Mozilla/5.0 (Linux; Android 13; SM-G991B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36"
	uaFirefoxAndroid = "Mozilla/5.0 (Android 13; Mobile; rv:127.0) Gecko/127.0 Firefox/127.0"
	uaSafariIOS      = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1"
	uaChromeIOS      = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/126.0.0.0 Mobile/15E148 Safari/604.1"
	uaFirefoxIOS     = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) FxiOS/127.0 Mobile/15E148 Safari/605.1.15"
	uaEdgeIOS        = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) EdgiOS/126.0.0.0 Mobile/15E148 Safari/605.1.15"

	uaInstagram   = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Instagram 334.0.0.32.98 (iPhone14,3; iOS 17_5; en_US)"
	uaTikTok      = "Mozilla/5.0 (Linux; Android 13; SM-G991B) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/126.0.0.0 Mobile Safari/537.36 trill_340203 JsSdk/1.0 NetType/WIFI Channel/googleplay AppName/musical_ly app_version/34.2.3 ByteLocale/en"
	uaFacebookIAB = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 [FBAN/FBIOS;FBAV/468.0.0.47.108;FBBV/609735723]"
	uaLine        = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Line/14.9.1"
	uaTwitterApp  = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Twitter for iPhone"
	uaSnapchat    = "Mozilla/5.0 (Linux; Android 13; SM-G991B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36 Snapchat/12.90.0.40"
	uaAndroidWV   = "Mozilla/5.0 (Linux; Android 13; SM-G991B Build/TP1A.220624.014; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/126.0.0.0 Mobile Safari/537.36"
	uaIOSWKWebVw  = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148"

	uaWhatsApp     = "WhatsApp/2.2402.7 A"
	uaTwitterbot   = "Mozilla/5.0 (compatible; Twitterbot/1.0)"
	uaFacebookHit  = "facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)"
	uaFacebot      = "Facebot/1.0"
	uaSlackbot     = "Slackbot-LinkExpanding 1.0 (+https://api.slack.com/robots)"
	uaSlackImg     = "Slack-ImgProxy (+https://api.slack.com/robots)"
	uaTelegramBot  = "TelegramBot (like TwitterBot)"
	uaDiscordbot   = "Mozilla/5.0 (compatible; Discordbot/2.0; +https://discordapp.com)"
	uaLinkedInBot  = "LinkedInBot/1.0 (compatible; Mozilla/5.0; Apache-HttpClient +http://www.linkedin.com)"
	uaPinterest    = "Pinterest/0.2 (+https://www.pinterest.com/bot.html)"
	uaCurl         = "curl/8.4.0"
	uaWget         = "Wget/1.21.4"
	uaPython       = "python-requests/2.32.3"
	uaGoHTTP       = "Go-http-client/1.1"
	uaHeadless     = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/126.0.0.0 Safari/537.36"
	uaSkypePreview = "SkypeUriPreview Preview/0.5"
	uaIframely     = "Iframely/1.3.1 (+https://iframely.com/docs/about)"
	uaEmbedly      = "Embedly/0.2 (+http://support.embed.ly/)"
	uaGoogleBot    = "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"

	// Known false positive of the `bot` substring rule: a real phone brand.
	uaCubot = "Mozilla/5.0 (Linux; Android 12; CUBOT NOTE 20) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36"
)

func TestClassifyRedirectClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
		ua     string
		header http.Header
		want   redirectClientClass
	}{
		// --- normal browsers -------------------------------------------------
		{name: "chrome desktop", ua: uaChromeDesktop, want: redirectClientNormal},
		{name: "safari macos", ua: uaSafariMacOS, want: redirectClientNormal},
		{name: "firefox desktop", ua: uaFirefoxDesktop, want: redirectClientNormal},
		{name: "chrome android", ua: uaChromeAndroid, want: redirectClientNormal},
		{name: "firefox android", ua: uaFirefoxAndroid, want: redirectClientNormal},
		{name: "safari ios", ua: uaSafariIOS, want: redirectClientNormal},
		{name: "chrome ios (CriOS keeps Safari)", ua: uaChromeIOS, want: redirectClientNormal},
		{name: "firefox ios (FxiOS keeps Safari)", ua: uaFirefoxIOS, want: redirectClientNormal},
		{name: "edge ios (EdgiOS keeps Safari)", ua: uaEdgeIOS, want: redirectClientNormal},

		// --- in-app browsers -------------------------------------------------
		{name: "instagram", ua: uaInstagram, want: redirectClientInApp},
		{name: "tiktok", ua: uaTikTok, want: redirectClientInApp},
		{name: "facebook in-app", ua: uaFacebookIAB, want: redirectClientInApp},
		{name: "line", ua: uaLine, want: redirectClientInApp},
		{name: "twitter app", ua: uaTwitterApp, want: redirectClientInApp},
		{name: "snapchat", ua: uaSnapchat, want: redirectClientInApp},
		{name: "android webview", ua: uaAndroidWV, want: redirectClientInApp},
		{name: "ios wkwebview (Mobile/ without Safari)", ua: uaIOSWKWebVw, want: redirectClientInApp},

		// --- crawlers / unfurlers -------------------------------------------
		{name: "whatsapp unfurler", ua: uaWhatsApp, want: redirectClientPreview},
		{name: "twitterbot beats Twitter", ua: uaTwitterbot, want: redirectClientPreview},
		{name: "facebookexternalhit", ua: uaFacebookHit, want: redirectClientPreview},
		{name: "facebot", ua: uaFacebot, want: redirectClientPreview},
		{name: "slackbot", ua: uaSlackbot, want: redirectClientPreview},
		{name: "slack imgproxy", ua: uaSlackImg, want: redirectClientPreview},
		{name: "telegrambot", ua: uaTelegramBot, want: redirectClientPreview},
		{name: "discordbot", ua: uaDiscordbot, want: redirectClientPreview},
		{name: "linkedinbot", ua: uaLinkedInBot, want: redirectClientPreview},
		{name: "pinterest", ua: uaPinterest, want: redirectClientPreview},
		{name: "curl", ua: uaCurl, want: redirectClientPreview},
		{name: "wget", ua: uaWget, want: redirectClientPreview},
		{name: "python requests", ua: uaPython, want: redirectClientPreview},
		{name: "go http client", ua: uaGoHTTP, want: redirectClientPreview},
		{name: "headless chrome", ua: uaHeadless, want: redirectClientPreview},
		{name: "skype uri preview", ua: uaSkypePreview, want: redirectClientPreview},
		{name: "iframely", ua: uaIframely, want: redirectClientPreview},
		{name: "embedly", ua: uaEmbedly, want: redirectClientPreview},
		{name: "googlebot", ua: uaGoogleBot, want: redirectClientPreview},

		// Documented cost of the broad `bot` rule: this is a real phone, and it
		// gets the manual fallback instead of the direct 302.
		{name: "CUBOT phone is a known false positive", ua: uaCubot, want: redirectClientPreview},

		// --- empty UA --------------------------------------------------------
		{name: "empty ua", ua: "", want: redirectClientPreview},
		{name: "whitespace-only ua", ua: "   \t ", want: redirectClientPreview},

		// --- HEAD ------------------------------------------------------------
		{name: "HEAD with a browser ua", method: http.MethodHead, ua: uaChromeDesktop, want: redirectClientPreview},
		{name: "HEAD with an in-app ua", method: http.MethodHead, ua: uaInstagram, want: redirectClientPreview},

		// --- purpose headers -------------------------------------------------
		{
			name:   "Sec-Purpose prefetch;prerender",
			ua:     uaChromeDesktop,
			header: http.Header{"Sec-Purpose": []string{"prefetch;prerender"}},
			want:   redirectClientPreview,
		},
		{
			name:   "Purpose prefetch",
			ua:     uaChromeDesktop,
			header: http.Header{"Purpose": []string{"prefetch"}},
			want:   redirectClientPreview,
		},
		{
			name:   "X-Purpose preview",
			ua:     uaChromeDesktop,
			header: http.Header{"X-Purpose": []string{"preview"}},
			want:   redirectClientPreview,
		},
		{
			name:   "X-Moz prefetch",
			ua:     uaChromeDesktop,
			header: http.Header{"X-Moz": []string{"prefetch"}},
			want:   redirectClientPreview,
		},
		{
			name:   "purpose token is case-insensitive",
			ua:     uaChromeDesktop,
			header: http.Header{"Purpose": []string{"PREFETCH"}},
			want:   redirectClientPreview,
		},
		{
			name:   "purpose token separated by comma",
			ua:     uaChromeDesktop,
			header: http.Header{"Sec-Purpose": []string{"foo, prerender"}},
			want:   redirectClientPreview,
		},
		{
			name:   "unrelated purpose value stays normal",
			ua:     uaChromeDesktop,
			header: http.Header{"Purpose": []string{"navigate"}},
			want:   redirectClientNormal,
		},
		{
			name:   "purpose substring is not a token",
			ua:     uaChromeDesktop,
			header: http.Header{"Purpose": []string{"prefetching"}},
			want:   redirectClientNormal,
		},

		// --- referrer --------------------------------------------------------
		{
			name:   "tiktok referrer, scrubbed ua",
			ua:     uaChromeAndroid,
			header: http.Header{"Referer": []string{"https://www.tiktok.com/@someone/video/123"}},
			want:   redirectClientInApp,
		},
		{
			name:   "tiktok apex referrer",
			ua:     uaChromeAndroid,
			header: http.Header{"Referer": []string{"https://tiktok.com/"}},
			want:   redirectClientInApp,
		},
		{
			name:   "tiktoklinksafety referrer",
			ua:     uaChromeAndroid,
			header: http.Header{"Referer": []string{"https://tiktoklinksafety.com/redirect?u=x"}},
			want:   redirectClientInApp,
		},
		{
			name:   "lookalike host is NOT tiktok",
			ua:     uaChromeAndroid,
			header: http.Header{"Referer": []string{"https://tiktok.com.evil.test/path"}},
			want:   redirectClientNormal,
		},
		{
			name:   "tiktok as a path substring is NOT tiktok",
			ua:     uaChromeAndroid,
			header: http.Header{"Referer": []string{"https://example.test/tiktok.com"}},
			want:   redirectClientNormal,
		},
		{
			name:   "unparseable referrer is ignored",
			ua:     uaChromeAndroid,
			header: http.Header{"Referer": []string{"://not a url"}},
			want:   redirectClientNormal,
		},
		{
			name:   "referrer without host is ignored",
			ua:     uaChromeAndroid,
			header: http.Header{"Referer": []string{"/relative/path"}},
			want:   redirectClientNormal,
		},

		// --- spoofing (documented limitation) --------------------------------
		// A bot claiming to be Chrome is indistinguishable from Chrome here and
		// WILL consume one attempt. UA is a heuristic, never an identity.
		{name: "spoofed bot presenting as chrome is normal", ua: uaChromeDesktop, want: redirectClientNormal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			method := tt.method
			if method == "" {
				method = http.MethodGet
			}
			got := classifyRedirectClient(method, tt.ua, tt.header)
			if got != tt.want {
				t.Errorf("classifyRedirectClient(%q, %q) = %q, want %q", method, tt.ua, got, tt.want)
			}
		})
	}
}

// The crawler rule must win over the in-app rule, otherwise Twitterbot (which
// contains "Twitter") would be treated as a user and count a click.
func TestClassifyRedirectClient_BotRulePrecedesInApp(t *testing.T) {
	t.Parallel()

	if got := classifyRedirectClient(http.MethodGet, uaTwitterbot, nil); got != redirectClientPreview {
		t.Errorf("Twitterbot = %q, want %q", got, redirectClientPreview)
	}
	if got := classifyRedirectClient(http.MethodGet, uaTwitterApp, nil); got != redirectClientInApp {
		t.Errorf("Twitter app = %q, want %q", got, redirectClientInApp)
	}
}

// HEAD is rule 1: it wins even over an otherwise-normal request with headers
// that would classify differently.
func TestClassifyRedirectClient_HeadWinsOverEverything(t *testing.T) {
	t.Parallel()

	h := http.Header{"Referer": []string{"https://www.tiktok.com/"}}
	if got := classifyRedirectClient(http.MethodHead, uaTikTok, h); got != redirectClientPreview {
		t.Errorf("HEAD with in-app signals = %q, want %q", got, redirectClientPreview)
	}
}

func TestClassifyRedirectClient_NilHeaderIsSafe(t *testing.T) {
	t.Parallel()

	if got := classifyRedirectClient(http.MethodGet, uaChromeDesktop, nil); got != redirectClientNormal {
		t.Errorf("nil header = %q, want %q", got, redirectClientNormal)
	}
}
