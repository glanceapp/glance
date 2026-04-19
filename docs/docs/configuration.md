# Configuring Dynacat
## Preconfigured page
If you don't want to spend time reading through all the available configuration options and just want something to get you going quickly you can use [this `dynacat.yml` file](dynacat.yml) and make changes to it as you see fit. It will give you a page that looks like the following:

![](images/preconfigured-page-preview.png)

Configure the widgets, add more of them, add extra pages, etc. Make it your own!

## The config file

### Auto reload
Automatic config reload is supported, meaning that you can make changes to the config file and have them take effect on save without having to restart the container/service. Making changes to environment variables does not trigger a reload and requires manual restart. Deleting a config file will stop that file from being watched, even if it is recreated.

> [!NOTE]
>
> If you attempt to start Dynacat with an invalid config it will exit with an error outright. If you successfully started Dynacat with a valid config and then made changes to it which result in an error, you'll see that error in the console and Dynacat will continue to run with the old configuration. You can then continue to make changes and when there are no errors the new configuration will be loaded.

> [!CAUTION]
>
> Reloading the configuration file clears your cached data, meaning that you have to request the data anew each time you do this. This can lead to rate limiting for some APIs if you do it too frequently. Having a cache that persists between reloads will be added in the future.

### Environment variables
Inserting environment variables is supported anywhere in the config. This is done via the `${ENV_VAR}` syntax. Attempting to use an environment variable that doesn't exist will result in an error and Dynacat will either not start or load your new config on save. Example:

```yaml
server:
  host: ${HOST}
  port: ${PORT}
```

Can also be in the middle of a string:

```yaml
- type: rss
  title: ${RSS_TITLE}
  feeds:
    - url: http://domain.com/rss/${RSS_CATEGORY}.xml
```

Works with any type of value, not just strings:

```yaml
- type: rss
  limit: ${RSS_LIMIT}
```

If you need to use the syntax `${NAME}` in your config without it being interpreted as an environment variable, you can escape it by prefixing with a backslash `\`:

```yaml
something: \${NOT_AN_ENV_VAR}
```

#### Other ways of providing tokens/passwords/secrets

You can use [Docker secrets](https://docs.docker.com/compose/how-tos/use-secrets/) with the following syntax:

```yaml
# This will be replaced with the contents of the file /run/secrets/github_token
# so long as the secret `github_token` is provided to the container
token: ${secret:github_token}
```

Alternatively, you can load the contents of a file who's path is provided by an environment variable:

`docker-compose.yml`
```yaml
services:
  dynacat:
    image: Panonim/dynacat
    environment:
      - TOKEN_FILE=/home/user/token
    volumes:
      - /home/user/token:/home/user/token
```

`dynacat.yml`
```yaml
token: ${readFileFromEnv:TOKEN_FILE}
```

> [!NOTE]
>
> The contents of the file will be stripped of any leading/trailing whitespace before being used.

### Including other config files
Including config files from within your main config file is supported. This is done via the `$include` directive along with a relative or absolute path to the file you want to include. If the path is relative, it will be relative to the main config file. Additionally, environment variables can be used within included files, and changes to the included files will trigger an automatic reload. Example:

```yaml
pages:
  - $include: home.yml
  - $include: videos.yml
  - $include: homelab.yml
```

The file you are including should not have any additional indentation, its values should be at the top level and the appropriate amount of indentation will be added automatically depending on where the file is included. Example:

`dynacat.yml`

```yaml
pages:
  - name: Home
    columns:
      - size: full
        widgets:
          - $include: rss.yml
  - name: News
    columns:
      - size: full
        widgets:
          - type: group
            widgets:
              - $include: rss.yml
              - type: reddit
                subreddit: news
```

`rss.yml`

```yaml
- type: rss
  title: News
  feeds:
    - url: ${RSS_URL}
```

The `$include` directive can be used anywhere in the config file, not just in the `pages` property, however it must be on its own line and have the appropriate indentation.

If you encounter YAML parsing errors when using the `$include` directive, the reported line numbers will likely be incorrect. This is because the inclusion of files is done before the YAML is parsed, as YAML itself does not support file inclusion. To help with debugging in cases like this, you can use the `config:print` command and pipe it into `less -N` to see the full config file with includes resolved and line numbers added:

```sh
dynacat --config /path/to/dynacat.yml config:print | less -N
```

This is a bit more convoluted when running Dynacat inside a Docker container:

```sh
docker run --rm -v ./dynacat.yml:/app/config/dynacat.yml Panonim/dynacat config:print | less -N
```

This assumes that the config you want to print is in your current working directory and is named `dynacat.yml`.

## Icons

For widgets which provide you with the ability to specify icons such as the monitor, bookmarks, docker containers, etc, you can use the `icon` property to specify a URL to an image or use icon names from multiple libraries via prefixes:

```yml
icon: si:immich # si for Simple icons https://simpleicons.org/
icon: sh:immich # sh for selfh.st icons https://selfh.st/icons/
icon: di:immich # di for Dashboard icons https://github.com/homarr-labs/dashboard-icons
icon: mdi:camera # mdi for Material Design icons https://pictogrammers.com/library/mdi/
```

You can also add an icon next to a widget title using `title-icon`:

```yaml
- type: custom-api
  title: Todoist
  title-icon: di:todoist
```

If you self-host icons through your configured `assets-path`, both `icon` and `title-icon` support that as well:

```yaml
icon: /assets/icons/todoist.png
title-icon: /assets/icons/todoist.png
```

> [!NOTE]
>
> The icons are loaded externally and are hosted on `cdn.jsdelivr.net`, if you do not wish to depend on a 3rd party you are free to download the icons individually and host them locally.

Icons from the Simple icons library as well as Material Design icons will automatically invert their color to match your light or dark theme, however you may want to enable this manually for other icons. To do this, you can use the `auto-invert` prefix:

```yaml
icon: auto-invert https://example.com/path/to/icon.png # with a URL
icon: auto-invert sh:dynacat-dark # with a selfh.st icon
```

This expects the icon to be black and will automatically invert it to white when using a dark theme.

The same icon syntax and prefixes also work for `title-icon`.

If an icon URL cannot be loaded (for example, the file does not exist or the host is unreachable), Dynacat will hide the icon and render the widget as if no icon was configured.

## Config schema

For property descriptions, validation and autocompletion of the config within your IDE, @not-first has kindly created a [schema](https://github.com/not-first/dynacat-schema). Massive thanks to them for this, go check it out and give them a star!

## Server
Server configuration is done through a top level `server` property. Example:

```yaml
server:
  port: 8080
  assets-path: /home/user/dynacat-assets
```

### Properties

| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| host | string | no |  |
| port | number | no | 8080 |
| proxied | boolean | no | false |
| base-url | string | no | |
| assets-path | string | no | /app/assets |
| cache-dir | string | no | .cache |
| db-path | string | no | /app/assets/dynacat.db |

#### `host`
The address which the server will listen on. Setting it to `localhost` means that only the machine that the server is running on will be able to access the dashboard. By default it will listen on all interfaces.

#### `port`
A number between 1 and 65,535, so long as that port isn't already used by anything else.

#### `proxied`
Set to `true` if you're using a reverse proxy in front of Dynacat. This will make Dynacat use the `X-Forwarded-*` headers to determine the original request details.

#### `base-url`
The base URL that Dynacat is hosted under. No need to specify this unless you're using a reverse proxy and are hosting Dynacat under a directory. If that's the case then you can set this value to `/dynacat` or whatever the directory is called. Note that the forward slash (`/`) in the beginning is required unless you specify the full domain and path.

> [!IMPORTANT]
> You need to strip the `base-url` prefix before forwarding the request to the Dynacat server.
> In Caddy you can do this using [`handle_path`](https://caddyserver.com/docs/caddyfile/directives/handle_path) or [`uri strip_prefix`](https://caddyserver.com/docs/caddyfile/directives/uri).

#### `assets-path`
The path to a directory that will be served by the server under the `/assets/` path. This is handy for widgets like the Monitor where you have to specify an icon URL and you want to self host all the icons rather than pointing to an external source. Defaults to `/app/assets`.

> [!IMPORTANT]
>
> When installing through docker the path will point to the files inside the container. Don't forget to mount your assets path to the same path inside the container.
> Example:
>
> If your assets are in:
> ```
> /home/user/dynacat-assets
> ```
>
> You should mount:
> ```
> /home/user/dynacat-assets:/app/assets
> ```
>
> And your config should contain:
> ```
> assets-path: /app/assets
> ```

##### Examples

Say you have a directory `dynacat-assets` with a file `gitea-icon.png` in it and you specify your assets path like:

```yaml
assets-path: /home/user/dynacat-assets
```

To be able to point to an asset from your assets path, use the `/assets/` path like such:

```yaml
icon: /assets/gitea-icon.png
```

#### `cache-dir`
Directory where Dynacat stores cached remote images (for example, widget icons). Cached files are served from `/.cache/` with long cache headers so browsers reuse them without refetching from the original host.

If the path is relative, it will be resolved relative to the Dynacat working directory. The directory will be created if it does not exist.

#### `db-path`
Path to the SQLite database file used for server-side todo storage. Only required when at least one `to-do` widget has `storage: server` set. If the path is relative, it will be resolved relative to the Dynacat working directory. The file will be created if it does not exist.

## Document
If you want to insert custom HTML into the `<head>` of the document for all pages, you can do so by using the `document` property. Example:

```yaml
document:
  head: |
    <script src="/assets/custom.js"></script>
```

## Branding
You can adjust the various parts of the branding through a top level `branding` property. Example:

```yaml
branding:
  custom-footer: |
    <p>Powered by <a href="https://github.com/Panonim/dynacat">Dynacat</a></p>
  logo-url: /assets/logo.png
  favicon-url: /assets/logo.png
  app-name: "My Dashboard"
  app-icon-url: "/assets/app-icon.svg"
  app-background-color: "#151519"
```

### Properties

| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| hide-footer | bool | no | false |
| custom-footer | string | no |  |
| logo-text | string | no | G |
| logo-url | string | no | |
| favicon-url | string | no | |
| app-name | string | no | Dynacat |
| app-icon-url | string | no | Dynacat's default icon |
| app-background-color | string | no | Dynacat's default background color |

#### `hide-footer`
Hides the footer when set to `true`.

#### `custom-footer`
Specify custom HTML to use for the footer.

#### `logo-text`
Specify custom text to use instead of the "G" found in the navigation.

#### `logo-url`
Specify a URL to a custom image to use instead of the "G" found in the navigation. If both `logo-text` and `logo-url` are set, only `logo-url` will be used.

#### `favicon-url`
Specify a URL to a custom image to use for the favicon.

#### `app-name`
Specify the name of the web app shown in browser tab and PWA.

#### `app-icon-url`
Specify URL for PWA and browser tab icon (512x512 PNG).

#### `app-background-color`
Specify background color for PWA. Must be a valid CSS color.

## Theme
Theming is done through a top level `theme` property. Values for the colors are in [HSL](https://giggster.com/guide/basics/hue-saturation-lightness/) (hue, saturation, lightness) format. You can use a color picker [like this one](https://hslpicker.com/) to convert colors from other formats to HSL. The values are separated by a space and `%` is not required for any of the numbers.

Example:

```yaml
theme:
  # This will be the default theme
  background-color: 100 20 10
  primary-color: 40 90 40
  contrast-multiplier: 1.1

  disable-picker: false
  presets:
    gruvbox-dark:
      background-color: 0 0 16
      primary-color: 43 59 81
      positive-color: 61 66 44
      negative-color: 6 96 59

    zebra:
      light: true
      background-color: 0 0 95
      primary-color: 0 0 10
      negative-color: 0 90 50
```

### Available themes
If you don't want to spend time configuring your own theme, there are [several available themes](themes.md) which you can simply copy the values for.

### Properties
| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| light | boolean | no | false |
| background-color | HSL | no | 240 8 9 |
| primary-color | HSL | no | 43 50 70 |
| positive-color | HSL | no | same as `primary-color` |
| negative-color | HSL | no | 0 70 70 |
| contrast-multiplier | number | no | 1 |
| text-saturation-multiplier | number | no | 1 |
| custom-css-file | string | no | |
| disable-picker | bool | false | |
| presets | object | no | |

#### `light`
Whether the scheme is light or dark. This does not change the background color, it inverts the text colors so that they look appropriately on a light background.

#### `background-color`
Color of the page and widgets.

#### `primary-color`
Color used across the page, largely to indicate unvisited links.

#### `positive-color`
Used to indicate that something is positive, such as stock price being up, twitch channel being live or a monitored site being online. If not set, the value of `primary-color` will be used.

#### `negative-color`
Oppposite of `positive-color`.

#### `contrast-multiplier`
Used to increase or decrease the contrast (in other words visibility) of the text. A value of `1.3` means that the text will be 30% lighter/darker depending on the scheme. Use this if you think that some of the text on the page is too dark and hard to read. Example:

![difference between 1 and 1.3 contrast](images/contrast-multiplier-example.png)

#### `text-saturation-multiplier`
Used to increase or decrease the saturation of text, useful when using a custom background color with a high amount of saturation and needing the text to have a more neutral color. `0.5` means that the saturation will be 50% lower and `1.5` means that it'll be 50% higher.

#### `custom-css-file`
Path to a custom CSS file, either external or one from within the server configured assets path. Example:

```yaml
theme:
  custom-css-file: /assets/my-style.css
```

> [!TIP]
>
> Because Dynacat uses a lot of utility classes it might be difficult to target some elements. To make it easier to style specific widgets, each widget has a `widget-type-{name}` class, so for example if you wanted to make the links inside just the RSS widget bigger you could use the following selector:
>
> ```css
> .widget-type-rss a {
>     font-size: 1.5rem;
> }
> ```
>
> In addition, you can also use the `css-class` property which is available on every widget to set custom class names for individual widgets.

#### `disable-picker`
When set to `true` hides the theme picker and disables the abiltity to switch between themes. All users who previously picked a non-default theme will be switched over to the default theme.

#### `presets`
Define additional theme presets that can be selected from the theme picker on the page. For each preset, you can specify the same properties as for the default theme, such as `background-color`, `primary-color`, `positive-color`, `negative-color`, `contrast-multiplier`, etc., except for the `custom-css-file` property.

Example:

```yaml
theme:
  presets:
    my-custom-dark-theme:
      background-color: 229 19 23
      contrast-multiplier: 1.2
      primary-color: 222 74 74
      positive-color: 96 44 68
      negative-color: 359 68 71
    my-custom-light-theme:
      light: true
      background-color: 220 23 95
      contrast-multiplier: 1.1
      primary-color: 220 91 54
      positive-color: 109 58 40
      negative-color: 347 87 44
```

To override the default dark and light themes, use the key names `default-dark` and `default-light`.

## Pages & Columns
![illustration of pages and columns](images/pages-and-columns-illustration.png)

Using pages and columns is how widgets are organized. Each page contains up to 3 columns and each column can have any number of widgets.

### Pages
Pages are defined through a top level `pages` property. The page defined first becomes the home page and all pages get automatically added to the navigation bar in the order that they were defined. Example:

```yaml
pages:
  - name: Home
    columns: ...

  - name: Videos
    columns: ...

  - name: Homelab
    columns: ...
```

### Properties
| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| name | string | yes | |
| slug | string | no | |
| dynamic-updates | boolean | no | true |
| width | string | no | |
| desktop-navigation-width | string | no | |
| center-vertically | boolean | no | false |
| hide-desktop-navigation | boolean | no | false |
| hide-from-navigation | boolean | no | false |
| show-mobile-header | boolean | no | false |
| head-widgets | array | no | |
| columns | array | yes | |

#### `name`
The name of the page which gets shown in the navigation bar.

#### `slug`
The URL friendly version of the title which is used to access the page. For example if the title of the page is "RSS Feeds" you can make the page accessible via `localhost:8080/feeds` by setting the slug to `feeds`. If not defined, it will automatically be generated from the title.

#### `dynamic-updates`
Controls whether automatic dynamic updates are enabled for this page. Defaults to `true`.

When set to `false`, the page will not perform automatic dynamic refreshes (SSE updates, page-level polling, and widget `update-interval` polling).

Example:

```yaml
pages:
  - name: "Home Page"
    slug: home
    dynamic-updates: false
    columns:
      - size: full
        widgets: ...
```

#### `width`
The maximum width of the page on desktop. Possible values are `default`, `slim` and `wide`.

#### `desktop-navigation-width`
The maximum width of the desktop navigation. Useful if you have a few pages that use a different width than the rest and don't want the navigation to jump abruptly when going to and away from those pages. Possible values are `default`, `slim` and `wide`.

Here are the pixel equivalents for each value:

* default: `1600px`
* slim: `1100px`
* wide: `1920px`

> [!NOTE]
>
> When using `slim`, the maximum number of columns allowed for that page is `2`.

#### `center-vertically`
When set to `true`, vertically centers the content on the page. Has no effect if the content is taller than the height of the viewport.

#### `hide-desktop-navigation`
Whether to show the navigation links at the top of the page on desktop.

#### `hide-from-navigation`
Whether the page should be omitted from both the desktop and mobile navigation menus. When `true`, the page remains accessible via its slug (or links you place elsewhere) but does not appear in the navigation bar or the mobile navigation drawer.

#### `show-mobile-header`
Whether to show a header displaying the name of the page on mobile. The header purposefully has a lot of vertical whitespace in order to push the content down and make it easier to reach on tall devices.

Preview:

![](images/mobile-header-preview.png)

#### `head-widgets`

Head widgets will be shown at the top of the page, above the columns, and take up the combined width of all columns. You can specify any widget, though some will look better than others, such as the markets, RSS feed with `horizontal-cards` style, and videos widgets. Example:

![](images/head-widgets-preview.png)

```yaml
pages:
  - name: Home
    head-widgets:
      - type: markets
        hide-header: true
        markets:
          - symbol: SPY
            name: S&P 500
          - symbol: BTC-USD
            name: Bitcoin
          - symbol: NVDA
            name: NVIDIA
          - symbol: AAPL
            name: Apple
          - symbol: MSFT
            name: Microsoft

    columns:
      - size: small
        widgets:
          - type: calendar
      - size: full
        widgets:
          - type: hacker-news
      - size: small
        widgets:
          - type: weather
            location: London, United Kingdom
```

### Columns
Columns are defined for each page using a `columns` property. There are two types of columns - `full` and `small`, which refers to their width. A small column takes up a fixed amount of width (300px) and a full column takes up the all of the remaining width. You can have up to 3 columns per page and you must have either 1 or 2 full columns. Example:

```yaml
pages:
  - name: Home
    columns:
      - size: small
        widgets: ...
      - size: full
        widgets: ...
      - size: small
        widgets: ...
```

### Properties
| Name | Type | Required |
| ---- | ---- | -------- |
| size | string | yes |
| widgets | array | no |

Here are some of the possible column configurations:

![column configuration small-full-small](images/column-configuration-1.png)

```yaml
columns:
  - size: small
    widgets: ...
  - size: full
    widgets: ...
  - size: small
    widgets: ...
```

![column configuration small-full-small](images/column-configuration-2.png)

```yaml
columns:
  - size: full
    widgets: ...
  - size: small
    widgets: ...
```

![column configuration small-full-small](images/column-configuration-3.png)

```yaml
columns:
  - size: full
    widgets: ...
  - size: full
    widgets: ...
```

## Widgets
### Bookmarks
Display a list of links which can be grouped.

Example:

```yaml
- type: bookmarks
  groups:
    - links:
        - title: Gmail
          url: https://mail.google.com/mail/u/0/
        - title: Amazon
          url: https://www.amazon.com/
        - title: Github
          url: https://github.com/
        - title: Wikipedia
          url: https://en.wikipedia.org/
    - title: Entertainment
      color: 10 70 50
      links:
        - title: Netflix
          url: https://www.netflix.com/
        - title: Disney+
          url: https://www.disneyplus.com/
        - title: YouTube
          url: https://www.youtube.com/
        - title: Prime Video
          url: https://www.primevideo.com/
    - title: Social
      color: 200 50 50
      links:
        - title: Reddit
          url: https://www.reddit.com/
        - title: Twitter
          url: https://twitter.com/
        - title: Instagram
          url: https://www.instagram.com/
```

Preview:

![](images/bookmarks-widget-preview.png)


#### Properties

| Name | Type | Required |
| ---- | ---- | -------- |
| groups | array | yes |

##### `groups`
An array of groups which can optionally have a title and a custom color.

###### Properties for each group
| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| title | string | no | |
| color | HSL | no | the primary color of the theme |
| links | array | yes | |
| same-tab | boolean | no | false |
| hide-arrow | boolean | no | false |
| target | string | no | |

> [!TIP]
>
> You can set `same-tab`, `hide-arrow` and `target` either on the group which will apply them to all links in that group, or on each individual link which will override the value set on the group.

###### Properties for each link
| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| title | string | yes | |
| url | string | yes | |
| description | string | no | |
| icon | string | no | |
| same-tab | boolean | no | false |
| hide-arrow | boolean | no | false |
| target | string | no | |

`icon`

See [Icons](#icons) for more information on how to specify icons.

`same-tab`

Whether to open the link in the same tab or a new one.

`hide-arrow`

Whether to hide the colored arrow on each link.

`target`

Set a custom value for the link's `target` attribute. Possible values are `_blank`, `_self`, `_parent` and `_top`, you can read more about what they do [here](https://developer.mozilla.org/en-US/docs/Web/HTML/Element/a#target). This property has precedence over `same-tab`.

### Calendar
Display a calendar.

Example:

```yaml
- type: calendar
  first-day-of-week: monday
```

Preview:

![](images/calendar-widget-preview.png)

#### Properties

| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| first-day-of-week | string | no | monday |

##### `first-day-of-week`
The day of the week that the calendar starts on. All week days are available as possible values.

### ChangeDetection.io
Display a list watches from changedetection.io.

Example

```yaml
- type: change-detection
  instance-url: https://changedetection.mydomain.com/
  token: ${CHANGE_DETECTION_TOKEN}
```

Preview:

![](images/change-detection-widget-preview.png)

#### Properties

| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| instance-url | string | no | `https://www.changedetection.io` |
| allow-insecure | boolean | no | false |
| token | string | no |  |
| limit | integer | no | 10 |
| collapse-after | integer | no | 5 |
| watches | array of strings | no |  |

##### `instance-url`
The URL pointing to your instance of `changedetection.io`.

##### `allow-insecure`
Whether to allow invalid/self-signed certificates when making requests to the service.

##### `token`
The API access token which can be found in `SETTINGS > API`. Optionally, you can specify this using an environment variable with the syntax `${VARIABLE_NAME}`.

##### `limit`
The maximum number of watches to show.

##### `collapse-after`
How many watches are visible before the "SHOW MORE" button appears. Set to `-1` to never collapse.

##### `watches`
By default all of the configured watches will be shown. Optionally, you can specify a list of UUIDs for the specific watches you want to have listed:

```yaml
  - type: change-detection
    watches:
      - 1abca041-6d4f-4554-aa19-809147f538d3
      - 705ed3e4-ea86-4d25-a064-822a6425be2c
```

### Clock
Display a clock showing the current time and date. Optionally, also display the the time in other timezones.

Example:

```yaml
- type: clock
  hour-format: 24h
  timezones:
    - timezone: Europe/Paris
      label: Paris
    - timezone: America/New_York
      label: New York
    - timezone: Asia/Tokyo
      label: Tokyo
```

Preview:

![](images/clock-widget-preview.png)

#### Properties

| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| hour-format | string | no | 24h |
| timezones | array | no |  |

##### `hour-format`
Whether to show the time in 12 or 24 hour format. Possible values are `12h` and `24h`.

#### Properties for each timezone

| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| timezone | string | yes | |
| label | string | no | |

##### `timezone`
A timezone identifier such as `Europe/London`, `America/New_York`, etc. The full list of available identifiers can be found [here](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones).

##### `label`
Optionally, override the display value for the timezone to something more meaningful such as "Home", "Work" or anything else.


### Custom API

Display data from a JSON API using a custom template.

> [!NOTE]
>
> The configuration of this widget requires some basic knowledge of programming, HTML, CSS, the Go template language and Dynacat-specific concepts.

Examples:

![](images/custom-api-preview-1.png)

<details>
<summary>View <code>dynacat.yml</code></summary>
<br>

```yaml
- type: custom-api
  title: Random Fact
  cache: 6h
  url: https://uselessfacts.jsph.pl/api/v2/facts/random
  template: |
    <p class="size-h4 color-paragraph">{{ .JSON.String "text" }}</p>
```
</details>
<br>

![](images/custom-api-preview-2.png)

<details>
<summary>View <code>dynacat.yml</code></summary>
<br>

```yaml
- type: custom-api
  title: Immich stats
  cache: 1d
  url: https://${IMMICH_URL}/api/server/statistics
  headers:
    x-api-key: ${IMMICH_API_KEY}
    Accept: application/json
  template: |
    <div class="flex justify-between text-center">
      <div>
          <div class="color-highlight size-h3">{{ .JSON.Int "photos" | formatNumber }}</div>
          <div class="size-h6">PHOTOS</div>
      </div>
      <div>
          <div class="color-highlight size-h3">{{ .JSON.Int "videos" | formatNumber }}</div>
          <div class="size-h6">VIDEOS</div>
      </div>
      <div>
          <div class="color-highlight size-h3">{{ div (.JSON.Int "usage" | toFloat) 1073741824 | toInt | formatNumber }}GB</div>
          <div class="size-h6">USAGE</div>
      </div>
    </div>
```
</details>
<br>

![](images/custom-api-preview-3.png)

<details>
<summary>View <code>dynacat.yml</code></summary>
<br>

```yaml
- type: custom-api
  title: Steam Specials
  cache: 12h
  url: https://store.steampowered.com/api/featuredcategories?cc=us
  template: |
    <ul class="list list-gap-10 collapsible-container" data-collapse-after="5">
    {{ range .JSON.Array "specials.items" }}
      <li>
        <a class="size-h4 color-highlight block text-truncate" href="https://store.steampowered.com/app/{{ .Int "id" }}/">{{ .String "name" }}</a>
        <ul class="list-horizontal-text">
          <li>{{ div (.Int "final_price" | toFloat) 100 | printf "$%.2f" }}</li>
          {{ $discount := .Int "discount_percent" }}
          <li{{ if ge $discount 40 }} class="color-positive"{{ end }}>{{ $discount }}% off</li>
        </ul>
      </li>
    {{ end }}
    </ul>
```
</details>

#### Properties
| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| url | string | no | |
| headers | key (string) & value (string) | no | |
| method | string | no | GET |
| body-type | string | no | json |
| body | any | no | |
| frameless | boolean | no | false |
| allow-insecure | boolean | no | false |
| skip-json-validation | boolean | no | false |
| template | string | yes | |
| options | map | no | |
| parameters | key (string) & value (string|array) | no | |
| subrequests | map of requests | no | |

##### `url`
The URL to fetch the data from. It must be accessible from the server that Dynacat is running on.

##### `headers`
Optionally specify the headers that will be sent with the request. Example:

```yaml
headers:
  x-api-key: your-api-key
  Accept: application/json
```

##### `method`
The HTTP method to use when making the request. Possible values are `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `OPTIONS` and `HEAD`.

##### `body-type`
The type of the body that will be sent with the request. Possible values are `json`, and `string`.

##### `body`
The body that will be sent with the request. It can be a string or a map. Example:

```yaml
body-type: json
body:
  key1: value1
  key2: value2
  multiple-items:
    - item1
    - item2
```

```yaml
body-type: string
body: |
  key1=value1&key2=value2
```

##### `frameless`
When set to `true`, removes the border and padding around the widget.

##### `allow-insecure`
Whether to ignore invalid/self-signed certificates.

##### `skip-json-validation`
When set to `true`, skips the JSON validation step. This is useful when the API returns JSON Lines/newline-delimited JSON, which is a format that consists of several JSON objects separated by newlines.

##### `template`
The template that will be used to display the data. It relies on Go's `html/template` package so it's recommended to go through [its documentation](https://pkg.go.dev/text/template) to understand how to do basic things such as conditionals, loops, etc. In addition, it also uses [tidwall's gjson](https://github.com/tidwall/gjson) package to parse the JSON data so it's worth going through its documentation if you want to use more advanced JSON selectors. You can view additional examples with explanations and function definitions [here](custom-api.md).

##### `options`
A map of options that will be passed to the template and can be used to modify the behavior of the widget.

<details>
<summary>View examples</summary>

<br>

Instead of defining options within the template and having to modify the template itself like such:

```yaml
- type: custom-api
  template: |
    {{ /* User configurable options */ }}
    {{ $collapseAfter := 5 }}
    {{ $showThumbnails := true }}
    {{ $showFlairs := false }}

     <ul class="list list-gap-10 collapsible-container" data-collapse-after="{{ $collapseAfter }}">
      {{ if $showThumbnails }}
        <li>
          <img src="{{ .JSON.String "thumbnail" }}" alt="thumbnail" />
        </li>
      {{ end }}
      {{ if $showFlairs }}
        <li>
          <span class="flair">{{ .JSON.String "flair" }}</span>
        </li>
      {{ end }}
     </ul>
```

You can use the `options` property to retrieve and define default values for these variables:

```yaml
- type: custom-api
  template: |
    <ul class="list list-gap-10 collapsible-container" data-collapse-after="{{ .Options.IntOr "collapse-after" 5 }}">
      {{ if (.Options.BoolOr "show-thumbnails" true) }}
        <li>
          <img src="{{ .JSON.String "thumbnail" }}" alt="thumbnail" />
        </li>
      {{ end }}
      {{ if (.Options.BoolOr "show-flairs" false) }}
        <li>
          <span class="flair">{{ .JSON.String "flair" }}</span>
        </li>
      {{ end }}
    </ul>
```

This way, you can optionally specify the `collapse-after`, `show-thumbnails` and `show-flairs` properties in the widget configuration:

```yaml
- type: custom-api
  options:
    collapse-after: 5
    show-thumbnails: true
    show-flairs: false
```

Which means you can reuse the same template for multiple widgets with different options:

```yaml
# Note that `custom-widgets` isn't a special property, it's just used to define the reusable "anchor", see https://support.atlassian.com/bitbucket-cloud/docs/yaml-anchors/
custom-widgets:
  - &example-widget
    type: custom-api
    template: |
      {{ .Options.StringOr "custom-option" "not defined" }}

pages:
  - name: Home
    columns:
      - size: full
        widgets:
          - <<: *example-widget
            options:
              custom-option: "Value 1"

          - <<: *example-widget
            options:
              custom-option: "Value 2"
```

Currently, the available methods on the `.Options` object are: `StringOr`, `IntOr`, `BoolOr` and `FloatOr`.

</details>

##### `parameters`
A list of keys and values that will be sent to the custom-api as query paramters.

##### `subrequests`
A map of additional requests that will be executed concurrently and then made available in the template via the `.Subrequest` property. Example:

```yaml
- type: custom-api
  cache: 2h
  subrequests:
    another-one:
      url: https://uselessfacts.jsph.pl/api/v2/facts/random
  title: Random Fact
  url: https://uselessfacts.jsph.pl/api/v2/facts/random
  template: |
    <p class="size-h4 color-paragraph">{{ .JSON.String "text" }}</p>
    <p class="size-h4 color-paragraph margin-top-15">{{ (.Subrequest "another-one").JSON.String "text" }}</p>
```

The subrequests support all the same properties as the main request, except for `subrequests` itself, so you can use `headers`, `parameters`, etc.

`(.Subrequest "key")` can be a little cumbersome to write, so you can define a variable to make it easier:

```yaml
  template: |
    {{ $anotherOne := .Subrequest "another-one" }}
    <p>{{ $anotherOne.JSON.String "text" }}</p>
```

You can also access the `.Response` property of a subrequest as you would with the main request:

```yaml
  template: |
    {{ $anotherOne := .Subrequest "another-one" }}
    <p>{{ $anotherOne.Response.StatusCode }}</p>
```

> [!NOTE]
>
> Setting this property will override any query parameters that are already in the URL.

```yaml
parameters:
  param1: value1
  param2:
    - item1
    - item2
```

### Dynawidgets

Display widgets from the [Dynawidgets community repository](https://github.com/Panonim/dynawidgets). These are pre-built templates that only require you to specify the widget slug and optional configuration.

> [!NOTE]
>
> Dynawidgets widgets are built by the community and leverage the custom-api widget system under the hood. Templates are automatically downloaded and cached locally.

Examples:

<details>
<summary>View <code>dynacat.yml</code></summary>
<br>

```yaml
- type: dynawidgets
  widget: daily-chess-puzzle
```

</details>
<br>

<details>
<summary>View <code>dynacat.yml</code> with custom title and options</summary>
<br>

```yaml
- type: dynawidgets
  widget: daily-chess-puzzle
  title: Chess Challenge
  cache: 1d
```

</details>

#### Properties
| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| widget | string | yes | |
| repo | string | no | main |
| url | string | no | |
| headers | key (string) & value (string) | no | |
| method | string | no | GET |
| body-type | string | no | json |
| body | any | no | |
| frameless | boolean | no | false |
| allow-insecure | boolean | no | false |
| skip-json-validation | boolean | no | false |
| options | map | no | |
| parameters | key (string) & value (string|array) | no | |
| subrequests | map of requests | no | |

##### `widget`
The slug of the widget from the dynawidgets repository. This is the only required property. The widget template will be automatically fetched and cached to `/app/assets/dynawidgets/{widget}.txt`. Example:

```yaml
widget: daily-chess-puzzle
```

##### `repo`
The branch/repository to fetch the widget from. This allows you to test widgets from different branches or forks. Defaults to `main`. Example:

```yaml
widget: daily-chess-puzzle
repo: testing/main
```

This will fetch from `https://raw.githubusercontent.com/Panonim/dynawidgets/refs/heads/testing/main/...`

##### `url`, `headers`, `method`, `body-type`, `body`, `frameless`, `allow-insecure`, `skip-json-validation`
These properties work the same as in the [custom-api widget](#custom-api). They override the default values defined in the widget's template `required` section.

##### `options`, `parameters`, `subrequests`
These properties work the same as in the [custom-api widget](#custom-api) and allow you to customize the widget's behavior and appearance.

Learn more about building and contributing widgets in the [Contributing to Dynawidgets](contributing.md) guide.

### DNS Stats
Display statistics from a self-hosted ad-blocking DNS resolver such as AdGuard Home, Pi-hole, or Technitium.

Example:

```yaml
- type: dns-stats
  service: adguard
  url: https://adguard.domain.com/
  username: admin
  password: ${ADGUARD_PASSWORD}
```

Preview:

![](images/dns-stats-widget-preview.png)

> [!NOTE]
>
> When using AdGuard Home the 3rd statistic on top will be the average latency and when using Pi-hole or Technitium it will be the total number of blocked domains from all adlists.

#### Properties

| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| service | string | no | pihole |
| allow-insecure | bool | no | false |
| url | string | yes |  |
| username | string | when service is `adguard` |  |
| password | string | when service is `adguard` or `pihole-v6` |  |
| token | string | when service is `pihole` |  |
| hide-graph | bool | no | false |
| hide-top-domains | bool | no | false |
| hour-format | string | no | 12h |

##### `service`
Either `adguard`, `technitium`, or `pihole` (major version 5 and below) or `pihole-v6` (major version 6 and above).

##### `allow-insecure`
Whether to allow invalid/self-signed certificates when making the request to the service.

##### `url`
The base URL of the service.

##### `username`
Only required when using AdGuard Home. The username used to log into the admin dashboard.

##### `password`
Required when using AdGuard Home, where the password is the one used to log into the admin dashboard.

Also required when using Pi-hole major version 6 and above, where the password is the one used to log into the admin dashboard or the application password, which can be found in `Settings -> Web Interface / API -> Configure app password`.

##### `token`
Required when using Pi-hole major version 5 or earlier. The API token which can be found in `Settings -> API -> Show API token`.

Also required when using Technitium, an API token can be generated at `Administration -> Sessions -> Create Token`.

##### `hide-graph`
Whether to hide the graph showing the number of queries over time.

##### `hide-top-domains`
Whether to hide the list of top blocked domains.

##### `hour-format`
Whether to display the relative time in the graph in `12h` or `24h` format.

### Docker Containers

Display the status of your Docker containers along with an icon and an optional short description.

![](images/docker-containers-preview.png)

```yaml
- type: docker-containers
  hide-by-default: false
```

> [!NOTE]
>
> The widget requires access to `docker.sock`. If you're running Dynacat inside a container, this can be done by mounting the socket as a volume:
>
> ```yaml
> services:
>   dynacat:
>     image: Panonim/dynacat
>     volumes:
>       - /var/run/docker.sock:/var/run/docker.sock
> ```

Configuration of the containers is done via labels applied to each container:

```yaml
  jellyfin:
    image: jellyfin/jellyfin:latest
    labels:
      dynacat.name: Jellyfin
      dynacat.icon: si:jellyfin
      dynacat.url: https://jellyfin.domain.com
      dynacat.description: Movies & shows
```

Alternatively, you can also define the values within your `dynacat.yml` via the `containers` property, where the key is the container name and each value is the same as the labels but without the "dynacat." prefix:

```yaml
- type: docker-containers
  containers:
    container_name_1:
      name: Container Name
      description: Description of the container
      url: https://container.domain.com
      icon: si:container-icon
      hide: false
```

For services with multiple containers you can specify a `dynacat.id` on the "main" container and `dynacat.parent` on each "child" container:

<details>
<summary>View <code>docker-compose.yml</code></summary>
<br>

```yaml
services:
  immich-server:
    image: ghcr.io/immich-app/immich-server
    labels:
      dynacat.name: Immich
      dynacat.icon: si:immich
      dynacat.url: https://immich.domain.com
      dynacat.description: Image & video management
      dynacat.id: immich

  redis:
    image: docker.io/redis:6.2-alpine
    labels:
      dynacat.parent: immich
      dynacat.name: Redis

  database:
    image: docker.io/tensorchord/pgvecto-rs:pg14-v0.2.0
    labels:
      dynacat.parent: immich
      dynacat.name: DB

  proxy:
    image: nginx:stable
    labels:
      dynacat.parent: immich
      dynacat.name: Proxy
```
</details>
<br>

This will place all child containers under the `Immich` container when hovering over its icon:

![](images/docker-container-parent.png)

If any of the child containers are down, their status will propagate up to the parent container:

![](images/docker-container-parent2.png)

#### Properties

| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| hide-by-default | boolean | no | false |
| format-container-names | boolean | no | false |
| sock-path | string | no | /var/run/docker.sock |
| category | string | no | |
| running-only | boolean | no | false |
| update-interval | string | no | 2m |

##### `hide-by-default`
Whether to hide the containers by default. If set to `true` you'll have to manually add a `dynacat.hide: false` label to each container you want to display. By default all containers will be shown and if you want to hide a specific container you can add a `dynacat.hide: true` label.

##### `format-container-names`
When set to `true`, automatically converts container names such as `container_name_1` into `Container Name 1`.

##### `sock-path`
The path to the Docker socket. This can also be a [remote socket](https://docs.docker.com/engine/daemon/remote-access/) or proxied socket using something like [docker-socket-proxy](https://github.com/Tecnativa/docker-socket-proxy).

###### `category`
Filter to only the containers which have this category specified via the `dynacat.category` label. Useful if you want to have multiple containers widgets, each showing a different set of containers.

<details>
<summary>View example</summary>
<br>


```yaml
services:
  jellyfin:
    image: jellyfin/jellyfin:latest
    labels:
      dynacat.name: Jellyfin
      dynacat.icon: si:jellyfin
      dynacat.url: https://jellyfin.domain.com
      dynacat.category: media

  gitea:
    image: gitea/gitea:latest
    labels:
      dynacat.name: Gitea
      dynacat.icon: si:gitea
      dynacat.url: https://gitea.domain.com
      dynacat.category: dev-tools

  vaultwarden:
    image: vaultwarden/server:latest
    labels:
      dynacat.name: Vaultwarden
      dynacat.icon: si:vaultwarden
      dynacat.url: https://vaultwarden.domain.com
      dynacat.category: dev-tools
```

Then you can use the `category` property to filter the containers:

```yaml
- type: docker-containers
  title: Dev tool containers
  category: dev-tools

- type: docker-containers
  title: Media containers
  category: media
```

</details>

##### `running-only`
Whether to only show running containers. If set to `true` only containers that are currently running will be displayed. If set to `false` all containers will be displayed regardless of their state.

#### Labels
| Name | Description |
| ---- | ----------- |
| dynacat.name | The name displayed in the UI. If not specified, the name of the container will be used. |
| dynacat.icon | See [Icons](#icons) for more information on how to specify icons |
| dynacat.url | The URL that the user will be redirected to when clicking on the container. |
| dynacat.same-tab | Whether to open the link in the same or a new tab. Default is `false`. |
| dynacat.description | A short description displayed in the UI. Default is empty. |
| dynacat.hide | Whether to hide the container. If set to `true` the container will not be displayed. Defaults to `false`. |
| dynacat.id | The custom ID of the container. Used to group containers under a single parent. |
| dynacat.parent | The ID of the parent container. Used to group containers under a single parent. |
| dynacat.category | The category of the container. Used to filter containers by category. |

### Docker Controller

Display and manage your Docker containers and images interactively. Start, stop, restart, and remove containers; pull and remove images directly from the dashboard.

Example:

```yaml
- type: docker-controller
  title: Docker
  show: both
  update-interval: 15s
```

> [!NOTE]
>
> The widget requires access to `docker.sock`. If you're running Dynacat inside a container, this can be done by mounting the socket as a volume:
>
> ```yaml
> services:
>   dynacat:
>     image: Panonim/dynacat
>     volumes:
>       - /var/run/docker.sock:/var/run/docker.sock
> ```

#### Features

- **Container Management**: Start, stop, restart, or remove containers with a single click
- **Image Management**: Pull images and remove unused images directly from the dashboard
- **State Visualization**: Visual indicators show container and image status (running, stopped, error, etc.)
- **Responsive Design**: Action buttons appear on hover; two-click confirmation for destructive actions
- **Filtering**: Display containers only, images only, or both

#### Properties

| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| show | string | no | both |
| sock-path | string | no | /var/run/docker.sock |
| format-container-names | boolean | no | false |
| collapse-after | integer | no | 4 |
| update-interval | string | no | 15s |

##### `show`
Controls what to display in the widget. Possible values are:
- `containers` - Display only containers
- `images` - Display only images
- `both` - Display both containers and images (default)

##### `sock-path`
The path to the Docker socket. This can also be a [remote socket](https://docs.docker.com/engine/daemon/remote-access/) or proxied socket using something like [docker-socket-proxy](https://github.com/Tecnativa/docker-socket-proxy).

##### `format-container-names`
When set to `true`, automatically converts container names such as `container_name_1` into `Container Name 1`.

##### `collapse-after`
The number of containers or images to display before showing a "SHOW MORE" button. Set to `-1` to never collapse.

##### `update-interval`
How often the widget polls for updates. The value is a string and must be a number followed by one of s (seconds), m (minutes) or h (hours). Default is `15s`.

#### Usage

**Container Actions:**
- Running containers show **Stop**, **Restart**, and **Remove** buttons
- Stopped containers show **Start** and **Remove** buttons
- Click remove button once to enter confirmation mode (icon changes to checkmark), click again to confirm
- Confirmation automatically cancels after 3 seconds if not confirmed

**Image Actions:**
- **Pull**: Enter an image name (e.g., `nginx:latest`, `ghcr.io/user/repo:tag`) in the input field and click the download button
- **Remove**: Click the remove button on an image to delete it (two-click confirmation)

### Extension
Display a widget provided by an external source (3rd party). If you want to learn more about developing extensions, checkout the [extensions documentation](extensions.md) (WIP).

```yaml
- type: extension
  url: https://domain.com/widget/display-a-message
  allow-potentially-dangerous-html: true
  parameters:
    message: Hello, world!
```

#### Properties
| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| url | string | yes | |
| fallback-content-type | string | no | |
| allow-potentially-dangerous-html | boolean | no | false |
| headers | key & value | no | |
| parameters | key & value | no | |

##### `url`
The URL of the extension. **Note that the query gets stripped from this URL and the one defined by `parameters` gets used instead.**

##### `fallback-content-type`
Optionally specify the fallback content type of the extension if the URL does not return a valid `Widget-Content-Type` header. Currently the only supported value for this property is `html`.

##### `headers`
Optionally specify the headers that will be sent with the request. Example:

```yaml
headers:
  x-api-key: ${SECRET_KEY}
```

##### `allow-potentially-dangerous-html`
Whether to allow the extension to display HTML.

> [!WARNING]
>
> There's a reason this property is scary-sounding. It's intended to be used by developers who are comfortable with developing and using their own extensions. Do not enable it if you have no idea what it means or if you're not **absolutely sure** that the extension URL you're using is safe.

##### `parameters`
A list of keys and values that will be sent to the extension as query paramters.

### Group
Group multiple widgets into one using tabs. Widgets are defined using a `widgets` property exactly as you would on a page column. The only limitation is that you cannot place a group widget or a split column widget within a group widget.

Example:

```yaml
- type: group
  widgets:
    - type: reddit
      subreddit: gamingnews
      show-thumbnails: true
      collapse-after: 6
    - type: reddit
      subreddit: games
    - type: reddit
      subreddit: pcgaming
      show-thumbnails: true
```

Preview:

![](images/group-widget-preview.png)

#### Sharing properties

To avoid repetition you can use [YAML anchors](https://support.atlassian.com/bitbucket-cloud/docs/yaml-anchors/) and share properties between widgets.

Example:

```yaml
- type: group
  define: &shared-properties
      type: reddit
      show-thumbnails: true
      collapse-after: 6
  widgets:
    - subreddit: gamingnews
      <<: *shared-properties
    - subreddit: games
      <<: *shared-properties
    - subreddit: pcgaming
      <<: *shared-properties
```

### Hacker News
Display a list of posts from [Hacker News](https://news.ycombinator.com/).

Example:

```yaml
- type: hacker-news
  limit: 15
  collapse-after: 5
```

Preview:
![](images/hacker-news-widget-preview.png)

#### Properties
| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| limit | integer | no | 15 |
| collapse-after | integer | no | 5 |
| comments-url-template | string | no | https://news.ycombinator.com/item?id={POST-ID} |
| sort-by | string | no | top |
| extra-sort-by | string | no | |

##### `comments-url-template`
Used to replace the default link for post comments. Useful if you want to use an alternative front-end. Example:

```yaml
comments-url-template: https://www.hckrnws.com/stories/{POST-ID}
```

Placeholders:

`{POST-ID}` - the ID of the post

##### `sort-by`
Used to specify the order in which the posts should get returned. Possible values are `top`, `new`, and `best`.

##### `extra-sort-by`
Can be used to specify an additional sort which will be applied on top of the already sorted posts. By default does not apply any extra sorting and the only available option is `engagement`.

The `engagement` sort tries to place the posts with the most points and comments on top, also prioritizing recent over old posts.

### HTML
Embed any HTML.

Example:

```yaml
- type: html
  source: |
    <p>Hello, <span class="color-primary">World</span>!</p>
```

Note the use of `|` after `source:`, this allows you to insert a multi-line string.

### iframe
Embed an iframe as a widget.

Example:

```yaml
- type: iframe
  source: <url>
  height: 400
```

#### Properties
| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| source | string | yes | |
| height | integer | no | 300 |

##### `source`
The source of the iframe.

##### `height`
The height of the iframe. The minimum allowed height is 50.

### Lobsters
Display a list of posts from [Lobsters](https://lobste.rs).

Example:

```yaml
- type: lobsters
  sort-by: hot
  tags:
    - go
    - security
    - linux
  limit: 15
  collapse-after: 5
```

Preview:
![](images/lobsters-widget-preview.png)

#### Properties
| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| instance-url | string | no | https://lobste.rs/ |
| custom-url | string | no | |
| limit | integer | no | 15 |
| collapse-after | integer | no | 5 |
| sort-by | string | no | hot |
| tags | array | no | |

##### `instance-url`
The base URL for a lobsters instance hosted somewhere other than on lobste.rs. Example:

```yaml
instance-url: https://www.journalduhacker.net/
```

##### `custom-url`
A custom URL to retrieve lobsters posts from. If this is specified, the `instance-url`, `sort-by` and `tags` properties are ignored.

##### `limit`
The maximum number of posts to show.

##### `collapse-after`
How many posts are visible before the "SHOW MORE" button appears. Set to `-1` to never collapse.

##### `sort-by`
The sort order in which posts are returned. Possible options are `hot` and `new`.

##### `tags`
Limit to posts containing one of the given tags. **You cannot specify a sort order when filtering by tags, it will default to `hot`.**

### Markets
Display a list of markets, their current value, change for the day and a small 21d chart. Data is taken from Yahoo Finance.

Example:

```yaml
- type: markets
  markets:
    - symbol: SPY
      name: S&P 500
    - symbol: BTC-USD
      name: Bitcoin
      chart-link: https://www.tradingview.com/chart/?symbol=INDEX:BTCUSD
    - symbol: NVDA
      name: NVIDIA
    - symbol: AAPL
      symbol-link: https://www.google.com/search?tbm=nws&q=apple
      name: Apple
```

Preview:

![](images/markets-widget-preview.png)

#### Properties

| Name | Type | Required |
| ---- | ---- | -------- |
| markets | array | yes |
| sort-by | string | no |
| chart-link-template | string | no |
| symbol-link-template | string | no |

##### `markets`
An array of markets for which to display information about.

##### `sort-by`
By default the markets are displayed in the order they were defined. You can customize their ordering by setting the `sort-by` property to `change` for descending order based on the stock's percentage change (e.g. 1% would be sorted higher than -1%) or `absolute-change` for descending order based on the stock's absolute price change (e.g. -1% would be sorted higher than +0.5%).

##### `chart-link-template`
A template for the link to go to when clicking on the chart that will be applied to all markets. The value `{SYMBOL}` will be replaced with the symbol of the market. You can override this on a per-market basis by specifying a `chart-link` property. Example:

```yaml
chart-link-template: https://www.tradingview.com/chart/?symbol={SYMBOL}
```

##### `symbol-link-template`
A template for the link to go to when clicking on the symbol that will be applied to all markets. The value `{SYMBOL}` will be replaced with the symbol of the market. You can override this on a per-market basis by specifying a `symbol-link` property. Example:

```yaml
symbol-link-template: https://www.google.com/search?tbm=nws&q={SYMBOL}
```

###### Properties for each market
| Name | Type | Required |
| ---- | ---- | -------- |
| symbol | string | yes |
| name | string | no |
| symbol-link | string | no |
| chart-link | string | no |

`symbol`

The symbol, as seen in Yahoo Finance.

`name`

The name that will be displayed under the symbol.

`symbol-link`

The link to go to when clicking on the symbol.

`chart-link`

The link to go to when clicking on the chart.

### Monitor
Display a list of sites and whether they are reachable (online) or not. This is determined by sending a GET request to the specified URL, if the response is 200 then the site is OK. The time it took to receive a response is also shown in milliseconds.

Example:

```yaml
- type: monitor
  cache: 1m
  title: Services
  sites:
    - title: Jellyfin
      url: https://jellyfin.yourdomain.com
      icon: /assets/jellyfin-logo.png
    - title: Gitea
      url: https://gitea.yourdomain.com
      icon: /assets/gitea-logo.png
    - title: Immich
      url: https://immich.yourdomain.com
      icon: /assets/immich-logo.png
    - title: AdGuard Home
      url: https://adguard.yourdomain.com
      icon: /assets/adguard-logo.png
    - title: Vaultwarden
      url: https://vault.yourdomain.com
      icon: /assets/vaultwarden-logo.png
```

Preview:

![](images/monitor-widget-preview.png)

You can hover over the "ERROR" text to view more information.

#### Properties

| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| sites | array | yes | |
| style | string | no | |
| show-failing-only | boolean | no | false |
| update-interval | string | no | 2m |

##### `show-failing-only`
Shows only a list of failing sites when set to `true`.

##### `style`
Used to change the appearance of the widget. Possible values are `compact`.

Preview of `compact`:

![](images/monitor-widget-compact-preview.png)

##### `sites`

Properties for each site:

| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| title | string | yes | |
| url | string | yes | |
| description | string | no | |
| check-url | string | no | |
| error-url | string | no | |
| icon | string | no | |
| timeout | string | no | 3s |
| allow-insecure | boolean | no | false |
| same-tab | boolean | no | false |
| alt-status-codes | array | no | |
| basic-auth | object | no | |

`title`

The title used to indicate the site.

`url`

The URL of the monitored service, which must be reachable by Dynacat, and will be used as the link to go to when clicking on the title. If `check-url` is not specified, this is used as the status check.

`check-url`

The URL which will be requested and its response will determine the status of the site. If not specified, the `url` property is used.

`error-url`

If the monitored service returns an error, the user will be redirected here. If not specified, the `url` property is used.

`icon`

See [Icons](#icons) for more information on how to specify icons.

`timeout`

How long to wait for a response from the server before considering it unreachable. The value is a string and must be a number followed by one of s, m, h, d. Example: `5s` for 5 seconds, `1m` for 1 minute, etc.

`allow-insecure`

Whether to ignore invalid/self-signed certificates.

`same-tab`

Whether to open the link in the same or a new tab.

`alt-status-codes`

Status codes other than 200 that you want to return "OK".

```yaml
alt-status-codes:
  - 403
```

`basic-auth`

HTTP Basic Authentication credentials for protected sites.

```yaml
basic-auth:
  username: your-username
  password: your-password
```

### Reddit
Display a list of posts from a specific subreddit.

> [!WARNING]
>
> Reddit does not allow unauthorized API access from VPS IPs, if you're hosting Dynacat on a VPS you will get a 403
> response. As a workaround you can either [register an app on Reddit](https://ssl.reddit.com/prefs/apps/) and use the
> generated ID and secret in the widget configuration to authenticate your requests (see `app-auth` property), use a proxy
> (see `proxy` property) or route the traffic from Dynacat through a VPN.

Example:

```yaml
- type: reddit
  subreddit: technology
```

#### Properties
| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| subreddit | string | yes |  |
| style | string | no | vertical-list |
| show-thumbnails | boolean | no | false |
| show-flairs | boolean | no | false |
| limit | integer | no | 15 |
| collapse-after | integer | no | 5 |
| comments-url-template | string | no | https://www.reddit.com/{POST-PATH} |
| request-url-template | string | no |  |
| proxy | string or multiple parameters | no |  |
| sort-by | string | no | hot |
| top-period | string | no | day |
| search | string | no | |
| extra-sort-by | string | no | |
| app-auth | object | no | |

##### `subreddit`
The subreddit for which to fetch the posts from.

##### `style`
Used to change the appearance of the widget. Possible values are `vertical-list`, `horizontal-cards` and `vertical-cards`. The first two were designed for full columns and the last for small columns.

`vertical-list`

![](images/reddit-widget-preview.png)

`horizontal-cards`

![](images/reddit-widget-horizontal-cards-preview.png)

`vertical-cards`

![](images/reddit-widget-vertical-cards-preview.png)

##### `show-thumbnails`
Shows or hides thumbnails next to the post. This only works if the `style` is `vertical-list`. Preview:

![](images/reddit-widget-vertical-list-thumbnails.png)

> [!NOTE]
>
> Thumbnails don't work for some subreddits due to Reddit's API not returning the thumbnail URL. No workaround for this yet.

##### `show-flairs`
Shows post flairs when set to `true`.

##### `limit`
The maximum number of posts to show.

##### `collapse-after`
How many posts are visible before the "SHOW MORE" button appears. Set to `-1` to never collapse. Not available when using the `vertical-cards` and `horizontal-cards` styles.

##### `comments-url-template`
Used to replace the default link for post comments. Useful if you want to use the old Reddit design or any other 3rd party front-end. Example:

```yaml
comments-url-template: https://old.reddit.com/{POST-PATH}
```

Placeholders:

`{POST-PATH}` - the full path to the post, such as:

```
r/selfhosted/comments/bsp01i/welcome_to_rselfhosted_please_read_this_first/
```

`{POST-ID}` - the ID that comes after `/comments/`

`{SUBREDDIT}` - the subreddit name

##### `request-url-template`
A custom request URL that will be used to fetch the data. This is useful when you're hosting Dynacat on a VPS where Reddit is blocking the requests and you want to route them through a proxy that accepts the URL as either a part of the path or a query parameter.

Placeholders:

`{REQUEST-URL}` - will be templated and replaced with the expanded request URL (i.e. https://www.reddit.com/r/selfhosted/hot.json). Example:

```
https://proxy/{REQUEST-URL}
https://your.proxy/?url={REQUEST-URL}
```

##### `proxy`
A custom HTTP/HTTPS proxy URL that will be used to fetch the data. This is useful when you're hosting Dynacat on a VPS where Reddit is blocking the requests and you want to bypass the restriction by routing the requests through a proxy. Example:

```yaml
proxy: http://user:pass@proxy.com:8080
proxy: https://user:pass@proxy.com:443
```

Alternatively, you can specify the proxy URL as well as additional options by using multiple parameters:

```yaml
proxy:
  url: http://proxy.com:8080
  allow-insecure: true
  timeout: 10s
```

###### `allow-insecure`
When set to `true`, allows the use of insecure connections such as when the proxy has a self-signed certificate.

###### `timeout`
The maximum time to wait for a response from the proxy. The value is a string and must be a number followed by one of s, m, h, d. Example: `10s` for 10 seconds, `1m` for 1 minute, etc

##### `sort-by`
Can be used to specify the order in which the posts should get returned. Possible values are `hot`, `new`, `top` and `rising`.

##### `top-period`
Available only when `sort-by` is set to `top`. Possible values are `hour`, `day`, `week`, `month`, `year` and `all`.

##### `search`
Keywords to search for. Searching within specific fields is also possible, **though keep in mind that Reddit may remove the ability to use any of these at any time**:

![](images/reddit-field-search.png)

##### `extra-sort-by`
Can be used to specify an additional sort which will be applied on top of the already sorted posts. By default does not apply any extra sorting and the only available option is `engagement`.

The `engagement` sort tries to place the posts with the most points and comments on top, also prioritizing recent over old posts.

##### `app-auth`
```yaml
widgets:
  - type: reddit
    subreddit: technology
    app-auth:
      name: ${REDDIT_APP_NAME}
      id: ${REDDIT_APP_CLIENT_ID}
      secret: ${REDDIT_APP_SECRET}
```

To register an app on Reddit, go to [this page](https://ssl.reddit.com/prefs/apps/).

### Releases
Display a list of latest releases for specific repositories on Github, GitLab, Codeberg or Docker Hub.

Example:

```yaml
- type: releases
  show-source-icon: true
  name-only: true
  repositories:
    - go-gitea/gitea
    - jellyfin/jellyfin
    - Panonim/dynacat
    - codeberg:redict/redict
    - gitlab:fdroid/fdroidclient
    - dockerhub:gotify/server
```

Preview:

![](images/releases-widget-preview.png)

#### Properties

| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| repositories | array | yes |  |
| show-source-icon | boolean | no | false |  |
| name-only | boolean | no | false |
| token | string | no | |
| gitlab-token | string | no | |
| limit | integer | no | 10 |
| collapse-after | integer | no | 5 |

##### `repositories`
A list of repositores to fetch the latest release for. Only the name/repo is required, not the full URL. A prefix can be specified for repositories hosted elsewhere such as GitLab, Codeberg and Docker Hub. Example:

```yaml
repositories:
  - gitlab:inkscape/inkscape
  - dockerhub:Panonim/dynacat
  - codeberg:redict/redict
```

Official images on Docker Hub can be specified by omitting the owner:

```yaml
repositories:
  - dockerhub:nginx
  - dockerhub:node
  - dockerhub:alpine
```

You can also specify exact tags for Docker Hub images:

```yaml
repositories:
  - dockerhub:nginx:latest
  - dockerhub:nginx:stable-alpine
```

To include prereleases you can specify the repository as an object and use the `include-prereleases` property:

**Note: This feature is currently only available for GitHub repositories.**

```yaml
repositories:
  - gitlab:inkscape/inkscape
  - repository: Panonim/dynacat
    include-prereleases: true
  - codeberg:redict/redict
```

##### `show-source-icon`
Shows an icon of the source (GitHub/GitLab/Codeberg/Docker Hub) next to the repository name when set to `true`.

##### `name-only`
Shows only the repository or image name without the owner or organization when set to `true`. For example, `linuxserver/docker-homeassistant` will be shown as `docker-homeassistant`.

##### `token`
Without authentication Github allows for up to 60 requests per hour. You can easily exceed this limit and start seeing errors if you're tracking lots of repositories or your cache time is low. To circumvent this you can [create a read only token from your Github account](https://github.com/settings/personal-access-tokens/new) and provide it here.

You can also specify the value for this token through an ENV variable using the syntax `${GITHUB_TOKEN}` where `GITHUB_TOKEN` is the name of the variable that holds the token. If you've installed Dynacat through docker you can specify the token in your docker-compose:

```yaml
services:
  dynacat:
    image: Panonim/dynacat
    environment:
      - GITHUB_TOKEN=<your token>
```

and then use it in your `dynacat.yml` like this:

```yaml
- type: releases
  token: ${GITHUB_TOKEN}
  repositories: ...
```

This way you can safely check your `dynacat.yml` in version control without exposing the token.

##### `gitlab-token`
Same as the above but used when fetching GitLab releases.

##### `limit`
The maximum number of releases to show.

#### `collapse-after`
How many releases are visible before the "SHOW MORE" button appears. Set to `-1` to never collapse.

### Repository
Display general information about a repository as well as a list of the latest open pull requests and issues.

Example:

```yaml
- type: repository
  repository: Panonim/dynacat
  pull-requests-limit: 5
  issues-limit: 3
  commits-limit: 3
```

Preview:

![](images/repository-preview.png)

#### Properties

| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| repository | string | yes |  |
| token | string | no | |
| pull-requests-limit | integer | no | 3 |
| issues-limit | integer | no | 3 |
| commits-limit | integer | no | -1 |

##### `repository`
The owner and repository name that will have their information displayed.

##### `token`
Without authentication Github allows for up to 60 requests per hour. You can easily exceed this limit and start seeing errors if your cache time is low or you have many instances of this widget. To circumvent this you can [create a read only token from your Github account](https://github.com/settings/personal-access-tokens/new) and provide it here.

##### `pull-requests-limit`
The maximum number of latest open pull requests to show. Set to `-1` to not show any.

##### `issues-limit`
The maximum number of latest open issues to show. Set to `-1` to not show any.

##### `commits-limit`
The maximum number of lastest commits to show from the default branch. Set to `-1` to not show any.

### RSS
Display a list of articles from multiple RSS feeds.

Example:

```yaml
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
```

#### Properties
| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| style | string | no | vertical-list |
| feeds | array | yes |
| thumbnail-height | float | no | 10 |
| card-height | float | no | 27 |
| limit | integer | no | 25 |
| preserve-order | bool | no | false |
| single-line-titles | boolean | no | false |
| collapse-after | integer | no | 5 |

##### `limit`
The maximum number of articles to show.

##### `collapse-after`
How many articles are visible before the "SHOW MORE" button appears. Set to `-1` to never collapse.

##### `preserve-order`
When set to `true`, the order of the articles will be preserved as they are in the feeds. Useful if a feed uses its own sorting order which denotes the importance of the articles. If you use this property while having a lot of feeds, it's recommended to set a `limit` to each individual feed since if the first defined feed has 15 articles, the articles from the second feed will start after the 15th article in the list.

##### `single-line-titles`
When set to `true`, truncates the title of each post if it exceeds one line. Only applies when the style is set to `vertical-list`.

##### `style`
Used to change the appearance of the widget. Possible values are:

* `vertical-list` - suitable for `full` and `small` columns
* `detailed-list` - suitable for `full` columns
* `horizontal-cards` - suitable for `full` columns
* `horizontal-cards-2` - suitable for `full` columns

Below is a preview of each style:

`vertical-list`

![preview of vertical-list style for RSS widget](images/rss-feed-vertical-list-preview.png)

`detailed-list`

![preview of detailed-list style for RSS widget](images/rss-widget-detailed-list-preview.png)

`horizontal-cards`

![preview of horizontal-cards style for RSS widget](images/rss-feed-horizontal-cards-preview.png)

`horizontal-cards-2`

![preview of horizontal-cards-2 style for RSS widget](images/rss-widget-horizontal-cards-2-preview.png)

##### `thumbnail-height`
Used to modify the height of the thumbnails. Works only when the style is set to `horizontal-cards`. The default value is `10` and the units are `rem`, if you want to for example double the height of the thumbnails you can set it to `20`.

##### `card-height`
Used to modify the height of cards when using the `horizontal-cards-2` style. The default value is `27` and the units are `rem`.

##### `feeds`
An array of RSS/atom feeds. The title can optionally be changed.

###### Properties for each feed
| Name | Type | Required | Default | Notes |
| ---- | ---- | -------- | ------- | ----- |
| url | string | yes | | |
| title | string | no | the title provided by the feed | |
| hide-categories | boolean | no | false | Only applicable for `detailed-list` style |
| hide-description | boolean | no | false | Only applicable for `detailed-list` style |
| limit | integer | no | | |
| item-link-prefix | string | no | | |
| headers | key (string) & value (string) | no | | |

###### `limit`
The maximum number of articles to show from that specific feed. Useful if you have a feed which posts a lot of articles frequently and you want to prevent it from excessively pushing down articles from other feeds.

###### `item-link-prefix`
If an RSS feed isn't returning item links with a base domain and Dynacat has failed to automatically detect the correct domain you can manually add a prefix to each link with this property.

###### `headers`
Optionally specify the headers that will be sent with the request. Example:

```yaml
- type: rss
  feeds:
    - url: https://domain.com/rss
      headers:
        User-Agent: Custom User Agent
```

### Search Widget
Display a search bar that can be used to search for specific terms on various search engines.

Example:

```yaml
- type: search
  search-engine: duckduckgo
  bangs:
    - title: YouTube
      shortcut: "!yt"
      url: https://www.youtube.com/results?search_query={QUERY}
```

Preview:

![](images/search-widget-preview.png)

#### Keyboard shortcuts
| Keys | Action | Condition |
| ---- | ------ | --------- |
| <kbd>S</kbd> | Focus the search bar | Not already focused on another input field |
| <kbd>Enter</kbd> | Perform search in the same tab | Search input is focused and not empty |
| <kbd>Ctrl</kbd> + <kbd>Enter</kbd> | Perform search in a new tab | Search input is focused and not empty |
| <kbd>Escape</kbd> | Leave focus / Close suggestions | Search input is focused |
| <kbd>Up</kbd> / <kbd>Down</kbd> | Insert the last search query / Navigate suggestions | Search input is focused |
| <kbd>↑</kbd> | Select previous suggestion | Autocomplete suggestions visible |
| <kbd>↓</kbd> | Select next suggestion | Autocomplete suggestions visible |

> [!TIP]
>
> You can use the property `new-tab` with a value of `true` if you want to show search results in a new tab by default. <kbd>Ctrl</kbd> + <kbd>Enter</kbd> will then show results in the same tab.

#### Properties
| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| search-engine | string | no | duckduckgo |
| new-tab | boolean | no | false |
| autofocus | boolean | no | false |
| target | string | no | _blank |
| placeholder | string | no | Type here to search… |
| autocomplete | boolean | no | true |
| bangs | array | no | |

##### `search-engine`
Either a value from the table below or a URL to a custom search engine. Use `{QUERY}` to indicate where the query value gets placed.

| Name | URL |
| ---- | --- |
| duckduckgo | `https://duckduckgo.com/?q={QUERY}` |
| google | `https://www.google.com/search?q={QUERY}` |
| bing | `https://www.bing.com/search?q={QUERY}` |
| perplexity | `https://www.perplexity.ai/search?q={QUERY}` |
| kagi | `https://kagi.com/search?q={QUERY}` |
| startpage | `https://www.startpage.com/search?q={QUERY}` |

##### `new-tab`
When set to `true`, swaps the shortcuts for showing results in the same or new tab, defaulting to showing results in a new tab.

##### `autofocus`
When set to `true`, automatically focuses the search input on page load.

##### `target`
The target to use when opening the search results in a new tab. Possible values are `_blank`, `_self`, `_parent` and `_top`.

##### `placeholder`
When set, modifies the text displayed in the input field before typing.

##### `autocomplete`
When set to `true` (default), displays search suggestions as you type. Navigate suggestions with <kbd>↑</kbd> and <kbd>↓</kbd> arrow keys, select with <kbd>Enter</kbd>, or dismiss with <kbd>Escape</kbd>. Set to `false` to disable autocompletion.

##### `bangs`
What now? [Bangs](https://duckduckgo.com/bangs). They're shortcuts that allow you to use the same search box for many different sites. Assuming you have it configured, if for example you start your search input with `!yt` you'd be able to perform a search on YouTube:

![](images/search-widget-bangs-preview.png)

##### Properties for each bang
| Name | Type | Required |
| ---- | ---- | -------- |
| title | string | no |
| shortcut | string | yes |
| url | string | yes |
| icon | string | no |

###### `title`
Optional title that will appear on the right side of the search bar when the query starts with the associated shortcut.

###### `shortcut`
Any value you wish to use as the shortcut for the search engine. It does not have to start with `!`.

> [!IMPORTANT]
>
> In YAML some characters have special meaning when placed in the beginning of a value. If your shortcut starts with `!` (and potentially some other special characters) you'll have to wrap the value in quotes:
> ```yaml
> shortcut: "!yt"
>```

###### `url`
The URL of the search engine. Use `{QUERY}` to indicate where the query value gets placed. Examples:

```yaml
url: https://www.reddit.com/search?q={QUERY}
url: https://store.steampowered.com/search/?term={QUERY}
url: https://www.amazon.com/s?k={QUERY}
```

###### `icon`
An optional icon to display in place of the default search icon when the bang is active. Supports the same icon syntax as other widgets:

```yaml
bangs:
  - title: YouTube
    shortcut: "!yt"
    url: https://www.youtube.com/results?search_query={QUERY}
    icon: di:youtube
```

See the [Icons](#icons) section for full syntax reference including `si:`, `di:`, `sh:`, `mdi:` prefixes, URLs, and `auto-invert`.

### Server Stats
Display statistics such as CPU usage, memory usage and disk usage of the server Dynacat is running on or other servers.

Example:

```yaml
- type: server-stats
  servers:
    - type: local
      name: Services
```

Preview:

![](images/server-stats-preview.gif)

> [!NOTE]
>
> This widget is currently under development, some features might not function as expected or may change.

To display data from a remote server you need to have the Dynacat Agent running on that server. You can download the agent from [here](https://github.com/glanceapp/agent), though keep in mind that it is still in development and may not work as expected. Support for other providers such as Dynacats will be added in the future.

In the event that the CPU temperature goes over 80°C, a flame icon will appear next to the CPU. The progress indicators will also turn red (or the equivalent of your negative color) to hopefully grab your attention if anything is unusually high:

![](images/server-stats-flame-icon.png)

#### Properties
| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| servers | array | no |  |

##### `servers`
If not provided it will display the statistics of the server Dynacat is running on.

##### Properties for both `local` and `remote` servers
| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| type | string | yes |  |
| name | string | no |  |
| hide-swap | boolean | no | false |

###### `type`
Whether to display statistics for the local server or a remote server. Possible values are `local` and `remote`.

###### `name`
The name of the server which will be displayed on the widget. If not provided it will default to the server's hostname.

###### `hide-swap`
Whether to hide the swap usage.

##### Properties for the `local` server
| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| cpu-temp-sensor | string | no |  |
| hide-mountpoints-by-default | boolean | no | false |
| mountpoints | map\[string\]object | no |  |

###### `cpu-temp-sensor`
The name of the sensor to use for the CPU temperature. When not provided the widget will attempt to find the correct one, if it fails to do so the temperature will not be displayed. To view the available sensors you can use `sensors` command.

###### `hide-mountpoints-by-default`
If set to `true` you'll have to manually make each mountpoint visible by adding a `hide: false` property to it like so:

```yaml
- type: server-stats
  servers:
    - type: local
      hide-mountpoints-by-default: true
      mountpoints:
        "/":
          hide: false
        "/mnt/data":
          hide: false
```

This is useful if you're running Dynacat inside of a container which usually mounts a lot of irrelevant filesystems.

###### `mountpoints`
A map of mountpoints to display disk usage for. The key is the path to the mountpoint and the value is an object with optional properties. Example:

```yaml
mountpoints:
  "/":
    name: Root
  "/mnt/data":
    name: Data
  "/boot/efi":
    hide: true
```

##### Properties for each `mountpoint`
| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| name | string | no |  |
| hide | boolean | no | false |

###### `name`
The name of the mountpoint which will be displayed on the widget. If not provided it will default to the mountpoint's path.

###### `hide`
Whether to hide this mountpoint from the widget.

##### Properties for `remote` servers
| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| url | string | yes |  |
| token | string | no |  |
| timeout | string | no | 3s |

###### `url`
The URL and port of the server to fetch the statistics from.

###### `token`
The authentication token to use when fetching the statistics.

###### `timeout`
The maximum time to wait for a response from the server. The value is a string and must be a number followed by one of s, m, h, d. Example: `10s` for 10 seconds, `1m` for 1 minute, etc

### Split Column
Splits a full sized column in half, allowing you to place widgets side by side horizontally. This is converted to a single column on mobile devices or if not enough width is available. Widgets are defined using a `widgets` property exactly as you would on a page column.

Two widgets side by side in a `full` column:

![](images/split-column-widget-preview.png)

<details>
<summary>View <code>dynacat.yml</code></summary>
<br>

```yaml
# ...
- size: full
  widgets:
    - type: split-column
      widgets:
        - type: hacker-news
          collapse-after: 3
        - type: lobsters
          collapse-after: 3

    - type: videos
# ...
```
</details>
<br>

You can also achieve a number of different full page layouts using just this widget, such as:

3 column layout where all columns have equal width:

![](images/split-column-widget-3-columns.png)

<details>
<summary>View <code>dynacat.yml</code></summary>
<br>

```yaml
pages:
  - name: Home
    columns:
      - size: full
        widgets:
          - type: split-column
            max-columns: 3
            widgets:
              - type: reddit
                subreddit: selfhosted
                collapse-after: 15
              - type: reddit
                subreddit: homelab
                collapse-after: 15
              - type: reddit
                subreddit: sysadmin
                collapse-after: 15
```
</details>
<br>

4 column layout where all columns have equal width (and the page is set to `width: wide`):

![](images/split-column-widget-4-columns.png)

<details>
<summary>View <code>dynacat.yml</code></summary>
<br>

```yaml
pages:
  - name: Home
    width: wide
    columns:
      - size: full
        widgets:
          - type: split-column
            max-columns: 4
            widgets:
              - type: reddit
                subreddit: selfhosted
                collapse-after: 15
              - type: reddit
                subreddit: homelab
                collapse-after: 15
              - type: reddit
                subreddit: linux
                collapse-after: 15
              - type: reddit
                subreddit: sysadmin
                collapse-after: 15
```
</details>
<br>

Masonry layout with up to 5 columns where all columns have equal width (and the page is set to `width: wide`):

![](images/split-column-widget-masonry.png)

<details>
<summary>View <code>dynacat.yml</code></summary>
<br>

```yaml
define:
  - &subreddit-settings
    type: reddit
    collapse-after: 5

pages:
  - name: Home
    width: wide
    columns:
      - size: full
        widgets:
          - type: split-column
            max-columns: 5
            widgets:
              - subreddit: selfhosted
                <<: *subreddit-settings
              - subreddit: homelab
                <<: *subreddit-settings
              - subreddit: linux
                <<: *subreddit-settings
              - subreddit: sysadmin
                <<: *subreddit-settings
              - subreddit: DevOps
                <<: *subreddit-settings
              - subreddit: Networking
                <<: *subreddit-settings
              - subreddit: DataHoarding
                <<: *subreddit-settings
              - subreddit: OpenSource
                <<: *subreddit-settings
              - subreddit: Privacy
                <<: *subreddit-settings
              - subreddit: FreeSoftware
                <<: *subreddit-settings
```
</details>
<br>

Just like the `group` widget, you can insert any widget type, you can even insert a `group` widget inside of a `split-column` widget, but you can't insert a `split-column` widget inside of a `group` widget.


### Stopwatch

A browser-based stopwatch widget. 

Example:

```yaml
- type: stopwatch
```

Preview:

![](images/stopwatch-widget-preview.png)

### Todo

A simple to-do list that allows you to add, edit and delete tasks. By default, tasks are stored in the browser's local storage. Optionally, tasks can be stored in a server-side SQLite database for persistence across browsers and devices.

Example:

```yaml
- type: to-do
```

Preview:

![](images/todo-widget-preview.png)

To reorder tasks, drag and drop them by grabbing the top side of the task:

![](images/reorder-todo-tasks-prevew.gif)

To delete a task, hover over it and click on the trash icon.

#### Properties

| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| id | string | no | |
| storage | string | no | local |
| collapse-after | integer | no | |

##### `id`

The ID of the todo list. If you want to have multiple todo lists, you must specify a different ID for each one. The ID is used to identify tasks in the chosen storage backend. Multiple todo lists with the same ID will share the same tasks.

##### `storage`

Controls where tasks are persisted. Accepted values:

- `local` (default) — tasks are stored in the browser's localStorage, same as before. No server-side setup required.
- `server` — tasks are stored in a SQLite database on the server. Tasks persist across browsers and server restarts. Requires `server.db-path` to be set (or uses the default `/app/assets/dynacat.db`).

##### `collapse-after`

When set, shows only the first N tasks and adds a "Show more" toggle for the rest. This is opt-in for the to-do widget and is disabled by default. Set to `-1` to explicitly disable collapsing.

Example with server storage:

```yaml
server:
  db-path: /data/dynacat.db

pages:
  - name: Home
    columns:
      - size: full
        widgets:
          - type: to-do
            id: my-list
            storage: server
```

#### Keyboard shortcuts
| Keys | Action | Condition |
| ---- | ------ | --------- |
| <kbd>Enter</kbd> | Add a task to the bottom of the list | When the "Add a task" field is focused |
| <kbd>Ctrl</kbd> + <kbd>Enter</kbd> | Add a task to the top of the list | When the "Add a task" field is focused |
| <kbd>Down Arrow</kbd> | Focus the last task that was added | When the "Add a task" field is focused |
| <kbd>Escape</kbd> | Focus the "Add a task" field | When a task is focused |

### Twitch Channels
Display a list of channels from Twitch.

Example:

```yaml
- type: twitch-channels
  channels:
    - jembawls
    - giantwaffle
    - asmongold
    - cohhcarnage
    - j_blow
    - xQc
```

Preview:

![](images/twitch-channels-widget-preview.png)

#### Properties
| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| channels | array | yes | |
| collapse-after | integer | no | 5 |
| sort-by | string | no | viewers |

##### `channels`
A list of channels to display.

##### `collapse-after`
How many channels are visible before the "SHOW MORE" button appears. Set to `-1` to never collapse.

##### `sort-by`
Can be used to specify the order in which the channels are displayed. Possible values are `viewers` and `live`.

### Twitch Top Games
Display a list of games with the most viewers on Twitch.

Example:

```yaml
- type: twitch-top-games
  exclude:
    - just-chatting
    - pools-hot-tubs-and-beaches
    - music
    - art
    - asmr
```

Preview:

![](images/twitch-top-games-widget-preview.png)

#### Properties
| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| exclude | array | no | |
| limit | integer | no | 10 |
| collapse-after | integer | no | 5 |

##### `exclude`
A list of categories that will never be shown. You must provide the slug found by clicking on the category and looking at the URL:

```
https://www.twitch.tv/directory/category/grand-theft-auto-v
                                         ^^^^^^^^^^^^^^^^^^
```

##### `limit`
The maximum number of games to show.

##### `collapse-after`
How many games are visible before the "SHOW MORE" button appears. Set to `-1` to never collapse.

### Videos
Display a list of the latest videos from specific YouTube channels.

Example:

```yaml
- type: videos
  channels:
    - UCXuqSBlHAE6Xw-yeJA0Tunw
    - UCBJycsmduvYEL83R_U4JriQ
    - UCHnyfMqiRRG1u-2MsSQLbXA
```

Preview:
![](images/videos-widget-preview.png)

#### Properties
| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| channels | array | yes | |
| playlists | array | no | |
| limit | integer | no | 25 |
| style | string | no | horizontal-cards |
| collapse-after | integer | no | 7 |
| collapse-after-rows | integer | no | 4 |
| include-shorts | boolean | no | false |
| video-url-template | string | no | https://www.youtube.com/watch?v={VIDEO-ID} |

##### `channels`
A list of channels IDs.

One way of getting the ID of a channel is going to the channel's page and clicking on its description:

![](images/videos-channel-description-example.png)

Then scroll down and click on "Share channel", then "Copy channel ID":

![](images/videos-copy-channel-id-example.png)

##### `playlists`

A list of playlist IDs:

```yaml
- type: videos
  playlists:
    - PL8mG-RkN2uTyZZ00ObwZxxoG_nJbs3qec
    - PL8mG-RkN2uTxTK4m_Vl2dYR9yE41kRdBg
```

The playlist ID can be found in its link which is in the form of
```
https://www.youtube.com...&list={ID}&...
```

##### `limit`
The maximum number of videos to show.

##### `collapse-after`
Specify the number of videos to show when using the `vertical-list` style before the "SHOW MORE" button appears.

##### `collapse-after-rows`
Specify the number of rows to show when using the `grid-cards` style before the "SHOW MORE" button appears.

##### `style`
Used to change the appearance of the widget. Possible values are `horizontal-cards`, `vertical-list` and `grid-cards`.

Preview of `vertical-list`:

![](images/videos-widget-vertical-list-preview.png)

Preview of `grid-cards`:

![](images/videos-widget-grid-cards-preview.png)

##### `video-url-template`
Used to replace the default link for videos. Useful when you're running your own YouTube front-end. Example:

```yaml
video-url-template: https://invidious.your-domain.com/watch?v={VIDEO-ID}
```

Placeholders:

`{VIDEO-ID}` - the ID of the video

### Weather
Display weather information for a specific location. The data is provided by https://open-meteo.com/.

Example:

```yaml
- type: weather
  units: metric
  hour-format: 12h
  location: London, United Kingdom
```

> [!NOTE]
>
> US cities which have common names can have their state specified as the second parameter as such:
>
> * Greenville, North Carolina, United States
> * Greenville, South Carolina, United States
> * Greenville, Mississippi, United States


Preview:

![](images/weather-widget-preview.png)

Each bar represents a 2 hour interval. The yellow background represents sunrise and sunset. The blue dots represent the times of the day where there is a high chance for precipitation. You can hover over the bars to view the exact temperature for that time.

#### Properties

| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| location | string | yes |  |
| units | string | no | metric |
| hour-format | string | no | 12h |
| hide-location | boolean | no | false |
| show-area-name | boolean | no | false |

##### `location`
The name of the city and country to fetch weather information for. Attempting to launch the applcation with an invalid location will result in an error. You can use the [gecoding API page](https://open-meteo.com/en/docs/geocoding-api) to search for your specific location. Dynacat will use the first result from the list if there are multiple.

##### `units`
Whether to show the temperature in celsius or fahrenheit, possible values are `metric` or `imperial`.

#### `hour-format`
Whether to show the hours of the day in 12-hour format or 24-hour format. Possible values are `12h` and `24h`.

##### `hide-location`
Optionally don't display the location name on the widget.

##### `show-area-name`
Whether to display the state/administrative area in the location name. If set to `true` the location will be displayed as:

```
Greenville, North Carolina, United States
```

Otherwise, if set to `false` (which is the default) it'll be displayed as:

```
Greenville, United States
```

# External Integrations
### Currently Playing

Display currently active media sessions from media servers (Plex, Jellyfin, Emby, Navidrome).

Example:

```yaml
- type: playing
  hosts:
    - url: plex:https://plex.example.com
      token: ${PLEX_TOKEN}
    - url: jellyfin:https://jellyfin.example.com
      token: ${JELLYFIN_API_KEY}
      allow-insecure: true
    - url: navidrome:https://music.example.com
      username: ${NAVIDROME_USER}
      token: ${NAVIDROME_PASSWORD}
  show-thumbnail: true
  show-progress-bar: true
  group-by-host: false
```

#### Properties
| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| hosts | array | yes | |
| play-state | string | no | indicator |
| show-thumbnail | boolean | no | true |
| show-paused | boolean | no | false |
| show-progress-bar | boolean | no | true |
| show-progress-info | boolean | no | true |
| group-by-host | boolean | no | false |
| update-interval | string | no | 30s |
| episode-title-format | string | no | series |

##### `hosts`

An array of media server hosts to check for active sessions.

**Important**: Each host URL must be prefixed with the server type (`plex:`, `jellyfin:`, `emby:`, or `navidrome:`). For example:
- `plex:https://192.168.1.10:32400`
- `jellyfin:http://jellyfin.local:8096`
- `emby:https://emby.example.com`
- `navidrome:https://music.example.com`

Properties for each host:

| Name | Type | Required | Notes |
| ---- | ---- | -------- | ----- |
| url | string | yes | Must include server type prefix |
| username | string | for Navidrome | Subsonic username (Navidrome only) |
| token | string | yes | API key/token, or password for Navidrome Subsonic auth |

Example:
```yaml
hosts:
  - url: plex:https://plex.example.com
    token: ${PLEX_TOKEN}
  - url: jellyfin:https://jellyfin.example.com
    token: ${JELLYFIN_API_KEY}
  - url: emby:https://emby.example.com
    token: ${EMBY_API_KEY}
  - url: navidrome:https://music.example.com
    username: ${NAVIDROME_USER}
    token: ${NAVIDROME_PASSWORD}
```

##### `play-state`
How to display the play state. Options:
- `indicator`: Pulsing dot (green for playing, gray for paused)
- `text`: Plain text "[playing]" or "[paused]"

##### `show-thumbnail`
When `true`, displays thumbnails for the currently playing media.

##### `show-paused`
When `true`, displays paused sessions in addition to actively playing sessions.

##### `show-progress-bar`
When `true`, displays an animated progress bar showing playback progress.

##### `show-progress-info`
When `true`, displays the estimated end time next to the progress bar. Requires `show-progress-bar` to be `true`.

##### `episode-title-format`
Controls how episode titles are displayed for episodic media. Options:
- `series`: (default) Shows the series name with season/episode as the main title (for example: "Arcane - S2E4") and the episode name as a smaller subtitle below.
- `episode`: shows the episode name as the main title and the series + SxEx as the subtitle.

Example:

```yaml
- type: playing
  hosts:
    - url: plex:https://plex.example.com
      token: ${PLEX_TOKEN}
  episode-title-format: series
```

##### `group-by-host`
When `true`, groups sessions by their media server. When `false`, displays all sessions in a unified list.

#### API Access & Tokens

**Plex:**
- Requires a Plex token. Follow [this guide](https://support.plex.tv/articles/204059436-finding-an-authentication-token-x-plex-token/) to obtain your token.

**Jellyfin:**
- Requires an API key. Generate one in: Administration → Dashboard → API Keys

**Emby:**
- Requires an API key. Generate one in: ⚙️ (settings icon) → Advanced → API Keys

**Navidrome:**
- Uses the Subsonic/OpenSubsonic API (`getNowPlaying`).
- Set `username` to your Navidrome username and `token` to your Navidrome password.

### Latest Media

Display a poster grid of recently added items from Plex, Jellyfin, and/or Emby. Each card shows a portrait thumbnail with an optional dark gradient overlay containing the title, year, duration (movies only), and how long ago it was added.

Example:

```yaml
- type: latest-media
  title: Recently Added
  update-interval: 30m
  item-count: 12
  columns: 4
  hosts:
    - url: jellyfin:https://jellyfin.example.com
      token: ${JELLYFIN_KEY}
    - url: plex:https://plex.example.com
      token: ${PLEX_TOKEN}
    - url: emby:https://emby.example.com
      token: ${EMBY_KEY}
```

Preview:
![](images/latest-media-preview.png)

#### Properties

| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| hosts | array | yes | |
| item-count | integer | no | 12 |
| columns | integer | no | 4 |
| small-column | boolean | no | false |
| show-overlay | boolean | no | true |
| update-interval | string | no | 30m |

##### `hosts`

An array of media server hosts to fetch recently added items from. Results from all hosts are merged and sorted by date added (newest first), then trimmed to `item-count`.

**Important**: Each host URL must be prefixed with the server type (`plex:`, `jellyfin:`, or `emby:`). For example:
- `plex:https://192.168.1.10:32400`
- `jellyfin:http://jellyfin.local:8096`
- `emby:https://emby.example.com`

Properties for each host:

| Name | Type | Required | Default | Notes |
| ---- | ---- | -------- | ------- | ----- |
| url | string | yes | | Must include server type prefix |
| token | string | yes | | API key or token for authentication |
| allow-insecure | boolean | no | false | Ignore invalid/self-signed certificates |
| libraries | array of strings | no | | Filter to specific library names; omit to fetch from all libraries |

Example with library filtering:

```yaml
hosts:
  - url: jellyfin:https://jellyfin.example.com
    token: ${JELLYFIN_API_KEY}
    libraries:
      - Movies
      - TV Shows
  - url: plex:https://plex.example.com
    token: ${PLEX_TOKEN}
    libraries:
      - Movies
```

##### `item-count`
The total number of items to display across all hosts combined.

##### `columns`
The number of columns in the poster grid. Default is `4`.

##### `small-column`
When set to `true`, halves the number of columns. Useful when placing the widget in a narrow column. For example, with `columns: 4` and `small-column: true`, the grid will use 2 columns.

##### `show-overlay`
When `true` (default), each card displays a dark gradient overlay at the bottom containing the title, year, duration (movies only), and relative time since it was added. Set to `false` to show only the poster thumbnail without any overlay text.

#### API Access & Tokens

**Plex:**
- Requires a Plex token. Follow [this guide](https://support.plex.tv/articles/204059436-finding-an-authentication-token-x-plex-token/) to obtain your token.

**Jellyfin:**
- Requires an API key. Generate one in: Administration → Dashboard → API Keys

**Emby:**
- Requires an API key. Generate one in: ⚙️ (settings icon) → Advanced → API Keys

**Self-Signed Certificates:**
When using self-signed or invalid certificates on your media server, set `allow-insecure: true` for that host:

```yaml
hosts:
  - url: jellyfin:https://jellyfin.local:8920
    token: ${JELLYFIN_API_KEY}
    allow-insecure: true
```

The server will use an insecure HTTP client to fetch both metadata and images, while the browser only receives the cached local URLs and never needs to establish a direct connection to your media server.

### Torrenting

Display active torrents from one or more qBittorrent, Deluge, or Transmission instances.

Example:

```yaml
- type: torrenting
  hosts:
    - url: http://192.168.1.1:8080
      username: admin
      password: adminadmin
    - url: http://192.168.1.2:8112
      client: deluge
      password: deluge
    - url: http://192.168.1.3:9091
      client: transmission
      username: admin
      password: adminadmin
```

#### Properties
| Name | Type | Required | Default |
| ---- | ---- | -------- | ------- |
| hosts | array | yes | |
| hide-completed | boolean | no | false |
| hide-inactive | boolean | no | false |
| hide-bar | boolean | no | false |
| wrap-text | boolean | no | false |
| collapse-after | number | no | 3 |
| update-interval | string | no | 30s |

##### `hosts`

An array of torrent client instances to connect to. Supports qBittorrent, Deluge, and Transmission. You can mix different clients in the same widget.

Properties for each host:

| Name | Type | Required |
| ---- | ---- | -------- |
| url | string | yes |
| client | string | no |
| username | string | yes (except Deluge) |
| password | string | yes |

###### `client`
The torrent client type. Supported values: `qbittorrent` (default), `deluge`, `transmission`.

Example with qBittorrent (default):

```yaml
hosts:
  - url: http://192.168.1.1:8080
    username: admin
    password: adminadmin
```

Example with Deluge:

```yaml
hosts:
  - url: http://192.168.1.1:8112
    client: deluge
    password: deluge
```

Example with Transmission:

```yaml
hosts:
  - url: http://192.168.1.1:9091
    client: transmission
    username: admin
    password: adminadmin
```

Example mixing multiple clients:

```yaml
hosts:
  - url: http://192.168.1.1:8080
    username: admin
    password: adminadmin
  - url: http://192.168.1.2:8112
    client: deluge
    password: deluge
  - url: http://192.168.1.3:9091
    client: transmission
    username: admin
    password: adminadmin
```

##### `hide-completed`
When `true`, hides torrents that have finished downloading (progress = 100%).

##### `hide-inactive`
When `true`, hides torrents that are not actively downloading or uploading.

##### `hide-bar`
When `true`, hides the progress bar and download stats for incomplete torrents.

##### `wrap-text`
When `true`, allows torrent titles to wrap across multiple lines instead of being truncated with ellipsis. This displays the full title of each torrent.

##### `collapse-after`
Number of torrents to show before collapsing the rest behind a "Show more" toggle. Set to `0` to disable collapsing.
