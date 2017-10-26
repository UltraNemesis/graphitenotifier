// GraphiteQuery.go
package graphitenotifier

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
)

type graphiteQueryResult struct {
	Items []struct {
		Target     string      `json:"target"`
		Datapoints [][]float64 `json:"datapoints"`
	}
}

type graphiteConf struct {
	Host     string `json:"host"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type graphiteClient struct {
	conf       *graphiteConf
	httpClient *http.Client
}

func newGraphiteClient(conf *graphiteConf) *graphiteClient {
	return &graphiteClient{
		conf:       conf,
		httpClient: &http.Client{},
	}
}

func (gc *graphiteClient) Query(query string) (*graphiteQueryResult, error) {
	var result graphiteQueryResult
	var err error

	req, _ := http.NewRequest("GET", gc.conf.Host+"/render?target="+query+"&format=json", nil)

	req.SetBasicAuth(gc.conf.Username, gc.conf.Password)

	log.Println("Perfoming Query : ", req.URL)

	resp, err := gc.httpClient.Do(req)

	if err == nil {
		bytes, _ := ioutil.ReadAll(resp.Body)

		err = json.Unmarshal(bytes, &result.Items)
	}

	return &result, err
}
