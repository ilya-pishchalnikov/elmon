package grafana

import "time"

// ClientParams defines parameters required for creating Grafana client
type ClientParams struct {
	URL        string
	Token      string
	Timeout    int // in seconds
	Retries    int // Number of times to retry the request
	RetryDelay int // Delay between retries in seconds
}

// DataSource represents a single Grafana data source
type DataSource struct {
	ID        int    `json:"id"`
	UID       string `json:"uid"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	URL       string `json:"url"`
	IsDefault bool   `json:"isDefault"`
	Database  string `json:"database"` 
}

// PostgreSQLDataSourceModel defines the JSON structure required by Grafana
// to create a new PostgreSQL data source.
// This structure will be serialized and sent in the POST request body.
type PostgreSQLDataSourceModel struct {
	Name      string `json:"name"`
	Type      string `json:"type"` // Must be "postgres"
	Access    string `json:"access"`
	URL       string `json:"url"`  // Host:Port, e.g., "127.0.0.1:5432"
	Database  string `json:"database"`
	User      string `json:"user"`
	Password  string `json:"password"`
	SSLMode   string `json:"sslmode"` // e.g., "disable", "require"
	IsDefault bool   `json:"isDefault"`
}

// CreateDataSourceResponse is the structure of the response from the Grafana API
// after successful creation.
type CreateDataSourceResponse struct {
	ID      int    `json:"id"`
	UID     string `json:"uid"`
	Name    string `json:"name"`
	Message string `json:"message"`
}

// Dashboard represents a Grafana dashboard
// Dashboard represents a Grafana dashboard
type Dashboard struct {
	ID          int                    `json:"id"`
	UID         string                 `json:"uid"`
	Title       string                 `json:"title"`
	Tags        []string               `json:"tags"`
	IsStarred   bool                   `json:"isStarred"`
	URI         string                 `json:"uri"`
	URL         string                 `json:"url"`
	Slug        string                 `json:"slug"`
	Type        string                 `json:"type"`
	FolderID    int                    `json:"folderId"`
	FolderUID   string                 `json:"folderUid"`
	FolderTitle string                 `json:"folderTitle"`
	FolderURL   string                 `json:"folderUrl"`
	Meta        DashboardMeta          `json:"meta,omitempty"`
}

// DashboardMeta contains metadata about dashboard
type DashboardMeta struct {
	IsStarred   bool      `json:"isStarred"`
	IsHome      bool      `json:"isHome"`
	IsSnapshot  bool      `json:"isSnapshot"`
	Type        string    `json:"type"`
	CanSave     bool      `json:"canSave"`
	CanEdit     bool      `json:"canEdit"`
	CanAdmin    bool      `json:"canAdmin"`
	CanStar     bool      `json:"canStar"`
	Slug        string    `json:"slug"`
	Expires     string    `json:"expires"`
	Created     time.Time `json:"created"`
	Updated     time.Time `json:"updated"`
	UpdatedBy   string    `json:"updatedBy"`
	CreatedBy   string    `json:"createdBy"`
	Version     int       `json:"version"`
}

// DashboardSearchResponse represents the response from dashboard search API
type DashboardSearchResponse []struct {
	ID          int      `json:"id"`
	UID         string   `json:"uid"`
	Title       string   `json:"title"`
	URI         string   `json:"uri"`
	URL         string   `json:"url"`
	Slug        string   `json:"slug"`
	Type        string   `json:"type"`
	Tags        []string `json:"tags"`
	IsStarred   bool     `json:"isStarred"`
	FolderID    int      `json:"folderId,omitempty"`
	FolderUID   string   `json:"folderUid,omitempty"`
	FolderTitle string   `json:"folderTitle,omitempty"`
	FolderURL   string   `json:"folderUrl,omitempty"`
}

// Folder represents a Grafana folder
type Folder struct {
	ID    int    `json:"id"`
	UID   string `json:"uid"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

// DashboardPathInfo contains parsed dashboard path information
type DashboardPathInfo struct {
	FolderUID   string
	FolderName  string
	DashboardSlug string
	FullPath    string
}

// DashboardImport represents dashboard import/creation request for Grafana API with inputs
type DashboardImport struct {
    Dashboard map[string]interface{} `json:"dashboard"`
    Inputs    []interface{}          `json:"inputs,omitempty"`
    FolderID  int                    `json:"folderId,omitempty"`
    FolderUID string                 `json:"folderUid,omitempty"`
    Overwrite bool                   `json:"overwrite"`
    Message   string                 `json:"message,omitempty"`
    Path      string                 `json:"path,omitempty"`
}

// DashboardInput represents a dashboard input variable
type DashboardInput struct {
    Name        string `json:"name"`
    Type        string `json:"type"`
    PluginID    string `json:"pluginId"`
    Value       string `json:"value"`
    Label       string `json:"label,omitempty"`
    Description string `json:"description,omitempty"`
}

// DashboardImportResponse represents response from dashboard import API
type DashboardImportResponse struct {
    ID      int    `json:"id"`
    UID     string `json:"uid"`
    URL     string `json:"url"`
    Status  string `json:"status"`
    Version int    `json:"version"`
    Slug    string `json:"slug"`
}

// DashboardFull represents the full dashboard structure from Grafana export
type DashboardFull struct {
    Dashboard map[string]interface{} `json:"dashboard"`
    Meta      map[string]interface{} `json:"meta"`
    // Inputs и другие поля могут быть в некоторых экспортах
}

// DashboardJSON represents the complete dashboard JSON from Grafana export
type DashboardJSON map[string]interface{}

// DataSourceTestResponse represents the response from data source test
type DataSourceTestResponse struct {
    Message string `json:"message"`
    Status  string `json:"status"`
}