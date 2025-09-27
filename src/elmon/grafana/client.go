package grafana

import (
	"elmon/config"
	"elmon/logger"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// APIClient represents a Grafana API client with dynamic endpoint support
type ApiClient struct {
    Url        string
    Token      string
    HttpClient *http.Client
	Headers    map[string]string
}

// New creates a new instance of Grafana APIClient
func NewClient(config config.GrafanaConfig) *ApiClient {
    
	if config.Timeout == 0 {
        config.Timeout = 30
    }
    
    client := &ApiClient{
        Url: strings.TrimSuffix(config.Url, "/"),
        Token:  config.Token,
        HttpClient: &http.Client{
            Timeout: time.Duration(config.Timeout) * time.Second,
		},
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

// Health performs a request to the Grafana health endpoint (/api/health)
// and returns the raw HTTP response or an error.
func ( client *ApiClient) Health(log *logger.Logger) (*http.Response, error) {
	// 1. Construct the full API URL
	endpoint := fmt.Sprintf("%s/api/health", client.Url)

	// 2. Create a new GET request
	request, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create health request: %w", err)
	}

	// 3. Set Headers
	for key, value := range client.Headers {
		request.Header.Set(key, value)
	}
	
	
	// 4. Execute the request using the internal http.Client
	response, err := client.HttpClient.Do(request)
	if err != nil {
		log.Error(err, "failed to execute grafana health request")
		return nil, fmt.Errorf("failed to execute health request: %w", err)
	}

	if response.StatusCode == 200 {
		log.Info("grafana health request passed")
	} else {
		log.Warn("grafana health request not passed", "StatusCode", response.StatusCode)
	}

	// 5. Return the raw response.
	// The caller is responsible for reading and closing the response body.
	return response, nil
}

// GetDashboardByID fetches a dashboard by its UID.
// This is an example of another method to illustrate the OOP style.
func (client *ApiClient) GetDashboardByID(log *logger.Logger, uid string) (*http.Response, error) {
	endpoint := fmt.Sprintf("%s/api/dashboards/uid/%s", client.Url, uid)
	
	request, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create dashboard request: %w", err)
	}

	for key, value := range client.Headers {
		request.Header.Set(key, value)
	}
	
	response, err := client.HttpClient.Do(request)
	if err != nil {
		log.Error(err, "failed to execute grafana dashboard request")
		return nil, fmt.Errorf("failed to execute dashboard request: %w", err)
	}

	if response.StatusCode == 200 {
		log.Info("grafana dashboard request passed", "dashboard_id", uid)
	} else {
		log.Warn("grafana dashboard request not passed", "dashboard_id", uid, "StatusCode", response.StatusCode)
	}
	
	return response, nil
}