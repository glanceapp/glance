# Preconfigured pages

Don't want to spend time configuring pages from scratch? No problem! Simply copy the config from the ones below.

Pull requests with your page configurations are welcome!

> [!NOTE]
>
> Pages must be placed under a top level `pages:` key, you can read more about that [here](configuration.md#pages).

## Startpage

![](images/startpage-preview.png)

<details>
<summary>View config (requires Glance <code>v0.6.0</code> or higher)</summary>

```yaml
- name: Startpage
  width: slim
  hide-desktop-navigation: true
  center-vertically: true
  columns:
    - size: full
      widgets:
        - type: search
          autofocus: true

        - type: monitor
          cache: 1m
          title: Services
          sites:
            - title: Jellyfin
              url: https://yourdomain.com/
              icon: si:jellyfin
            - title: Gitea
              url: https://yourdomain.com/
              icon: si:gitea
            - title: qBittorrent # only for Linux ISOs, of course
              url: https://yourdomain.com/
              icon: si:qbittorrent
            - title: Immich
              url: https://yourdomain.com/
              icon: si:immich
            - title: AdGuard Home
              url: https://yourdomain.com/
              icon: si:adguard
            - title: Vaultwarden
              url: https://yourdomain.com/
              icon: si:vaultwarden

        - type: bookmarks
          groups:
            - title: General
              links:
                - title: Gmail
                  url: https://mail.google.com/mail/u/0/
                - title: Amazon
                  url: https://www.amazon.com/
                - title: Github
                  url: https://github.com/
            - title: Entertainment
              links:
                - title: YouTube
                  url: https://www.youtube.com/
                - title: Prime Video
                  url: https://www.primevideo.com/
                - title: Disney+
                  url: https://www.disneyplus.com/
            - title: Social
              links:
                - title: Reddit
                  url: https://www.reddit.com/
                - title: Twitter
                  url: https://twitter.com/
                - title: Instagram
                  url: https://www.instagram.com/
```
</details>

## Markets

![](images/markets-page-preview.png)

<details>
<summary>View config (requires Glance <code>v0.6.0</code> or higher)</summary>

```yaml
- name: Markets
  columns:
    - size: small
      widgets:
        - type: markets
          title: Indices
          markets:
            - symbol: SPY
              name: S&P 500
            - symbol: DX-Y.NYB
              name: Dollar Index

        - type: markets
          title: Crypto
          markets:
            - symbol: BTC-USD
              name: Bitcoin
            - symbol: ETH-USD
              name: Ethereum

        - type: markets
          title: Stocks
          sort-by: absolute-change
          markets:
            - symbol: NVDA
              name: NVIDIA
            - symbol: AAPL
              name: Apple
            - symbol: MSFT
              name: Microsoft
            - symbol: GOOGL
              name: Google
            - symbol: AMD
              name: AMD
            - symbol: RDDT
              name: Reddit
            - symbol: AMZN
              name: Amazon
            - symbol: TSLA
              name: Tesla
            - symbol: INTC
              name: Intel
            - symbol: META
              name: Meta

    - size: full
      widgets:
        - type: rss
          title: News
          style: horizontal-cards
          feeds:
            - url: https://feeds.bloomberg.com/markets/news.rss
              title: Bloomberg
            - url: https://moxie.foxbusiness.com/google-publisher/markets.xml
              title: Fox Business
            - url: https://moxie.foxbusiness.com/google-publisher/technology.xml
              title: Fox Business

        - type: group
          widgets:
            - type: reddit
              show-thumbnails: true
              subreddit: technology
            - type: reddit
              show-thumbnails: true
              subreddit: wallstreetbets

        - type: videos
          style: grid-cards
          collapse-after-rows: 3
          channels:
            - UCvSXMi2LebwJEM1s4bz5IBA # New Money
            - UCV6KDgJskWaEckne5aPA0aQ # Graham Stephan
            - UCAzhpt9DmG6PnHXjmJTvRGQ # Federal Reserve

    - size: small
      widgets:
        - type: rss
          title: News
          limit: 30
          collapse-after: 13
          feeds:
            - url: https://www.ft.com/technology?format=rss
              title: Financial Times
            - url: https://feeds.a.dj.com/rss/RSSMarketsMain.xml
              title: Wall Street Journal
```
</details>

## Gaming

![](images/gaming-page-preview.png)

<details>
<summary>View config (requires Glance <code>v0.6.0</code> or higher)</summary>

```yaml
- name: Gaming
  columns:
    - size: small
      widgets:
        - type: twitch-top-games
          limit: 20
          collapse-after: 13
          exclude:
            - just-chatting
            - pools-hot-tubs-and-beaches
            - music
            - art
            - asmr

    - size: full
      widgets:
        - type: group
          widgets:
            - type: reddit
              show-thumbnails: true
              subreddit: pcgaming
            - type: reddit
              subreddit: games

        - type: videos
          style: grid-cards
          collapse-after-rows: 3
          channels:
            - UCNvzD7Z-g64bPXxGzaQaa4g # gameranx
            - UCZ7AeeVbyslLM_8-nVy2B8Q # Skill Up
            - UCHDxYLv8iovIbhrfl16CNyg # GameLinked
            - UC9PBzalIcEQCsiIkq36PyUA # Digital Foundry

    - size: small
      widgets:
        - type: reddit
          subreddit: gamingnews
          limit: 7
          style: vertical-cards
```
</details>

## Cloud & SaaS status

![](images/cloud-saas-status-page-preview.png)

<details>
<summary>View config (requires Glance <code>v0.7.0</code> or higher)</summary>

This page uses OutageDeck's keyless public API. Six widgets refreshed every 10 minutes
use 36 of the anonymous API's 120 requests per hour. Swap any provider slug in both
the `url` and `title-url` fields to monitor a different dependency.

```yaml
define:
  - &provider-status
    type: custom-api
    cache: 10m
    template: |
      {{ $status := .JSON.String "data.currentStatus.code" }}
      <div class="flex justify-between items-center">
        <div class="size-h3 {{ if eq $status "operational" }}color-positive{{ else if or (eq $status "maintenance") (eq $status "unknown") }}color-primary{{ else }}color-negative{{ end }}">
          {{ .JSON.String "data.currentStatus.label" }}
        </div>
        <div class="size-h5 color-subdue">
          {{ .JSON.Int "data.counts.activeIncidents" }} active incidents
        </div>
      </div>
      <p class="margin-top-10">{{ .JSON.String "data.currentStatus.headline" }}</p>
      <ul class="list list-gap-4 margin-top-10">
      {{ range .JSON.Array "data.services" }}
        {{ $serviceStatus := .String "status" }}
        <li class="flex justify-between">
          <span>{{ .String "name" }}</span>
          <span class="{{ if eq $serviceStatus "operational" }}color-positive{{ else if or (eq $serviceStatus "maintenance") (eq $serviceStatus "unknown") }}color-primary{{ else }}color-negative{{ end }}">
            {{ if eq $serviceStatus "operational" }}Operational{{ else if eq $serviceStatus "degraded" }}Degraded{{ else if eq $serviceStatus "partial_outage" }}Partial outage{{ else if eq $serviceStatus "major_outage" }}Major outage{{ else if eq $serviceStatus "maintenance" }}Maintenance{{ else }}Unknown{{ end }}
          </span>
        </li>
      {{ end }}
      </ul>
      <p class="margin-top-10 size-h5 color-subdue">
        Source checked <span {{ .JSON.String "data.source.checkedAt" | parseTime "rfc3339" | toRelativeTime }}></span>
      </p>

pages:
  - name: Cloud status
    width: wide
    columns:
      - size: full
        widgets:
          - type: split-column
            max-columns: 3
            widgets:
              - title: AWS
                title-url: https://outagedeck.com/providers/aws?utm_source=glance&utm_medium=dashboard&utm_campaign=glance_status_dashboard
                url: https://outagedeck.com/api/v1/providers/aws
                <<: *provider-status

              - title: Cloudflare
                title-url: https://outagedeck.com/providers/cloudflare?utm_source=glance&utm_medium=dashboard&utm_campaign=glance_status_dashboard
                url: https://outagedeck.com/api/v1/providers/cloudflare
                <<: *provider-status

              - title: GitHub
                title-url: https://outagedeck.com/providers/github?utm_source=glance&utm_medium=dashboard&utm_campaign=glance_status_dashboard
                url: https://outagedeck.com/api/v1/providers/github
                <<: *provider-status

              - title: OpenAI
                title-url: https://outagedeck.com/providers/openai?utm_source=glance&utm_medium=dashboard&utm_campaign=glance_status_dashboard
                url: https://outagedeck.com/api/v1/providers/openai
                <<: *provider-status

              - title: Anthropic
                title-url: https://outagedeck.com/providers/anthropic?utm_source=glance&utm_medium=dashboard&utm_campaign=glance_status_dashboard
                url: https://outagedeck.com/api/v1/providers/anthropic
                <<: *provider-status

              - title: Google Cloud
                title-url: https://outagedeck.com/providers/google-cloud?utm_source=glance&utm_medium=dashboard&utm_campaign=glance_status_dashboard
                url: https://outagedeck.com/api/v1/providers/google-cloud
                <<: *provider-status
```
</details>
