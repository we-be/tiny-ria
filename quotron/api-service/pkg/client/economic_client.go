package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/pkg/errors"
)

// EconomicFactorsClient is a client for fetching economic data from the proxy service
type EconomicFactorsClient struct {
	httpClient *http.Client
	baseURL    string
}

// NewEconomicFactorsClient creates a new client for the economic factors proxy
func NewEconomicFactorsClient() *EconomicFactorsClient {
	baseURL := os.Getenv("ECONOMIC_PROXY_URL")
	if baseURL == "" {
		baseURL = "http://localhost:5002"
	}

	return &EconomicFactorsClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: baseURL,
	}
}

// GetIndicator fetches data for a specific economic indicator
func (c *EconomicFactorsClient) GetIndicator(indicator, period string) (interface{}, error) {
	// Construct URL
	apiURL, err := url.Parse(fmt.Sprintf("%s/indicator/%s", c.baseURL, url.PathEscape(indicator)))
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse URL")
	}

	// Add period query parameter
	q := apiURL.Query()
	q.Set("period", period)
	apiURL.RawQuery = q.Encode()

	// Make request
	resp, err := c.httpClient.Get(apiURL.String())
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch indicator data")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-200 status code: %d", resp.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	// Unmarshal into a generic map
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal indicator data")
	}

	return data, nil
}

// GetAllIndicators fetches a list of all available economic indicators
func (c *EconomicFactorsClient) GetAllIndicators() (interface{}, error) {
	// Construct URL
	apiURL := fmt.Sprintf("%s/indicators", c.baseURL)

	// Make request
	resp, err := c.httpClient.Get(apiURL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch indicators list")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-200 status code: %d", resp.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	// Unmarshal into a generic map
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal indicators data")
	}

	return data, nil
}

// GetSummary fetches a summary of key economic indicators
func (c *EconomicFactorsClient) GetSummary() (interface{}, error) {
	// Construct URL
	apiURL := fmt.Sprintf("%s/summary", c.baseURL)

	// Make request
	resp, err := c.httpClient.Get(apiURL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch economic summary")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-200 status code: %d", resp.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	// Unmarshal into a generic map
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal summary data")
	}

	return data, nil
}