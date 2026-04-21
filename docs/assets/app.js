'use strict';

/* ─── Page metadata ──────────────────────────────────────────────────────── */

// Populated dynamically from docs/manifest.json
let PAGES = {
  home: { title: 'Welcome', label: 'Welcome' },
};

let DOC_PATHS = {};
let ROUTE_ALIASES = new Map();

function normalizeRouteToken(value) {
  return String(value || '')
    .trim()
    .toLowerCase()
    .replace(/^\/+|\/+$/g, '')
    .replace(/\.(md|html)$/i, '')
    .replace(/\s+/g, '-')
    .replace(/[^a-z0-9_\/-]+/g, '-');
}

function registerRouteAlias(aliasMap, alias, pageId) {
  const normalized = normalizeRouteToken(alias);
  if (!normalized) return;
  if (normalized === 'index') return;
  if (!aliasMap.has(normalized)) aliasMap.set(normalized, pageId);
}

function rebuildRouteAliases() {
  const aliases = new Map();

  registerRouteAlias(aliases, 'home', 'home');

  Object.keys(PAGES).forEach(pageId => {
    registerRouteAlias(aliases, pageId, pageId);
    registerRouteAlias(aliases, pageId.replace(/-/g, '_'), pageId);

    const title = PAGES[pageId] && (PAGES[pageId].title || PAGES[pageId].label);
    if (title) registerRouteAlias(aliases, title, pageId);

    const docPath = DOC_PATHS[pageId];
    if (!docPath) return;

    const pathSegments = String(docPath).split('/').filter(Boolean);
    const fileName = pathSegments[pathSegments.length - 1] || '';
    const stem = fileName.replace(/\.(md|html)$/i, '');

    registerRouteAlias(aliases, stem, pageId);
    registerRouteAlias(aliases, stem.replace(/_/g, '-'), pageId);
  });

  ROUTE_ALIASES = aliases;
}

function resolvePageId(rawValue) {
  const normalized = normalizeRouteToken(rawValue);
  if (!normalized) return null;

  const tokens = normalized.split('/').filter(Boolean);
  const candidates = [normalized];
  if (tokens.length > 0) candidates.push(tokens[tokens.length - 1]);

  if (tokens[0] === 'docs' && tokens.length > 1) {
    candidates.push(tokens.slice(1).join('/'));
    candidates.push(tokens[tokens.length - 1]);
  }

  for (const candidate of candidates) {
    const exact = ROUTE_ALIASES.get(candidate);
    if (exact) return exact;

    const hyphen = ROUTE_ALIASES.get(candidate.replace(/_/g, '-'));
    if (hyphen) return hyphen;

    const underscore = ROUTE_ALIASES.get(candidate.replace(/-/g, '_'));
    if (underscore) return underscore;
  }

  return null;
}

function parsePageFromPathname(pathname) {
  const normalizedPath = String(pathname || '')
    .split('?')[0]
    .split('#')[0]
    .replace(/\/+$/g, '');

  if (!normalizedPath || normalizedPath === '/') return null;

  const tokens = normalizedPath.split('/').filter(Boolean);
  if (tokens.length === 0) return null;

  const lastToken = tokens[tokens.length - 1].toLowerCase();
  if (lastToken === 'index' || lastToken === 'index.html') {
    tokens.pop();
  }

  if (tokens.length === 0) return null;

  const pathWithoutLeadingSlash = tokens.join('/');
  const candidates = [
    pathWithoutLeadingSlash,
    tokens[tokens.length - 1],
  ];

  for (const candidate of candidates) {
    const resolved = resolvePageId(candidate);
    if (resolved) return resolved;
  }

  return null;
}

function parseHashRoute(rawHash) {
  const normalizedHash = String(rawHash || '').replace(/^#/, '').trim();
  if (!normalizedHash) return null;

  const hashWithoutNavPrefix = normalizedHash.startsWith('nav:')
    ? normalizedHash.slice(4)
    : normalizedHash;

  const slashIdx = hashWithoutNavPrefix.indexOf('/');
  const rawPageId = slashIdx === -1
    ? hashWithoutNavPrefix
    : hashWithoutNavPrefix.slice(0, slashIdx);
  const resolvedPageId = resolvePageId(rawPageId);
  if (!resolvedPageId) return null;

  const headingHash = slashIdx === -1
    ? null
    : hashWithoutNavPrefix.slice(slashIdx + 1);

  return {
    pageId: resolvedPageId,
    hash: headingHash || null,
  };
}

function parseCurrentRoute(pathname = location.pathname, hash = location.hash) {
  const parsedFromHash = parseHashRoute(hash);
  if (parsedFromHash) return parsedFromHash;

  const pageFromPath = parsePageFromPathname(pathname);
  const rawHash = String(hash || '').replace(/^#/, '').trim();

  // Support URLs like /configuration#getting-started where hash is a heading.
  if (pageFromPath && rawHash) {
    return { pageId: pageFromPath, hash: rawHash };
  }

  return { pageId: pageFromPath || 'home', hash: null };
}

function buildPathCandidates(path) {
  const trimmed = String(path || '').trim();
  if (!trimmed) return [];

  const withoutLeadingSlash = trimmed.replace(/^\/+/, '');
  const withoutDocsPrefix = withoutLeadingSlash.replace(/^docs\//, '');

  const candidates = [
    withoutLeadingSlash,
    withoutDocsPrefix,
    `docs/${withoutDocsPrefix}`,
  ];

  return [...new Set(candidates.filter(Boolean))];
}

async function fetchFirstOkText(pathCandidates, cacheMode) {
  let lastError = null;

  for (const path of pathCandidates) {
    try {
      const res = await fetch(path, { cache: cacheMode });
      if (!res.ok) {
        lastError = new Error(`${path} fetch failed (${res.status})`);
        continue;
      }
      const text = await res.text();
      return { text, path };
    } catch (error) {
      lastError = error;
    }
  }

  throw lastError || new Error('Unable to fetch resource');
}

async function loadManifest() {
  try {
    const { text } = await fetchFirstOkText(
      buildPathCandidates('docs/manifest.json').concat('manifest.json'),
      'default'
    );
    const manifest = JSON.parse(text);

    const newPages = { home: { title: 'Welcome', label: 'Welcome' } };
    const newPaths = {};

    for (const entry of manifest.nav) {
      if (entry.section || !entry.id) continue;
      newPages[entry.id] = { title: entry.title, label: entry.title };
      if (entry.file) newPaths[entry.id] = entry.file;
    }

    PAGES = newPages;
    DOC_PATHS = newPaths;
    rebuildSearchIndex();
    buildNav(manifest.nav);
  } catch (err) {
    console.warn('Could not load manifest.json, using fallback nav', err);
    rebuildSearchIndex();
    buildNavFallback();
  }
}

function buildNav(entries) {
  navEl.innerHTML = '';
  for (const entry of entries) {
    if (entry.section) {
      const label = document.createElement('div');
      label.className = 'nav-label';
      label.textContent = entry.section;
      navEl.appendChild(label);
    } else if (entry.id) {
      const btn = document.createElement('button');
      btn.className = 'nav-item';
      btn.dataset.page = entry.id;
      btn.textContent = entry.title;
      navEl.appendChild(btn);
    }
  }
}

function buildNavFallback() {
  // Keep whatever is already in the static HTML nav
}

function rebuildSearchIndex() {
  docCache.clear();
  allDocsPromise = null;
  SEARCHABLE_PAGE_IDS = Object.keys(DOC_PATHS);

  const lookup = new Map();
  SEARCHABLE_PAGE_IDS.forEach(pageId => {
    lookup.set(normalizeSearchToken(pageId), pageId);
    if (PAGE_NAMES[pageId]) lookup.set(normalizeSearchToken(PAGE_NAMES[pageId]), pageId);
  });

  PAGE_LOOKUP = lookup;
  rebuildRouteAliases();
}

/* ─── State ──────────────────────────────────────────────────────────────── */

let currentPage = 'home';
let forceInstantScroll = false;
let navigationRequestId = 0;
let isHomeCursorTrackingActive = false;
let spinnerTimer = null;

const docCache = new Map();
const pendingDocLoads = new Map();
let allDocsPromise = null;

async function loadDoc(pageId) {
  if (docCache.has(pageId)) return docCache.get(pageId);
  if (pendingDocLoads.has(pageId)) return pendingDocLoads.get(pageId);

  const docPath = DOC_PATHS[pageId];
  if (!docPath) return null;

  const pending = fetchFirstOkText(buildPathCandidates(docPath), 'default')
    .then(({ text }) => {
      docCache.set(pageId, text);
      pendingDocLoads.delete(pageId);
      return text;
    })
    .catch(error => {
      pendingDocLoads.delete(pageId);
      throw error;
    });

  pendingDocLoads.set(pageId, pending);
  return pending;
}

function loadAllDocs() {
  if (allDocsPromise) return allDocsPromise;

  const pageIds = Object.keys(DOC_PATHS);
  allDocsPromise = Promise.all(
    pageIds.map(pageId => loadDoc(pageId).catch(error => {
      console.warn(`Unable to load page \"${pageId}\" for search`, error);
      return null;
    }))
  ).then(() => docCache);

  return allDocsPromise;
}

/* ─── DOM refs ───────────────────────────────────────────────────────────── */

const contentEl      = document.getElementById('content');
const mainEl         = document.querySelector('.main');
const bcPageEl       = document.getElementById('bc-page');
const navEl          = document.getElementById('nav');
const sidebarEl      = document.getElementById('sidebar');
const overlayEl      = document.getElementById('overlay');
const hamburgerEl    = document.getElementById('hamburger');
const mobileNavCloseEl = document.getElementById('mobile-nav-close');
const themeToggleEl  = document.getElementById('theme-toggle');
const themeLabelEl   = document.getElementById('theme-label');
const searchInputEl  = document.getElementById('search-input');
const searchResultsEl = document.getElementById('search-results');
const goToTopEl      = document.getElementById('go-to-top');

/* ─── Scrolling helpers ──────────────────────────────────────────────────── */

function getHeadingOffset() {
  const raw = getComputedStyle(document.documentElement).getPropertyValue('--topbar-height');
  const topbar = Number.parseFloat(raw) || 56;
  return topbar + 16;
}

function scrollToElementFast(el, options = {}) {
  if (!el) return;

  const { instant = false } = options;

  const scrollRoot = mainEl || document.scrollingElement || document.documentElement;
  const rootRectTop = mainEl ? mainEl.getBoundingClientRect().top : 0;
  const currentY = mainEl ? mainEl.scrollTop : window.scrollY;

  const targetY = Math.max(0, el.getBoundingClientRect().top - rootRectTop + currentY - getHeadingOffset());
  const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  if (reduceMotion || instant) {
    if (mainEl) {
      mainEl.scrollTo(0, targetY);
    } else {
      window.scrollTo(0, targetY);
    }
    return;
  }

  const startY = mainEl ? mainEl.scrollTop : window.scrollY;
  const delta = targetY - startY;
  if (Math.abs(delta) < 2) return;

  const distance = Math.abs(delta);
  const duration = Math.min(520, Math.max(240, distance * 0.45));
  const start = performance.now();
  const easeInOutCubic = t => (t < 0.5)
    ? 4 * t * t * t
    : 1 - Math.pow(-2 * t + 2, 3) / 2;

  function frame(now) {
    const t = Math.min(1, (now - start) / duration);
    const nextY = Math.round(startY + delta * easeInOutCubic(t));
    if (mainEl) {
      scrollRoot.scrollTo(0, nextY);
    } else {
      window.scrollTo(0, nextY);
    }
    if (t < 1) requestAnimationFrame(frame);
  }

  requestAnimationFrame(frame);
}

function slugifyHeading(text) {
  return text
    .trim()
    .toLowerCase()
    .replace(/[^\w\s-]/g, '')
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-');
}

function buildUniqueHeadingId(baseId, usedIds) {
  const normalizedBase = (baseId || 'section').trim() || 'section';
  if (!usedIds.has(normalizedBase)) {
    usedIds.add(normalizedBase);
    return normalizedBase;
  }

  let n = 2;
  let candidate = `${normalizedBase}-${n}`;
  while (usedIds.has(candidate)) {
    n += 1;
    candidate = `${normalizedBase}-${n}`;
  }

  usedIds.add(candidate);
  return candidate;
}

/* ─── Theme ──────────────────────────────────────────────────────────────── */

function applyTheme(theme) {
  document.documentElement.setAttribute('data-theme', theme);
  themeLabelEl.textContent = theme === 'dark' ? 'Light mode' : 'Dark mode';
  localStorage.setItem('theme', theme);
}

function toggleTheme() {
  const current = document.documentElement.getAttribute('data-theme');
  applyTheme(current === 'dark' ? 'light' : 'dark');
}

const savedTheme = localStorage.getItem('theme') || 'dark';
applyTheme(savedTheme);
themeToggleEl.addEventListener('click', toggleTheme);

/* ─── Mobile sidebar ─────────────────────────────────────────────────────── */

function openSidebar() {
  sidebarEl.classList.add('open');
  overlayEl.classList.add('active');
  hamburgerEl.classList.add('active');
  document.body.style.overflow = 'hidden';
}

function closeSidebar() {
  sidebarEl.classList.remove('open');
  overlayEl.classList.remove('active');
  hamburgerEl.classList.remove('active');
  document.body.style.overflow = '';
}

hamburgerEl.addEventListener('click', () => {
  sidebarEl.classList.contains('open') ? closeSidebar() : openSidebar();
});

overlayEl.addEventListener('click', closeSidebar);

if (mobileNavCloseEl) {
  mobileNavCloseEl.addEventListener('click', closeSidebar);
}

let touchStartX = null;
let touchStartY = null;
const TOUCH_EDGE_OPEN_RATIO = 0.25;
const TOUCH_EDGE_CLOSE_RATIO = 0.25;
const MIN_TOUCH_EDGE_OPEN_WIDTH = 28;
const MIN_TOUCH_EDGE_CLOSE_WIDTH = 28;
const MIN_SWIPE_DISTANCE = 60;

function isMobileSidebarMode() {
  return window.matchMedia('(max-width: 768px)').matches;
}

function getTouchEdgeOpenWidth() {
  return Math.max(MIN_TOUCH_EDGE_OPEN_WIDTH, window.innerWidth * TOUCH_EDGE_OPEN_RATIO);
}

function getTouchEdgeCloseWidth() {
  return Math.max(MIN_TOUCH_EDGE_CLOSE_WIDTH, window.innerWidth * TOUCH_EDGE_CLOSE_RATIO);
}

document.addEventListener('touchstart', e => {
  if (e.touches.length !== 1) return;
  const { clientX, clientY } = e.touches[0];
  touchStartX = clientX;
  touchStartY = clientY;
});

document.addEventListener('touchmove', e => {
  if (touchStartX === null) return;
  const { clientX, clientY } = e.touches[0];
  const dx = clientX - touchStartX;
  const dy = clientY - touchStartY;
  const touchEdgeOpenWidth = getTouchEdgeOpenWidth();
  const touchEdgeCloseWidth = getTouchEdgeCloseWidth();

  if (isMobileSidebarMode() && touchStartX <= touchEdgeOpenWidth && dx > MIN_SWIPE_DISTANCE && Math.abs(dy) < 40) {
    openSidebar();
    touchStartX = null;
    touchStartY = null;
    return;
  }

  if (
    isMobileSidebarMode() &&
    sidebarEl.classList.contains('open') &&
    touchStartX >= (window.innerWidth - touchEdgeCloseWidth) &&
    dx < -MIN_SWIPE_DISTANCE &&
    Math.abs(dy) < 40
  ) {
    closeSidebar();
    touchStartX = null;
    touchStartY = null;
    return;
  }

  if (Math.abs(dy) > 40 || dx < -MIN_SWIPE_DISTANCE) {
    touchStartX = null;
    touchStartY = null;
  }
});

document.addEventListener('touchend', () => {
  touchStartX = null;
  touchStartY = null;
});

/* ─── Go to Top ──────────────────────────────────────────────────────────── */

if (mainEl && goToTopEl) {
  mainEl.addEventListener('scroll', () => {
    if (mainEl.scrollTop > 300) {
      goToTopEl.classList.add('visible');
    } else {
      goToTopEl.classList.remove('visible');
    }
  });

  goToTopEl.addEventListener('click', () => {
    scrollToElementFast(contentEl, { instant: false });
  });
} else if (goToTopEl) {
  window.addEventListener('scroll', () => {
    if (window.scrollY > 300) {
      goToTopEl.classList.add('visible');
    } else {
      goToTopEl.classList.remove('visible');
    }
  });

  goToTopEl.addEventListener('click', () => {
    window.scrollTo({ top: 0, behavior: 'smooth' });
  });
}

/* ─── Navigation ─────────────────────────────────────────────────────────── */

function setActiveNav(pageId) {
  navEl.querySelectorAll('.nav-item').forEach(el => {
    el.classList.toggle('active', el.dataset.page === pageId);
  });
}

function handleHomeCursorMove(e) {
  const leftEye = document.getElementById('eye-left');
  const rightEye = document.getElementById('eye-right');
  if (!leftEye || !rightEye) return;

  const wrapper = leftEye.parentElement;
  const wrapperRect = wrapper.getBoundingClientRect();
  const wrapperCenterX = wrapperRect.left + wrapperRect.width / 2;
  const wrapperCenterY = wrapperRect.top + wrapperRect.height / 2;

  // Calculate angle from wrapper center to cursor
  const deltaX = e.clientX - wrapperCenterX;
  const deltaY = e.clientY - wrapperCenterY;
  const angle = Math.atan2(deltaY, deltaX);

  // Both eyes look at same angle, but movement is limited
  const maxDist = 8;
  const dist = Math.min(maxDist, Math.hypot(deltaX, deltaY) / 50);

  const moveX = Math.cos(angle) * dist;
  const moveY = Math.sin(angle) * dist;

  // Both eyes move together in the same direction
  leftEye.style.transform = `translate(${moveX}px, ${moveY}px)`;
  rightEye.style.transform = `translate(${moveX}px, ${moveY}px)`;
}

function setHomeCursorTracking(enabled) {
  if (enabled) {
    if (!isHomeCursorTrackingActive) {
      document.addEventListener('mousemove', handleHomeCursorMove);
      isHomeCursorTrackingActive = true;
    }
    return;
  }

  if (isHomeCursorTrackingActive) {
    document.removeEventListener('mousemove', handleHomeCursorMove);
    isHomeCursorTrackingActive = false;
  }
}

async function navigateTo(pageId, hash, skipPushState) {
  const requestId = ++navigationRequestId;
  clearTimeout(spinnerTimer);
  spinnerTimer = setTimeout(() => {
    if (requestId !== navigationRequestId) return;
    contentEl.innerHTML = '<div class="page-loading"><div class="page-spinner" aria-hidden="true"></div></div>';
  }, 250);

  pageId = pageId || 'home';
  const isNotFound = pageId !== 'home' && !PAGES[pageId];

  if (isNotFound) {
    const unknownPageId = pageId;
    setActiveNav('');
    bcPageEl.textContent = '404';
    document.title = 'Page Not Found - Dynacat';
    const mainContainer = document.querySelector('.main');
    if (mainContainer) mainContainer.classList.remove('is-home');
    setHomeCursorTracking(false);
    if (!skipPushState) {
      history.pushState({ pageId: unknownPageId }, '', `#${unknownPageId}`);
    }
    destroyToc();
    render404(unknownPageId);
    closeSidebar();
    return;
  }

  currentPage = pageId;
  setActiveNav(pageId);
  bcPageEl.textContent = PAGES[pageId].label;
  document.title = `${PAGES[pageId].title} - Dynacat`;

  // Toggle home-specific layout (hide topbar, full-width)
  const mainContainer = document.querySelector('.main');
  const isHomePage = pageId === 'home';
  if (mainContainer) mainContainer.classList.toggle('is-home', isHomePage);
  setHomeCursorTracking(isHomePage);

  if (!skipPushState) {
    const newHash = pageId === 'home'
      ? ''
      : hash ? `#${pageId}/${hash}` : `#${pageId}`;
    history.pushState({ pageId, hash }, '', newHash || location.pathname);
  }

  if (pageId === 'home') {
    destroyToc();
    await renderHome();
    if (mainEl) {
      mainEl.scrollTo(0, 0);
    } else {
      window.scrollTo(0, 0);
    }
  } else {
    if (contentEl.childElementCount === 0 || contentEl.querySelector('.home-welcome')) {
      contentEl.innerHTML = '';
    }
    await renderDoc(pageId, hash, requestId);
  }

  closeSidebar();
}

function parseLocationHash() {
  return parseCurrentRoute();
}

/* ─── Markdown rendering ─────────────────────────────────────────────────── */

function parseMarkdown(md) {
  // Preprocess: fix internal links
  md = md.replace(/\(([a-z-]+)\.md(#[^\)]+)?\)/g, (_, page, hash) => {
    return `(#nav:${page}${hash || ''})`;
  });

  // Preprocess: fix dynacat.yml link
  md = md.replace(/\(dynacat\.yml\)/g, '(docs/dynacat.yml)');

  return marked.parse(md);
}

function fixImagePaths(container) {
  container.querySelectorAll('img').forEach(img => {
    const src = img.getAttribute('src');
    if (src && !src.startsWith('http') && !src.startsWith('data:') && !src.startsWith('docs/')) {
      img.setAttribute('src', `docs/${src}`);
    }
  });
}

function addHeadingIds(container) {
  const usedIds = new Set();
  container.querySelectorAll('h1, h2, h3, h4, h5, h6').forEach(h => {
    const baseId = h.id || slugifyHeading(h.textContent);
    h.id = buildUniqueHeadingId(baseId, usedIds);

    // Add anchor link
    const anchor = document.createElement('a');
    anchor.className = 'heading-link';
    anchor.href = `#${h.id}`;
    anchor.textContent = '#';
    anchor.title = 'Link to section';
    anchor.addEventListener('click', e => {
      e.preventDefault();
      scrollToElementFast(h);
      const hashStr = currentPage === 'home' ? `#${h.id}` : `#${currentPage}/${h.id}`;
      history.pushState({ pageId: currentPage, hash: h.id }, '', hashStr);
    });
    h.appendChild(anchor);
  });
}

function processCallouts(container) {
  const calloutStyleAliases = {
    note: 'note',
    info: 'note',
    abstract: 'note',
    summary: 'note',
    tldr: 'note',

    tip: 'tip',
    hint: 'tip',
    success: 'tip',
    check: 'tip',

    caution: 'warning',
    warning: 'warning',
    attention: 'warning',

    important: 'important',
    danger: 'important',
    error: 'important',
    bug: 'important',
    failure: 'important',
    fail: 'important',
  };

  container.querySelectorAll('blockquote').forEach(bq => {
    const firstP = bq.querySelector('p:first-child');
    if (!firstP) return;

    const markerPattern = /^\s*\[!([A-Z0-9_-]+)\]\s*/i;
    const match = firstP.innerHTML.match(markerPattern);
    if (!match) {
      bq.classList.add('callout', 'callout-quote');
      bq.dataset.calloutType = 'quote';
      return;
    }

    const rawType = match[1];
    const normalizedType = rawType.toLowerCase().replace(/_/g, '-');
    const styleType = calloutStyleAliases[normalizedType] || 'note';

    firstP.innerHTML = firstP.innerHTML.replace(markerPattern, '');
    if (firstP.textContent.trim().length === 0) {
      firstP.remove();
    }

    bq.className = `callout callout-${styleType}`;
    bq.dataset.calloutType = normalizedType;

    const titleDiv = document.createElement('div');
    titleDiv.className = 'callout-title';
    titleDiv.textContent = rawType.replace(/_/g, ' ').replace(/-/g, ' ');
    bq.insertBefore(titleDiv, bq.firstChild);
  });
}

function highlightCode(container) {
  container.querySelectorAll('pre code').forEach(block => {
    // Determine language
    const cls = block.className || '';
    const langMatch = cls.match(/language-(\w+)/);
    const lang = langMatch ? langMatch[1] : '';

    hljs.highlightElement(block);

    // Add header to pre
    const pre = block.parentElement;
    const header = document.createElement('div');
    header.className = 'code-header';

    const langEl = document.createElement('span');
    langEl.className = 'code-lang';
    langEl.textContent = lang || 'code';

    const copyBtn = document.createElement('button');
    copyBtn.className = 'code-copy';
    copyBtn.textContent = 'Copy';
    copyBtn.addEventListener('click', () => {
      const text = block.textContent;
      navigator.clipboard.writeText(text).then(() => {
        copyBtn.textContent = 'Copied!';
        copyBtn.classList.add('copied');
        setTimeout(() => {
          copyBtn.textContent = 'Copy';
          copyBtn.classList.remove('copied');
        }, 2000);
      }).catch(() => {
        // Fallback for file:// protocol
        const ta = document.createElement('textarea');
        ta.value = text;
        ta.style.position = 'fixed';
        ta.style.opacity = '0';
        document.body.appendChild(ta);
        ta.select();
        document.execCommand('copy');
        document.body.removeChild(ta);
        copyBtn.textContent = 'Copied!';
        copyBtn.classList.add('copied');
        setTimeout(() => {
          copyBtn.textContent = 'Copy';
          copyBtn.classList.remove('copied');
        }, 2000);
      });
    });

    header.appendChild(langEl);
    header.appendChild(copyBtn);
    pre.insertBefore(header, block);
  });
}

function wrapTables(container) {
  container.querySelectorAll('table').forEach(table => {
    if (table.closest('.table-scroll')) return;

    const wrapper = document.createElement('div');
    wrapper.className = 'table-scroll';
    table.parentNode.insertBefore(wrapper, table);
    wrapper.appendChild(table);
  });
}

function clearSearchHighlights(container) {
  if (!container) return;
  container.querySelectorAll('.search-hit-flash').forEach(el => {
    const parent = el.parentNode;
    if (!parent) return;
    while (el.firstChild) {
      parent.insertBefore(el.firstChild, el);
    }
    parent.removeChild(el);
  });
}

function normalizeHighlightPhrase(value) {
  return (value || '')
    .toLowerCase()
    .replace(/\s+/g, ' ')
    .trim();
}

function extractQueryPhrase(query) {
  if (!query) return '';

  const tokens = query.trim().split(/\s+/).filter(Boolean);
  const textTokens = tokens
    .filter(token => {
      const lower = token.toLowerCase();
      return !lower.startsWith('$in:') && !lower.startsWith('$has:');
    })
    .map(token => token.replace(/^\W+|\W+$/g, ''))
    .filter(Boolean);

  return normalizeHighlightPhrase(textTokens.join(' '));
}

function collectSearchTerms(query) {
  const queryPhrase = extractQueryPhrase(query);
  const phrases = [queryPhrase]
    .filter(Boolean)
    .filter(phrase => phrase.length >= 3);

  return [...new Set(phrases)].sort((a, b) => b.length - a.length);
}

function shouldSkipSearchHighlight(node) {
  const parent = node.parentElement;
  if (!parent) return true;
  return !!parent.closest('script, style');
}

function flashTermInScope(scope, term) {
  const needle = term.toLowerCase();
  const walker = document.createTreeWalker(scope, NodeFilter.SHOW_TEXT, {
    acceptNode(node) {
      if (!node.nodeValue) return NodeFilter.FILTER_REJECT;
      if (shouldSkipSearchHighlight(node)) return NodeFilter.FILTER_REJECT;
      return NodeFilter.FILTER_ACCEPT;
    },
  });

  const textNodes = [];
  let node = walker.nextNode();
  while (node) {
    textNodes.push(node);
    node = walker.nextNode();
  }

  if (textNodes.length === 0) return [];

  let fullText = '';
  const nodeMetas = textNodes.map(n => {
    const start = fullText.length;
    fullText += n.nodeValue.toLowerCase();
    return { node: n, start, end: fullText.length, orig: n.nodeValue };
  });

  const parts = needle.split(/\s+/).map(p => p.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'));
  const needleRegex = new RegExp(parts.join('[\\W]+'), 'g');
  
  const matches = [];
  let match;
  while ((match = needleRegex.exec(fullText)) !== null) {
    if (match[0].length === 0) break;
    matches.push({ start: match.index, end: match.index + match[0].length });
  }

  const highlights = [];
  const nodeChunks = new Map();
  textNodes.forEach(n => nodeChunks.set(n, []));

  matches.forEach(m => {
    nodeMetas.forEach(meta => {
      if (m.end <= meta.start || m.start >= meta.end) return;
      const startInNode = Math.max(0, m.start - meta.start);
      const endInNode = Math.min(meta.end - meta.start, m.end - meta.start);
      nodeChunks.get(meta.node).push({ start: startInNode, end: endInNode });
    });
  });

  for (const meta of nodeMetas) {
    const chunks = nodeChunks.get(meta.node);
    if (chunks.length === 0) continue;

    chunks.sort((a, b) => a.start - b.start);

    const frag = document.createDocumentFragment();
    let last = 0;
    const val = meta.orig;

    for (const c of chunks) {
      if (c.start > last) {
        frag.appendChild(document.createTextNode(val.slice(last, c.start)));
      }
      const span = document.createElement('span');
      span.className = 'search-hit-flash';
      span.textContent = val.slice(c.start, c.end);
      frag.appendChild(span);
      highlights.push(span);
      last = c.end;
    }
    if (last < val.length) {
      frag.appendChild(document.createTextNode(val.slice(last)));
    }

    meta.node.parentNode.replaceChild(frag, meta.node);
  }

  return highlights;
}

function getSectionSearchScopes(wrapper, hash) {
  if (!hash) return [wrapper];

  const heading = document.getElementById(hash);
  if (!heading || !wrapper.contains(heading)) return [wrapper];

  const levelMatch = heading.tagName.match(/^H([1-6])$/);
  if (!levelMatch) return [heading, wrapper];

  const sectionLevel = Number(levelMatch[1]);
  const sectionNodes = [heading];
  let cursor = heading.nextElementSibling;

  while (cursor) {
    const headingMatch = cursor.tagName.match(/^H([1-6])$/);
    if (headingMatch && Number(headingMatch[1]) <= sectionLevel) break;
    sectionNodes.push(cursor);
    cursor = cursor.nextElementSibling;
  }

  return [...sectionNodes, wrapper];
}

function findSnippetHighlight(highlights, snippet) {
  if (!snippet || highlights.length <= 1) return highlights[0];

  const normalizedSnippet = snippet.toLowerCase().replace(/\s+/g, ' ').trim().slice(0, 80);

  for (const h of highlights) {
    const block = h.closest('p, li, td, blockquote, pre, div, dt, dd, h1, h2, h3, h4, h5, h6');
    if (!block) continue;
    const blockText = block.textContent.toLowerCase().replace(/\s+/g, ' ').trim();
    if (blockText.includes(normalizedSnippet)) return h;
  }

  return highlights[0];
}

function flashSearchHighlight(container, hash, query, snippet) {
  if (!container) return false;

  clearSearchHighlights(container);
  const terms = collectSearchTerms(query);
  if (terms.length === 0) return false;

  const scopes = getSectionSearchScopes(container, hash);

  const allHighlights = [];
  for (const scope of scopes) {
    for (const term of terms) {
      allHighlights.push(...flashTermInScope(scope, term));
    }
    if (allHighlights.length > 0) break;
  }

  if (allHighlights.length === 0) return false;

  const HIGHLIGHT_LIFETIME_MS = 3000;
  const HIGHLIGHT_FADEOUT_MS = 320;

  // Scroll to the highlight matching the selected result's snippet.
  const scrollTarget = findSnippetHighlight(allHighlights, snippet);
  scrollToElementFast(scrollTarget);

  // Images (and other late-loading content) above the target may not be laid out yet,
  // causing getBoundingClientRect() to undercount the scroll distance.
  // Re-scroll after images load (with an rAF to ensure reflow) and again at 600ms
  // as a catch-all for fonts, syntax highlighting, and other layout shifts.
  const reScroll = () => requestAnimationFrame(() => {
    if (scrollTarget.isConnected) scrollToElementFast(scrollTarget, { instant: true });
  });

  const pendingImages = [...container.querySelectorAll('img')].filter(img => !img.complete);
  if (pendingImages.length > 0) {
    Promise.all(pendingImages.map(img => new Promise(r => {
      img.addEventListener('load', r, { once: true });
      img.addEventListener('error', r, { once: true });
    }))).then(reScroll);
  }

  setTimeout(reScroll, 600);

  // Start enter animation after paint so text is visible first.
  requestAnimationFrame(() => {
    requestAnimationFrame(() => {
      allHighlights.forEach(highlightEl => {
        if (!highlightEl.isConnected) return;
        highlightEl.classList.add('search-hit-enter');
      });
    });
  });

  setTimeout(() => {
    allHighlights.forEach(highlightEl => {
      if (!highlightEl.isConnected) return;
      highlightEl.classList.remove('search-hit-enter');
      highlightEl.classList.add('search-hit-fadeout');
    });
  }, HIGHLIGHT_LIFETIME_MS - HIGHLIGHT_FADEOUT_MS);

  setTimeout(() => {
    allHighlights.forEach(highlightEl => {
      if (!highlightEl.isConnected) return;
      const parent = highlightEl.parentNode;
      if (!parent) return;
      while (highlightEl.firstChild) {
        parent.insertBefore(highlightEl.firstChild, highlightEl);
      }
      parent.removeChild(highlightEl);
    });
  }, HIGHLIGHT_LIFETIME_MS);

  return true;
}

function handleContentLinks(container) {
  container.addEventListener('click', e => {
    const a = e.target.closest('a');
    if (!a) return;

    const href = a.getAttribute('href');
    if (!href) return;

    // Internal doc navigation (preprocessed)
    if (href.startsWith('#nav:')) {
      e.preventDefault();
      const rest = href.slice(5);
      const [page, hash] = rest.split('#');
      navigateTo(page, hash);
      return;
    }

    // Anchor links on current page
    if (href.startsWith('#') && !href.startsWith('#nav:')) {
      e.preventDefault();
      const target = document.getElementById(href.slice(1));
      if (target) scrollToElementFast(target);
      return;
    }

    // Same-origin links can map to docs pages (e.g. /configuration, /docs/configuration).
    const resolvedUrl = new URL(href, location.href);
    if (resolvedUrl.origin === location.origin) {
      const route = parseCurrentRoute(resolvedUrl.pathname, resolvedUrl.hash);
      if (route.pageId !== 'home' || route.hash) {
        e.preventDefault();
        navigateTo(route.pageId, route.hash);
        return;
      }
    }

    // External links
    if (href.startsWith('http://') || href.startsWith('https://')) {
      e.preventDefault();
      window.open(href, '_blank', 'noopener noreferrer');
    }
  });
}

/* ─── Floating Table of Contents ─────────────────────────────────────────── */

let tocObserver = null;
let tocScrollTarget = null;
let tocScrollHandler = null;

function destroyToc() {
  if (tocObserver) { tocObserver.disconnect(); tocObserver = null; }
  if (tocScrollTarget && tocScrollHandler) {
    tocScrollTarget.removeEventListener('scroll', tocScrollHandler);
  }
  tocScrollTarget = null;
  tocScrollHandler = null;
  const existing = document.getElementById('toc-float');
  if (existing) existing.remove();
}

function setFloatingTocActive(hash) {
  if (!hash) return;
  const toc = document.getElementById('toc-float');
  if (!toc) return;

  const links = toc.querySelectorAll('.toc-link');
  let hasMatch = false;
  links.forEach(link => {
    const isActive = link.getAttribute('href') === `#${hash}`;
    link.classList.toggle('toc-link--active', isActive);
    if (isActive) hasMatch = true;
  });

  if (!hasMatch) return;

  const activeLink = toc.querySelector('.toc-link--active');
  if (activeLink && typeof activeLink.scrollIntoView === 'function' && window.innerWidth >= 1280) {
    activeLink.scrollIntoView({ block: 'nearest' });
  }
}

function buildFloatingToc(wrapper) {
  const headingCandidates = [...wrapper.querySelectorAll('h2, h3, h4')];
  if (headingCandidates.length === 0) {
    headingCandidates.push(...wrapper.querySelectorAll('h1'));
  }

  const tocEntries = headingCandidates
    .map(heading => {
      const headingClone = heading.cloneNode(true);
      headingClone.querySelectorAll('.heading-link, code').forEach(node => node.remove());
      const label = (headingClone.textContent || '').trim().replace(/\s+/g, ' ');

      return {
        id: heading.id,
        label,
        level: Number(heading.tagName.slice(1)),
      };
    })
    .filter(entry => {
      if (!entry.id || !entry.label) return false;

      const normalizedLabel = entry.label.toLowerCase();
      // Skip repetitive subheadings that add noise in long config docs.
      if (normalizedLabel === 'properties' || normalizedLabel.startsWith('properties for ')) {
        return false;
      }

      return true;
    });

  if (tocEntries.length <= 1) return;

  const baseLevel = Math.min(...tocEntries.map(entry => entry.level));

  // Some docs include a hand-written TOC list at the top; remove it if detected.
  const firstUl = wrapper.querySelector('ul');
  const firstHeading = wrapper.querySelector('h1, h2, h3, h4, h5, h6');
  if (firstUl && firstHeading) {
    const listBeforeHeading = !!(firstUl.compareDocumentPosition(firstHeading) & Node.DOCUMENT_POSITION_FOLLOWING);
    if (listBeforeHeading) {
      const links = [...firstUl.querySelectorAll('a[href^="#"]')];
      const nonAnchorLinks = firstUl.querySelectorAll('a:not([href^="#"])').length;
      if (links.length >= 2 && nonAnchorLinks === 0) {
        firstUl.remove();
      }
    }
  }

  // Build TOC nav
  const toc = document.createElement('nav');
  toc.id = 'toc-float';
  toc.className = 'toc-float';
  toc.innerHTML = '<div class="toc-title">On this page</div>';

  const ul = document.createElement('ul');
  ul.className = 'toc-list';

  tocEntries.forEach(entry => {
    const depth = Math.max(0, entry.level - baseLevel);
    const li = document.createElement('li');
    li.className = 'toc-item' + (depth > 0 ? ' toc-item--nested' : '');
    li.style.setProperty('--toc-depth', String(depth));

    const link = document.createElement('a');
    link.className = 'toc-link';
    link.href = `#${entry.id}`;
    link.textContent = entry.label;
    link.addEventListener('click', e => {
      e.preventDefault();
      const id = link.getAttribute('href').slice(1);
      const target = document.getElementById(id);
      if (target) {
        scrollToElementFast(target);
        const hashStr = currentPage === 'home' ? `#${id}` : `#${currentPage}/${id}`;
        history.pushState({ pageId: currentPage, hash: id }, '', hashStr);
      }
    });

    li.appendChild(link);
    ul.appendChild(li);
  });

  toc.appendChild(ul);

  // Prepend to wrapper so it flows naturally on top for mobile, 
  // while desktop styling uses position: fixed to pull it out of flow.
  wrapper.insertBefore(toc, wrapper.firstChild);

  // Scrollspy via position sampling + IntersectionObserver nudges.
  const headingEls = tocEntries.map(entry => document.getElementById(entry.id)).filter(Boolean);

  function setActive(id) {
    ul.querySelectorAll('.toc-link').forEach(l => {
      l.classList.toggle('toc-link--active', l.getAttribute('href') === `#${id}`);
    });
  }

  function findActiveHeadingId() {
    if (headingEls.length === 0) return null;

    if (!mainEl) {
      let bestId = headingEls[0].id;
      for (const heading of headingEls) {
        if (heading.getBoundingClientRect().top <= 130) {
          bestId = heading.id;
        } else {
          break;
        }
      }
      return bestId;
    }

    const rootRect = mainEl.getBoundingClientRect();
    const cutoff = rootRect.top + 120;
    let bestId = headingEls[0].id;

    for (const heading of headingEls) {
      if (heading.getBoundingClientRect().top <= cutoff) {
        bestId = heading.id;
      } else {
        break;
      }
    }

    return bestId;
  }

  if (headingEls.length) {
    tocScrollTarget = mainEl || window;
    tocScrollHandler = () => {
      const activeId = findActiveHeadingId();
      if (activeId) setActive(activeId);
    };
    tocScrollTarget.addEventListener('scroll', tocScrollHandler, { passive: true });

    tocObserver = new IntersectionObserver(entries => {
      if (!entries.some(entry => entry.isIntersecting)) return;
      const activeId = findActiveHeadingId();
      if (activeId) setActive(activeId);
    }, {
      root: mainEl || null,
      rootMargin: '0px 0px -60% 0px',
      threshold: 0,
    });

    headingEls.forEach(h => tocObserver.observe(h));
    const initialId = findActiveHeadingId();
    if (initialId) setActive(initialId);
  }
}

async function renderDoc(pageId, hash, requestId) {
  let md;
  try {
    md = await loadDoc(pageId);
  } catch (error) {
    if (requestId !== navigationRequestId) return;
    contentEl.innerHTML = '<div class="md-content"><p>Failed to load page.</p></div>';
    console.error(error);
    return;
  }

  if (requestId !== navigationRequestId) return;

  if (!md) {
    contentEl.innerHTML = '<div class="md-content"><p>Page not found.</p></div>';
    return;
  }

  const wrapper = document.createElement('div');
  wrapper.className = 'md-content';
  wrapper.innerHTML = parseMarkdown(md);

  fixImagePaths(wrapper);
  processCallouts(wrapper);
  addHeadingIds(wrapper);
  highlightCode(wrapper);
  handleContentLinks(wrapper);

  clearTimeout(spinnerTimer);
  destroyToc();
  contentEl.innerHTML = '';
  contentEl.appendChild(wrapper);
  buildFloatingToc(wrapper);
  wrapTables(wrapper);

  // Scroll to hash if provided
  if (hash) {
    setTimeout(() => {
      if (requestId !== navigationRequestId) return;
      const target = document.getElementById(hash);
      if (target) {
        scrollToElementFast(target, { instant: forceInstantScroll });
        setFloatingTocActive(hash);
      }
      forceInstantScroll = false;
    }, 80);
  } else {
    if (mainEl) {
      mainEl.scrollTo(0, 0);
    } else {
      window.scrollTo(0, 0);
    }
    forceInstantScroll = false;
  }
}

/* ─── 404 page ───────────────────────────────────────────────────────────── */

function render404(unknownPageId) {
  contentEl.innerHTML = `
    <div class="not-found">
      <div class="not-found-code">404</div>
      <h1 class="not-found-title">Page not found</h1>
      <p class="not-found-desc">
        The page you're looking for doesn't exist or may have been moved.
      </p>
    </div>
  `;
  if (mainEl) mainEl.scrollTo(0, 0);
  else window.scrollTo(0, 0);
}

/* ─── Home page ──────────────────────────────────────────────────────────── */

async function getVersionTag() {
  const cacheKey = 'version_tag_cache';
  const cacheTTL = 1 * 24 * 60 * 60 * 1000; // 1 day in milliseconds

  try {
    const cached = localStorage.getItem(cacheKey);
    if (cached) {
      const { data, timestamp } = JSON.parse(cached);
      if (Date.now() - timestamp < cacheTTL) {
        return data;
      }
    }
  } catch (e) {
    // Cache read failed, continue with fetch
  }

  try {
    const res = await fetch('https://api.github.com/repos/panonim/dynacat/releases/latest');
    if (res.ok) {
      const data = await res.json();
      const versionTag = `v${data.tag_name}`;

      // Store in cache
      try {
        localStorage.setItem(cacheKey, JSON.stringify({
          data: versionTag,
          timestamp: Date.now()
        }));
      } catch (e) {}

      return versionTag;
    }
  } catch (e) {
    // Fetch failed
  }

  return ''; // fallback
}

async function getGitHubStargazers() {
  const cacheKey = 'github_stargazers_cache';
  const cacheTTL = 2 * 24 * 60 * 60 * 1000; // 2 days in milliseconds

  try {
    const cached = localStorage.getItem(cacheKey);
    if (cached) {
      const { data, timestamp } = JSON.parse(cached);
      if (Date.now() - timestamp < cacheTTL) {
        return data;
      }
    }
  } catch (e) {
    // Cache read failed, continue with fetch
  }

  try {
    const res = await fetch('https://api.github.com/repos/panonim/dynacat');
    if (res.ok) {
      const data = await res.json();
      const stargazersCount = data.stargazers_count;

      // Store in cache
      try {
        localStorage.setItem(cacheKey, JSON.stringify({
          data: stargazersCount,
          timestamp: Date.now()
        }));
      } catch (e) {
        // Cache write failed, just return the data
      }

      return stargazersCount;
    }
  } catch (e) {
    // Fetch failed
  }

  return 0; // fallback
}

async function getDockerPullCount() {
  const cacheKey = 'docker_pull_count_cache';
  const cacheTTL = 2 * 24 * 60 * 60 * 1000; // 2 days in milliseconds

  try {
    const cached = localStorage.getItem(cacheKey);
    if (cached) {
      const { data, timestamp } = JSON.parse(cached);
      if (Date.now() - timestamp < cacheTTL) {
        return data;
      }
    }
  } catch (e) {
    // Cache read failed, continue with fetch
  }

  // Try multiple CORS proxy services
  const proxies = [
    'https://corsproxy.io/?https://hub.docker.com/v2/repositories/panonim/dynacat/',
    'https://api.allorigins.win/raw?url=https://hub.docker.com/v2/repositories/panonim/dynacat/',
  ];

  for (const proxyUrl of proxies) {
    try {
      const res = await fetch(proxyUrl, { signal: AbortSignal.timeout(5000) });
      if (res.ok) {
        const data = await res.json();
        const pullCount = data.pull_count || 0;

        // Store in cache
        try {
          localStorage.setItem(cacheKey, JSON.stringify({
            data: pullCount,
            timestamp: Date.now()
          }));
        } catch (e) {}

        return pullCount;
      }
    } catch (e) {
      // Try next proxy
      continue;
    }
  }

  return 0; // fallback
}

async function renderHome() {
  let versionTag = '';
  let stargazersCount = 0;
  let pullCount = 0;

  // Helper: check if cache is expired
  function isCacheExpired(cacheKey, cacheTTL) {
    try {
      const cached = localStorage.getItem(cacheKey);
      if (!cached) return true;
      const { timestamp } = JSON.parse(cached);
      return Date.now() - timestamp >= cacheTTL;
    } catch (e) {
      return true;
    }
  }

  // Try to get cached values
  try {
    const cached = localStorage.getItem('github_stargazers_cache');
    if (cached) {
      const { data } = JSON.parse(cached);
      stargazersCount = data;
    }
  } catch (e) {}

  try {
    const cached = localStorage.getItem('docker_pull_count_cache');
    if (cached) {
      const { data } = JSON.parse(cached);
      pullCount = data;
    }
  } catch (e) {}

  try {
    const cached = localStorage.getItem('version_tag_cache');
    if (cached) {
      const { data } = JSON.parse(cached);
      versionTag = data;
    }
  } catch (e) {}

  // Fetch version tag in background only if cache expired
  const versionCacheTTL = 1 * 24 * 60 * 60 * 1000;
  if (isCacheExpired('version_tag_cache', versionCacheTTL)) {
    getVersionTag().then(tag => {
      if (tag && tag !== versionTag) {
        const versionEl = document.querySelector('.home-version');
        if (versionEl) versionEl.textContent = tag;
      }
    }).catch(e => {});
  }

  // Fetch GitHub stargazers in background only if cache expired
  const stargazersCacheTTL = 2 * 24 * 60 * 60 * 1000;
  if (isCacheExpired('github_stargazers_cache', stargazersCacheTTL)) {
    getGitHubStargazers().then(count => {
      if (count > 0 && count !== stargazersCount) {
        const el = document.querySelector('[data-stat="stargazers"]');
        if (el) el.textContent = count;
      }
    }).catch(e => {});
  }

  // Fetch Docker pull count in background only if cache expired
  const dockerCacheTTL = 2 * 24 * 60 * 60 * 1000;
  if (isCacheExpired('docker_pull_count_cache', dockerCacheTTL)) {
    getDockerPullCount().then(count => {
      if (count > 0 && count !== pullCount) {
        const el = document.querySelector('[data-stat="docker-pulls"]');
        if (el) el.textContent = count;
      }
    }).catch(e => {});
  }

  const features = [
    {
      icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"><path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.92-10.26l-3.8 3.8"/></svg>`,
      title: 'Dynamic Updates',
      desc: 'Content refreshes automatically on a configurable schedule - your dashboard is always current without manual intervention.'
    },
    {
      icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22v-5"/><path d="M9 8V2"/><path d="M15 8V2"/><path d="M18 8v5a4 4 0 0 1-4 4h-4a4 4 0 0 1-4-4V8Z"/></svg>`,
      title: 'External Integrations',
      desc: 'Compared to original, Dynacat offers easy integration with services like Jellyfin, Plex, qBittorrent and more.'
    },
    {
      icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"><rect width="7" height="7" x="3" y="3" rx="1"/><rect width="7" height="7" x="14" y="3" rx="1"/><rect width="7" height="7" x="14" y="14" rx="1"/><rect width="7" height="7" x="3" y="14" rx="1"/></svg>`,
      title: 'Widget Library',
      desc: 'Pre-built widgets for every use-case - from system stats to Docker containers - all configured in YAML.'
    },
    {
      icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"><path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/><line x1="10" y1="9" x2="8" y2="9"/></svg>`,
      title: 'YAML Configuration',
      desc: 'One clean, readable file controls everything. No databases, no UI builders - just config and deploy.'
    },
    {
      icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"><path d="m19 11-8-8-8.6 8.6a2 2 0 0 0 0 2.8l5.2 5.2c.8.8 2 .8 2.8 0L19 11Z"/><path d="m5 2 5 5"/><path d="M2 13h15"/><path d="M22 20a2 2 0 1 1-4 0c0-1.6 1.7-2.4 2-4 .3 1.6 2 2.4 2 4Z"/></svg>`,
      title: 'Themeable',
      desc: 'Ship with beautiful dark and light themes. Full color customisation with CSS variables and pre-built palettes.'
    },
    {
      icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"><line x1="16.5" x2="7.5" y1="9.4" y2="4.21"/><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/><polyline points="3.27 6.96 12 12.01 20.73 6.96"/><line x1="12" x2="12" y1="22.08" y2="12"/></svg>`,
      title: 'Docker Native',
      desc: 'Single binary under 20MB. Runs anywhere with Docker or natively - minimal memory, sub-second page loads.'
    },
  ];

  const docs = [
    { page: 'configuration', title: 'Configuration', desc: 'Full reference for config options and widgets', icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H7a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V7z"/><path d="M14 2v5h5"/><path d="M9 12h6"/><path d="M9 16h6"/></svg>` },
    { page: 'docker-options', title: 'Docker Options', desc: 'Environment variables and deployment', icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="4" width="18" height="6" rx="1.5"/><rect x="3" y="14" width="18" height="6" rx="1.5"/><path d="M8 7h.01"/><path d="M8 17h.01"/><path d="M12 7h6"/><path d="M12 17h6"/></svg>` },
    { page: 'themes', title: 'Themes', desc: 'Browse dark and light theme options', icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z"/></svg>` },
    { page: 'preconfigured-pages', title: 'Preconfigured Pages', desc: 'Ready-to-use page layouts', icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="4" width="18" height="16" rx="2"/><path d="M3 10h18"/><path d="M8 10v10"/></svg>` },
    { page: 'custom-api', title: 'Custom API', desc: 'Build widgets from any HTTP JSON API', icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"><path d="M8 6L3 12l5 6"/><path d="M16 6l5 6-5 6"/><path d="M13 4l-2 16"/></svg>` },
    { page: 'extensions', title: 'Extensions', desc: 'Extend Dynacat with custom HTML and scripts', icon: `<svg fill="currentColor" width="800px" height="800px" viewBox="0 0 32 32" version="1.1" xmlns="http://www.w3.org/2000/svg"><path d="M16 32.005c-0.634 0-1.231-0.246-1.68-0.693l-13.641-13.64c-0.923-0.927-0.923-2.436-0.002-3.361l4.997-4.996c0.308-0.309 0.782-0.382 1.17-0.179 0.386 0.202 0.598 0.632 0.522 1.062-0.051 0.286-0.074 0.515-0.074 0.723 0 2.188 1.78 4.005 3.969 4.005 2.191 0 3.79-1.817 3.79-4.005 0-2.191-1.599-3.971-3.79-3.971-0.205 0-0.433 0.024-0.719 0.074-0.434 0.080-0.86-0.135-1.062-0.521-0.202-0.388-0.129-0.862 0.179-1.17l4.659-4.66c0.902-0.898 2.463-0.896 3.361-0.002l3.411 3.413c0.712-2.213 2.79-3.82 5.235-3.82 3.032 0 5.499 2.468 5.499 5.501 0 2.446-1.605 4.523-3.82 5.234l3.314 3.312c0.925 0.928 0.925 2.436 0.001 3.363l-13.639 13.64c-0.451 0.45-1.047 0.696-1.681 0.696zM5.465 12.351l-3.372 3.371c-0.145 0.146-0.145 0.389 0.002 0.537l13.636 13.637c0.191 0.189 0.342 0.192 0.537-0.002l13.636-13.637c0.146-0.148 0.146-0.387-0.001-0.536l-4.809-4.806c-0.301-0.301-0.379-0.76-0.194-1.143s0.589-0.61 1.016-0.557l0.152 0.020c0.084 0.011 0.168 0.025 0.256 0.025 1.93 0 3.499-1.569 3.499-3.497 0-1.931-1.57-3.501-3.499-3.501s-3.498 1.571-3.498 3.501c0 0.080 0.012 0.158 0.023 0.236l0.021 0.178c0.045 0.422-0.18 0.826-0.563 1.009-0.38 0.181-0.838 0.104-1.137-0.196l-4.904-4.907c-0.192-0.19-0.342-0.192-0.537 0.002l-3.035 3.035c2.603 0.644 4.356 2.999 4.356 5.798 0 3.291-2.498 6.004-5.79 6.004-2.798-0-5.152-1.972-5.795-4.572z"></path></svg>` }
  ];

  const featureCards = features.map((f) => `
    <div class="home-card">
      <div class="home-card-icon">${f.icon}</div>
      <div class="home-card-title">${f.title}</div>
      <div class="home-card-desc">${f.desc}</div>
    </div>
  `).join('');

  const docCards = docs.map((d) => `
    <button class="home-doc-card" data-page="${d.page}">
      <div class="home-doc-icon">${d.icon}</div>
      <div class="home-doc-body">
        <div class="home-doc-title">${d.title}</div>
        <div class="home-doc-desc">${d.desc}</div>
      </div>
      <svg class="home-doc-arrow" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M5 12h14"/><path d="M12 5l7 7-7 7"/></svg>
    </button>
  `).join('');

  clearTimeout(spinnerTimer);
  contentEl.innerHTML = `
    <div class="home-welcome">

      <!-- Hero -->
      <div class="home-hero">
        <div class="home-hero-inner">
          <div class="home-hero-row" style="display: flex; align-items: center; gap: 56px; width: 100%;">
            <div class="home-hero-copy">
              <h1 class="home-title">Your dashboard, <span class="home-title-accent">your way.</span></h1>
              <p class="home-subtitle">
                A Glance fork focused on dynamic reloading and easy app integration
                without writing custom widgets. Aggregate RSS, weather, markets, system
                stats, and more into a single self-hosted dashboard.
              </p>
              <div class="home-actions">
                <button class="home-btn-primary" data-page="installation">
                  Get Started
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M5 12h14"/><path d="M12 5l7 7-7 7"/></svg>
                </button>
                <span class="home-version">${versionTag}</span>
              </div>
            </div>
            <div class="home-logo-wrapper" style="position: relative; width: 280px; height: 280px; flex-shrink: 0; margin-left: auto; margin-top: -40px;">
              <img src="assets/model/model_noeyes.svg" alt="Dynacat" style="position: absolute; top: 0; left: 0; width: 100%; height: 100%;">
              <img src="assets/model/eyeright.svg" alt="" id="eye-left" style="position: absolute; width: 3.152%; height: 7.0%; top: 52.94%; left: 19.51%; transition: transform 0.1s ease-out; transform-origin: center;">
              <img src="assets/model/eyeright.svg" alt="" id="eye-right" style="position: absolute; width: 4.456%; height: 7.444%; top: 53.83%; left: 56.14%; transition: transform 0.1s ease-out; transform-origin: center;">
            </div>
          </div>
        </div>
        <div class="home-hero-glow"></div>
      </div>

      <!-- What is Dynacat -->
      <section class="home-about">
        <div class="home-section-label">About</div>
        <h2 class="home-section-heading">What is Dynacat?</h2>
        <div class="home-about-grid">
          <div class="home-about-text">
            <p>
              Dynacat is a self-hosted dashboard built for people who want their
              information in one place. Forked from <strong>Glance</strong>, it focuses on
              <em>dynamic content updates</em> and <em>seamless integration with
              external applications</em> - without requiring you to write custom widgets.
            </p>
            <p>
              Configure everything in a single YAML file, deploy with Docker, and
              let Dynacat handle the rest. Widgets auto-refresh on configurable
              schedules, so your dashboard stays current without manual intervention.
            </p>
          </div>
          <div class="home-about-stats">
            <div class="home-stat">
              <div class="home-stat-value" data-stat="stargazers">${stargazersCount || '...'}</div>
              <div class="home-stat-label">Stars on Github</div>
            </div>
            <div class="home-stat">
              <div class="home-stat-value" data-stat="docker-pulls">${pullCount || '...'}</div>
              <div class="home-stat-label">Docker hub pulls</div>
            </div>
            <div class="home-stat">
              <div class="home-stat-value">&lt;1s</div>
              <div class="home-stat-label">Page load</div>
            </div>
          </div>
        </div>
      </section>

      <!-- Documentation -->
      <section class="home-docs-section">
        <div class="home-section-label">Quick Start</div>
        <h2 class="home-section-heading">Explore the Documentation</h2>
        <div class="home-doc-grid">${docCards}</div>
      </section>

      <!-- Community -->
      <section class="home-community-section">
        <div class="home-community-grid">
          <a class="home-community-card" href="https://github.com/Panonim/dynacat" target="_blank" rel="noopener">
            <div class="home-community-icon">
              <svg viewBox="0 0 16 16" fill="currentColor" width="20" height="20"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/></svg>
            </div>
            <span class="home-community-label">GitHub</span>
          </a>
          <a class="home-community-card home-community-card--discord" href="https://discord.com/invite/mUqTzrfjFP" target="_blank" rel="noopener">
            <div class="home-community-icon home-community-icon--discord">
              <svg viewBox="0 0 24 24" fill="currentColor" width="20" height="20"><path d="M20.317 4.37a19.791 19.791 0 0 0-4.885-1.515.074.074 0 0 0-.079.037c-.21.375-.444.864-.608 1.25a18.27 18.27 0 0 0-5.487 0 12.64 12.64 0 0 0-.617-1.25.077.077 0 0 0-.079-.037A19.736 19.736 0 0 0 3.677 4.37a.07.07 0 0 0-.032.027C.533 9.046-.32 13.58.099 18.057a.082.082 0 0 0 .031.057 19.9 19.9 0 0 0 5.993 3.03.078.078 0 0 0 .084-.028c.462-.63.874-1.295 1.226-1.994a.076.076 0 0 0-.041-.106 13.107 13.107 0 0 1-1.872-.892.077.077 0 0 1-.008-.128 10.2 10.2 0 0 0 .372-.292.074.074 0 0 1 .077-.01c3.928 1.793 8.18 1.793 12.062 0a.074.074 0 0 1 .078.01c.12.098.246.198.373.292a.077.077 0 0 1-.006.127 12.299 12.299 0 0 1-1.873.892.077.077 0 0 0-.041.107c.36.698.772 1.362 1.225 1.993a.076.076 0 0 0 .084.028 19.839 19.839 0 0 0 6.002-3.03.077.077 0 0 0 .032-.054c.5-5.177-.838-9.674-3.549-13.66a.061.061 0 0 0-.031-.03zM8.02 15.33c-1.183 0-2.157-1.086-2.157-2.419 0-1.333.956-2.419 2.157-2.419 1.21 0 2.176 1.095 2.157 2.42 0 1.332-.956 2.418-2.157 2.418zm7.975 0c-1.183 0-2.157-1.086-2.157-2.419 0-1.333.955-2.419 2.157-2.419 1.21 0 2.176 1.095 2.157 2.42 0 1.332-.946 2.418-2.157 2.418z"/></svg>
            </div>
            <span class="home-community-label">Discord</span>
          </a>
          <a class="home-community-card home-community-card--paypal" href="https://paypal.me/imartur" target="_blank" rel="noopener">
            <div class="home-community-icon home-community-icon--paypal">
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 30 30" width="20" height="20"><path d="M 7.0605469 3 L 6.890625 3.7871094 C 5.7016624 9.2479375 3.6254717 18.772869 3.1191406 21.123047 L 2.859375 22.332031 L 9.0507812 22.332031 L 8.3027344 25.789062 L 8.0410156 27 L 14.269531 27 L 15.611328 20.818359 L 15.615234 20.791016 C 15.636144 20.657946 15.818727 20.3125 16.251953 20.3125 L 19.037109 20.3125 C 22.964605 20.3125 26.033464 17.949799 26.826172 14.244141 C 27.206892 12.461829 26.958544 10.794863 25.96875 9.5625 L 25.966797 9.5605469 C 25.181609 8.5834265 24.021371 7.9626769 22.751953 7.7460938 L 22.746094 7.7460938 L 22.740234 7.7441406 C 22.740234 7.7441406 22.096706 7.7226549 21.791016 7.7011719 L 21.791016 7.6796875 L 21.789062 7.6621094 C 21.696016 6.1144528 20.953798 4.8776 19.953125 4.109375 C 18.952452 3.34115 17.731428 3 16.566406 3 L 7.0605469 3 z M 8.671875 5 L 16.566406 5 C 17.328384 5 18.143298 5.2415375 18.734375 5.6953125 C 19.300581 6.1299939 19.687687 6.7308496 19.773438 7.6679688 L 12.773438 7.6679688 C 12.457437 7.6679688 12.205719 7.8836875 12.136719 8.1796875 L 12.134766 8.1796875 L 11.072266 13.033203 L 11.0625 13.03125 L 9.4824219 20.332031 L 5.3378906 20.332031 C 5.9673617 17.422949 7.5530735 10.1381 8.671875 5 z M 21.613281 9.6757812 C 22.121649 9.694727 22.420091 9.7203931 22.421875 9.7207031 C 23.258178 9.8651652 23.974812 10.27449 24.408203 10.814453 C 24.969043 11.511889 25.16621 12.435467 24.869141 13.826172 L 24.871094 13.826172 C 24.24976 16.730513 22.225613 18.3125 19.037109 18.3125 L 16.251953 18.3125 C 14.825179 18.3125 13.815761 19.353539 13.638672 20.480469 L 12.65625 25 L 10.519531 25 L 10.675781 24.277344 L 11.529297 20.332031 L 12.539062 15.662109 L 13.826172 15.662109 C 17.611668 15.662109 20.730896 13.612438 21.613281 9.6757812 z" fill="currentColor"/></svg>
            </div>
            <span class="home-community-label">Sponsor</span>
          </a>
        </div>
      </section>

      <!-- Footer -->
      <footer class="home-footer">
        <div class="home-footer-inner">
          <span class="home-footer-credit">Made with <span style="color: rgb(199, 41, 41);">❤︎</span> by <a href="https://artur.zone" target="_blank" rel="noopener">Artur Flis</a></span>
        </div>
      </footer>

    </div>
  `;

  // Wire up navigation buttons
  contentEl.querySelectorAll('[data-page]').forEach(el => {
    el.addEventListener('click', () => navigateTo(el.dataset.page));
  });
}

/* ─── Search ─────────────────────────────────────────────────────────────── */

const PAGE_NAMES = new Proxy({}, {
  get(_, pageId) {
    return (PAGES[pageId] && (PAGES[pageId].title || PAGES[pageId].label)) || undefined;
  },
  has(_, pageId) {
    return pageId in PAGES;
  },
});

let SEARCHABLE_PAGE_IDS = [];

function escapeHtml(str) {
  return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

function highlightMatch(text, query) {
  const escaped = escapeHtml(text);
  const regex = new RegExp(`(${query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')})`, 'gi');
  return escaped.replace(regex, '<mark>$1</mark>');
}

function normalizeSearchToken(value) {
  return value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

let PAGE_LOOKUP = new Map();

function parseSearchQuery(rawQuery) {
  if (!rawQuery || rawQuery.trim().length < 2) return null;

  const includedPages = new Set();
  const hasFeatures = new Set();
  const textTerms = [];
  const tokens = rawQuery.trim().split(/\s+/);

  tokens.forEach(token => {
    const lower = token.toLowerCase();

    if (lower.startsWith('$in:')) {
      const rawValue = token.slice(4);
      rawValue.split(/[\/,|+]/).forEach(part => {
        const pageId = PAGE_LOOKUP.get(normalizeSearchToken(part));
        if (pageId) includedPages.add(pageId);
      });
      return;
    }

    if (lower.startsWith('$has:')) {
      const rawValue = token.slice(5).toLowerCase();
      rawValue.split(/[\/,|+]/).forEach(part => {
        if (part === 'link' || part === 'code' || part === 'image') {
          hasFeatures.add(part);
        }
      });
      return;
    }

    textTerms.push(token);
  });

  return {
    rawQuery: rawQuery.trim(),
    textQuery: textTerms.join(' ').trim().toLowerCase(),
    includedPages: [...includedPages],
    hasFeatures: [...hasFeatures],
  };
}

function lineHasFeatureNearby(lines, lineIndex, feature, inCodeFence) {
  const start = Math.max(0, lineIndex - 2);
  const end = Math.min(lines.length - 1, lineIndex + 2);
  const nearby = lines.slice(start, end + 1).join('\n');

  if (feature === 'link') {
    return /\[[^\]]+\]\([^)]+\)/.test(nearby) || /\bhttps?:\/\/\S+/i.test(nearby);
  }

  if (feature === 'image') {
    return /!\[[^\]]*\]\([^)]+\)/.test(nearby) || /<img\b/i.test(nearby);
  }

  if (feature === 'code') {
    return inCodeFence || /`[^`]+`/.test(nearby) || nearby.includes('```') || /^\s{4}\S/m.test(nearby);
  }

  return false;
}

function performSearch(parsedQuery, docsByPage) {
  if (!parsedQuery) return [];

  const q = parsedQuery.textQuery;
  const results = [];

  for (const pageId of SEARCHABLE_PAGE_IDS) {
    const content = docsByPage.get(pageId);
    if (!content) continue;

    if (parsedQuery.includedPages.length > 0 && !parsedQuery.includedPages.includes(pageId)) continue;

    const lines = content.split('\n');
    const usedHeadingIds = new Set();
    let currentHeading = PAGE_NAMES[pageId] || pageId;
    let currentHeadingId = '';
    let inCodeFence = false;

    for (let i = 0; i < lines.length; i++) {
      const line = lines[i];
      const trimmedLine = line.trim();

      if (trimmedLine.startsWith('```')) {
        inCodeFence = !inCodeFence;
        continue;
      }

      const headingMatch = line.match(/^#{1,4}\s+(.+)/);
      if (headingMatch) {
        currentHeading = headingMatch[1].trim();
        currentHeadingId = buildUniqueHeadingId(slugifyHeading(currentHeading), usedHeadingIds);

        if (q && currentHeading.toLowerCase().includes(q) && parsedQuery.hasFeatures.length === 0) {
          results.push({
            pageId,
            pageName: PAGE_NAMES[pageId],
            heading: currentHeading,
            headingId: currentHeadingId || '',
            snippet: currentHeading.slice(0, 120),
          });

          if (results.length >= 12) return results;
        }

        continue;
      }

      if (trimmedLine.length < 8) continue;
      if (q && line.startsWith('    ')) continue;
      if (q && !line.toLowerCase().includes(q)) continue;

      if (parsedQuery.hasFeatures.length > 0) {
        const hasExpectedFeature = parsedQuery.hasFeatures.some(feature => {
          return lineHasFeatureNearby(lines, i, feature, inCodeFence);
        });
        if (!hasExpectedFeature) continue;
      }

      results.push({
        pageId,
        pageName: PAGE_NAMES[pageId],
        heading: currentHeading,
        headingId: currentHeadingId || '',
        snippet: trimmedLine.slice(0, 120),
      });

      if (results.length >= 12) return results;
    }
  }

  return results;
}

let searchRequestId = 0;

async function renderSearchResults(query) {
  const requestId = ++searchRequestId;

  if (!query || query.trim().length < 2) {
    searchResultsEl.hidden = true;
    return;
  }

  const parsedQuery = parseSearchQuery(query);
  await loadAllDocs();
  if (requestId !== searchRequestId) return;

  const results = performSearch(parsedQuery, docCache);
  searchResultsEl.hidden = false;

  if (results.length === 0) {
    searchResultsEl.innerHTML = `<div class="search-empty">No results for "${escapeHtml(query)}"</div>`;
    return;
  }

  searchResultsEl.innerHTML = results.map((r, index) => `
    <button class="search-result" data-page="${r.pageId}" data-hash="${escapeHtml(r.headingId || '')}" data-snippet="${escapeHtml(r.snippet)}" data-result-index="${index}">
      <div class="sr-page">${escapeHtml(r.pageName)}</div>
      <div class="sr-heading">${escapeHtml(r.heading)}</div>
      <div class="sr-snippet">${parsedQuery.textQuery ? highlightMatch(r.snippet, parsedQuery.textQuery) : escapeHtml(r.snippet)}</div>
    </button>
  `).join('');

  searchResultsEl.querySelectorAll('.search-result').forEach(el => {
    el.addEventListener('click', async () => {
      searchResultsEl.hidden = true;
      forceInstantScroll = true;

      const targetHash = el.dataset.hash || null;
      const targetSnippet = el.dataset.snippet || '';

      await navigateTo(el.dataset.page, targetHash);
      // Wait for navigateTo's internal setTimeout(80) heading scroll to fire first,
      // so our scroll to the matched text wins.
      await new Promise(resolve => setTimeout(resolve, 100));

      const wrapper = contentEl.querySelector('.md-content');
      const rawQuery = query || '';
      if (targetHash) setFloatingTocActive(targetHash);
      flashSearchHighlight(wrapper, targetHash, rawQuery, targetSnippet);
      searchInputEl.value = '';
    });
  });
}

let searchTimeout;
searchInputEl.addEventListener('input', () => {
  clearTimeout(searchTimeout);
  searchTimeout = setTimeout(() => renderSearchResults(searchInputEl.value), 180);
});

searchInputEl.addEventListener('keydown', e => {
  if (e.key === 'Escape') {
    searchInputEl.value = '';
    searchResultsEl.hidden = true;
    searchInputEl.blur();
  }
});

document.addEventListener('click', e => {
  if (!e.target.closest('.search-wrap') && !e.target.closest('.search-results')) {
    searchResultsEl.hidden = true;
  }
});

/* ─── Nav click delegation ───────────────────────────────────────────────── */

navEl.addEventListener('click', e => {
  const btn = e.target.closest('.nav-item');
  if (btn && btn.dataset.page) navigateTo(btn.dataset.page);
});

/* ─── Logo click ─────────────────────────────────────────────────────────── */

document.querySelector('.site-logo').addEventListener('click', () => navigateTo('home'));

/* ─── Keyboard shortcut: / to focus search ───────────────────────────────── */

document.addEventListener('keydown', e => {
  if (e.key === 'Escape' && sidebarEl.classList.contains('open')) {
    closeSidebar();
  }

  if (e.key === '/' && document.activeElement !== searchInputEl) {
    const tag = document.activeElement.tagName;
    if (tag !== 'INPUT' && tag !== 'TEXTAREA') {
      e.preventDefault();
      searchInputEl.focus();
    }
  }
});

window.addEventListener('resize', () => {
  if (!isMobileSidebarMode()) {
    closeSidebar();
  }
});

/* ─── Browser back/forward ───────────────────────────────────────────────── */

window.addEventListener('popstate', () => {
  const { pageId, hash } = parseLocationHash();
  navigateTo(pageId, hash, true);
});

/* ─── Init ───────────────────────────────────────────────────────────────── */

loadManifest().then(() => {
  const { pageId: initPage, hash: initHash } = parseLocationHash();
  navigateTo(initPage, initHash, true);
});

