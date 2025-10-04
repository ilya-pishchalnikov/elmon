package grafana

// ClientParams defines parameters required for creating Grafana client
type ClientParams struct {
	URL        string
	Token      string
	Timeout    int // in seconds
	Retries    int // Number of times to retry the request
	RetryDelay int // Delay between retries in seconds
}