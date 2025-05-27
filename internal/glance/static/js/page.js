import { setupPopovers } from './popover.js';
import { setupMasonries } from './masonry.js';
import { throttledDebounce, isElementVisible, openURLInNewTab } from './utils.js';
import { elem, find, findAll } from './templating.js';

async function fetchPageContent(pageData) {
    // TODO: handle non 200 status codes/time outs
    // TODO: add retries
    const response = await fetch(`${pageData.baseURL}/api/pages/${pageData.slug}/content/`);
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
        window.addEventListener("resize", determineSideCutoffsRateLimited);

        afterContentReady(determineSideCutoffs);
    }
}

const minuteInSeconds = 60;
const hourInSeconds = minuteInSeconds * 60;
const dayInSeconds = hourInSeconds * 24;
const monthInSeconds = dayInSeconds * 30.4;
const yearInSeconds = dayInSeconds * 365;

function timestampToRelativeTime(timestamp) {
    let delta = Math.round((Date.now() / 1000) - timestamp);
    let prefix = "";

    if (delta < 0) {
        delta = -delta;
        prefix = "in ";
    }

    if (delta < minuteInSeconds) {
        return prefix + "1m";
    }
    if (delta < hourInSeconds) {
        return prefix + Math.floor(delta / minuteInSeconds) + "m";
    }
    if (delta < dayInSeconds) {
        return prefix + Math.floor(delta / hourInSeconds) + "h";
    }
    if (delta < monthInSeconds) {
        return prefix + Math.floor(delta / dayInSeconds) + "d";
    }
    if (delta < yearInSeconds) {
        return prefix + Math.floor(delta / monthInSeconds) + "mo";
    }

    return prefix + Math.floor(delta / yearInSeconds) + "y";
}

function updateRelativeTimeForElements(elements)
{
    for (let i = 0; i < elements.length; i++)
    {
        const element = elements[i];
        const timestamp = element.dataset.dynamicRelativeTime;

        if (timestamp === undefined)
            continue

        element.textContent = timestampToRelativeTime(timestamp);
    }
}

function setupSearchBoxes() {
    const searchWidgets = document.getElementsByClassName("search");

    if (searchWidgets.length == 0) {
        return;
    }

    // Simple fuzzy matching function
    function fuzzyMatch(text, query) {
        const lowerText = text.toLowerCase();
        const lowerQuery = query.toLowerCase();

        // Exact match gets highest priority
        if (lowerText === lowerQuery) return { score: 1000, type: 'exact' };
        if (lowerText.includes(lowerQuery)) return { score: 500, type: 'contains' };

        // Simple fuzzy matching
        let score = 0;
        let textIndex = 0;

        for (let i = 0; i < lowerQuery.length; i++) {
            const char = lowerQuery[i];
            const foundIndex = lowerText.indexOf(char, textIndex);

            if (foundIndex === -1) return { score: 0, type: 'none' };

            // Bonus for consecutive characters
            if (foundIndex === textIndex) score += 10;
            else score += 1;

            textIndex = foundIndex + 1;
        }

        return { score, type: 'fuzzy' };
    }

    for (let i = 0; i < searchWidgets.length; i++) {
        const widget = searchWidgets[i];
        const defaultSearchUrl = widget.dataset.defaultSearchUrl;
        const target = widget.dataset.target || "_blank";
        const newTab = widget.dataset.newTab === "true";
        const suggestionsEnabled = widget.dataset.suggestionsEnabled === "true";
        const suggestionEngine = widget.dataset.suggestionEngine;
        const inputElement = widget.getElementsByClassName("search-input")[0];
        const bangElement = widget.getElementsByClassName("search-bang")[0];
        const dropdownElement = widget.getElementsByClassName("search-shortcuts-dropdown")[0];
        const shortcutsListElement = widget.getElementsByClassName("search-shortcuts-list")[0];
        const bangs = widget.querySelectorAll(".search-bangs > input");
        const shortcuts = widget.querySelectorAll(".search-shortcuts > input");
        const bangsMap = {};
        const shortcutsArray = [];
        const kbdElement = widget.getElementsByTagName("kbd")[0];
        let currentBang = null;
        let lastQuery = "";
        let highlightedIndex = -1;
        let filteredResults = [];
        let currentSuggestions = [];

        for (let j = 0; j < bangs.length; j++) {
            const bang = bangs[j];
            bangsMap[bang.dataset.shortcut] = bang;
        }

        for (let j = 0; j < shortcuts.length; j++) {
            const shortcut = shortcuts[j];
            shortcutsArray.push({
                title: shortcut.dataset.title,
                url: shortcut.dataset.url,
                shortcut: shortcut.dataset.shortcut || ""
            });
        }

        function hideDropdown() {
            dropdownElement.classList.add("hidden");
            highlightedIndex = -1;
        }

        function showDropdown() {
            if (filteredResults.length > 0) {
                dropdownElement.classList.remove("hidden");
            }
        }

        function updateDropdown(query) {
            if (!query) {
                hideDropdown();
                return;
            }

            // Filter and score shortcuts
            const shortcutMatches = shortcutsArray.map(shortcut => {
                const titleMatch = fuzzyMatch(shortcut.title, query);
                const shortcutMatch = shortcut.shortcut ? fuzzyMatch(shortcut.shortcut, query) : { score: 0, type: 'none' };
                const bestMatch = titleMatch.score > shortcutMatch.score ? titleMatch : shortcutMatch;

                return {
                    ...shortcut,
                    type: 'shortcut',
                    score: bestMatch.score,
                    matchType: bestMatch.type,
                    isExact: titleMatch.type === 'exact' || shortcutMatch.type === 'exact'
                };
            }).filter(item => item.score > 0)
              .sort((a, b) => b.score - a.score);

            // Start with shortcuts
            filteredResults = [...shortcutMatches];
            highlightedIndex = -1;

            // Fetch search suggestions if enabled and no bang is active
            if (suggestionsEnabled && suggestionEngine && currentBang === null) {
                fetchSuggestions(query).then(suggestions => {
                    currentSuggestions = suggestions.map(suggestion => ({
                        type: 'suggestion',
                        title: suggestion,
                        url: null,
                        score: 1 // Lower priority than shortcuts
                    }));

                    // Combine shortcuts and suggestions
                    filteredResults = [...shortcutMatches, ...currentSuggestions];
                    renderDropdown();
                    showDropdown();
                });
            }

            renderDropdown();
            if (filteredResults.length > 0) {
                showDropdown();
            } else {
                hideDropdown();
            }
        }

        function renderDropdown() {
            const shortcutIcon = `<svg class="search-shortcut-icon" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" d="M13.19 8.688a4.5 4.5 0 0 1 1.242 7.244l-4.5 4.5a4.5 4.5 0 0 1-6.364-6.364l1.757-1.757m13.35-.622 1.757-1.757a4.5 4.5 0 0 0-6.364-6.364l-4.5 4.5a4.5 4.5 0 0 0 1.242 7.244" />
            </svg>`;

            const suggestionIcon = `<svg class="search-shortcut-icon" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
            </svg>`;

            shortcutsListElement.innerHTML = filteredResults.map((item, index) => {
                if (item.type === 'shortcut') {
                    return `
                        <div class="search-shortcut-item ${item.isExact ? 'exact-match' : ''}" data-index="${index}" data-type="shortcut">
                            ${shortcutIcon}
                            <div class="search-shortcut-content">
                                <div class="search-shortcut-title">${item.title}</div>
                                <div class="search-shortcut-url hide-on-mobile">${item.url}</div>
                            </div>
                            ${item.shortcut ? `<div class="search-shortcut-shortcut">${item.shortcut}</div>` : ''}
                        </div>
                    `;
                } else {
                    return `
                        <div class="search-shortcut-item" data-index="${index}" data-type="suggestion">
                            ${suggestionIcon}
                            <div class="search-shortcut-title">${item.title}</div>
                        </div>
                    `;
                }
            }).join('');

            // Add click event listeners
            shortcutsListElement.querySelectorAll('.search-shortcut-item').forEach((item, index) => {
                item.addEventListener('click', () => {
                    const result = filteredResults[index];
                    if (result.type === 'shortcut') {
                        navigateToShortcut(result);
                    } else {
                        performSearch(result.title);
                    }
                });
            });
        }

        async function fetchSuggestions(query) {
            try {
                const response = await fetch(`/api/search/suggestions?query=${encodeURIComponent(query)}&suggestion_engine=${encodeURIComponent(suggestionEngine)}`, {
                    method: 'POST'
                });

                if (!response.ok) {
                    return [];
                }

                const data = await response.json();
                return data.suggestions || [];
            } catch (error) {
                console.error('Failed to fetch suggestions:', error);
                return [];
            }
        }

        function performSearch(query) {
            const url = defaultSearchUrl.replace("!QUERY!", encodeURIComponent(query));
            if (newTab) {
                window.open(url, target).focus();
            } else {
                window.location.href = url;
            }
            inputElement.value = "";
            hideDropdown();
        }

        function navigateToShortcut(shortcut) {
            if (newTab) {
                window.open(shortcut.url, target).focus();
            } else {
                window.location.href = shortcut.url;
            }
            inputElement.value = "";
            hideDropdown();
        }

        function highlightItem(index) {
            const items = shortcutsListElement.querySelectorAll('.search-shortcut-item');
            items.forEach(item => item.classList.remove('highlighted'));

            if (index >= 0 && index < items.length) {
                items[index].classList.add('highlighted');
                highlightedIndex = index;
                
                // Scroll the highlighted item into view
                items[index].scrollIntoView({
                    behavior: 'smooth',
                    block: 'nearest',
                    inline: 'nearest'
                });
            } else {
                highlightedIndex = -1;
            }
        }

        const handleKeyDown = (event) => {
            if (event.key == "Escape") {
                hideDropdown();
                inputElement.blur();
                return;
            }

            // Handle dropdown navigation
            if (!dropdownElement.classList.contains("hidden")) {
                if (event.key === "ArrowDown" || (event.key === "Tab" && !event.shiftKey)) {
                    event.preventDefault();
                    const newIndex = Math.min(highlightedIndex + 1, filteredResults.length - 1);
                    highlightItem(newIndex);
                    return;
                }

                if (event.key === "ArrowUp" || (event.key === "Tab" && event.shiftKey)) {
                    event.preventDefault();
                    const newIndex = Math.max(highlightedIndex - 1, -1);
                    highlightItem(newIndex);
                    return;
                }

                if (event.key === "Enter" && highlightedIndex >= 0) {
                    event.preventDefault();
                    const result = filteredResults[highlightedIndex];
                    if (result.type === 'shortcut') {
                        navigateToShortcut(result);
                    } else {
                        performSearch(result.title);
                    }
                    return;
                }
            }

            if (event.key == "Enter") {
                const input = inputElement.value.trim();

                // Check for exact shortcut match first
                const exactMatch = shortcutsArray.find(s =>
                    s.title.toLowerCase() === input.toLowerCase() ||
                    (s.shortcut && s.shortcut.toLowerCase() === input.toLowerCase())
                );

                if (exactMatch) {
                    navigateToShortcut(exactMatch);
                    return;
                }

                let query;
                let searchUrlTemplate;

                if (currentBang != null) {
                    query = input.slice(currentBang.dataset.shortcut.length + 1);
                    searchUrlTemplate = currentBang.dataset.url;
                } else {
                    query = input;
                    searchUrlTemplate = defaultSearchUrl;
                }

                if (query.length == 0 && currentBang == null) {
                    return;
                }

                const url = searchUrlTemplate.replace("!QUERY!", encodeURIComponent(query));

                if (newTab && !event.ctrlKey || !newTab && event.ctrlKey) {
                    window.open(url, target).focus();
                } else {
                    window.location.href = url;
                }

                lastQuery = query;
                inputElement.value = "";
                hideDropdown();
                return;
            }

            if (event.key == "ArrowUp" && lastQuery.length > 0 && dropdownElement.classList.contains("hidden")) {
                inputElement.value = lastQuery;
                return;
            }
        };

        const changeCurrentBang = (bang) => {
            currentBang = bang;
            bangElement.textContent = bang != null ? bang.dataset.title : "";
        }

        const handleInput = (event) => {
            const value = event.target.value.trim();

            // Check for bangs first
            if (value in bangsMap) {
                changeCurrentBang(bangsMap[value]);
                hideDropdown();
                return;
            }

            const words = value.split(" ");
            if (words.length >= 2 && words[0] in bangsMap) {
                changeCurrentBang(bangsMap[words[0]]);
                hideDropdown();
                return;
            }

            changeCurrentBang(null);

            // Update shortcuts dropdown
            updateDropdown(value);
        };

        // Close dropdown when clicking outside
        document.addEventListener('click', (event) => {
            if (!widget.contains(event.target)) {
                hideDropdown();
            }
        });

        inputElement.addEventListener("focus", () => {
            document.addEventListener("keydown", handleKeyDown);
            document.addEventListener("input", handleInput);
            if (inputElement.value.trim()) {
                updateDropdown(inputElement.value.trim());
            }
        });

        inputElement.addEventListener("blur", (event) => {
            // Delay hiding dropdown to allow for clicks
            setTimeout(() => {
                if (!widget.contains(document.activeElement)) {
                    hideDropdown();
                    document.removeEventListener("keydown", handleKeyDown);
                    document.removeEventListener("input", handleInput);
                }
            }, 150);
        });

        document.addEventListener("keydown", (event) => {
            if (['INPUT', 'TEXTAREA'].includes(document.activeElement.tagName)) return;
            if (event.key != "s") return;

            inputElement.focus();
            event.preventDefault();
        });

        kbdElement.addEventListener("mousedown", () => {
            requestAnimationFrame(() => inputElement.focus());
        });
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

function setupGroups() {
    const groups = document.getElementsByClassName("widget-type-group");

    if (groups.length == 0) {
        return;
    }

    for (let g = 0; g < groups.length; g++) {
        const group = groups[g];
        const titles = group.getElementsByClassName("widget-header")[0].children;
        const tabs = group.getElementsByClassName("widget-group-contents")[0].children;
        let current = 0;

        for (let t = 0; t < titles.length; t++) {
            const title = titles[t];

            if (title.dataset.titleUrl !== undefined) {
                title.addEventListener("mousedown", (event) => {
                    if (event.button != 1) {
                        return;
                    }

                    openURLInNewTab(title.dataset.titleUrl, false);
                    event.preventDefault();
                });
            }

            title.addEventListener("click", () => {
                if (t == current) {
                    if (title.dataset.titleUrl !== undefined) {
                        openURLInNewTab(title.dataset.titleUrl);
                    }

                    return;
                }

                for (let i = 0; i < titles.length; i++) {
                    titles[i].classList.remove("widget-group-title-current");
                    titles[i].setAttribute("aria-selected", "false");
                    tabs[i].classList.remove("widget-group-content-current");
                    tabs[i].setAttribute("aria-hidden", "true");
                }

                if (current < t) {
                    tabs[t].dataset.direction = "right";
                } else {
                    tabs[t].dataset.direction = "left";
                }

                current = t;

                title.classList.add("widget-group-title-current");
                title.setAttribute("aria-selected", "true");
                tabs[t].classList.add("widget-group-content-current");
                tabs[t].setAttribute("aria-hidden", "false");
            });
        }
    }
}

function setupLazyImages() {
    const images = document.querySelectorAll("img[loading=lazy]");

    if (images.length == 0) {
        return;
    }

    function imageFinishedTransition(image) {
        image.classList.add("finished-transition");
    }

    afterContentReady(() => {
        setTimeout(() => {
            for (let i = 0; i < images.length; i++) {
                const image = images[i];

                if (image.complete) {
                    image.classList.add("cached");
                    setTimeout(() => imageFinishedTransition(image), 1);
                } else {
                    // TODO: also handle error event
                    image.addEventListener("load", () => {
                        image.classList.add("loaded");
                        setTimeout(() => imageFinishedTransition(image), 400);
                    });
                }
            }
        }, 1);
    });
}

function attachExpandToggleButton(collapsibleContainer) {
    const showMoreText = "Show more";
    const showLessText = "Show less";

    let expanded = false;
    const button = document.createElement("button");
    const icon = document.createElement("span");
    icon.classList.add("expand-toggle-button-icon");
    const textNode = document.createTextNode(showMoreText);
    button.classList.add("expand-toggle-button");
    button.append(textNode, icon);
    button.addEventListener("click", () => {
        expanded = !expanded;

        if (expanded) {
            collapsibleContainer.classList.add("container-expanded");
            button.classList.add("container-expanded");
            textNode.nodeValue = showLessText;
            return;
        }

        const topBefore = button.getClientRects()[0].top;

        collapsibleContainer.classList.remove("container-expanded");
        button.classList.remove("container-expanded");
        textNode.nodeValue = showMoreText;

        const topAfter = button.getClientRects()[0].top;

        if (topAfter > 0)
            return;

        window.scrollBy({
            top: topAfter - topBefore,
            behavior: "instant"
        });
    });

    collapsibleContainer.after(button);

    return button;
};


function setupCollapsibleLists() {
    const collapsibleLists = document.querySelectorAll(".list.collapsible-container");

    if (collapsibleLists.length == 0) {
        return;
    }

    for (let i = 0; i < collapsibleLists.length; i++) {
        const list = collapsibleLists[i];

        if (list.dataset.collapseAfter === undefined) {
            continue;
        }

        const collapseAfter = parseInt(list.dataset.collapseAfter);

        if (collapseAfter == -1) {
            continue;
        }

        if (list.children.length <= collapseAfter) {
            continue;
        }

        attachExpandToggleButton(list);

        for (let c = collapseAfter; c < list.children.length; c++) {
            const child = list.children[c];
            child.classList.add("collapsible-item");
            child.style.animationDelay = ((c - collapseAfter) * 20).toString() + "ms";
        }
    }
}

function setupCollapsibleGrids() {
    const collapsibleGridElements = document.querySelectorAll(".cards-grid.collapsible-container");

    if (collapsibleGridElements.length == 0) {
        return;
    }

    for (let i = 0; i < collapsibleGridElements.length; i++) {
        const gridElement = collapsibleGridElements[i];

        if (gridElement.dataset.collapseAfterRows === undefined) {
            continue;
        }

        const collapseAfterRows = parseInt(gridElement.dataset.collapseAfterRows);

        if (collapseAfterRows == -1) {
            continue;
        }

        const getCardsPerRow = () => {
            return parseInt(getComputedStyle(gridElement).getPropertyValue('--cards-per-row'));
        };

        const button = attachExpandToggleButton(gridElement);

        let cardsPerRow;

        const resolveCollapsibleItems = () => requestAnimationFrame(() => {
            const hideItemsAfterIndex = cardsPerRow * collapseAfterRows;

            if (hideItemsAfterIndex >= gridElement.children.length) {
                button.style.display = "none";
            } else {
                button.style.removeProperty("display");
            }

            let row = 0;

            for (let i = 0; i < gridElement.children.length; i++) {
                const child = gridElement.children[i];

                if (i >= hideItemsAfterIndex) {
                    child.classList.add("collapsible-item");
                    child.style.animationDelay = (row * 40).toString() + "ms";

                    if (i % cardsPerRow + 1 == cardsPerRow) {
                        row++;
                    }
                } else {
                    child.classList.remove("collapsible-item");
                    child.style.removeProperty("animation-delay");
                }
            }
        });

        const observer = new ResizeObserver(() => {
            if (!isElementVisible(gridElement)) {
                return;
            }

            const newCardsPerRow = getCardsPerRow();

            if (cardsPerRow == newCardsPerRow) {
                return;
            }

            cardsPerRow = newCardsPerRow;
            resolveCollapsibleItems();
        });

        afterContentReady(() => observer.observe(gridElement));
    }
}

const contentReadyCallbacks = [];

function afterContentReady(callback) {
    contentReadyCallbacks.push(callback);
}

const weekDayNames = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];
const monthNames = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];

function makeSettableTimeElement(element, hourFormat) {
    const fragment = document.createDocumentFragment();
    const hour = document.createElement('span');
    const minute = document.createElement('span');
    const amPm = document.createElement('span');
    fragment.append(hour, document.createTextNode(':'), minute);

    if (hourFormat == '12h') {
        fragment.append(document.createTextNode(' '), amPm);
    }

    element.append(fragment);

    return (date) => {
        const hours = date.getHours();

        if (hourFormat == '12h') {
            amPm.textContent = hours < 12 ? 'AM' : 'PM';
            hour.textContent = hours % 12 || 12;
        } else {
            hour.textContent = hours < 10 ? '0' + hours : hours;
        }

        const minutes = date.getMinutes();
        minute.textContent = minutes < 10 ? '0' + minutes : minutes;
    };
};

function timeInZone(now, zone) {
    let timeInZone;

    try {
        timeInZone = new Date(now.toLocaleString('en-US', { timeZone: zone }));
    } catch (e) {
        // TODO: indicate to the user that this is an invalid timezone
        console.error(e);
        timeInZone = now
    }

    const diffInMinutes = Math.round((timeInZone.getTime() - now.getTime()) / 1000 / 60);

    return { time: timeInZone, diffInMinutes: diffInMinutes };
}

function zoneDiffText(diffInMinutes) {
    if (diffInMinutes == 0) {
        return "";
    }

    const sign = diffInMinutes < 0 ? "-" : "+";
    const signText = diffInMinutes < 0 ? "behind" : "ahead";

    diffInMinutes = Math.abs(diffInMinutes);

    const hours = Math.floor(diffInMinutes / 60);
    const minutes = diffInMinutes % 60;
    const hourSuffix = hours == 1 ? "" : "s";

    if (minutes == 0) {
        return { text: `${sign}${hours}h`, title: `${hours} hour${hourSuffix} ${signText}` };
    }

    if (hours == 0) {
        return { text: `${sign}${minutes}m`, title: `${minutes} minutes ${signText}` };
    }

    return { text: `${sign}${hours}h~`, title: `${hours} hour${hourSuffix} and ${minutes} minutes ${signText}` };
}

function setupClocks() {
    const clocks = document.getElementsByClassName('clock');

    if (clocks.length == 0) {
        return;
    }

    const updateCallbacks = [];

    for (var i = 0; i < clocks.length; i++) {
        const clock = clocks[i];
        const hourFormat = clock.dataset.hourFormat;
        const localTimeContainer = clock.querySelector('[data-local-time]');
        const localDateElement = localTimeContainer.querySelector('[data-date]');
        const localWeekdayElement = localTimeContainer.querySelector('[data-weekday]');
        const localYearElement = localTimeContainer.querySelector('[data-year]');
        const timeZoneContainers = clock.querySelectorAll('[data-time-in-zone]');

        const setLocalTime = makeSettableTimeElement(
            localTimeContainer.querySelector('[data-time]'),
            hourFormat
        );

        updateCallbacks.push((now) => {
            setLocalTime(now);
            localDateElement.textContent = now.getDate() + ' ' + monthNames[now.getMonth()];
            localWeekdayElement.textContent = weekDayNames[now.getDay()];
            localYearElement.textContent = now.getFullYear();
        });

        for (var z = 0; z < timeZoneContainers.length; z++) {
            const timeZoneContainer = timeZoneContainers[z];
            const diffElement = timeZoneContainer.querySelector('[data-time-diff]');

            const setZoneTime = makeSettableTimeElement(
                timeZoneContainer.querySelector('[data-time]'),
                hourFormat
            );

            updateCallbacks.push((now) => {
                const { time, diffInMinutes } = timeInZone(now, timeZoneContainer.dataset.timeInZone);
                setZoneTime(time);
                const { text, title } = zoneDiffText(diffInMinutes);
                diffElement.textContent = text;
                diffElement.title = title;
            });
        }
    }

    const updateClocks = () => {
        const now = new Date();

        for (var i = 0; i < updateCallbacks.length; i++)
            updateCallbacks[i](now);

        setTimeout(updateClocks, (60 - now.getSeconds()) * 1000);
    };

    updateClocks();
}

async function setupCalendars() {
    const elems = document.getElementsByClassName("calendar");
    if (elems.length == 0) return;

    // TODO: implement prefetching, currently loads as a nasty waterfall of requests
    const calendar = await import ('./calendar.js');

    for (let i = 0; i < elems.length; i++)
        calendar.default(elems[i]);
}

async function setupTodos() {
    const elems = document.getElementsByClassName("todo");
    if (elems.length == 0) return;

    const todo = await import ('./todo.js');

    for (let i = 0; i < elems.length; i++)
        todo.default(elems[i]);
}

function setupTruncatedElementTitles() {
    const elements = document.querySelectorAll(".text-truncate, .single-line-titles .title, .text-truncate-2-lines, .text-truncate-3-lines");

    if (elements.length == 0) {
        return;
    }

    for (let i = 0; i < elements.length; i++) {
        const element = elements[i];
        if (element.getAttribute("title") === null) element.title = element.innerText;
    }
}

async function changeTheme(key, onChanged) {
    const themeStyleElem = find("#theme-style");

    const response = await fetch(`${pageData.baseURL}/api/set-theme/${key}`, {
        method: "POST",
    });

    if (response.status != 200) {
        alert("Failed to set theme: " + response.statusText);
        return;
    }
    const newThemeStyle = await response.text();

    const tempStyle = elem("style")
        .html("* { transition: none !important; }")
        .appendTo(document.head);

    themeStyleElem.html(newThemeStyle);
    document.documentElement.setAttribute("data-theme", key);
    document.documentElement.setAttribute("data-scheme", response.headers.get("X-Scheme"));
    typeof onChanged == "function" && onChanged();
    setTimeout(() => { tempStyle.remove(); }, 10);
}

function initThemePicker() {
    const themeChoicesInMobileNav = find(".mobile-navigation .theme-choices");
    if (!themeChoicesInMobileNav) return;

    const themeChoicesInHeader = find(".header-container .theme-choices");

    if (themeChoicesInHeader) {
        themeChoicesInHeader.replaceWith(
            themeChoicesInMobileNav.cloneNode(true)
        );
    }

    const presetElems = findAll(".theme-choices .theme-preset");
    let themePreviewElems = document.getElementsByClassName("current-theme-preview");
    let isLoading = false;

    presetElems.forEach((presetElement) => {
        const themeKey = presetElement.dataset.key;

        if (themeKey === undefined) {
            return;
        }

        if (themeKey == pageData.theme) {
            presetElement.classList.add("current");
        }

        presetElement.addEventListener("click", () => {
            if (themeKey == pageData.theme) return;
            if (isLoading) return;

            isLoading = true;
            changeTheme(themeKey, function() {
                isLoading = false;
                pageData.theme = themeKey;
                presetElems.forEach((e) => { e.classList.remove("current"); });

                Array.from(themePreviewElems).forEach((preview) => {
                    preview.querySelector(".theme-preset").replaceWith(
                        presetElement.cloneNode(true)
                    );
                })

                presetElems.forEach((e) => {
                    if (e.dataset.key != themeKey) return;
                    e.classList.add("current");
                });
            });
        });
    })
}

async function setupPage() {
    initThemePicker();

    const pageElement = document.getElementById("page");
    const pageContentElement = document.getElementById("page-content");
    const pageContent = await fetchPageContent(pageData);

    pageContentElement.innerHTML = pageContent;

    try {
        setupPopovers();
        setupClocks()
        await setupCalendars();
        await setupTodos();
        setupCarousels();
        setupSearchBoxes();
        setupCollapsibleLists();
        setupCollapsibleGrids();
        setupGroups();
        setupMasonries();
        setupDynamicRelativeTime();
        setupLazyImages();
    } finally {
        pageElement.classList.add("content-ready");
        pageElement.setAttribute("aria-busy", "false");

        for (let i = 0; i < contentReadyCallbacks.length; i++) {
            contentReadyCallbacks[i]();
        }

        setTimeout(() => {
            setupTruncatedElementTitles();
        }, 50);

        setTimeout(() => {
            document.body.classList.add("page-columns-transitioned");
        }, 300);
    }
}

setupPage();
