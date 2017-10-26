// Server.go
package graphitenotifier

type Configuration struct {
	JobScheduler  jobSchedulerConf  `json:"jobScheduler"`
	AlertExecutor alertExecutorConf `json:"alertExecutor"`
	Logging       LogOptions        `json:logging`
}

type GraphiteNotifyServer struct {
	conf          *Configuration
	jobScheduler  *jobScheduler
	alertExecutor *alertExecutor
}

func NewServer(conf Configuration) *GraphiteNotifyServer {
	return &GraphiteNotifyServer{
		conf:          &conf,
		jobScheduler:  newJobScheduler(&conf.JobScheduler),
		alertExecutor: newAlertExecutor(&conf.AlertExecutor),
	}
}

func (gns *GraphiteNotifyServer) Start() {
	initLogging(&gns.conf.Logging)
	gns.jobScheduler.Init()
	gns.jobScheduler.ScheduleJob(gns.alertExecutor)
}
