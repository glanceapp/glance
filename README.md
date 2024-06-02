<p align="center"><em>What if you could see everything at a...</em></p>
<h1 align="center">Glance</h1>
<p align="center"><a href="#installation">Install</a> • <a href="docs/configuration.md">Configuration</a> • <a href="docs/themes.md">Themes</a></p>

![example homepage](docs/images/readme-main-image.png)

### Features
#### Various widgets
* RSS feeds
* Subreddit posts
* Weather
* Bookmarks
* Hacker News
* Lobsters
* Latest YouTube videos from specific channels
* Clock
* Calendar
* Stocks
* iframe
* Twitch channels & top games
* GitHub releases
* Repository overview
* Site monitor
* Search box

#### Themeable
![multiple color schemes example](docs/images/themes-example.png)

#### Optimized for mobile devices
![mobile device previews](docs/images/mobile-preview.png)

#### Fast and lightweight
* Minimal JS, no bloated frameworks
* Very few dependencies
* Single, easily distributed <15mb binary and just as small docker container
* All requests are parallelized, uncached pages usually load within ~1s (depending on internet speed and number of widgets)

### Configuration
Checkout the [configuration docs](docs/configuration.md) to learn more. A [preconfigured page](docs/configuration.md#preconfigured-page) is also available to get you started quickly.

### Installation
> [!CAUTION]
>
> The project is under active development, expect things to break every once in a while.

#### Manual
Checkout the [releases page](https://github.com/glanceapp/glance/releases) for available binaries. You can place the binary inside `/opt/glance/` and have it start with your server via a [systemd service](https://linuxhandbook.com/create-systemd-services/). To specify a different path for the config file use the `--config` option:

```bash
/opt/glance/glance --config /etc/glance.yml
```

#### Docker
> [!IMPORTANT]
>
> Make sure you have a valid `glance.yml` file in the same directory before running the container.

```bash
docker run -d -p 8080:8080 \
  -v ./glance.yml:/app/glance.yml \
  -v /etc/timezone:/etc/timezone:ro \
  -v /etc/localtime:/etc/localtime:ro \
  glanceapp/glance
```

Or if you prefer docker compose:

```yaml
services:
  glance:
    image: glanceapp/glance
    volumes:
      - ./glance.yml:/app/glance.yml
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - 8080:8080
    restart: unless-stopped
```

### Building from source

Requirements: [Go](https://go.dev/dl/) >= v1.22

To build:

```bash
go build -o build/glance .
```

To run:

```bash
go run .
```

### Building Docker image

Build the image:

**Make sure to replace "owner" with your name or organization.**

```bash
docker build -t owner/glance:latest -f Dockerfile.single-platform .
```

Push the image to your registry:

```bash
docker push owner/glance:latest
```
