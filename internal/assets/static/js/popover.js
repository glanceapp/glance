const defaultShowDelayMs = 200;
const defaultHideDelayMs = 500;
const defaultMaxWidth = "300px";
const defaultDistanceFromTarget = "0px"
const htmlContentSelector = "[data-popover-html]";

let activeTarget = null;
let pendingTarget = null;
let cleanupOnHidePopover = null;
let togglePopoverTimeout = null;

const containerElement = document.createElement("div");
const containerComputedStyle = getComputedStyle(containerElement);
containerElement.addEventListener("mouseenter", clearTogglePopoverTimeout);
containerElement.addEventListener("mouseleave", handleMouseLeave);
containerElement.classList.add("popover-container");

const frameElement = document.createElement("div");
frameElement.classList.add("popover-frame");

const contentElement = document.createElement("div");
contentElement.classList.add("popover-content");

frameElement.append(contentElement);
containerElement.append(frameElement);
document.body.append(containerElement);

const observer = new ResizeObserver(repositionContainer);

function handleMouseEnter(event) {
    clearTogglePopoverTimeout();
    const target = event.target;
    pendingTarget = target;
    const showDelay = target.dataset.popoverShowDelay || defaultShowDelayMs;

    if (activeTarget !== null) {
        if (activeTarget !== target) {
            hidePopover();
            setTimeout(showPopover, 5);
        }

        return;
    }

    togglePopoverTimeout = setTimeout(showPopover, showDelay);
}

function handleMouseLeave(event) {
    clearTogglePopoverTimeout();
    const target = activeTarget || event.target;
    togglePopoverTimeout = setTimeout(hidePopover, target.dataset.popoverHideDelay || defaultHideDelayMs);
}

function clearTogglePopoverTimeout() {
    clearTimeout(togglePopoverTimeout);
}

function showPopover() {
    activeTarget = pendingTarget;
    pendingTarget = null;

    const popoverType = activeTarget.dataset.popoverType;
    const contentMaxWidth = activeTarget.dataset.popoverMaxWidth || defaultMaxWidth;

    if (popoverType === "text") {
        const text = activeTarget.dataset.popoverText;
        if (text === undefined || text === "") return;
        contentElement.textContent = text;
    } else if (popoverType === "html") {
        const htmlContent = activeTarget.querySelector(htmlContentSelector);
        if (htmlContent === null) return;
        /**
         * The reason for all of the below shenanigans is that I want to preserve
         * all attached event listeners of the original HTML content. This is so I don't have to
         * re-setup events for things like lazy images, they'd just work as expected.
         */
        const placeholder = document.createComment("");
        htmlContent.replaceWith(placeholder);
        contentElement.replaceChildren(htmlContent);
        htmlContent.removeAttribute("data-popover-html");
        cleanupOnHidePopover = () => {
            htmlContent.setAttribute("data-popover-html", "");
            placeholder.replaceWith(htmlContent);
            placeholder.remove();
        };
    } else {
        return;
    }

    contentElement.style.maxWidth = contentMaxWidth;
    containerElement.style.display = "block";
    activeTarget.classList.add("popover-active");
    document.addEventListener("keydown", handleHidePopoverOnEscape);
    window.addEventListener("resize", repositionContainer);
    observer.observe(containerElement);
}

function repositionContainer() {
    const activeTargetBounds = activeTarget.getBoundingClientRect();
    const containerBounds = containerElement.getBoundingClientRect();
    const containerInlinePadding = parseInt(containerComputedStyle.getPropertyValue("padding-inline"));
    const activeTargetBoundsWidthOffset = activeTargetBounds.width * (activeTarget.dataset.popoverOffset || 0.5);
    const left = activeTargetBounds.left + activeTargetBoundsWidthOffset - (containerBounds.width / 2);

    if (left < 0) {
        containerElement.style.left = 0;
        containerElement.style.removeProperty("right");
        containerElement.style.setProperty("--triangle-offset", activeTargetBounds.left - containerInlinePadding + activeTargetBoundsWidthOffset + "px");
    } else if (left + containerBounds.width > window.innerWidth) {
        containerElement.style.removeProperty("left");
        containerElement.style.right = 0;
        containerElement.style.setProperty("--triangle-offset", containerBounds.width - containerInlinePadding - (window.innerWidth - activeTargetBounds.left - activeTargetBoundsWidthOffset) + "px");
    } else {
        containerElement.style.removeProperty("right");
        containerElement.style.left = left + "px";
        containerElement.style.removeProperty("--triangle-offset");
    }

    frameElement.style.marginTop = activeTarget.dataset.popoverMargin || defaultDistanceFromTarget;
    containerElement.style.top = activeTargetBounds.top + window.scrollY + activeTargetBounds.height + "px";
}

function hidePopover() {
    if (activeTarget === null) return;

    activeTarget.classList.remove("popover-active");
    containerElement.style.display = "none";
    document.removeEventListener("keydown", handleHidePopoverOnEscape);
    window.removeEventListener("resize", repositionContainer);
    observer.unobserve(containerElement);

    if (cleanupOnHidePopover !== null) {
        cleanupOnHidePopover();
        cleanupOnHidePopover = null;
    }

    activeTarget = null;
}

function handleHidePopoverOnEscape(event) {
    if (event.key === "Escape") {
        hidePopover();
    }
}

export function setupPopovers() {
    const targets = document.querySelectorAll("[data-popover-type]");

    for (let i = 0; i < targets.length; i++) {
        const target = targets[i];

        target.addEventListener("mouseenter", handleMouseEnter);
        target.addEventListener("mouseleave", handleMouseLeave);
    }
}
