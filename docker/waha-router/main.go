// waha-router — thin reverse proxy in front of one or more WAHA instances.
//
// Purpose:
//  1. STRUCTURAL ANTI-DESTRUCTION: block DELETE /api/sessions/{s} and
//     POST /api/sessions/{s}/logout for a PAIRED session (me != null) unless the
//     caller sends header `X-Confirm-Unpair: true`. This stops the backend from
//     irreversibly unpairing real numbers, even before the backend code is fixed.
//  2. SHARDING: route each request to the WAHA instance that owns the session;
//     merge GET /api/sessions across instances; send new-session creation to the
//     least-loaded instance under the cap.
//
// Transparent by default: anything not specially handled is proxied verbatim to
// the owning upstream (or the primary), so the blast radius of a bug is minimal.
// Emergency bypass: point the backend's WAHA_API_URL back to http://waha:3000.
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Upstream struct {
	Name  string
	URL   *url.URL
	Proxy *httputil.ReverseProxy
}

var (
	upstreams []*Upstream
	byName    = map[string]*Upstream{}
	apiKey    string
	capN      = 367

	mu        sync.RWMutex
	sessOwner = map[string]string{}
	sessCount = map[string]int{}

	httpc = &http.Client{Timeout: 15 * time.Second}

	// Um refresh custa GET /api/sessions?all=true em TODOS os shards (>1200
	// sessões por resposta). ownerOf() dispara um a cada sessão desconhecida, e
	// um shard doente deixa todas as sessões dele desconhecidas — em 06/08 isso
	// virou ~4 refreshes/s e afogou até o shard que estava são. Estas duas
	// variáveis limitam o refresh sob demanda a um por vez, no máximo a cada
	// refreshMinInterval. O refresh periódico do loop principal não é afetado.
	lastRefreshUnix atomic.Int64
	refreshGate     sync.Mutex
)

const refreshMinInterval = 5 * time.Second

// /api/sessions/{session}[/...] — session-management endpoints (name in path segment 3).
var reSessionPath = regexp.MustCompile(`^/api/sessions/([^/?]+)`)

// /api/{session}/... — session-SCOPED endpoints where the session name is the FIRST
// path segment after /api/ (e.g. /api/{session}/auth/qr, /api/{session}/chats,
// /api/{session}/groups). WAHA exposes ~74 such routes. The first segment is only a
// session name when it is NOT one of the reserved API keywords below and is not a
// send* endpoint (those carry the session in the JSON body instead).
var reApiScoped = regexp.MustCompile(`^/api/([^/?]+)/`)

// ---- Per-session outbound proxy assignment (for new pairings) ----
//
// New sessions land on the least-loaded instance (waha-2 today). WhatsApp
// unlinks freshly-paired devices at a high rate when hundreds of accounts share
// one datacenter IP. To spread new pairings across clean residential/ISP IPs, we
// inject a `config.proxy` into the POST /api/sessions body, round-robin across a
// configured pool. Established sessions on waha-1 are never touched.
//
// Pool via env WAHA_SESSION_PROXIES = comma-separated "server;username;password"
// entries, where server is "host:port" or "scheme://host:port" (scheme optional,
// http assumed by WAHA). Empty/unset => no injection (behaviour unchanged).
type ProxyCfg struct{ Server, Username, Password string }

var (
	sessionProxies []ProxyCfg
	proxyCursor    uint64
)

func loadProxies() {
	raw := os.Getenv("WAHA_SESSION_PROXIES")
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		seg := strings.Split(part, ";")
		if len(seg) < 1 || strings.TrimSpace(seg[0]) == "" {
			continue
		}
		pc := ProxyCfg{Server: strings.TrimSpace(seg[0])}
		if len(seg) >= 2 {
			pc.Username = strings.TrimSpace(seg[1])
		}
		if len(seg) >= 3 {
			pc.Password = strings.TrimSpace(seg[2])
		}
		sessionProxies = append(sessionProxies, pc)
	}
	if len(sessionProxies) > 0 {
		log.Printf("loaded %d session proxies for new-session assignment", len(sessionProxies))
	}
}

// nextProxy returns the next proxy in round-robin order, or nil if none configured.
func nextProxy() *ProxyCfg {
	n := len(sessionProxies)
	if n == 0 {
		return nil
	}
	i := atomic.AddUint64(&proxyCursor, 1) - 1
	p := sessionProxies[int(i%uint64(n))]
	return &p
}

// injectProxy adds config.proxy (round-robin) to a POST /api/sessions body,
// MERGING into any existing config so webhooks/other keys survive. No-op if no
// pool is configured, the body is not a JSON object, or the caller already set a
// proxy. Always restores a readable body + correct Content-Length for proxying.
func injectProxy(r *http.Request) {
	p := nextProxy()
	var body []byte
	if r.Body != nil {
		body, _ = io.ReadAll(r.Body)
		r.Body.Close()
	}
	restore := func(b []byte) {
		r.Body = io.NopCloser(bytes.NewReader(b))
		r.ContentLength = int64(len(b))
	}
	if p == nil {
		restore(body)
		return
	}
	var m map[string]interface{}
	if len(body) == 0 || json.Unmarshal(body, &m) != nil || m == nil {
		restore(body) // not a JSON object we understand: leave untouched
		return
	}
	cfg, _ := m["config"].(map[string]interface{})
	if cfg == nil {
		cfg = map[string]interface{}{}
	}
	if _, exists := cfg["proxy"]; !exists {
		proxy := map[string]interface{}{"server": p.Server}
		if p.Username != "" {
			proxy["username"] = p.Username
		}
		if p.Password != "" {
			proxy["password"] = p.Password
		}
		cfg["proxy"] = proxy
	}
	m["config"] = cfg
	nb, err := json.Marshal(m)
	if err != nil {
		restore(body)
		return
	}
	restore(nb)
	r.Header.Set("Content-Type", "application/json")
	name, _ := m["name"].(string)
	log.Printf("CREATE-PROXY session=%s proxy=%s", name, p.Server) // never log credentials
}

// First-segment keywords under /api/ that are NOT session names. Anything with a
// "send" prefix (sendText, sendImage, send/buttons/reply, …) is also excluded because
// those endpoints put the session in the request body, handled by sessionFromBody.
var reservedFirstSeg = map[string]bool{
	"sessions":          true,
	"server":            true,
	"apps":              true,
	"keys":              true,
	"contacts":          true,
	"messages":          true,
	"reaction":          true,
	"reply":             true,
	"star":              true,
	"screenshot":        true,
	"version":           true,
	"checkNumberStatus": true,
	"forwardMessage":    true,
	"startTyping":       true,
	"stopTyping":        true,
}

// sessionFromPath returns the session name encoded in the URL path, for both the
// /api/sessions/{s} management form and the /api/{session}/... scoped form. Empty
// string means "not a path-scoped request" (caller then tries body/query).
func sessionFromPath(path string) string {
	if m := reSessionPath.FindStringSubmatch(path); m != nil {
		return m[1]
	}
	if m := reApiScoped.FindStringSubmatch(path); m != nil {
		seg := m[1]
		if reservedFirstSeg[seg] || strings.HasPrefix(seg, "send") {
			return ""
		}
		return seg
	}
	return ""
}

func loadUpstreams() {
	raw := os.Getenv("WAHA_UPSTREAMS")
	if raw == "" {
		raw = "waha-1=http://waha:3000"
	}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		u, err := url.Parse(strings.TrimSpace(kv[1]))
		if err != nil {
			log.Fatalf("bad upstream url %q: %v", kv[1], err)
		}
		p := httputil.NewSingleHostReverseProxy(u)
		p.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
			log.Printf("proxy error to %s%s: %v", u, r.URL.Path, e)
			http.Error(w, "upstream error", http.StatusBadGateway)
		}
		up := &Upstream{Name: strings.TrimSpace(kv[0]), URL: u, Proxy: p}
		upstreams = append(upstreams, up)
		byName[up.Name] = up
	}
	if len(upstreams) == 0 {
		log.Fatal("no upstreams configured")
	}
}

func primary() *Upstream { return upstreams[0] }

// statusOf extracts a session's WAHA status ("WORKING", "FAILED", "STARTING",
// "SCAN_QR_CODE", "STOPPED", ...).
func statusOf(s map[string]interface{}) string {
	st, _ := s["status"].(string)
	return st
}

// fetchUptime returns the shard's uptime in ms via GET /api/server/status, or 0
// on any error. Best-effort: the score never depends on it.
func fetchUptime(up *Upstream) float64 {
	req, _ := http.NewRequest("GET", up.URL.String()+"/api/server/status", nil)
	req.Header.Set("X-Api-Key", apiKey)
	resp, err := httpc.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var m map[string]interface{}
	if json.Unmarshal(body, &m) != nil {
		return 0
	}
	if u, ok := m["uptime"].(float64); ok {
		return u
	}
	return 0
}

// refresh polls every upstream for its session list, rebuilding the
// session->owner and owner->count maps (unchanged semantics) AND a richer
// per-shard health snapshot (latency, availability, session-state breakdown,
// uptime, request rate) consumed by the scorer and /router/metrics.
func refresh() {
	owner := map[string]string{}
	count := map[string]int{}
	stats := map[string]ShardHealth{}
	now := time.Now()
	rates := requestRates(now)
	lastRefreshUnix.Store(now.UnixNano())

	// Snapshot do mapeamento atual. Se um shard não responder NESTA rodada, as
	// sessões dele são reaproveitadas de prev (ver keepPrevious): descartá-las
	// as tornaria órfãs, e ownerOf() passaria a mandar o tráfego delas para
	// primary() — que num apagão é justamente o shard doente.
	mu.RLock()
	prev := make(map[string]string, len(sessOwner))
	for n, o := range sessOwner {
		prev[n] = o
	}
	prevCount := make(map[string]int, len(sessCount))
	for n, c := range sessCount {
		prevCount[n] = c
	}
	mu.RUnlock()

	for _, up := range upstreams {
		h := ShardHealth{Name: up.Name, RequestRPS: rates[up.Name]}

		req, _ := http.NewRequest("GET", up.URL.String()+"/api/sessions?all=true", nil)
		req.Header.Set("X-Api-Key", apiKey)
		start := time.Now()
		resp, err := httpc.Do(req)
		h.LatencyMs = float64(time.Since(start).Milliseconds())
		if err != nil {
			log.Printf("refresh %s failed: %v", up.Name, err)
			h.Available = false
			stats[up.Name] = h
			keepPrevious(owner, count, prev, prevCount, up.Name)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var arr []map[string]interface{}
		if json.Unmarshal(body, &arr) != nil {
			h.Available = false
			stats[up.Name] = h
			keepPrevious(owner, count, prev, prevCount, up.Name)
			continue
		}
		h.Available = true
		h.Total = len(arr)
		count[up.Name] = len(arr)
		for _, s := range arr {
			if n, ok := s["name"].(string); ok {
				owner[n] = up.Name
			}
			switch strings.ToUpper(statusOf(s)) {
			case "WORKING":
				h.Connected++
			case "FAILED":
				h.Failed++
			case "STARTING":
				h.Starting++
			case "SCAN_QR_CODE":
				h.Reconnecting++
			case "STOPPED":
				h.Stopped++
			}
		}
		h.UptimeMs = fetchUptime(up)
		stats[up.Name] = h
	}

	// Publish the health snapshot every cycle so scoring/failover always see the
	// latest availability, even during a total outage.
	statsMu.Lock()
	shardStats = stats
	statsMu.Unlock()

	if len(owner) == 0 && len(count) == 0 {
		return // all upstreams unreachable: keep the previous owner/count map
	}
	mu.Lock()
	sessOwner, sessCount = owner, count
	mu.Unlock()
}

// keepPrevious reaproveita o mapeamento já conhecido de um shard que falhou na
// rodada de refresh corrente, para que suas sessões não virem órfãs.
func keepPrevious(owner map[string]string, count map[string]int, prev map[string]string, prevCount map[string]int, name string) {
	for n, o := range prev {
		if o == name {
			owner[n] = o
		}
	}
	if c, ok := prevCount[name]; ok {
		count[name] = c
	}
}

// refreshOnce roda um refresh sob demanda respeitando o intervalo mínimo e
// deixando apenas UM em voo: sem isso, um shard doente faz cada request órfão
// pagar um refresh completo (que ainda espera o timeout do shard doente).
func refreshOnce() {
	if time.Since(time.Unix(0, lastRefreshUnix.Load())) < refreshMinInterval {
		return
	}
	if !refreshGate.TryLock() {
		return
	}
	defer refreshGate.Unlock()
	if time.Since(time.Unix(0, lastRefreshUnix.Load())) < refreshMinInterval {
		return
	}
	refresh()
}

func ownerOf(name string) *Upstream {
	mu.RLock()
	n, ok := sessOwner[name]
	mu.RUnlock()
	if ok {
		if up := byName[n]; up != nil {
			return up
		}
	}
	refreshOnce()
	mu.RLock()
	n, ok = sessOwner[name]
	mu.RUnlock()
	if ok {
		if up := byName[n]; up != nil {
			return up
		}
	}
	return primary()
}

// pickForCreate chooses the healthiest shard for a NEW session using the
// multi-factor health score (memory, CPU, connected sessions, errors, churn,
// latency, activity, capacity). It NEVER moves existing sessions — it only
// decides placement. Saturated/unavailable shards are skipped; if every shard
// is saturated (capacity emergency) it still places on the least-loaded one so a
// pairing is never refused, and logs it for alerting.
func pickForCreate() *Upstream {
	best, emergency := selectBest(snapshot(time.Now()))
	if best == nil {
		log.Printf("CREATE-PICK no shard under cap/available -> primary %s", primary().Name)
		return primary()
	}
	if emergency {
		log.Printf("CREATE-PICK CRIT all shards saturated; least-loaded=%s score=%.3f mem=%.0f util=%.0f%%",
			best.Name, best.Score, best.MemPct, best.utilPct())
	} else {
		log.Printf("CREATE-PICK shard=%s score=%.3f connected=%d mem=%.0f util=%.0f%% rps=%.1f",
			best.Name, best.Score, best.Connected, best.MemPct, best.utilPct(), best.RequestRPS)
	}
	if up := byName[best.Name]; up != nil {
		return up
	}
	return primary()
}

// isPaired reports whether the session has a linked device (me != null).
// On any uncertainty it returns true (fail-safe: prefer blocking a destroy).
func isPaired(up *Upstream, name string) bool {
	req, _ := http.NewRequest("GET", up.URL.String()+"/api/sessions/"+name, nil)
	req.Header.Set("X-Api-Key", apiKey)
	resp, err := httpc.Do(req)
	if err != nil {
		return true
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var s map[string]interface{}
	if json.Unmarshal(body, &s) != nil {
		return true
	}
	me, ok := s["me"]
	return ok && me != nil
}

func sessionFromBody(r *http.Request) (string, []byte) {
	if r.Body == nil {
		return "", nil
	}
	body, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if len(body) == 0 {
		return "", body
	}
	var m map[string]interface{}
	if json.Unmarshal(body, &m) == nil {
		if s, ok := m["session"].(string); ok {
			return s, body
		}
	}
	return "", body
}

func mergeSessions(w http.ResponseWriter, r *http.Request) {
	all := []json.RawMessage{}
	for _, up := range upstreams {
		u := up.URL.String() + "/api/sessions"
		if r.URL.RawQuery != "" {
			u += "?" + r.URL.RawQuery
		}
		req, _ := http.NewRequest("GET", u, nil)
		req.Header.Set("X-Api-Key", apiKey)
		resp, err := httpc.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var arr []json.RawMessage
		if json.Unmarshal(body, &arr) == nil {
			all = append(all, arr...)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	out, _ := json.Marshal(all)
	w.Write(out)
}

func handler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	if path == "/router/health" {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
		return
	}
	// GET /router/metrics -> per-shard health snapshot (observability + alerting).
	if path == "/router/metrics" && r.Method == "GET" {
		handleMetrics(w, r)
		return
	}
	// POST /router/shard-metrics -> ingest pushed mem/CPU from the sentinel.
	if path == "/router/shard-metrics" && r.Method == "POST" {
		handleIngest(w, r)
		return
	}

	// GET /api/sessions (list) -> merge across instances
	if (path == "/api/sessions" || path == "/api/sessions/") && r.Method == "GET" {
		mergeSessions(w, r)
		return
	}
	// POST /api/sessions (create) -> least-loaded instance under cap,
	// with a round-robin outbound proxy injected for the new pairing.
	if path == "/api/sessions" && r.Method == "POST" {
		injectProxy(r)
		serveVia(pickForCreate(), w, r)
		return
	}

	// Resolve the session name so we can route to its owning instance.
	// Order: URL path (management + scoped forms) -> JSON body -> ?session= query.
	sessName := sessionFromPath(path)
	if sessName == "" && (r.Method == "POST" || r.Method == "PUT") {
		s, body := sessionFromBody(r)
		// ALWAYS restore the exact original bytes so the proxied request is identical
		r.Body = io.NopCloser(bytes.NewReader(body))
		r.ContentLength = int64(len(body))
		if s != "" {
			sessName = s
		}
	}
	if sessName == "" {
		if q := r.URL.Query().Get("session"); q != "" {
			sessName = q
		}
	}

	// DELETE-block: only for /api/sessions/{s} (DELETE) and /api/sessions/{s}/logout (POST).
	rest := strings.TrimPrefix(path, "/api/sessions/")
	isDelete := r.Method == "DELETE" && reSessionPath.MatchString(path) && !strings.Contains(rest, "/")
	isLogout := r.Method == "POST" && strings.HasPrefix(path, "/api/sessions/") && strings.HasSuffix(path, "/logout")
	if (isDelete || isLogout) && sessName != "" {
		up := ownerOf(sessName)
		if isPaired(up, sessName) && strings.ToLower(r.Header.Get("X-Confirm-Unpair")) != "true" {
			log.Printf("DELETE-BLOCK method=%s path=%s session=%s remote=%s (paired, no X-Confirm-Unpair)",
				r.Method, path, sessName, r.RemoteAddr)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(403)
			w.Write([]byte(`{"error":"blocked by waha-router: refusing to unpair a paired session without X-Confirm-Unpair:true","session":"` + sessName + `"}`))
			return
		}
		serveVia(up, w, r)
		return
	}

	// default: route to the session's owner, or primary
	if sessName != "" {
		serveVia(ownerOf(sessName), w, r)
	} else {
		serveVia(primary(), w, r)
	}
}

func main() {
	apiKey = os.Getenv("WAHA_API_KEY")
	if v := os.Getenv("WAHA_CAP"); v != "" {
		if n, e := strconv.Atoi(v); e == nil {
			capN = n
		}
	}
	loadUpstreams()
	loadProxies()
	initHealth() // loads Config + wires health scoring (must follow loadUpstreams)
	if cfg.PollTimeout > 0 {
		httpc.Timeout = cfg.PollTimeout
	}
	refresh()
	go func() {
		for {
			time.Sleep(cfg.RefreshInterval)
			refresh()
		}
	}()
	http.HandleFunc("/", handler)
	log.Printf("waha-router on :3000 upstreams=%d cap=%d refresh=%s", len(upstreams), capN, cfg.RefreshInterval)
	srv := &http.Server{Addr: ":3000", ReadHeaderTimeout: 20 * time.Second}
	log.Fatal(srv.ListenAndServe())
}
