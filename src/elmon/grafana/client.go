package grafana

import (
	"bytes"
	"elmon/logger"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// ApiClient represents Grafana API client
type ApiClient struct {
	URL        string
	Token      string
	HttpClient *http.Client
	Headers    map[string]string
	Retries    int
	RetryDelay time.Duration
}

// NewClient now accepts local ClientParams type
func NewClient(params ClientParams) *ApiClient {
	if params.Timeout == 0 {
		params.Timeout = 30 // Default timeout value in seconds
	}
	if params.Retries < 0 {
		params.Retries = 3 // Default number of retries
	}
	if params.RetryDelay <= 0 {
		params.RetryDelay = 5 // Default retry delay in seconds
	}

	client := &ApiClient{
		URL:   strings.TrimSuffix(params.URL, "/"),
		Token: params.Token,
		HttpClient: &http.Client{
			Timeout: time.Duration(params.Timeout) * time.Second,
		},
		Retries:    params.Retries,
		RetryDelay: time.Duration(params.RetryDelay) * time.Second,
	}

	client.setDefaultHeaders()
	return client
}

// setDefaultHeaders sets default HTTP headers for API requests
func (apiClient *ApiClient) setDefaultHeaders() {
	if apiClient.Headers == nil {
		apiClient.Headers = make(map[string]string)
	}

	apiClient.Headers["Authorization"] = "Bearer " + apiClient.Token
	apiClient.Headers["Content-Type"] = "application/json"
	apiClient.Headers["Accept"] = "application/json"
}

// doRequestWithRetries executes an HTTP request with retries on failure
func (client *ApiClient) doRequestWithRetries(log *logger.Logger, request *http.Request, requestName string) (*http.Response, error) {
	var response *http.Response
	var err error

	if request.Body != nil {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			log.Error(err, "error while read body")
			return nil, err
		}

		err = os.WriteFile("grafana_request_body.json", body, 0644)
		if err != nil {
			log.Error(err, "error while write body to file")
			return nil, err
		}
	}


	// We start with 0 retries performed, so total attempts is Retries + 1
	for attempt := 0; attempt <= client.Retries; attempt++ {
		// 1. Set Headers on the request
		for key, value := range client.Headers {
			request.Header.Set(key, value)
		}

		// 2. Execute the request
		response, err = client.HttpClient.Do(request)

		// 3. Check for successful request or if no more retries should be attempted
		if err == nil && response.StatusCode >= 200 && response.StatusCode < 300 {
			log.Info(fmt.Sprintf("grafana %s request passed", requestName))
			return response, nil
		}

		// If there was an error or a non-success status code, log and check if we should retry
		if attempt < client.Retries {
			// Log the attempt failure
			if err != nil {
				log.Warn(fmt.Sprintf("grafana %s request failed. Retrying in %v...", requestName, client.RetryDelay), "attempt", attempt+1, "max_retries", client.Retries, "error", err)
			} else {

				responseBody, err := io.ReadAll(response.Body)
				if err != nil {
					log.Error(err, "error while read body")
					return nil, err
				}
				log.Warn(fmt.Sprintf("grafana %s request failed. Retrying in %v...", requestName, client.RetryDelay), "attempt", attempt+1, "max_retries", client.Retries, "error", err, "StatusCode", response.StatusCode, "ResponseBody", string(responseBody))
			}
			
			
			// Wait before the next attempt
			time.Sleep(client.RetryDelay)
		} else {
			// This was the final attempt, return the error or non-success response
			if err != nil {
				log.Error(err, fmt.Sprintf("failed to execute grafana %s request after %d attempts", requestName, client.Retries+1))
				return nil, fmt.Errorf("failed to execute %s request after %d attempts: %w", requestName, client.Retries+1, err)
			}
			// If no error, but bad status code, log and return the response
			log.Warn(fmt.Sprintf("grafana %s request not passed after %d attempts", requestName, client.Retries+1), "StatusCode", response.StatusCode)
			return response, nil
		}
	}
	
	// Should not be reached, but is here for completeness
	return nil, fmt.Errorf("request execution logic error for %s", requestName)
}

// Health performs a request to the Grafana health endpoint (/api/health)
// and returns the raw HTTP response or an error.
func (client *ApiClient) Health(log *logger.Logger) (*http.Response, error) {
	// 1. Construct the full API URL
	endpoint := fmt.Sprintf("%s/api/health", client.URL)

	// 2. Create a a new GET request
	request, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create health request: %w", err)
	}

	// 3. Execute the request using retries
	response, err := client.doRequestWithRetries(log, request, "health")

	// 4. Return the raw response.
	// The caller is responsible for reading and closing the response body.
	return response, err
}

// GetDashboardByID fetches a dashboard by its UID.
func (client *ApiClient) GetDashboardByID(log *logger.Logger, uid string) (*http.Response, error) {
	// 1. Construct the full API URL
	endpoint := fmt.Sprintf("%s/api/dashboards/uid/%s", client.URL, uid)

	// 2. Create a a new GET request
	request, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create dashboard request: %w", err)
	}

	// 3. Execute the request using retries
	response, err := client.doRequestWithRetries(log, request, "dashboard")

	// 4. Return the raw response.
	return response, err
}

// checkResponse checks if the response status is within the 2xx range.
// It returns an error if the status is not successful.
func checkResponse(response *http.Response) error {
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("request failed with status code %d: %s", response.StatusCode, response.Status)
}

// extractDatabaseFromJSONData извлекает значение database из jsonData
// func extractDatabaseFromJSONData(jsonData map[string]interface{}) string {
// 	if jsonData == nil {
// 		return ""
// 	}
	
// 	// Поле database может быть в разных типах данных источников
// 	if database, exists := jsonData["database"]; exists {
// 		switch v := database.(type) {
// 		case string:
// 			return v
// 		case []byte:
// 			return string(v)
// 		default:
// 			// Для других типов попробуем преобразовать в строку
// 			return fmt.Sprintf("%v", v)
// 		}
// 	}
	
// 	return ""
// }

// GetDataSources fetches the list of data sources and returns a slice of DataSource structs.
func (client *ApiClient) GetDataSources(log *logger.Logger) ([]DataSource, error) {
	// 1. Construct the full API URL
	endpoint := fmt.Sprintf("%s/api/datasources", client.URL)

	// 2. Create a new GET request
	request, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create data sources request: %w", err)
	}

	// 3. Execute the request using retries
	response, err := client.doRequestWithRetries(log, request, "datasources")
	if err != nil {
		return nil, err
	}
	// Always close the response body when the request is successful (even if it's an HTTP error)
	defer response.Body.Close()

	// 4. Check for a successful HTTP status code (2xx)
	if err := checkResponse(response); err != nil {
		// Log.Warn was already called inside doRequestWithRetries, we just return the error
		return nil, err
	}

	// 5. Read the response body
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read data sources response body: %w", err)
	}

	// 6. Десериализация в промежуточную структуру для извлечения database из jsonData
	var rawDataSources []struct {
		ID        int                    `json:"id"`
		UID       string                 `json:"uid"`
		Name      string                 `json:"name"`
		Type      string                 `json:"type"`
		URL       string                 `json:"url"`
		IsDefault bool                   `json:"isDefault"`
		Datebase  string                 `json:"database"`
		JSONData  map[string]interface{} `json:"jsonData"`
	}
	
	if err := json.Unmarshal(body, &rawDataSources); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data sources response: %w", err)
	}

	// 7. Преобразование в конечную структуру DataSource с извлечением database
	dataSources := make([]DataSource, len(rawDataSources))
	for i, rawSource := range rawDataSources {
		dataSources[i] = DataSource{
			ID:        rawSource.ID,
			UID:       rawSource.UID,
			Name:      rawSource.Name,
			Type:      rawSource.Type,
			URL:       rawSource.URL,
			IsDefault: rawSource.IsDefault,
			Database:  rawSource.Datebase,
		}
	}

	log.Info("grafana datasources request successfully parsed")

	// 8. Return the struct
	return dataSources, nil
}

/// CreateDataSource creates a new data source in Grafana using the provided model.
// It returns the response structure with ID/UID or an error.
func (client *ApiClient) AddDataSource(log *logger.Logger, model PostgreSQLDataSourceModel) (*CreateDataSourceResponse, error) {
    // Создаем правильную структуру для Grafana API
    requestData := map[string]interface{}{
        "name":      model.Name,
        "type":      model.Type,
        "access":    model.Access,
        "url":       model.URL,
        "database":  model.Database,
        "user":      model.User,
        "isDefault": model.IsDefault,
        "jsonData": map[string]interface{}{
            "sslmode":         model.SSLMode,
            "postgresVersion": 1300, // Укажите версию PostgreSQL
            "timescaledb":     false,
        },
        "secureJsonData": map[string]string{
            "password": model.Password,
        },
    }

    // Сериализация структуры в JSON
    requestBody, err := json.Marshal(requestData)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal data source model: %w", err)
    }

    // Конструирование полного URL
    endpoint := fmt.Sprintf("%s/api/datasources", client.URL)

    // Создание нового POST запроса с телом
    request, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(requestBody))
    if err != nil {
        return nil, fmt.Errorf("failed to create data source request: %w", err)
    }

    // Выполнение запроса с повторными попытками
    response, err := client.doRequestWithRetries(log, request, "create_datasource")
    if err != nil {
        return nil, err
    }
    defer response.Body.Close()

    // Проверка успешного HTTP статуса (2xx)
    if err := checkResponse(response); err != nil {
        // Попытаемся прочитать тело ошибки для более детального сообщения
        errorBody, readErr := io.ReadAll(response.Body)
        if readErr == nil && len(errorBody) > 0 {
            var errorResp map[string]interface{}
            if jsonErr := json.Unmarshal(errorBody, &errorResp); jsonErr == nil {
                if message, exists := errorResp["message"]; exists {
                    return nil, fmt.Errorf("%w: %s", err, message)
                }
            }
            return nil, fmt.Errorf("%w: %s", err, string(errorBody))
        }
        return nil, err
    }

    // Десериализация успешного ответа
    body, err := io.ReadAll(response.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read create data source response body: %w", err)
    }

    var createResp CreateDataSourceResponse
    if err := json.Unmarshal(body, &createResp); err != nil {
        return nil, fmt.Errorf("failed to unmarshal create data source response: %w", err)
    }

    log.Info(fmt.Sprintf("grafana data source '%s' successfully created with ID: %d", createResp.Name, createResp.ID))

    return &createResp, nil
}


// AddDataSourceIfNotExists checks for an existing data source with the same type, URL, and database.
// If none exists, it creates the new data source. If a data source with the same name exists,
// it tries to use an incremented name (name_1, name_2, etc.).
func (client *ApiClient) AddDataSourceIfNotExists(log *logger.Logger, newModel PostgreSQLDataSourceModel) (*CreateDataSourceResponse, error) {
    // Получение списка всех существующих источников данных
    existingSources, err := client.GetDataSources(log)
    if err != nil {
        return nil, fmt.Errorf("failed to list existing data sources: %w", err)
    }

    // Проверка, существует ли источник данных с тем же типом, URL и базой данных
    for _, source := range existingSources {
        if source.Type == newModel.Type && source.URL == newModel.URL && source.Database == newModel.Database {
            log.Info(fmt.Sprintf("data source of type '%s' with URL '%s' and database '%s' already exists (ID: %d). Skipping creation.", 
                source.Type, source.URL, source.Database, source.ID))
            return &CreateDataSourceResponse{
				ID: source.ID,
				UID: source.UID,
				Name: source.Name,
				Message: "Already exists",
			}, nil
        }
    }


    // Проверка на дублирование имени и инкремент
    sourceToCreate := newModel
    baseName := newModel.Name

    for i := 0; ; i++ {
        currentName := baseName
        if i > 0 {
            currentName = fmt.Sprintf("%s_%d", baseName, i)
        }
        
        // Проверяем, существует ли источник с текущим именем
        nameConflict := false
        for _, source := range existingSources {
            if source.Name == currentName {
                nameConflict = true
                break
            }
        }

        if !nameConflict {
            sourceToCreate.Name = currentName
            break
        }
    }

    // Создание источника данных с уникальным именем
    log.Info(fmt.Sprintf("creating new data source with unique name: %s", sourceToCreate.Name))
    
    createdSource, err := client.AddDataSource(log, sourceToCreate)
    if err != nil {
        return nil, fmt.Errorf("failed to create data source %s: %w", sourceToCreate.Name, err)
    }

    return createdSource, nil
}

// GetAllDashboards fetches all dashboards and returns them as an array of Dashboard structs
func (client *ApiClient) GetAllDashboards(log *logger.Logger) ([]Dashboard, error) {
	// 1. Construct the full API URL for dashboard search
	endpoint := fmt.Sprintf("%s/api/search?type=dash-db", client.URL)

	// 2. Create a new GET request
	request, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create dashboards request: %w", err)
	}

	// 3. Execute the request using retries
	response, err := client.doRequestWithRetries(log, request, "get_dashboards")
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	// 4. Check for a successful HTTP status code (2xx)
	if err := checkResponse(response); err != nil {
		return nil, err
	}

	// 5. Read the response body
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read dashboards response body: %w", err)
	}

	// 6. Parse the search response
	var searchResponse DashboardSearchResponse
	if err := json.Unmarshal(body, &searchResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal dashboards response: %w", err)
	}

	// 7. Convert to our Dashboard structs
	dashboards := make([]Dashboard, len(searchResponse))
	for i, item := range searchResponse {
		dashboards[i] = Dashboard{
			ID:          item.ID,
			UID:         item.UID,
			Title:       item.Title,
			Tags:        item.Tags,
			IsStarred:   item.IsStarred,
			URI:         item.URI,
			URL:         item.URL,
			Slug:        item.Slug,
			Type:        item.Type,
			FolderID:    item.FolderID,
			FolderUID:   item.FolderUID,
			FolderTitle: item.FolderTitle,
			FolderURL:   item.FolderURL,
		}
	}

	log.Info(fmt.Sprintf("successfully fetched %d dashboards", len(dashboards)))

	return dashboards, nil
}

// GetDashboardByUID fetches full dashboard data by UID
func (client *ApiClient) GetDashboardByUID(log *logger.Logger, uid string) (*DashboardFull, error) {
	// 1. Construct the full API URL
	endpoint := fmt.Sprintf("%s/api/dashboards/uid/%s", client.URL, uid)

	// 2. Create a new GET request
	request, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create dashboard request: %w", err)
	}

	// 3. Execute the request using retries
	response, err := client.doRequestWithRetries(log, request, "get_dashboard")
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	// 4. Check for a successful HTTP status code (2xx)
	if err := checkResponse(response); err != nil {
		return nil, err
	}

	// 5. Read the response body
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read dashboard response body: %w", err)
	}

	// 6. Parse the full dashboard response
	var dashboardFull DashboardFull
	if err := json.Unmarshal(body, &dashboardFull); err != nil {
		return nil, fmt.Errorf("failed to unmarshal dashboard response: %w", err)
	}

	log.Info(fmt.Sprintf("successfully fetched dashboard: %s", uid))

	return &dashboardFull, nil
}

// GetAllDashboardsWithDetails fetches all dashboards with their full details
func (client *ApiClient) GetAllDashboardsWithDetails(log *logger.Logger) ([]DashboardFull, error) {
	// 1. First get the list of all dashboards
	dashboards, err := client.GetAllDashboards(log)
	if err != nil {
		return nil, fmt.Errorf("failed to get dashboard list: %w", err)
	}

	// 2. Fetch full details for each dashboard
	var dashboardsWithDetails []DashboardFull

	for _, dashboard := range dashboards {
		fullDashboard, err := client.GetDashboardByUID(log, dashboard.UID)
		if err != nil {
			log.Warn(fmt.Sprintf("failed to fetch details for dashboard %s: %v", dashboard.UID, err))
			continue // Skip this dashboard but continue with others
		}

		dashboardsWithDetails = append(dashboardsWithDetails, *fullDashboard)
	}

	log.Info(fmt.Sprintf("successfully fetched details for %d out of %d dashboards", 
		len(dashboardsWithDetails), len(dashboards)))

	return dashboardsWithDetails, nil
}

// SearchDashboards searches dashboards with custom query parameters
func (client *ApiClient) SearchDashboards(log *logger.Logger, queryParams map[string]string) ([]Dashboard, error) {
	// 1. Construct the base URL
	endpoint := fmt.Sprintf("%s/api/search", client.URL)

	// 2. Add query parameters
	if len(queryParams) > 0 {
		query := make([]string, 0, len(queryParams))
		for key, value := range queryParams {
			query = append(query, fmt.Sprintf("%s=%s", key, value))
		}
		endpoint += "?" + strings.Join(query, "&")
	}

	// 3. Create a new GET request
	request, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create dashboard search request: %w", err)
	}

	// 4. Execute the request using retries
	response, err := client.doRequestWithRetries(log, request, "search_dashboards")
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	// 5. Check for a successful HTTP status code (2xx)
	if err := checkResponse(response); err != nil {
		return nil, err
	}

	// 6. Read and parse the response
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read dashboard search response body: %w", err)
	}

	var searchResponse DashboardSearchResponse
	if err := json.Unmarshal(body, &searchResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal dashboard search response: %w", err)
	}

	// 7. Convert to our Dashboard structs
	dashboards := make([]Dashboard, len(searchResponse))
	for i, item := range searchResponse {
		dashboards[i] = Dashboard{
			ID:          item.ID,
			UID:         item.UID,
			Title:       item.Title,
			Tags:        item.Tags,
			IsStarred:   item.IsStarred,
			URI:         item.URI,
			URL:         item.URL,
			Slug:        item.Slug,
			Type:        item.Type,
			FolderID:    item.FolderID,
			FolderUID:   item.FolderUID,
			FolderTitle: item.FolderTitle,
			FolderURL:   item.FolderURL,
		}
	}

	log.Info(fmt.Sprintf("search returned %d dashboards", len(dashboards)))

	return dashboards, nil
}

// GetAllFolders fetches all folders from Grafana
func (client *ApiClient) GetAllFolders(log *logger.Logger) ([]Folder, error) {
	// 1. Construct the full API URL for folders
	endpoint := fmt.Sprintf("%s/api/folders", client.URL)

	// 2. Create a new GET request
	request, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create folders request: %w", err)
	}

	// 3. Execute the request using retries
	response, err := client.doRequestWithRetries(log, request, "get_folders")
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	// 4. Check for a successful HTTP status code (2xx)
	if err := checkResponse(response); err != nil {
		return nil, err
	}

	// 5. Read the response body
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read folders response body: %w", err)
	}

	// 6. Parse the folders response
	var folders []Folder
	if err := json.Unmarshal(body, &folders); err != nil {
		return nil, fmt.Errorf("failed to unmarshal folders response: %w", err)
	}

	log.Info(fmt.Sprintf("successfully fetched %d folders", len(folders)))

	return folders, nil
}

// GetFolderByUID fetches folder by its UID
func (client *ApiClient) GetFolderByUID(log *logger.Logger, uid string) (*Folder, error) {
	// 1. Construct the full API URL
	endpoint := fmt.Sprintf("%s/api/folders/%s", client.URL, uid)

	// 2. Create a new GET request
	request, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create folder request: %w", err)
	}

	// 3. Execute the request using retries
	response, err := client.doRequestWithRetries(log, request, "get_folder")
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	// 4. Check for a successful HTTP status code (2xx)
	if err := checkResponse(response); err != nil {
		return nil, err
	}

	// 5. Read the response body
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read folder response body: %w", err)
	}

	// 6. Parse the folder response
	var folder Folder
	if err := json.Unmarshal(body, &folder); err != nil {
		return nil, fmt.Errorf("failed to unmarshal folder response: %w", err)
	}

	return &folder, nil
}

// parseDashboardPath parses dashboard URL path and extracts folder UID and dashboard slug
func parseDashboardPath(path string) (*DashboardPathInfo, error) {
	// Remove leading/trailing slashes and split by "/"
	cleanPath := strings.Trim(path, "/")
	parts := strings.Split(cleanPath, "/")

	// Expected format: "dashboards/f/{folderUID}/{dashboardSlug}"
	if len(parts) < 4 || parts[0] != "dashboards" || parts[1] != "f" {
		return nil, fmt.Errorf("invalid dashboard path format: %s", path)
	}

	folderUID := parts[2]
	dashboardSlug := parts[3]

	return &DashboardPathInfo{
		FolderUID:     folderUID,
		DashboardSlug: dashboardSlug,
		FullPath:      path,
	}, nil
}

// ResolveFolderName resolves folder name from dashboard path and returns "folder_name/dashboard_slug" format
func (client *ApiClient) ResolveFolderName(log *logger.Logger, dashboardPath string) (string, error) {
	// 1. Parse the dashboard path
	pathInfo, err := parseDashboardPath(dashboardPath)
	if err != nil {
		return "", fmt.Errorf("failed to parse dashboard path: %w", err)
	}

	// 2. Get folder by UID
	folder, err := client.GetFolderByUID(log, pathInfo.FolderUID)
	if err != nil {
		// If folder not found, try to get all folders and cache them
		log.Warn(fmt.Sprintf("folder with UID %s not found, fetching all folders", pathInfo.FolderUID))
		
		allFolders, err := client.GetAllFolders(log)
		if err != nil {
			return "", fmt.Errorf("failed to fetch folders: %w", err)
		}

		// Search for folder in the list
		var foundFolder *Folder
		for _, f := range allFolders {
			if f.UID == pathInfo.FolderUID {
				foundFolder = &f
				break
			}
		}

		if foundFolder == nil {
			return "", fmt.Errorf("folder with UID %s not found", pathInfo.FolderUID)
		}
		folder = foundFolder
	}

	// 3. Construct the final path in "folder_name/dashboard_slug" format
	folderName := normalizeFolderName(folder.Title)
	result := fmt.Sprintf("%s/%s", folderName, pathInfo.DashboardSlug)

	log.Info(fmt.Sprintf("resolved path: %s -> %s", dashboardPath, result))

	return result, nil
}

// normalizeFolderName normalizes folder name for filesystem compatibility
func normalizeFolderName(folderName string) string {
	// Replace spaces and special characters with underscores
	reg := regexp.MustCompile(`[^\w\-]`)
	normalized := reg.ReplaceAllString(folderName, "_")
	
	// Remove multiple consecutive underscores
	reg = regexp.MustCompile(`_+`)
	normalized = reg.ReplaceAllString(normalized, "_")
	
	// Trim underscores from start and end
	normalized = strings.Trim(normalized, "_")
	
	// If empty after normalization, use "unknown_folder"
	if normalized == "" {
		return "unknown_folder"
	}
	
	return normalized
}

// ResolveDashboardPaths resolves folder names for multiple dashboards
func (client *ApiClient) ResolveDashboardPaths(log *logger.Logger, dashboards []Dashboard) (map[string]string, error) {
	// Pre-fetch all folders for better performance
	folders, err := client.GetAllFolders(log)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch folders: %w", err)
	}

	// Create folder UID to name mapping
	folderMap := make(map[string]string)
	for _, folder := range folders {
		folderMap[folder.UID] = normalizeFolderName(folder.Title)
	}

	// Resolve paths for each dashboard
	result := make(map[string]string)
	
	for _, dashboard := range dashboards {
		pathInfo, err := parseDashboardPath(dashboard.URI)
		if err != nil {
			log.Warn(fmt.Sprintf("failed to parse dashboard path %s: %v", dashboard.URI, err))
			continue
		}

		folderName, exists := folderMap[pathInfo.FolderUID]
		if !exists {
			folderName = "unknown_folder"
			log.Warn(fmt.Sprintf("folder with UID %s not found for dashboard %s", pathInfo.FolderUID, dashboard.Title))
		}

		resolvedPath := fmt.Sprintf("%s/%s", folderName, pathInfo.DashboardSlug)
		result[dashboard.UID] = resolvedPath
	}

	return result, nil
}

// CreateDashboard creates a dashboard using the dashboard API (alternative method)
func (client *ApiClient) CreateDashboard(log *logger.Logger, dashboard map[string]interface{}, folderUID string, overwrite bool) (*DashboardImportResponse, error) {
	// Prepare the request
	importRequest := DashboardImport{
		Dashboard: dashboard,
		Overwrite: overwrite,
		Message:   "Created via API",
	}

	if folderUID != "" {
		importRequest.FolderUID = folderUID
	}

	requestBody, err := json.Marshal(importRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal dashboard request: %w", err)
	}

	// Use the dashboards/db endpoint
	endpoint := fmt.Sprintf("%s/api/dashboards/db", client.URL)

	request, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create dashboard request: %w", err)
	}

	response, err := client.doRequestWithRetries(log, request, "create_dashboard")
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if err := checkResponse(response); err != nil {
		errorBody, _ := io.ReadAll(response.Body)
		return nil, fmt.Errorf("%w: %s", err, string(errorBody))
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read create dashboard response: %w", err)
	}

	var createResp DashboardImportResponse
	if err := json.Unmarshal(body, &createResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal create dashboard response: %w", err)
	}

	log.Info(fmt.Sprintf("dashboard created successfully: %s", createResp.UID))

	return &createResp, nil
}

// ExportDashboard exports a dashboard as JSON by UID
func (client *ApiClient) ExportDashboard(log *logger.Logger, uid string) ([]byte, error) {
	endpoint := fmt.Sprintf("%s/api/dashboards/uid/%s", client.URL, uid)

	request, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create export dashboard request: %w", err)
	}

	response, err := client.doRequestWithRetries(log, request, "export_dashboard")
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if err := checkResponse(response); err != nil {
		return nil, err
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read export dashboard response: %w", err)
	}

	log.Info(fmt.Sprintf("dashboard %s exported successfully", uid))

	return body, nil
}

// LoadDashboardFromFile loads dashboard JSON from file
func LoadDashboardFromFile(filename string) ([]byte, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read dashboard file: %w", err)
	}
	return data, nil
}

// ImportDashboard imports a dashboard from exported Grafana JSON
func (client *ApiClient) ImportDashboard(log *logger.Logger, dashboardJSON []byte, folderUID string, overwrite bool, inputValues map[string]string) (*DashboardImportResponse, error) {
    // 1. Parse the exported Grafana JSON
    var exportedData map[string]interface{}
    if err := json.Unmarshal(dashboardJSON, &exportedData); err != nil {
        return nil, fmt.Errorf("failed to parse dashboard JSON: %w", err)
    }

    // // 2. Extract the dashboard object from exported data
    // dashboardObj, exists := exportedData["dashboard"]
    // if !exists {
    //     return nil, fmt.Errorf("exported JSON does not contain 'dashboard' object")
    // }

    // dashboard, ok := dashboardObj.(map[string]interface{})
    // if !ok {
    //     return nil, fmt.Errorf("dashboard object is not a valid JSON object")
    // }

	dashboard := exportedData

	// 2. Prepare inputs from the exported data or provided values
	var inputs []interface{}
	if exportedInputs, exists := exportedData["__inputs"]; exists {
		inputsSlice, ok := exportedInputs.([]interface{})
		if ok {
			inputs = processInputs(inputsSlice, inputValues)
		}
	}

    // 3. Prepare import request according to Grafana API
    importRequest := DashboardImport{
        Dashboard: dashboard,
        Inputs:    inputs,
        Overwrite: overwrite,
        Message:   "Imported via API",
    }

    // 4. Set folder if provided
    if folderUID != "" {
        importRequest.FolderUID = folderUID
    }

    // 5. Serialize import request
    requestBody, err := json.Marshal(importRequest)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal import request: %w", err)
    }

    // 6. Construct the API endpoint
    endpoint := fmt.Sprintf("%s/api/dashboards/import", client.URL)

    // 7. Create POST request
    request, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(requestBody))
    if err != nil {
        return nil, fmt.Errorf("failed to create import dashboard request: %w", err)
    }

    // 8. Execute the request
    response, err := client.doRequestWithRetries(log, request, "import_dashboard")
    if err != nil {
        return nil, err
    }
     defer response.Body.Close()

    // 9. Check response status
    if err := checkResponse(response); err != nil {
        // Read error details
        errorBody, _ := io.ReadAll(response.Body)
        return nil, fmt.Errorf("%w: %s", err, string(errorBody))
    }

    // 10. Parse successful response
    body, err := io.ReadAll(response.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read import response body: %w", err)
    }

    var importResp DashboardImportResponse
    if err := json.Unmarshal(body, &importResp); err != nil {
        return nil, fmt.Errorf("failed to unmarshal import response: %w", err)
    }

    log.Info(fmt.Sprintf("dashboard imported successfully: %s (ID: %d)", importResp.UID, importResp.ID))

    return &importResp, nil
}

// processInputs processes input variables and sets their values
func processInputs(inputs []interface{}, inputValues map[string]string) []interface{} {
    var processedInputs []interface{}

    for _, input := range inputs {
        inputMap, ok := input.(map[string]interface{})
        if !ok {
            continue
        }

        // Get input name
        name, exists := inputMap["name"].(string)
        if !exists {
            continue
        }

        // Set value from provided values or use default
        if value, exists := inputValues[name]; exists {
            inputMap["value"] = value
        } else {
            // Try to use the default value from the input
            if currentValue, exists := inputMap["value"]; exists {
                inputMap["value"] = currentValue
            }
        }

        processedInputs = append(processedInputs, inputMap)
    }

    return processedInputs
}

// ExtractInputsFromDashboard extracts input variable names from dashboard JSON
func ExtractInputsFromDashboard(dashboardJSON []byte) ([]string, error) {
    var exportedData map[string]interface{}
    if err := json.Unmarshal(dashboardJSON, &exportedData); err != nil {
        return nil, err
    }

    inputs, exists := exportedData["inputs"]
    if !exists {
        return []string{}, nil
    }

    inputsSlice, ok := inputs.([]interface{})
    if !ok {
        return []string{}, nil
    }

    var inputNames []string
    for _, input := range inputsSlice {
        inputMap, ok := input.(map[string]interface{})
        if !ok {
            continue
        }

        if name, exists := inputMap["name"].(string); exists {
            inputNames = append(inputNames, name)
        }
    }

    return inputNames, nil
}

// // CreateDashboardFromFile creates a dashboard from a JSON file
// func (client *ApiClient) CreateDashboardFromFile(log *logger.Logger, filename string, folderUID string, overwrite bool) (*DashboardImportResponse, error) {
//     // Load JSON from file
//     dashboardJSON, err := os.ReadFile(filename)
//     if err != nil {
//         return nil, fmt.Errorf("failed to read dashboard file: %w", err)
//     }

//     return client.ImportDashboard(log, dashboardJSON, folderUID, overwrite)
// }

// PrepareDashboardForImport prepares exported dashboard JSON for import
func PrepareDashboardForImport(dashboardJSON []byte, newTitle string, newUID string, updateDatasourceUID string) ([]byte, error) {
    
	var dashboardData map[string]interface{}
    if err := json.Unmarshal(dashboardJSON, &dashboardData); err != nil {
        return nil, err
    }

    // Update dashboard properties
    if newTitle != "" {
        dashboardData["title"] = newTitle
    }
    if newUID != "" {
        dashboardData["uid"] = newUID
    }

    // Remove fields that should be regenerated
    delete(dashboardData, "id")
    delete(dashboardData, "version")

	return json.Marshal(dashboardData)
}

// // TestDataSourceByUID tests a data source connection by UID
// func (client *ApiClient) TestDataSourceByUID(log *logger.Logger, uid string) (*DataSourceTestResponse, error) {
//     // 1. Construct the full API URL
//     endpoint := fmt.Sprintf("%s/api/datasources/uid/%s/health", client.URL, uid)

//     // 2. Create a new GET request
//     request, err := http.NewRequest("GET", endpoint, nil)
//     if err != nil {
//         return nil, fmt.Errorf("failed to create data source test request: %w", err)
//     }

//     // 3. Execute the request using retries
//     response, err := client.doRequestWithRetries(log, request, "test_datasource")
//     if err != nil {
//         return nil, err
//     }
//     defer response.Body.Close()

//     // 4. Check for a successful HTTP status code (2xx)
//     if err := checkResponse(response); err != nil {
//         // For test endpoints, we might get different status codes
//         // Let's read the response body to get the actual test result
//         body, readErr := io.ReadAll(response.Body)
//         if readErr != nil {
//             return nil, fmt.Errorf("data source test failed with status %d and unable to read response: %w", response.StatusCode, err)
//         }

//         var testResp DataSourceTestResponse
//         if jsonErr := json.Unmarshal(body, &testResp); jsonErr != nil {
//             return nil, fmt.Errorf("data source test failed with status %d: %s", response.StatusCode, string(body))
//         }

//         // Return the test response even if HTTP status is not 2xx
//         // as it contains the actual test result
//         return &testResp, nil
//     }

//     // 5. Read the response body for successful HTTP request
//     body, err := io.ReadAll(response.Body)
//     if err != nil {
//         return nil, fmt.Errorf("failed to read data source test response body: %w", err)
//     }

//     // 6. Parse the test response
//     var testResp DataSourceTestResponse
//     if err := json.Unmarshal(body, &testResp); err != nil {
//         return nil, fmt.Errorf("failed to unmarshal data source test response: %w", err)
//     }

//     log.Info(fmt.Sprintf("data source test completed: %s - %s", testResp.Status, testResp.Message))

//     return &testResp, nil
// }

// // IsDataSourceHealthy checks if a data source is healthy (status is "OK" or "success")
// func (client *ApiClient) IsDataSourceHealthy(log *logger.Logger, uid string) (bool, error) {
//     testResult, err := client.TestDataSourceByUID(log, uid)
//     if err != nil {
//         return false, err
//     }

//     // Check if the status indicates a healthy data source
//     healthy := testResult.Status == "OK" || testResult.Status == "success" || 
//                testResult.Status == "green" || strings.Contains(strings.ToLower(testResult.Message), "success")

//     return healthy, nil
// }
