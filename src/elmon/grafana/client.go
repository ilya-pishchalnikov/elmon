package grafana

import (
	"elmon/logger"
	"fmt"
	"net/http"
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
				log.Warn(fmt.Sprintf("grafana %s request failed. Retrying in %v...", requestName, client.RetryDelay), "attempt", attempt+1, "max_retries", client.Retries, "error", err, "StatusCode", response.StatusCode)
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