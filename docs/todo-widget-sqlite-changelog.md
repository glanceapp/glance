# Todo Widget SQLite Migration - Change Log

## 🚀 Major Update: SQLite Persistence

The todo widget has been upgraded from browser localStorage to persistent SQLite database storage.

### ✨ New Features

- **Server-side SQLite storage** - Tasks persist across browser sessions and server restarts
- **Multiple device access** - Access your todos from any device connected to your Glance server
- **Enhanced data reliability** - No more lost tasks due to browser data clearing
- **RESTful API endpoints** - Programmatic access for automation and integrations
- **Improved performance** - Optimized database queries with proper indexing

### 🔧 Configuration Changes

#### New Properties Added:
- `data-path` (optional) - Specify custom SQLite database location
- Enhanced `id` property - Now used for database namespacing instead of localStorage keys

#### Updated Configuration Format:
```yaml
# Old format (still works with auto-generated IDs)
- type: to-do
  title: My Tasks

# New recommended format
- type: to-do
  title: My Tasks
  id: personal
  data-path: ./data
```

### 🗃️ Database Schema

```sql
CREATE TABLE todo_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    todo_id TEXT NOT NULL,           -- Widget ID for namespace separation
    text TEXT NOT NULL,              -- Task description
    checked BOOLEAN NOT NULL,        -- Completion status
    order_index INTEGER NOT NULL,    -- Display order (drag & drop)
    created_at DATETIME,             -- Creation timestamp
    updated_at DATETIME              -- Last modification timestamp
);
```

### 🔗 API Endpoints

The widget now exposes REST API endpoints:

- `GET /api/widgets/{widget-id}/items` - List all tasks
- `POST /api/widgets/{widget-id}/items` - Create new task
- `PUT /api/widgets/{widget-id}/items/{item-id}` - Update task
- `DELETE /api/widgets/{widget-id}/items/{item-id}` - Delete task
- `POST /api/widgets/{widget-id}/reorder` - Reorder tasks

### 📁 File Structure

```
your-glance-directory/
├── glance
├── glance.yml
└── data/              # Default data directory
    └── todos.db       # SQLite database file
```

### 🔄 Migration Notes

- **Automatic**: No manual migration required
- **Backward Compatible**: Old localStorage data remains untouched
- **Fresh Start**: New SQLite storage starts empty
- **Manual Transfer**: Re-create important tasks in the new system if needed

### 🛡️ Benefits

1. **Data Persistence** - Tasks survive browser clearing and server restarts
2. **Multi-Device Support** - Access from any device on your network
3. **Backup Friendly** - Standard SQLite backup tools work
4. **Performance** - Faster loading and updates with database indexing
5. **API Integration** - Build custom tools and automations
6. **Reliability** - Database transactions ensure data consistency

### 🔧 Troubleshooting

- **Database Location**: Check `{data-path}/todos.db` file
- **Permissions**: Ensure Glance can write to the data directory
- **Multiple Widgets**: Use different `id` values for separate todo lists
- **API Testing**: Use curl or similar tools to test REST endpoints

This upgrade provides a solid foundation for future enhancements while maintaining the simple, intuitive interface users love.
