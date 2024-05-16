function throttledDebounce(callback, maxDebounceTimes, debounceDelay) {
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


async function fetchPageContent(pageSlug) {
    // TODO: handle non 200 status codes/time outs
    // TODO: add retries
    const response = await fetch(`/api/pages/${pageSlug}/content/`);
    const content = await response.text();

    return content;
}

function setupCarousels() {
    const carouselElements = document.getElementsByClassName("carousel-container");

    if (carouselElements.length == 0) {
        return;
    }

    for (let i = 0; i < carouselElements.length; i++) {
        const carousel = carouselElements[i];
        carousel.classList.add("show-right-cutoff");
        const itemsContainer = carousel.getElementsByClassName("carousel-items-container")[0];

        const determineSideCutoffs = () => {
            if (itemsContainer.scrollLeft != 0) {
                carousel.classList.add("show-left-cutoff");
            } else {
                carousel.classList.remove("show-left-cutoff");
            }

            if (Math.ceil(itemsContainer.scrollLeft) + itemsContainer.clientWidth < itemsContainer.scrollWidth) {
                carousel.classList.add("show-right-cutoff");
            } else {
                carousel.classList.remove("show-right-cutoff");
            }
        }

        const determineSideCutoffsRateLimited = throttledDebounce(determineSideCutoffs, 20, 100);

        itemsContainer.addEventListener("scroll", determineSideCutoffsRateLimited);
        document.addEventListener("resize", determineSideCutoffsRateLimited);

        setTimeout(determineSideCutoffs, 1);
    }
}

const minuteInSeconds = 60;
const hourInSeconds = minuteInSeconds * 60;
const dayInSeconds = hourInSeconds * 24;
const monthInSeconds = dayInSeconds * 30;
const yearInSeconds = monthInSeconds * 12;

function relativeTimeSince(timestamp) {
    const delta = Math.round((Date.now() / 1000) - timestamp);

    if (delta < minuteInSeconds) {
        return "1m";
    }
    if (delta < hourInSeconds) {
        return Math.floor(delta / minuteInSeconds) + "m";
    }
    if (delta < dayInSeconds) {
        return Math.floor(delta / hourInSeconds) + "h";
    }
    if (delta < monthInSeconds) {
        return Math.floor(delta / dayInSeconds) + "d";
    }
    if (delta < yearInSeconds) {
        return Math.floor(delta / monthInSeconds) + "mo";
    }

    return Math.floor(delta / yearInSeconds) + "y";
}

function updateRelativeTimeForElements(elements)
{
    for (let i = 0; i < elements.length; i++)
    {
        const element = elements[i];
        const timestamp = element.dataset.dynamicRelativeTime;

        if (timestamp === undefined)
            continue

        element.innerText = relativeTimeSince(timestamp);
    }
}

function setupDynamicRelativeTime() {
    const elements = document.querySelectorAll("[data-dynamic-relative-time]");
    const updateInterval = 60 * 1000;
    let lastUpdateTime = Date.now();

    updateRelativeTimeForElements(elements);

    const updateElementsAndTimestamp = () => {
        updateRelativeTimeForElements(elements);
        lastUpdateTime = Date.now();
    };

    const scheduleRepeatingUpdate = () => setInterval(updateElementsAndTimestamp, updateInterval);

    if (document.hidden === undefined) {
        scheduleRepeatingUpdate();
        return;
    }

    let timeout = scheduleRepeatingUpdate();

    document.addEventListener("visibilitychange", () => {
        if (document.hidden) {
            clearTimeout(timeout);
            return;
        }

        const delta = Date.now() - lastUpdateTime;

        if (delta >= updateInterval) {
            updateElementsAndTimestamp();
            timeout = scheduleRepeatingUpdate();
            return;
        }

        timeout = setTimeout(() => {
            updateElementsAndTimestamp();
            timeout = scheduleRepeatingUpdate();
        }, updateInterval - delta);
    });
}

function setupLazyImages() {
    const images = document.querySelectorAll("img[loading=lazy]");

    if (images.length == 0) {
        return;
    }

    function imageFinishedTransition(image) {
        image.classList.add("finished-transition");
    }

    setTimeout(() => {
        for (let i = 0; i < images.length; i++) {
            const image = images[i];

            if (image.complete) {
                image.classList.add("cached");
                setTimeout(() => imageFinishedTransition(image), 5);
            } else {
                // TODO: also handle error event
                image.addEventListener("load", () => {
                    image.classList.add("loaded");
                    setTimeout(() => imageFinishedTransition(image), 500);
                });
            }
        }
    }, 5);
}

function setupCollapsibleLists() {
    const collapsibleListElements = document.getElementsByClassName("list-collapsible");

    if (collapsibleListElements.length == 0) {
        return;
    }

    const showMoreText = "Show more";
    const showLessText = "Show less";

    const attachExpandToggleButton = (listElement) => {
        let expanded = false;
        const button = document.createElement("button");
        const arrowElement = document.createElement("span");
        arrowElement.classList.add("list-collapsible-label-icon");
        const textNode = document.createTextNode(showMoreText);
        button.classList.add("list-collapsible-label");
        button.append(textNode, arrowElement);
        button.addEventListener("click", () => {
            expanded = !expanded;

            if (expanded) {
                listElement.classList.add("list-collapsible-expanded");
                button.classList.add("list-collapsible-label-expanded");
                textNode.nodeValue = showLessText;
                return;
            }

            const topBefore = button.getClientRects()[0].top;

            listElement.classList.remove("list-collapsible-expanded");
            button.classList.remove("list-collapsible-label-expanded");
            textNode.nodeValue = showMoreText;

            const topAfter = button.getClientRects()[0].top;

            if (topAfter > 0)
                return;

            window.scrollBy({
                top: topAfter - topBefore,
                behavior: "instant"
            });
        });

        listElement.after(button);
    };

    for (let i = 0; i < collapsibleListElements.length; i++) {
        const listElement = collapsibleListElements[i];

        if (listElement.dataset.collapseAfter === undefined) {
            continue;
        }

        const collapseAfter = parseInt(listElement.dataset.collapseAfter);

        if (listElement.children.length <= collapseAfter) {
            continue;
        }

        attachExpandToggleButton(listElement);

        for (let c = collapseAfter; c < listElement.children.length; c++) {
            const child = listElement.children[c];
            child.classList.add("list-collapsible-item");
            child.style.animationDelay = ((c - collapseAfter) * 20).toString() + "ms";
        }
    }
}

async function setupPage() {
    const pageElement = document.getElementById("page");
    const pageContentElement = document.getElementById("page-content");
    const pageContent = await fetchPageContent(pageData.slug);

    pageContentElement.innerHTML = pageContent;

    setTimeout(() => {
        document.body.classList.add("animate-element-transition");
    }, 200);

    try {
        setupLazyImages();
        setupCarousels();
        setupCollapsibleLists();
        setupDynamicRelativeTime();
    } finally {
        pageElement.classList.add("content-ready");
    }
}

if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", setupPage);
} else {
    setupPage();
}
