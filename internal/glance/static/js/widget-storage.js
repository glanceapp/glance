/**
 * Universal storage abstraction for widgets
 * Supports both browser localStorage and server-side storage
 */
export class WidgetStorage {
    constructor(widgetType, storageType, widgetId = null) {
        this.widgetType = widgetType;
        this.storageType = storageType;
        this.widgetId = widgetId;
    }

    /**
     * Load data from storage
     * @param {string} key - The storage key
     * @returns {Promise<any>} - The loaded data
     */
    async load(key) {
        if (this.storageType === "server") {
            return this.loadFromServer(key);
        }
        return this.loadFromBrowser(key);
    }

    /**
     * Save data to storage
     * @param {string} key - The storage key
     * @param {any} data - The data to save
     * @returns {Promise<void>}
     */
    async save(key, data) {
        if (this.storageType === "server") {
            return this.saveToServer(key, data);
        }
        return this.saveToBrowser(key, data);
    }

    /**
     * Load data from server
     * @param {string} key - The storage key
     * @returns {Promise<any>} - The loaded data
     */
    async loadFromServer(key) {
        const res = await fetch(`/api/widgets/${this.widgetType}/${key}`);
        return await res.json();
    }

    /**
     * Save data to server
     * @param {string} key - The storage key
     * @param {any} data - The data to save
     * @returns {Promise<void>}
     */
    async saveToServer(key, data) {
        await fetch(`/api/widgets/${this.widgetType}/${key}`, {
            method: "PUT",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(data),
        });
    }

    /**
     * Load data from browser localStorage
     * @param {string} key - The storage key
     * @returns {any} - The loaded data
     */
    loadFromBrowser(key) {
        const storageKey = `${this.widgetType}-${key}`;
        const data = localStorage.getItem(storageKey);
        return data ? JSON.parse(data) : [];
    }

    /**
     * Save data to browser localStorage
     * @param {string} key - The storage key
     * @param {any} data - The data to save
     */
    saveToBrowser(key, data) {
        const storageKey = `${this.widgetType}-${key}`;
        localStorage.setItem(storageKey, JSON.stringify(data));
    }
}
