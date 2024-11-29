export function throttledDebounce(callback, maxDebounceTimes, debounceDelay) {
    let debounceTimeout;
    let timesDebounced = 0;

    return function () {
        if (timesDebounced == maxDebounceTimes) {
            clearTimeout(debounceTimeout);
            timesDebounced = 0;
            callback();
            return;
        }

        clearTimeout(debounceTimeout);
        timesDebounced++;

        debounceTimeout = setTimeout(() => {
            timesDebounced = 0;
            callback();
        }, debounceDelay);
    };
};

export function isElementVisible(element) {
    return !!(element.offsetWidth || element.offsetHeight || element.getClientRects().length);
}

export function clamp(value, min, max) {
    return Math.min(Math.max(value, min), max);
}

// NOTE: inconsistent behavior between browsers when it comes to
// whether the newly opened tab gets focused or not, potentially
// depending on the event that this function is called from
export function openURLInNewTab(url, focus = true) {
    const newWindow = window.open(url, '_blank', 'noopener,noreferrer');

    if (focus && newWindow != null) newWindow.focus();
}
