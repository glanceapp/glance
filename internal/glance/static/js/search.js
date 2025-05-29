export default function SearchBox(widget) {
    const defaultSearchUrl = widget.dataset.defaultSearchUrl;
    const target = widget.dataset.target || "_blank";
    const newTab = widget.dataset.newTab === "true";
    const suggestionsEnabled = widget.dataset.suggestionsEnabled === "true";
    const widgetId = widget.dataset.widgetId;
    const inputElement = widget.getElementsByClassName("search-input")[0];
    const bangElement = widget.getElementsByClassName("search-bang")[0];
    const dropdownElement = widget.getElementsByClassName("search-shortcuts-dropdown")[0];
    const shortcutsListElement = widget.getElementsByClassName("search-shortcuts-list")[0];
    const kbdElement = widget.getElementsByTagName("kbd")[0];

    const bangsMap = {};
    const shortcutsArray = [];
    let currentBang = null;
    let lastQuery = "";
    let highlightedIndex = -1;
    let filteredResults = [];
    let currentSuggestions = [];
    let debounceTimer = null;

    // Initialize bangs
    const bangs = widget.querySelectorAll(".search-bangs > input");
    for (let j = 0; j < bangs.length; j++) {
        const bang = bangs[j];
        bangsMap[bang.dataset.shortcut] = bang;
    }

    // Initialize shortcuts
    const shortcuts = widget.querySelectorAll(".search-shortcuts > input");
    for (let j = 0; j < shortcuts.length; j++) {
        const shortcut = shortcuts[j];
        shortcutsArray.push({
            title: shortcut.dataset.title,
            url: shortcut.dataset.url,
            alias: shortcut.dataset.alias || ""
        });
    }

    // URL detection function
    function isUrl(input) {
        const trimmed = input.trim();

        // Check for protocol-prefixed URLs
        if (/^https?:\/\/.+/.test(trimmed)) {
            return trimmed;
        }

        // Check for IP addresses with optional port
        if (/^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}(:\d+)?$/.test(trimmed)) {
            return `http://${trimmed}`;
        }

        // Check for domain patterns (including localhost)
        if (/^(www\.)?[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z]{2,})+$/.test(trimmed) ||
            /^localhost(:\d+)?$/.test(trimmed)) {
            return `https://${trimmed}`;
        }

        // Check for domain with port (like example.com:8080)
        if (/^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z]{2,})+:\d+$/.test(trimmed)) {
            return `https://${trimmed}`;
        }

        return null;
    }

    // Fuzzy matching function
    function fuzzyMatch(text, query) {
        const lowerText = text.toLowerCase();
        const lowerQuery = query.toLowerCase();

        // Exact match gets highest priority
        if (lowerText === lowerQuery) return { score: 1000, type: 'exact' };

        // Check for substring matches with position-based scoring
        const substringIndex = lowerText.indexOf(lowerQuery);
        if (substringIndex !== -1) {
            // Higher score for matches at the beginning
            const positionBonus = Math.max(0, 100 - substringIndex * 5);
            return { score: 500 + positionBonus, type: 'contains' };
        }

        // Fuzzy matching with position-aware scoring
        let score = 0;
        let textIndex = 0;
        let consecutiveMatches = 0;
        let firstMatchIndex = -1;

        for (let i = 0; i < lowerQuery.length; i++) {
            const char = lowerQuery[i];
            const foundIndex = lowerText.indexOf(char, textIndex);

            if (foundIndex === -1) return { score: 0, type: 'none' };

            // Track first match position
            if (firstMatchIndex === -1) {
                firstMatchIndex = foundIndex;
            }

            // Bonus for consecutive characters
            if (foundIndex === textIndex) {
                consecutiveMatches++;
                score += 15; // Higher bonus for consecutive chars
            } else {
                consecutiveMatches = 0;
                score += 2;
            }

            // Extra bonus for consecutive streaks
            if (consecutiveMatches > 1) {
                score += consecutiveMatches * 3;
            }

            textIndex = foundIndex + 1;
        }

        // Position bonus: higher score for matches starting earlier
        const positionBonus = Math.max(0, 50 - firstMatchIndex * 3);
        score += positionBonus;

        return { score, type: 'fuzzy' };
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
            const aliasMatch = shortcut.alias ? fuzzyMatch(shortcut.alias, query) : { score: 0, type: 'none' };
            const bestMatch = titleMatch.score > aliasMatch.score ? titleMatch : aliasMatch;

            return {
                ...shortcut,
                type: 'shortcut',
                score: bestMatch.score,
                matchType: bestMatch.type,
                isExact: titleMatch.type === 'exact' || aliasMatch.type === 'exact'
            };
        }).filter(item => item.score > 0)
          .sort((a, b) => b.score - a.score)
          .slice(0, 5);

        // Start with shortcuts
        filteredResults = [...shortcutMatches];
        highlightedIndex = -1;

        renderDropdown();
        if (filteredResults.length > 0) {
            showDropdown();
        } else if (!suggestionsEnabled || !widgetId || currentBang !== null) {
            hideDropdown();
        }

        // Fetch search suggestions if enabled and no bang is active (with debouncing)
        if (suggestionsEnabled && widgetId && currentBang === null) {
            clearTimeout(debounceTimer);
            debounceTimer = setTimeout(() => {
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
            }, 200);
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
                        ${item.alias ? `<div class="search-shortcut-alias">${item.alias}</div>` : ''}
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

    function showErrorIndicator() {
        // Find the widget header in the parent widget container
        const widgetContainer = widget.closest('.widget');
        const widgetHeader = widgetContainer?.querySelector('.widget-header');

        if (!widgetHeader) return;

        // Check if error indicator already exists
        if (widgetHeader.querySelector('.notice-icon-major')) {
            return;
        }

        const errorIndicator = document.createElement('div');
        errorIndicator.className = 'notice-icon notice-icon-major';
        errorIndicator.title = 'Search suggestions service error';
        widgetHeader.appendChild(errorIndicator);
    }

    function hideErrorIndicator() {
        const widgetContainer = widget.closest('.widget');
        const errorIndicator = widgetContainer?.querySelector('.widget-header .notice-icon-major');
        if (errorIndicator) {
            errorIndicator.remove();
        }
    }

    async function fetchSuggestions(query) {
        try {
            const response = await fetch(`/api/search/suggestions?query=${encodeURIComponent(query)}&widget_id=${encodeURIComponent(widgetId)}`, {
                method: 'POST'
            });

            if (!response.ok) {
                showErrorIndicator();
                return [];
            }

            // Clear error indicator on successful response
            hideErrorIndicator();

            const data = await response.json();
            return data.suggestions || [];
        } catch (error) {
            console.error('Failed to fetch suggestions:', error);
            showErrorIndicator();
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

    const changeCurrentBang = (bang) => {
        currentBang = bang;
        bangElement.textContent = currentBang?.dataset.title || "";
    };

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
                (s.alias && s.alias.toLowerCase() === input.toLowerCase())
            );

            if (exactMatch) {
                navigateToShortcut(exactMatch);
                return;
            }

            // Check if input is a URL
            const detectedUrl = isUrl(input);
            if (detectedUrl) {
                if (newTab) {
                    window.open(detectedUrl, target).focus();
                } else {
                    window.location.href = detectedUrl;
                }
                inputElement.value = "";
                hideDropdown();
                return;
            }

            let query;
            let searchUrlTemplate;

            if (currentBang == null) {
                query = input;
                searchUrlTemplate = defaultSearchUrl;
            } else {
                query = input;
                searchUrlTemplate = defaultSearchUrl;
            }

            if (query.length == 0 && currentBang == null) {
                return;
            }

            const url = searchUrlTemplate.replace("!QUERY!", encodeURIComponent(query));

            if (newTab) {
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

    const attachFocusListeners = () => {
        document.addEventListener("keydown", handleKeyDown);
        document.addEventListener("input", handleInput);
        if (inputElement.value.trim()) {
            updateDropdown(inputElement.value.trim());
        }
    };

    const detachFocusListeners = () => {
        hideDropdown();
        document.removeEventListener("keydown", handleKeyDown);
        document.removeEventListener("input", handleInput);
    };

    inputElement.addEventListener("focus", () => {
        attachFocusListeners();
    });

    inputElement.addEventListener("blur", (event) => {
        // Delay hiding dropdown to allow for clicks
        setTimeout(() => {
            if (!widget.contains(document.activeElement)) {
                detachFocusListeners();
            }
        }, 150);
    });

    // Check if input is already focused (e.g., due to autofocus)
    if (document.activeElement === inputElement) {
        attachFocusListeners();
    }

    document.addEventListener("keydown", (event) => {
        if ((event.metaKey || event.ctrlKey) && event.key == "/") {
            event.preventDefault();
            inputElement.focus();
            return;
        }

        if (event.key == kbdElement.textContent.toLowerCase() && !event.metaKey && !event.ctrlKey && !event.altKey && document.activeElement != inputElement) {
            event.preventDefault();
            inputElement.focus();
            return;
        }
    });

    kbdElement.addEventListener("mousedown", () => {
        requestAnimationFrame(() => inputElement.focus());
    });
}