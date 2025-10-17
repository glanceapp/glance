package glance

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
)

var todoWidgetTemplate = mustParseTemplate("todo.html", "widget-base.html")

type TodoItem struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	Checked bool   `json:"checked"`
	Order   int    `json:"order"`
}

type todoWidget struct {
	widgetBase `yaml:",inline"`
	cachedHTML template.HTML `yaml:"-"`
	TodoID     string        `yaml:"id"`
	DataPath   string        `yaml:"data-path"`
	db         *sql.DB       `yaml:"-"`
	mutex      sync.RWMutex  `yaml:"-"`
}

func (widget *todoWidget) initialize() error {
	widget.withTitle("To-do").withError(nil)

	// Set default values
	if widget.TodoID == "" {
		widget.TodoID = fmt.Sprintf("default-%d", widget.GetID())
	}
	if widget.DataPath == "" {
		widget.DataPath = "./data"
	}

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(widget.DataPath, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Initialize database
	if err := widget.initDatabase(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	widget.cachedHTML = widget.renderTemplate(widget, todoWidgetTemplate)
	return nil
}

func (widget *todoWidget) initDatabase() error {
	dbPath := filepath.Join(widget.DataPath, "todos.db")

	var err error
	widget.db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}

	// Create table if it doesn't exist
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS todo_items (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		todo_id TEXT NOT NULL,
		text TEXT NOT NULL,
		checked BOOLEAN NOT NULL DEFAULT 0,
		order_index INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_todo_id ON todo_items(todo_id);
	CREATE INDEX IF NOT EXISTS idx_todo_order ON todo_items(todo_id, order_index);
	`

	_, err = widget.db.Exec(createTableSQL)
	return err
}

func (widget *todoWidget) Render() template.HTML {
	return widget.cachedHTML
}

func (widget *todoWidget) handleRequest(w http.ResponseWriter, r *http.Request) {
	widget.mutex.Lock()
	defer widget.mutex.Unlock()

	// Extract the path after /api/widgets/{id}/
	path := strings.TrimPrefix(r.URL.Path, fmt.Sprintf("/api/widgets/%d/", widget.GetID()))

	switch {
	case path == "items" && r.Method == http.MethodGet:
		widget.handleGetItems(w, r)
	case path == "items" && r.Method == http.MethodPost:
		widget.handleAddItem(w, r)
	case strings.HasPrefix(path, "items/") && r.Method == http.MethodPut:
		itemID := strings.TrimPrefix(path, "items/")
		widget.handleUpdateItem(w, r, itemID)
	case strings.HasPrefix(path, "items/") && r.Method == http.MethodDelete:
		itemID := strings.TrimPrefix(path, "items/")
		widget.handleDeleteItem(w, r, itemID)
	case path == "reorder" && r.Method == http.MethodPost:
		widget.handleReorderItems(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (widget *todoWidget) handleGetItems(w http.ResponseWriter, r *http.Request) {
	rows, err := widget.db.Query(`
		SELECT id, text, checked, order_index
		FROM todo_items
		WHERE todo_id = ?
		ORDER BY order_index ASC
	`, widget.TodoID)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var items []TodoItem
	for rows.Next() {
		var item TodoItem
		var id int64
		err := rows.Scan(&id, &item.Text, &item.Checked, &item.Order)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
		item.ID = fmt.Sprintf("%d", id)
		items = append(items, item)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

func (widget *todoWidget) handleAddItem(w http.ResponseWriter, r *http.Request) {
	var item TodoItem
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Get the next order index
	var maxOrder int
	err := widget.db.QueryRow(`
		SELECT COALESCE(MAX(order_index), -1) + 1
		FROM todo_items
		WHERE todo_id = ?
	`, widget.TodoID).Scan(&maxOrder)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Insert new item
	result, err := widget.db.Exec(`
		INSERT INTO todo_items (todo_id, text, checked, order_index, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, widget.TodoID, item.Text, item.Checked, maxOrder)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	item.ID = fmt.Sprintf("%d", id)
	item.Order = maxOrder

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

func (widget *todoWidget) handleUpdateItem(w http.ResponseWriter, r *http.Request, itemID string) {
	var updateData TodoItem
	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(itemID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid item ID", http.StatusBadRequest)
		return
	}

	result, err := widget.db.Exec(`
		UPDATE todo_items
		SET text = ?, checked = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND todo_id = ?
	`, updateData.Text, updateData.Checked, id, widget.TodoID)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		http.Error(w, "Item not found", http.StatusNotFound)
		return
	}

	updateData.ID = itemID
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updateData)
}

func (widget *todoWidget) handleDeleteItem(w http.ResponseWriter, r *http.Request, itemID string) {
	id, err := strconv.ParseInt(itemID, 10, 64)
	if err != nil {
		http.Error(w, "Invalid item ID", http.StatusBadRequest)
		return
	}

	result, err := widget.db.Exec(`
		DELETE FROM todo_items
		WHERE id = ? AND todo_id = ?
	`, id, widget.TodoID)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		http.Error(w, "Item not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (widget *todoWidget) handleReorderItems(w http.ResponseWriter, r *http.Request) {
	var itemIDs []string
	if err := json.NewDecoder(r.Body).Decode(&itemIDs); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Start transaction
	tx, err := widget.db.Begin()
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Update order for each item
	for i, itemID := range itemIDs {
		id, err := strconv.ParseInt(itemID, 10, 64)
		if err != nil {
			http.Error(w, "Invalid item ID", http.StatusBadRequest)
			return
		}

		_, err = tx.Exec(`
			UPDATE todo_items
			SET order_index = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ? AND todo_id = ?
		`, i, id, widget.TodoID)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}
