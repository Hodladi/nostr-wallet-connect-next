package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

func (api *api) RequestEsploraApi(endpoint string) (interface{}, error) {
	url := api.svc.GetConfig().GetEnv().LDKEsploraServer + endpoint

	client := http.Client{
		Timeout: time.Second * 10,
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		api.logger.WithError(err).WithFields(logrus.Fields{
			"url": url,
		}).Error("Failed to create http request")
		return nil, err
	}

	res, err := client.Do(req)
	if err != nil {
		api.logger.WithError(err).WithFields(logrus.Fields{
			"url": url,
		}).Error("Failed to send request")
		return nil, err
	}

	defer res.Body.Close()

	body, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		api.logger.WithError(err).WithFields(logrus.Fields{
			"url": url,
		}).Error("Failed to read response body")
		return nil, errors.New("failed to read response body")
	}

	var jsonContent interface{}
	jsonErr := json.Unmarshal(body, &jsonContent)
	if jsonErr != nil {
		api.logger.WithError(jsonErr).WithFields(logrus.Fields{
			"url": url,
		}).Error("Failed to deserialize json")
		return nil, fmt.Errorf("failed to deserialize json %s %s", url, string(body))
	}
	return jsonContent, nil
}
