import { clamp } from "./utils.js";

export function setupMasonries(options = {}, selector = ".masonry") {
    options = {
        minColumnWidth: 300,
        maxColumns: 6,
         ...options
    };

    const masonryContainers = document.querySelectorAll(selector);

    for (let i = 0; i < masonryContainers.length; i++) {
        const container = masonryContainers[i];
        const items = Array.from(container.children);
        let previousColumnsCount = 0;

        const render = function() {
            const columnsCount = clamp(
                Math.floor(container.offsetWidth / options.minColumnWidth),
                1,
                Math.min(options.maxColumns, items.length)
            );

            if (columnsCount === previousColumnsCount) {
                return;
            } else {
                container.textContent = "";
                previousColumnsCount = columnsCount;
            }

            const columnsFragment = document.createDocumentFragment();

            for (let i = 0; i < columnsCount; i++) {
                const column = document.createElement("div");
                column.className = "masonry-column";
                columnsFragment.append(column);
            }

            // poor man's masonry
            // TODO: add an option that allows placing items in the
            // shortest column instead of iterating the columns in order
            for (let i = 0; i < items.length; i++) {
                columnsFragment.children[i % columnsCount].appendChild(items[i]);
            }

            container.append(columnsFragment);
        };

        const observer = new ResizeObserver(() => requestAnimationFrame(render));
        observer.observe(container);
    }
}
