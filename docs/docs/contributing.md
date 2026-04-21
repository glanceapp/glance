# Contributing

This repository is a community catalog of Dynacat widgets. Contributions are welcome, but they should follow the project structure and the widget-specific rules below.

## Governance

These rules may change as the repository grows and the community evolves.

- Everyone can contribute widgets to this repository.
- Duplicate widget types are allowed when they are meaningfully different, such as:
  - different APIs,
  - different information,
  - different presentation or styling.
- Pull requests require at least one vouch from the community before merge, as long as they meet the guidelines.
- Anyone can vouch for a pull request by leaving a comment or a 👍 after testing the widget.
- The author of each widget is responsible for maintaining it, including updates for new Dynacat versions.
- The author of each widget is responsible for responding to issues and pull requests related to that widget.
- If a widget is broken for an extended period and its author is unresponsive, it may be removed or transferred to a new maintainer.
- If you submit a pull request that changes another author’s widget, mention them in the pull request description and give them time to review it.
- If you no longer want to maintain a widget, open an issue.

## Repository Rules

PR validation expects the following:

- PRs must target the `testing` branch.
- PRs should only change files under `widgets/`.
- Widget structure must pass validation in CI.

## Submitting a `custom-api` Widget

If you want to share a custom widget with the community, follow these steps:

1. Fork the repository.
2. Create a new directory under `widgets/` for your widget, using a clear name tied to the widget example: `dynacat-weather`.
3. Follow the guidelines in this document.
4. Add a `template.txt` file containing only the template body. Include only the text after `template: |`, not the `template: |` line itself.

   Wrong:

   ```yaml
   - type: custom-api
     title: Hourly Stats
     update-interval: 1h # Updates every hour
     url: https://api.example.com/stats
     template: |
       <div>{{ .JSON.String "count" }}</div>
   ```

   Right:

   ```text
   <div>{{ .JSON.String "count" }}</div>
   ```

   **With required section:**

   ```text
   <div>{{ .JSON.String "count" }}</div>

   required: |
     url: https://api.example.com/stats
   ```

5. Add a `required:` section at the bottom of `template.txt` to specify default configuration values. This is optional but recommended for any API that requires a URL or other configuration. The format is YAML, with supported fields:
   - `url`: The default API endpoint to fetch data from

   When users add your widget to their config, they can override these defaults:

   ```yaml
   - type: dynawidgets
     widget: your-widget-slug
     # Uses the URL from required: section by default
   ```

   Or with a custom URL:

   ```yaml
   - type: dynawidgets
     widget: your-widget-slug
     url: https://custom-api.example.com/data
   ```

6. Add a `widget.md` in that directory with a preview of the widget, the YAML configuration, and any setup or usage notes needed to run it. Use existing widgets as a reference.
7. Add a `preview.png` in the /images directory (optional).
8. Add a `meta.yml` file with:

```yaml
title: Your widget's title
description: A short description of the widget
author: your-github-username
```

9. Commit your changes, push them to your fork, and open a pull request against `testing`.

## Widget Guidelines

### Use suffixed CSS classes

If you need custom CSS, suffix classes with the widget name so styles stay isolated:

```css
.{class}-{widget-name} {
  text-align: center;
  margin-top: 1.5rem;
}
```

### Use environment variables for configurable values

Do not hardcode local addresses or secrets.

Bad:

```yaml
url: https://192.168.0.50:8080/api/server/statistics
headers:
  x-api-key: 1234567890
```

Good:

```yaml
url: https://${IMMICH_URL}/api/server/statistics
headers:
  x-api-key: ${IMMICH_API_KEY}
```

### Use reasonable cache times

Choose cache times that fit the data source:

- a few minutes or less for local services,
- hours for external data that changes slowly.

### Do not depend on extra local APIs

Widgets should be copy-pasteable and work with minimal setup. If your idea needs a separate local service to parse or transform data, submit it as an extension widget or request the missing capability in the main Dynacat repository.

### Apply custom styles directly to elements

Avoid relying on CSS classes that belong to existing widgets. Inline the needed styles instead:

```html
<img style="width: 5rem; aspect-ratio: 3 / 4; border-radius: var(--border-radius);" src="...">
```

Utility classes such as `flex`, `color-primary`, `size-h3`, and `text-center` are fine to use.

### Do not hardcode colors

Use the shared utility classes instead of fixed color values:

- `color-primary`
- `color-positive`
- `color-negative`
- `color-highlight`
- `color-subdue`

### Multiple variants are allowed

You can include multiple styles, layouts, or API variants for the same widget. Keep them in the same widget directory and document each one clearly in the `README.md`.

## Before Opening a PR

Run the validation script locally if you can:

```bash
node scripts/validate-widgets.js
```

The generated README section and database are handled by CI on the `testing` branch, so focus your PR on the widget content itself.

## If You Are Maintaining a Widget

- Keep the widget working with new Dynacat releases.
- Respond to issues and pull requests related to your widget.
- If you can no longer maintain it, open an issue so ownership can be reassigned.
