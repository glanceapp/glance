import { reinitWidget, runWidgetCleanup } from './page.js';

// Per-widget interval handles, keyed by widget id. Outerhtml replacement
// creates new DOM nodes for the widget, so id is the stable key.
const intervals = new Map();
const intervalSeconds = new Map();
const lastRefreshAt = new Map();

let baseURL = "";
let visibilityListenerInstalled = false;

export function setupWidgetRefresh(_baseURL) {
    baseURL = _baseURL;

    document.querySelectorAll(".widget[data-refresh-interval]").forEach((widgetEl) => {
        const seconds = parseInt(widgetEl.dataset.refreshInterval, 10);
        if (!seconds || seconds < 1) return;
        const id = widgetEl.dataset.widgetId;
        if (!id) return;
        intervalSeconds.set(id, seconds);
        startInterval(id, seconds);
    });

    if (visibilityListenerInstalled) return;
    visibilityListenerInstalled = true;
    if (document.hidden === undefined) return;

    document.addEventListener("visibilitychange", () => {
        if (document.hidden) {
            for (const handle of intervals.values()) clearInterval(handle);
            intervals.clear();
            return;
        }

        const now = Date.now();
        intervalSeconds.forEach((seconds, id) => {
            const last = lastRefreshAt.get(id) || 0;
            if (now - last >= seconds * 1000) refreshWidget(id, false);
            startInterval(id, seconds);
        });
    });
}

function startInterval(id, seconds) {
    const existing = intervals.get(id);
    if (existing !== undefined) clearInterval(existing);
    intervals.set(id, setInterval(() => refreshWidget(id, false), seconds * 1000));
}

async function refreshWidget(id, force) {
    const widgetEl = document.querySelector(`.widget[data-widget-id="${CSS.escape(id)}"]`);
    if (widgetEl === null) {
        // Widget was removed (e.g., by an earlier refresh). Drop its interval.
        const handle = intervals.get(id);
        if (handle !== undefined) {
            clearInterval(handle);
            intervals.delete(id);
        }
        intervalSeconds.delete(id);
        return;
    }

    widgetEl.setAttribute("aria-busy", "true");
    lastRefreshAt.set(id, Date.now());

    let html;
    try {
        const url = `${baseURL}/api/widgets/${encodeURIComponent(id)}/refresh${force ? "?force=1" : ""}`;
        const res = await fetch(url, { credentials: "same-origin" });
        if (!res.ok) {
            widgetEl.removeAttribute("aria-busy");
            return;
        }
        html = await res.text();
    } catch (e) {
        console.error("widget refresh failed", id, e);
        widgetEl.removeAttribute("aria-busy");
        return;
    }

    // Re-find in case something else swapped during the await.
    const target = document.querySelector(`.widget[data-widget-id="${CSS.escape(id)}"]`);
    if (target === null) return;

    runWidgetCleanup(target);

    const placeholder = document.createElement("div");
    placeholder.innerHTML = html;
    const newWidgetEl = placeholder.firstElementChild;
    if (newWidgetEl === null) {
        target.removeAttribute("aria-busy");
        return;
    }
    target.replaceWith(newWidgetEl);
    reinitWidget(newWidgetEl);
}
