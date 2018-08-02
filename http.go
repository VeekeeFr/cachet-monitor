package cachet

import (
	"crypto/tls"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
)

// Investigating template
var defaultHTTPInvestigatingTpl = MessageTemplate{
	Subject: `{{ .Monitor.Name }} - {{ .SystemName }}`,
	Message: `{{ .Monitor.Name }} check **failed** (server time: {{ .now }})

{{ .FailReason }}`,
}

// Fixed template
var defaultHTTPFixedTpl = MessageTemplate{
	Subject: `{{ .Monitor.Name }} - {{ .SystemName }}`,
	Message: `**Resolved** - {{ .now }}

- - -

{{ .incident.Message }}`,
}

type HTTPMonitor struct {
	AbstractMonitor `mapstructure:",squash"`

	Method             string
	ExpectedStatusCode int `mapstructure:"expected_status_code"`
	Headers            map[string]string

	// compiled to Regexp
	ExpectedBody string `mapstructure:"expected_body"`
	bodyRegexp   *regexp.Regexp
	internalBodyRegexp   string
}

func (monitor *HTTPMonitor) setBodyRegexp(errs []string) {
	monitor.internalBodyRegexp = monitor.ExpectedBody;
	monitor.bodyRegexp = nil

	if len(monitor.internalBodyRegexp) > 0 {
		currentTime := time.Now()

		monitor.internalBodyRegexp = strings.Replace(monitor.internalBodyRegexp, "%year%", currentTime.Format("2006"), -1)
		monitor.internalBodyRegexp = strings.Replace(monitor.internalBodyRegexp, "%month%", currentTime.Format("01"), -1)
		monitor.internalBodyRegexp = strings.Replace(monitor.internalBodyRegexp, "%day%", currentTime.Format("02"), -1)

		exp, err := regexp.Compile(monitor.internalBodyRegexp)
		if err != nil {
			if errs != nil {
				errs = append(errs, "Regexp compilation failure: "+err.Error())
			}
		} else {
			monitor.bodyRegexp = exp
		}
	}
}

// TODO: test
func (monitor *HTTPMonitor) test(l *logrus.Entry) bool {

	req, err := http.NewRequest(monitor.Method, monitor.Target, nil)
	for k, v := range monitor.Headers {
		req.Header.Add(k, v)
	}
	req.Header.Set("User-Agent", "Cachet-Monitor")

	client := &http.Client{
		Timeout:   time.Duration(monitor.Timeout),
		Transport: &http.Transport{
	                TLSClientConfig: &tls.Config{
	                        InsecureSkipVerify: (! monitor.Strict),
	                },
		 },
	}
	l.Debugf("InsecureSkipVerify: %t", (! monitor.Strict))

	resp, err := client.Do(req)
	if err != nil {
		monitor.lastFailReason = err.Error()
		l.Infof("HTTP call failure: %s", monitor.lastFailReason)
		return false
	}

	defer resp.Body.Close()

	if monitor.ExpectedStatusCode > 0 && resp.StatusCode != monitor.ExpectedStatusCode {
		monitor.lastFailReason = "Expected HTTP response status: " + strconv.Itoa(monitor.ExpectedStatusCode) + ", got: " + strconv.Itoa(resp.StatusCode)
		l.Infof("%s", monitor.lastFailReason)
		return false
	}

	monitor.setBodyRegexp(nil)

	responseBody, err := ioutil.ReadAll(resp.Body)

	if monitor.bodyRegexp != nil {
		if err != nil {
			monitor.lastFailReason = err.Error()
			l.Infof("HTTP response error: %s", monitor.lastFailReason)
			return false
		}
		if !monitor.bodyRegexp.Match(responseBody) {
			monitor.lastFailReason = "Unexpected body: " + string(responseBody) + ".\nExpected to match: " + monitor.internalBodyRegexp
			l.Infof("HTTP response error: Unexpected body")
			return false
		}
	}

	monitor.triggerShellHook(l, "on_success", monitor.ShellHookOnSuccess, string(responseBody))

	return true
}

// TODO: test
func (mon *HTTPMonitor) Validate() []string {
	mon.Template.Investigating.SetDefault(defaultHTTPInvestigatingTpl)
	mon.Template.Fixed.SetDefault(defaultHTTPFixedTpl)

	errs := mon.AbstractMonitor.Validate()

	if len(mon.Target) == 0 {
		errs = append(errs, "'Target' has not been set")
	}

	if len(mon.ExpectedBody) == 0 && mon.ExpectedStatusCode == 0 {
		errs = append(errs, "Both 'expected_body' and 'expected_status_code' fields empty")
	}

	mon.setBodyRegexp(errs)

	mon.Method = strings.ToUpper(mon.Method)
	switch mon.Method {
		case "GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD":
			break
		case "":
			mon.Method = "GET"
		default:
			errs = append(errs, "Unsupported HTTP method: "+mon.Method)
	}

	return errs
}

func (mon *HTTPMonitor) Describe() []string {
	features := mon.AbstractMonitor.Describe()
	features = append(features, "Method: "+mon.Method)
	features = append(features, "Insecure: "+ strconv.FormatBool(!mon.Strict))

	return features
}
