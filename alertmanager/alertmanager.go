package alertmanager

import (
	"fmt"
	"github.com/go-kit/kit/log/level"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
)

type APIClient struct {
	Url            *url.URL
	ConfigPath     string
	ConfigTemplate string
	HTTPClient     *http.Client
	Id             int
	Key            string
	logger     log.Logger
}

type AlertmanagerConfig struct {
	Receivers    string
	Routes       string
	InhibitRules string
}

// reload alertmanager
func (c *APIClient) Reload() (error,int) {
	return c.doPost(c.Url.String())
}

// do post request
func (c *APIClient) doPost(url string) (error,int) {
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err, 0
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		for strings.Contains(err.Error(), "connection refused") {
			level.Error(c.logger).Log("msg", "Failed to reload alertmanager.yml. Perhaps Alertmanager is not ready. Waiting for 8 seconds and retry again...", "err", err.Error())
			time.Sleep(8 * time.Second)
			resp, err = c.HTTPClient.Do(req)
			if err == nil {
				break
			}
		}
	}
	if err != nil {
		return err, 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code returned from Alertmanager (got: %d, expected: 200, msg:%s)", resp.StatusCode, resp.Status), resp.StatusCode
	}
	return nil, 0
}

// return a new APIClient
func New(baseUrl *url.URL, configPath string, configTemplate string, id int, key string, logger log.Logger) *APIClient {
	return &APIClient{
		Url:    baseUrl,
		ConfigPath: configPath,
		ConfigTemplate : configTemplate,
		HTTPClient: http.DefaultClient,
		Id:         id,
		Key:        key,
		logger:     logger,
	}
}
