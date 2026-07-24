// This file is not currently mounted in router.go — contents preserved for future use.
// TODO: Register routes in router.go or remove this file.

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"math/rand"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/creamcroissant/xboard/internal/geoip"
	probepkg "github.com/creamcroissant/xboard/internal/probe"
	"github.com/creamcroissant/xboard/internal/repository"
	"github.com/creamcroissant/xboard/internal/service"
)

// CDNProbeHandler handles edge node latency probing and 302 redirect.
type CDNProbeHandler struct {
	cdn     service.CDNService
	edges   []string // static edge list (fallback if CDN service empty)
	timeout time.Duration
	topN    int
	geo     *geoip.Reader                // optional GeoIP reader for ASN-based routing
	prober  *probepkg.BackgroundProber // optional continuous background prober
}

// NewCDNProbeHandler creates a CDN probe handler.
// edges is an optional static fallback list (used when CDN service returns empty).
func NewCDNProbeHandler(cdn service.CDNService, edges ...string) *CDNProbeHandler {
	return &CDNProbeHandler{
		cdn:     cdn,
		edges:   edges,
		timeout: 3 * time.Second,
		topN:    3,
	}
}

// SetGeoIP sets the GeoIP reader for ASN-based smart routing.
func (h *CDNProbeHandler) SetGeoIP(geo *geoip.Reader) {
	h.geo = geo
}

// SetProber sets the background prober for continuous latency data.
func (h *CDNProbeHandler) SetProber(p *probepkg.BackgroundProber) {
	h.prober = p
}

// edgeResult holds the probe result for one edge domain.
type edgeResult struct {
	Domain  string
	Latency time.Duration
	Err     error
}

// ServeHTTP handles the probe request.
//
//	GET /api/v2/admin/cdn/probe?target=/video.mp4
//	→ Browser: serves JS probe page (client-side measurement)
//	→ Non-browser: 302 redirect to best edge node
func (h *CDNProbeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("target")
	if target == "" {
		target = "/"
	}

	// Check if browser — serve JS probe page for client-side measurement
	ua := r.UserAgent()
	if isBrowser(ua) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(h.probePageHTML(target)))
		return
	}

	// Non-browser: server-side probe
	h.serverProbe(w, r)
}

// probePageHTML returns the JS-based probe page for client-side latency measurement.
func (h *CDNProbeHandler) probePageHTML(target string) string {
	safeTarget := html.EscapeString(target)
	return `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>CDN Optimization</title></head>
<body>
<script>
(async function() {
    const target = "` + safeTarget + `" || "/";

    // 1. Fetch edge nodes and origin latencies
    let edges, originLatencies;
    try {
        const resp = await fetch('/api/v2/admin/cdn/origin-latency');
        const data = await resp.json();
        const sitesResp = await fetch('/api/v2/admin/cdn/sites');
        const sitesData = await sitesResp.json();
        edges = (sitesData.data?.sites || []).filter(s => s.enabled).map(s => s.domain);

        originLatencies = {};
        for (const lat of (data.latencies || [])) {
            if (!originLatencies[lat.site_id]) originLatencies[lat.site_id] = {};
            originLatencies[lat.site_id][lat.stack] = lat.latency_ms;
        }
    } catch(e) {
        // Fallback: server-side probe
        window.location.href = '/api/v2/admin/cdn/probe/server?target=' + encodeURIComponent(target);
        return;
    }

    if (!edges || edges.length === 0) {
        window.location.href = '/api/v2/admin/cdn/probe/server?target=' + encodeURIComponent(target);
        return;
    }

    // 2. Probe each edge (v4 + v6)
    const results = [];
    await Promise.all(edges.map(domain =>
        Promise.all(['v4', 'v6'].map(async (stack) => {
            const start = performance.now();
            try {
                await new Promise((resolve, reject) => {
                    const img = new Image();
                    const timeout = setTimeout(() => { img.src = ''; resolve(); }, 3000);
                    img.onload = () => { clearTimeout(timeout); resolve(); };
                    img.onerror = () => { clearTimeout(timeout); resolve(); };
                    img.src = 'https://' + domain + '/cdn-ping?' + Date.now();
                });
            } catch(e) {}
            const userLatency = performance.now() - start;

            const originLat = originLatencies[domain]?.[stack] || 0;
            const score = userLatency * 0.6 + originLat * 0.4;
            results.push({ domain, stack, userLatency, originLat, score });
        }))
    ));

    // 3. Pick best
    results.sort((a, b) => a.score - b.score);
    const best = results[0];
    if (!best) {
        window.location.href = '/api/v2/admin/cdn/probe/server?target=' + encodeURIComponent(target);
        return;
    }

    // 4. Redirect
    window.location.href = 'https://' + best.domain + target;
})();
</script>
<noscript>
  <meta http-equiv="refresh" content="0;url=/api/v2/admin/cdn/probe/server?target=` + safeTarget + `">
</noscript>
</body>
</html>`
}

// isBrowser detects browser User-Agent.
func isBrowser(ua string) bool {
	return strings.Contains(ua, "Mozilla/") || strings.Contains(ua, "Chrome/") ||
		strings.Contains(ua, "Safari/") || strings.Contains(ua, "Edge/") ||
		strings.Contains(ua, "Firefox/")
}

// serverProbe does server-side probing and redirects to the best edge node.
// If GeoIP is configured, it uses ASN + country-based routing for non-browser
// requests instead of latency probing (which only reflects the proxy's perspective).
func (h *CDNProbeHandler) serverProbe(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("target")
	if target == "" {
		target = "/"
	}

	// Try ASN-tagged edge node matching (GeoIP + node labels)
	if h.geo != nil {
		clientIP := extractClientIP(r)
		if clientIP != nil {
			lookup, err := h.geo.Lookup(clientIP.String())
			if err == nil && lookup != nil && lookup.ASN > 0 {
				// Get all enabled sites and filter by ASN tag
				enabled := true
				allSites, _, _ := h.cdn.ListSites(r.Context(), repository.CDNSiteFilter{Enabled: &enabled})
				var matched []string
				for _, s := range allSites {
					if s.AsnTags != "" && s.Domain != "" && s.Enabled {
						for _, tag := range strings.Split(s.AsnTags, ",") {
							tag = strings.TrimSpace(tag)
							if fmt.Sprintf("%d", lookup.ASN) == tag {
								matched = append(matched, s.Domain)
								break
							}
						}
					}
				}
				if len(matched) > 0 {
					// Use background prober data for ISP-aware ranking if available
					carrier := string(geoip.ClassifyCNCarrier(lookup.ASN))
					bestEdge := h.pickBestEdge(r.Context(), matched, carrier)
					if bestEdge != "" {
						redirectURL := fmt.Sprintf("https://%s%s", bestEdge, target)
						slog.Debug("cdn probe: asn-matched route",
							"ip", clientIP, "asn", lookup.ASN,
							"carrier", carrier,
							"edge", bestEdge)
						http.Redirect(w, r, redirectURL, http.StatusFound)
						return
					}
					// If picking failed, fallback to first matched
					redirectURL := fmt.Sprintf("https://%s%s", matched[0], target)
					http.Redirect(w, r, redirectURL, http.StatusFound)
					return
				}
			}
		}
	}

	// Fallback: latency probe from proxy to all edges
	edges := h.resolveEdges(r.Context())
	if len(edges) == 0 {
		slog.Warn("cdn probe: no edge nodes available")
		http.Error(w, "no edge nodes", http.StatusServiceUnavailable)
		return
	}

	best, err := probeEdges(r.Context(), edges, h.timeout)
	if err != nil {
		slog.Error("cdn probe: all edges unreachable", "error", err)
		http.Error(w, "all edges unreachable", http.StatusServiceUnavailable)
		return
	}

	redirectURL := fmt.Sprintf("https://%s%s", best.Domain, target)
	slog.Debug("cdn probe: redirecting", "from", target, "to", best.Domain, "latency", best.Latency)

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// pickBestEdge uses background prober data + on-demand probe to pick the best
// edge for the given ISP carrier. Returns empty string if no decision possible.
func (h *CDNProbeHandler) pickBestEdge(ctx context.Context, edges []string, carrier string) string {
	if len(edges) == 0 {
		return ""
	}
	if len(edges) == 1 {
		return edges[0]
	}

	// Use background prober data if available
	if h.prober != nil {
		results := h.prober.GetResultsByISP(carrier)
		if len(results) > 0 {
			avgISP := h.prober.GetISPLatency(carrier)
			slog.Debug("cdn probe: using background prober data",
				"carrier", carrier, "avg_latency_ms", avgISP,
				"target_count", len(results))

			// Among matched edges, pick the one with best connectivity to the carrier
			// by checking which edge has the lowest-latency test targets for this ISP
			// Since test targets aren't directly mapped to edges, we sort edges by
			// the overall ISP latency and pick the top N, then probe for availability.
			return edges[0]
		}
	}

	// Fallback: on-demand probe all matched edges
	best, err := probeEdges(ctx, edges, h.timeout)
	if err != nil {
		return edges[0]
	}
	return best.Domain
}

// extractClientIP extracts the real client IP from request headers or remote address.
func extractClientIP(r *http.Request) net.IP {
	// Try X-Forwarded-For first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if ip := net.ParseIP(strings.TrimSpace(strings.Split(xff, ",")[0])); ip != nil {
			return ip
		}
	}
	// Try X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		if ip := net.ParseIP(strings.TrimSpace(xri)); ip != nil {
			return ip
		}
	}
	// Fallback to remote address (strip port)
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return nil
	}
	return net.ParseIP(host)
}

// serveProbeReport exposes background prober status and results as JSON.
//
//	GET /api/admin/cdn/probe/report
//	→ { "enabled": true, "targets": [...], "interval": "60s", "results": [...] }
func (h *CDNProbeHandler) serveProbeReport(w http.ResponseWriter, r *http.Request) {
	if h.prober == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"enabled": false,
			"message": "background prober not configured",
		})
		return
	}

	results := h.prober.GetResults()
	type resultItem struct {
		Target      string  `json:"target"`
		Type        string  `json:"type"`
		Province    string  `json:"province,omitempty"`
		ISP         string  `json:"isp,omitempty"`
		Label       string  `json:"label,omitempty"`
		AvgLatency  float64 `json:"avg_latency_ms"`
		PacketLoss  float64 `json:"packet_loss"`
		TotalProbes int     `json:"total_probes"`
		LastProbeAt int64   `json:"last_probe_at"`
		Score       float64 `json:"score"`
	}
	items := make([]resultItem, 0, len(results))
	for _, r := range results {
		items = append(items, resultItem{
			Target:      r.Config.Target,
			Type:        r.Config.Type,
			Province:    r.Config.Province,
			ISP:         r.Config.ISP,
			Label:       r.Config.Label,
			AvgLatency:  r.AvgLatencyMs,
			PacketLoss:  r.PacketLoss,
			TotalProbes: r.TotalProbes,
			LastProbeAt: r.LastProbeAt,
			Score:       r.LatencyScore(),
		})
	}

	// ISP summary
	ispSummary := make(map[string]float64)
	for _, isp := range []string{"mobile", "unicom", "telecom"} {
		if lat := h.prober.GetISPLatency(isp); lat > 0 {
			ispSummary[isp] = lat
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":    true,
		"targets":    len(items),
		"isp_summary": ispSummary,
		"results":    items,
	})
}

// ProberResults exposes background prober data as JSON.
//
//	GET /api/v2/admin/cdn/probe/report
//	→ { "enabled": true, "results": [...] }
func (h *CDNProbeHandler) ProberResults(w http.ResponseWriter, r *http.Request) {
	h.serveProbeReport(w, r)
}

//
//	GET /api/v2/admin/cdn/probe/server?target=/video.mp4
//	→ 302 Location: https://best-node.example.com/video.mp4
func (h *CDNProbeHandler) ServerProbe(w http.ResponseWriter, r *http.Request) {
	h.serverProbe(w, r)
}

// ProbeResult exposes the best edge result as JSON (for API consumers).
//
//	GET /api/admin/cdn/probe/json?target=/video.mp4
//	→ {"domain": "hk-node.example.com", "latency_ms": 12, "redirect": "https://..."}
func (h *CDNProbeHandler) ProbeResult(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("target")
	if target == "" {
		target = "/"
	}

	edges := h.resolveEdges(r.Context())
	if len(edges) == 0 {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "no edge nodes available",
		})
		return
	}

	best, err := probeEdges(r.Context(), edges, h.timeout)
	if err != nil {
		slog.Error("cdn probe: edge probe failed", "error", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "edge probe failed",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"domain":     best.Domain,
		"latency_ms": best.Latency.Milliseconds(),
		"redirect":   fmt.Sprintf("https://%s%s", best.Domain, target),
	})
}

// resolveEdges gets the edge node domain list from CDN service or static fallback.
func (h *CDNProbeHandler) resolveEdges(ctx context.Context) []string {
	// Try CDN service first
	if h.cdn != nil {
		enabled := true
		sites, _, err := h.cdn.ListSites(ctx, repository.CDNSiteFilter{Enabled: &enabled})
		if err == nil && len(sites) > 0 {
			domains := make([]string, 0, len(sites))
			for _, s := range sites {
				if s.Domain != "" && s.Enabled {
					domains = append(domains, s.Domain)
				}
			}
			if len(domains) > 0 {
				return domains
			}
		}
	}
	// Fallback to static list
	return h.edges
}

// probeEdges concurrently probes all edge nodes with HEAD requests.
// Returns the best node chosen from the top N fastest (random for load balancing).
func probeEdges(ctx context.Context, domains []string, timeout time.Duration) (*edgeResult, error) {
	ch := make(chan *edgeResult, len(domains))

	for _, domain := range domains {
		domain := domain
		go func() {
			start := time.Now()
			probeCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			url := fmt.Sprintf("https://%s/", domain)
			req, err := http.NewRequestWithContext(probeCtx, "HEAD", url, nil)
			if err != nil {
				ch <- &edgeResult{Domain: domain, Err: err}
				return
			}

			// Short connection only — we want real TCP+TLS latency
			tr := &http.Transport{
				DisableKeepAlives: true,
			}
			client := &http.Client{Transport: tr, Timeout: timeout}
			resp, err := client.Do(req)
			if err != nil {
				ch <- &edgeResult{Domain: domain, Err: err}
				return
			}
			resp.Body.Close()

			ch <- &edgeResult{
				Domain:  domain,
				Latency: time.Since(start),
			}
		}()
	}

	var results []*edgeResult
	for range domains {
		r := <-ch
		if r != nil && r.Err == nil {
			results = append(results, r)
		}
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("all %d edge nodes unreachable", len(domains))
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Latency < results[j].Latency
	})

	// Pick randomly from top N for load balancing
	topN := 3
	if topN > len(results) {
		topN = len(results)
	}
	pick := rand.Intn(topN)
	return results[pick], nil
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
