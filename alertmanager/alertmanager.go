package alertmanager

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

// APIClient of alertmanager
type APIClient struct {
	URL            *url.URL
	ConfigPath     string
	ConfigTemplate string
	HTTPClient     *http.Client
	ID             int
	Key            string
	logger         log.Logger
}

// Config of alertmanager
type Config struct {
	Receivers    string
	Routes       string
	InhibitRules string
}

// Reload alertmanager
func (c *APIClient) Reload() (int, error) {
	return c.doPost(c.URL.String())
}

// do post request
func (c *APIClient) doPost(url string) (int, error) {
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		for strings.Contains(err.Error(), "connection refused") {
			//nolint:errcheck,lll
			level.Error(c.logger).Log(
				"msg", "Failed to reload alertmanager.yml. Perhaps Alertmanager is not ready. Waiting for 8 seconds and retry again...",
				"err", err.Error())
			time.Sleep(8 * time.Second)
			resp, err = c.HTTPClient.Do(req)
			if err == nil {
				break
			}
		}
	}
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		//nolint:lll
		return resp.StatusCode, fmt.Errorf("unexpected status code returned from Alertmanager (got: %d, expected: 200, msg:%s)",
			resp.StatusCode, resp.Status)
	}
	return 0, nil
}

// New return an APIClient
func New(baseURL *url.URL, configPath string, configTemplate string, id int, key string, logger log.Logger) *APIClient {
	return &APIClient{
		URL:            baseURL,
		ConfigPath:     configPath,
		ConfigTemplate: configTemplate,
		HTTPClient:     http.DefaultClient,
		ID:             id,
		Key:            key,
		logger:         logger,
	}
}
