package cachet

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/Sirupsen/logrus"
)

type CachetAPI struct {
	URL      string `json:"url"`
	Token    string `json:"token"`
	Insecure bool   `json:"insecure"`
}

type CachetResponse struct {
	Data json.RawMessage `json:"data"`
}

// TODO: test
func (api CachetAPI) Ping() error {
	resp, _, err := api.NewRequest("GET", "/ping", nil)
	if err != nil {
		return err
	}

	if resp != nil {
		if resp.StatusCode != 200 {
			return errors.New("API responded with non-200 status code")
		}
	} else {
		return errors.New("API didn't respond")
	}

	return nil
}

// SendMetric adds a data point to a cachet monitor - Deprecated
func (api CachetAPI) SendMetric(l *logrus.Entry, id int, lag int64) {
	api.SendMetrics(l, "lag", []int { id }, lag)
}

// CheckAPIStatus displays and error message if return values are invalid
func (api CachetAPI) CheckAPIStatus(l *logrus.Entry, label string, resp *http.Response, err error) bool {
	returnCode := false

	if err != nil  {
		if l != nil {
			l.Warnf("%s returns an error (err: %v)", label, err)
		}
	} else {
		if resp != nil {
			if resp.StatusCode == 200 {
				if l != nil {
					l.Debugf("%s returns %d", label, resp.StatusCode)
				}
				returnCode = true
			} else {
				if l != nil {
					l.Warnf("%s returns (response code: %d)", label, resp.StatusCode)
				}
			}
		} else {
			if l != nil {
				l.Warnf("%s didn't response", label)
			}
		}
	}

	return returnCode
}

// SendMetrics adds a data point to a cachet monitor
func (api CachetAPI) SendMetrics(l *logrus.Entry, metricname string, arr []int, val int64) {
	for _, v := range arr {
		l.Infof("Sending %s metric ID:%d => %v", metricname, v, val)

		jsonBytes, _ := json.Marshal(map[string]interface{}{
			"value":     val,
			"timestamp": time.Now().Unix(),
		})

		resp,_, err := api.NewRequest("POST", "/metrics/"+strconv.Itoa(v)+"/points", jsonBytes)
		api.CheckAPIStatus(l, metricname+" metric (id: "+strconv.Itoa(v)+" => "+strconv.FormatInt(val, 10)+")", resp, err)
	}
}

// TODO: test
// GetComponentData
func (api CachetAPI) GetComponentData(compid int) (Component) {
	l := logrus.WithFields(logrus.Fields{ "id": compid })
	l.Debugf("Getting data from component ID:%d", compid)

	resp, body, err := api.NewRequest("GET", "/components/"+strconv.Itoa(compid), []byte(""))

	var compInfo Component
	if api.CheckAPIStatus(l, "Component data (id: "+strconv.Itoa(compid)+")", resp, err) {
		err = json.Unmarshal(body.Data, &compInfo)
	}
	return compInfo
}

// SetComponentStatus
func (api CachetAPI) SetComponentStatus(comp *AbstractMonitor, status int) (Component) {
	l := logrus.WithFields(logrus.Fields{ "id": comp.ComponentID })
	l.Debugf("Setting new status (%d) to component ID: %d (instead of %d)", status, comp.ComponentID, comp.currentStatus)

	jsonBytes, _ := json.Marshal(map[string]interface{}{
		"status":     status,
	})

	resp, body, err := api.NewRequest("PUT", "/components/"+strconv.Itoa(comp.ComponentID), jsonBytes)

	var compInfo Component
	if api.CheckAPIStatus(l, "Component data (id: "+strconv.Itoa(comp.ComponentID)+")", resp, err) {
		comp.currentStatus = status
		err = json.Unmarshal(body.Data, &compInfo)
	}
	return compInfo
}

// TODO: test
// NewRequest wraps http.NewRequest
func (api CachetAPI) NewRequest(requestType, url string, reqBody []byte) (*http.Response, CachetResponse, error) {
	req, err := http.NewRequest(requestType, api.URL+url, bytes.NewBuffer(reqBody))

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Cachet-Token", api.Token)

	transport := http.DefaultTransport.(*http.Transport)
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: api.Insecure}
	client := &http.Client{
		Transport: transport,
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, CachetResponse{}, err
	}

	var body struct {
		Data json.RawMessage `json:"data"`
	}
	err = json.NewDecoder(res.Body).Decode(&body)

	return res, body, err
}
