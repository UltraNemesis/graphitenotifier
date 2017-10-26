// AlertExecutor.go
package graphitenotifier

import (
	"bytes"
	"crypto/tls"
	"html/template"
	"log"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/oleksandr/conditions"
	"gopkg.in/gomail.v2"
)

type alertExecutorConf struct {
	AlertSchedule     string       `json:"alertSchedule"`
	AlertFileLocation string       `json:"alertFileLocation"`
	Graphite          graphiteConf `json:"grahite"`
	Mail              mailConf     `json:"mail"`
}

type mailConf struct {
	Server string `json:"server"`
	From   string `json:"from"`
}

type alertExecutor struct {
	conf              *alertExecutorConf
	graphite          *graphiteClient
	oldQueryDataStore map[string]map[string]map[string]interface{}
	running           bool
}

type alertParams struct {
	Metric    string
	Subject   string
	Condition string
	DateTime  string
	Values    map[string]map[string]interface{}
}

func newAlertExecutor(conf *alertExecutorConf) *alertExecutor {
	return &alertExecutor{
		conf:              conf,
		graphite:          newGraphiteClient(&conf.Graphite),
		oldQueryDataStore: make(map[string]map[string]map[string]interface{}),
	}
}

func (ae *alertExecutor) Schedule() string {
	return ae.conf.AlertSchedule
}

func (ae *alertExecutor) Run() {
	if !ae.running {
		ae.running = true

		pattern := ae.conf.AlertFileLocation + "/*.json"

		log.Println("Enumerating Rule Files with pattern : ", pattern)

		files, _ := filepath.Glob(pattern)

		log.Println("Rule Files found : ", len(files))

		for _, file := range files {
			alert, err := loadAlertFromFile(file)

			log.Println(alert)

			if err == nil {
				ae.executeAlert(alert)
			}
		}

		ae.running = false
	} else {
		log.Println("Last run not finished yet")
	}
}

func (ae *alertExecutor) executeAlert(alert *Alert) {
	queryData, queryErr := ae.query(alert)

	if queryErr != nil {
		log.Println(queryErr)
	} else {
		log.Println(queryData)

		titleFormatter := func(name string) string {
			output := name
			for index := 0; index < len(alert.DisplayFormat.Title); index++ {
				matchRegEx, regExErr := regexp.Compile(alert.DisplayFormat.Title[index].Match)

				if regExErr != nil {
					log.Println(regExErr)
					break
				} else {
					output = matchRegEx.ReplaceAllString(output, alert.DisplayFormat.Title[index].Replace)
				}
			}

			return output
		}

		valueFormatter := func(value float64) string {
			result := strconv.Itoa(int(value))
			switch alert.DisplayFormat.Units {
			case "milliseconds":
				duration := time.Duration(int(value)) * time.Millisecond

				result = duration.Round(time.Millisecond).String()
			case "bytes":
				result = humanize.IBytes(uint64(value))
			default:

			}
			return result
		}

		oldQueryData, exists := ae.oldQueryDataStore[alert.Name]

		filtered := queryData

		if exists {
			filtered = ae.filterQueryData(queryData, oldQueryData)
		}

		for conditionName, condition := range alert.AlertConditions {
			log.Println("Evaluating ", conditionName, " with condition ", condition)

			results := ae.filterByCondition(filtered, condition)

			if len(results) > 0 {
				formatted := formatForDisplay(results, titleFormatter, valueFormatter)

				ap := &alertParams{
					Metric:    alert.Name,
					Subject:   alert.Mail.Subject,
					Condition: conditionName,
					DateTime:  time.Now().UTC().Format(time.RFC850),
					Values:    formatted,
				}

				mailBody, renderErr := ae.renderAlertMail(alert.Mail.HTMLTemplate, ap)

				if renderErr != nil {
					log.Println(renderErr)
				} else {
					ae.sendAlertMail(alert.Mail.ToList, alert.Mail.CCList, alert.Mail.Subject, mailBody)
				}
			} else {
				log.Println("No alerts to send")
			}
		}
		ae.oldQueryDataStore[alert.Name] = queryData
	}
}

func (ae *alertExecutor) filterQueryData(queryData, oldQueryData map[string]map[string]interface{}) map[string]map[string]interface{} {
	filtered := make(map[string]map[string]interface{})

	for name, oldValues := range oldQueryData {
		values, exists := queryData[name]

		if (exists && values["$currentTime"] != oldValues["$currentTime"]) || !exists {
			filtered[name] = values
			log.Println("Adding ", name)
		} else {
			log.Println("Excluding ", name)
		}
	}

	return filtered
}

func formatForDisplay(data map[string]map[string]interface{}, titleFormatter func(string) string, valueFormatter func(float64) string) map[string]map[string]interface{} {
	formatted := make(map[string]map[string]interface{})

	for name, metrics := range data {
		name := titleFormatter(name)

		formattedValues := make(map[string]interface{})
		for key, value := range metrics {
			switch value.(type) {
			case float64:
				formattedValues[key] = valueFormatter(value.(float64))
			default:
				formattedValues[key] = value
			}
		}

		formatted[name] = formattedValues
	}

	log.Println(formatted)

	return formatted
}

func (ae *alertExecutor) query(alert *Alert) (map[string]map[string]interface{}, error) {
	result, queryErr := ae.graphite.Query(alert.Query)
	m := make(map[string]map[string]interface{})

	if queryErr != nil {
		log.Println(queryErr)
	} else {

		stateFunc := ae.getStateFunc(alert.States)

		for targetIndex := 0; targetIndex < len(result.Items); targetIndex++ {
			v := make(map[string]interface{})

			currentTime := time.Duration(result.Items[targetIndex].Datapoints[len(result.Items[targetIndex].Datapoints)-1][1]) * time.Second
			previousTime := time.Duration(result.Items[targetIndex].Datapoints[len(result.Items[targetIndex].Datapoints)-2][1]) * time.Second

			currentValue := result.Items[targetIndex].Datapoints[len(result.Items[targetIndex].Datapoints)-1][0]
			previousValue := result.Items[targetIndex].Datapoints[len(result.Items[targetIndex].Datapoints)-2][0]

			v["$currentValue"] = currentValue
			v["$previousValue"] = previousValue
			v["$previousState"] = stateFunc(previousValue)
			v["$currentState"] = stateFunc(currentValue)

			v["$currentTime"] = currentTime
			v["$previousTime"] = previousTime

			m[result.Items[targetIndex].Target] = v
		}
	}

	log.Printf("%#v", m)

	return m, queryErr
}

func (ae *alertExecutor) getStateFunc(states map[string]string) func(value float64) string {
	replacer := strings.NewReplacer("$value", "$0")

	stateExpressions := make(map[string]conditions.Expr)

	for state, condition := range states {
		parser := conditions.NewParser(strings.NewReader(replacer.Replace(condition)))
		expr, parseErr := parser.Parse()

		if parseErr != nil {
			log.Println(parseErr)
		} else {
			stateExpressions[state] = expr
		}
	}

	return func(value float64) string {
		result := "undefined"

		params := map[string]interface{}{
			"$0": value,
		}

		for state, expr := range stateExpressions {
			satisfied, err := conditions.Evaluate(expr, params)

			if err == nil && satisfied {
				result = state
			}
		}

		return result
	}
}

func (ae *alertExecutor) filterByCondition(data map[string]map[string]interface{}, condition string) map[string]map[string]interface{} {
	replacer := strings.NewReplacer("$currentValue", "$0", "$previousValue", "$1", "$currentState", "$2", "$previousState", "$3")

	newCondition := replacer.Replace(condition)

	log.Println("Updated Condition = ", newCondition)

	parser := conditions.NewParser(strings.NewReader(newCondition))
	expr, parseErr := parser.Parse()

	if parseErr != nil {
		log.Println(parseErr)
	}

	output := make(map[string]map[string]interface{})

	for target, values := range data {
		params := map[string]interface{}{
			"$0": values["$currentValue"],
			"$1": values["$previousValue"],
			"$2": values["$currentState"],
			"$3": values["$previousState"],
		}
		satisfied, err := conditions.Evaluate(expr, params)

		if err != nil {
			log.Println(err)
		}

		if satisfied {
			output[target] = values
		}

		log.Println(params, "Satisfied :", satisfied)
	}

	return output
}

func (ae *alertExecutor) renderAlertMail(alertTemplateFile string, data interface{}) (string, error) {
	var outBytes bytes.Buffer
	var err error

	alertTemplate, err := template.ParseFiles(alertTemplateFile)

	if err == nil {
		err = alertTemplate.Execute(&outBytes, data)
	}

	return outBytes.String(), err
}

func (ae *alertExecutor) sendAlertMail(toList []string, ccList []string, subject string, body string) {
	splits := strings.Split(ae.conf.Mail.Server, ":")
	mailHost := splits[0]
	mailPort := 25

	if len(splits) > 1 {
		mailPort, _ = strconv.Atoi(splits[1])
	}

	log.Println("Sending mail to Recipients : ", toList, ", CC : ", ccList)

	gmd := gomail.NewDialer(mailHost, mailPort, "", "")

	gmd.TLSConfig = &tls.Config{InsecureSkipVerify: true}

	m := gomail.NewMessage()
	m.SetHeader("From", ae.conf.Mail.From)
	m.SetHeader("To", toList...)
	m.SetHeader("Cc", ccList...)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)

	if err := gmd.DialAndSend(m); err != nil {
		log.Println(err)
	}
}
