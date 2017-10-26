// Rule.go
package graphitenotifier

import (
	"encoding/json"
	"io/ioutil"
)

type Alert struct {
	Name          string `json:"name"`
	Query         string `json:"query"`
	DisplayFormat struct {
		Title []struct {
			Match   string `json:"match"`
			Replace string `json:"replace"`
		}
		Units string `json:"units"`
	} `json:"displayFormat"`
	States          map[string]string `json:"states"`
	AlertConditions map[string]string `json:"alertConditions"`
	Mail            struct {
		HTMLTemplate string   `json:"htmlTemplate"`
		ToList       []string `json:"toList"`
		CCList       []string `json:"ccList"`
		Subject      string   `json:"subject"`
	} `json:"mail"`
}

func loadAlertFromFile(file string) (*Alert, error) {
	var alert Alert
	var err error
	bytes, err := ioutil.ReadFile(file)

	if err == nil {
		err = json.Unmarshal(bytes, &alert)
	}

	return &alert, err
}
