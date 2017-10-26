// AlertScheduler.go
package graphitenotifier

import (
	"html/template"
	"net/http"

	"github.com/bamzi/jobrunner"
)

type jobSchedulerConf struct {
	Schedule           string `json:"schedule"`
	WebConsoleEndPoint string `json:"webConsoleEndPoint"`
}

type scheduledJob interface {
	Schedule() string
	Run()
}

type jobScheduler struct {
	conf *jobSchedulerConf
}

var statusTmplText string = `
<html>
	<head>
		<style>
body {
	margin: 30px 0 0 0;
  font-size: 11px;
  font-family: sans-serif;
  color: #345;

}
h1 {
	font-size: 24px;
	text-align: center;
	padding: 10 0 30px;
}
table {
	/*max-width: 80%;*/
	margin: 0 auto;
  border-collapse: collapse;
  border: none;
}
table td, table th {
	min-width: 25px;
	width: auto;
  padding: 15px 20px;
  border: none;
}


table tr:nth-child(odd) {
  background-color: #f0f0f0;
}
table tr:nth-child(1) {
  background-color: #345;
  color: white;
}
th {
  text-align: left;
}

		</style>
	</head>
	<body>

<h1>AutoSiteSpeed Status Console</h1>

<table>
	<tr><th>ID</th><th>Name</th><th>Status</th><th>Last run</th><th>Next run</th><th>Latency</th></tr>
{{range .}}

	<tr>
		<td>{{.Id}}</td>
		<td>{{.JobRunner.Name}}</td>
		<td>{{.JobRunner.Status}}</td>
		<td>{{if not .Prev.IsZero}}{{.Prev.Format "2006-01-02 15:04:05"}}{{end}}</td>
		<td>{{if not .Next.IsZero}}{{.Next.Format "2006-01-02 15:04:05"}}{{end}}</td>
		<td>{{.JobRunner.Latency}}</td>
	</tr>
{{end}}
</table>
</body>
`
var statusTmpl *template.Template = nil

func init() {
	if statusTmpl == nil {
		statusTmpl, _ = template.New("status").Parse(statusTmplText)
	}
}

func newJobScheduler(conf *jobSchedulerConf) *jobScheduler {
	return &jobScheduler{
		conf: conf,
	}
}

func (as *jobScheduler) Init() {
	jobrunner.Start()
	http.HandleFunc("/status", as.statusHandler)
	go http.ListenAndServe(as.conf.WebConsoleEndPoint, nil)
}

func (as *jobScheduler) ScheduleJob(job scheduledJob) {
	jobrunner.Now(job)
	jobrunner.Schedule(job.Schedule(), job)

}

func (ts *jobScheduler) statusHandler(w http.ResponseWriter, r *http.Request) {
	statusTmpl.Execute(w, jobrunner.StatusPage())
}
