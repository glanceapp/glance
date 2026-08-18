# Custom Glance Widgets Collection

This directory contains production-ready, dynamic `custom-api` widgets designed for Glance dashboards.

---

## 📦 Included Widgets

| Widget File | Type | Description & Key Features |
| :--- | :--- | :--- |
| **`truenas-metrics.yml`** | `custom-api` | System load average, live uptime, active TrueNAS alerts counter, CPU model |
| **`jellyfin-stats.yml`** | `custom-api` | Live active playback streams (turns green when active), total movies & series count |
| **`immich-stats.yml`** | `custom-api` | Total photo count, video count, and calculated GB storage usage |
| **`radarr-stats.yml`** | `custom-api` | Total movies in library, missing movies count, active download queue |
| **`sonarr-stats.yml`** | `custom-api` | Total series in library, missing episodes count, active download queue |
| **`cloudflare-tunnel.yml`** | `custom-api` | Real-time connector tunnel health (`healthy`) and public origin IP address |
| **`tailscale.yml`** | `custom-api` | Tailnet machine IP, relative last-seen timestamp, key expiry countdown |
| **`netbird.yml`** | `custom-api` | Real-time total connected network peer count |

---

## 🔑 1. Environment Variables Setup

Add the required API keys and IDs to your Glance environment (e.g. `docker-compose.yml` or `.env` file):

```env
# TrueNAS
TRUENAS_API_KEY=your_truenas_api_key

# Media Services
JELLYFIN_API_KEY=your_jellyfin_api_key
IMMICH_API_KEY=your_immich_api_key
RADARR_API_KEY=your_radarr_api_key
SONARR_API_KEY=your_sonarr_api_key

# Cloudflare Tunnel
CF_ACCOUNT_ID=your_cloudflare_account_id
CF_TUNNEL_ID=your_cloudflare_tunnel_id
CF_API_TOKEN=your_cloudflare_api_token

# Tailscale & Netbird
TAILSCALE_TAILNET=your_tailnet_name
TAILSCALE_API_KEY=your_tailscale_api_key
NETBIRD_API_KEY=your_netbird_api_key
```

---

## 🛠️ 2. How to Use in Your Glance Configuration

### Method A: Direct Embedding (Copy & Paste)

Copy the widget definition directly into the `widgets:` section of any column in your page YAML file (e.g. `pages/home.yml`):

```yaml
- name: Home
  columns:
    - size: full
      widgets:
        # Paste the widget snippet here
        - type: custom-api
          refresh-interval: 5s
          title: Jellyfin Stats
          cache: 2s
          url: http://<JELLYFIN_IP>:8096/Sessions?api_key=${JELLYFIN_API_KEY}
          subrequests:
            counts:
              url: http://<JELLYFIN_IP>:8096/Items/Counts?api_key=${JELLYFIN_API_KEY}
          template: |
            {{- $activeStreams := 0 -}}
            {{- range .JSON.Array "" -}}
              {{- if .Exists "NowPlayingItem" -}}
                {{- $activeStreams = add $activeStreams 1 -}}
              {{- end -}}
            {{- end -}}
            <div class="flex justify-between text-center">
              <div>
                <div class="{{ if gt $activeStreams 0 }}color-positive{{ else }}color-highlight{{ end }} size-h3">{{ $activeStreams }}</div>
                <div class="size-h6{{ if gt $activeStreams 0 }} color-positive{{ end }}">SESSIONS</div>
              </div>
              <div>
                <div class="color-highlight size-h3">{{ (.Subrequest "counts").JSON.Int "MovieCount" }}</div>
                <div class="size-h6">MOVIES</div>
              </div>
              <div>
                <div class="color-highlight size-h3">{{ (.Subrequest "counts").JSON.Int "SeriesCount" }}</div>
                <div class="size-h6">SERIES</div>
              </div>
            </div>
```

---

### Method B: Modular File Inclusion (`$include`)

You can keep your page YAML files clean by using Glance's `$include` directive:

```yaml
- name: Home
  columns:
    - size: small
      widgets:
        - $include: custom-widgets/truenas-metrics.yml
        - $include: custom-widgets/cloudflare-tunnel.yml
        - $include: custom-widgets/tailscale.yml
        - $include: custom-widgets/netbird.yml
```

---

## 📐 3. Layout Examples

### Example 1: 2x2 Media Stats Grid
Arrange 4 media stats widgets side-by-side using `split-column` with `max-columns: 2`:

```yaml
- type: split-column
  max-columns: 2
  widgets:
    - $include: custom-widgets/jellyfin-stats.yml
    - $include: custom-widgets/immich-stats.yml
    - $include: custom-widgets/radarr-stats.yml
    - $include: custom-widgets/sonarr-stats.yml
```

### Example 2: 3-Across Network Tunnels Row
Place Cloudflare Tunnel, Tailscale, and Netbird in a 3-column row:

```yaml
- type: split-column
  max-columns: 3
  widgets:
    - $include: custom-widgets/cloudflare-tunnel.yml
    - $include: custom-widgets/tailscale.yml
    - $include: custom-widgets/netbird.yml
```

### Example 3: Compact Sidebar Column
Stack system metrics and network health vertically in a `size: small` sidebar:

```yaml
- size: small
  widgets:
    - type: server-stats
      title: Host Server
      servers:
        - type: local
          name: TrueNAS
    - $include: custom-widgets/truenas-metrics.yml
    - $include: custom-widgets/cloudflare-tunnel.yml
    - $include: custom-widgets/tailscale.yml
    - $include: custom-widgets/netbird.yml
```

---

## ⚙️ Configuration Notes

1. **IP Addresses & Ports:** Replace placeholder IPs (e.g. `<TRUENAS_IP>`, `<JELLYFIN_IP>`) with your actual host IP addresses and ports.
2. **Tailscale Device Filtering:** In `tailscale.yml`, replace `<TAILSCALE_DEVICE_IP>` with your target device's Tailscale 100.x.y.z IP address.
3. **Live Refresh Rates:** All widgets use `refresh-interval: 5s` and `cache: 2s` for low-latency live updates without overloading your APIs.
