package cachet

import (
	"sync"
	"time"
	"strconv"
	"os/exec"

	"github.com/Sirupsen/logrus"
)

const DefaultInterval = 60
const DefaultTimeout = 1
const DefaultTimeFormat = "15:04:05 Jan 2 MST"
const DefaultHistorySize = 10

type MonitorInterface interface {
	ClockStart(*CachetMonitor, MonitorInterface, *sync.WaitGroup)
	ClockStop()
	tick(MonitorInterface)
	test(l *logrus.Entry) bool

	Init(*CachetMonitor) bool
	Validate() []string
	GetMonitor() *AbstractMonitor
	Describe() []string
}

// AbstractMonitor data model
type AbstractMonitor struct {
	Name   string
	Target string
	Enabled bool

	// (default)http / dns
	Type   string
	Strict bool

	Interval time.Duration
	Timeout  time.Duration
	Resync  int

	MetricID    int `mapstructure:"metric_id"`
	ComponentID int `mapstructure:"component_id"`

	// Metric stuff
	Metrics struct {
		ResponseTime []int	`mapstructure:"response_time"`
		Availability []int	`mapstructure:"availability"`
		IncidentCount []int	`mapstructure:"incident_count"`
	}

	// ShellHook stuff
	ShellHookOnSuccess string	`mapstructure:"on_success"`
	ShellHookOnFailure string	`mapstructure:"on_failure"`

	// Templating stuff
	Template struct {
		Investigating MessageTemplate
		Fixed         MessageTemplate
	}

	// Threshold = percentage / number of down incidents
	HistorySize      int `mapstructure:"history_size"`

	Threshold      int
	ThresholdCount int `mapstructure:"threshold_count"`

	CriticalThreshold      int `mapstructure:"threshold_critical"`
	CriticalThresholdCount int `mapstructure:"threshold_critical_count"`

	PartialThreshold      int `mapstructure:"threshold_partial"`
	PartialThresholdCount int `mapstructure:"threshold_partial_count"`

	// lag / average(lagHistory) * 100 = percentage above average lag
	// PerformanceThreshold sets the % limit above which this monitor will trigger degraded-performance
	// PerformanceThreshold float32

	resyncMod	int
	currentStatus	int
	currentDownCount int
	currentUpCount	int
	history		[]bool
	// lagHistory   []float32
	lastFailReason	string
	incident       	*Incident
	config         	*CachetMonitor

	// Closed when mon.Stop() is called
	stopC chan bool
}

func (mon *AbstractMonitor) Validate() []string {
	errs := []string{}

	if len(mon.Name) == 0 {
		errs = append(errs, "Name is required")
	}

	if mon.Interval < 1 {
		mon.Interval = DefaultInterval
	}

	if mon.Timeout < 1 {
		mon.Timeout = DefaultTimeout
	}

	mon.Interval = mon.Interval * time.Second
	mon.Timeout = mon.Timeout * time.Second

	if mon.Timeout > mon.Interval {
		errs = append(errs, "Timeout monitor ("+ mon.Timeout.String() +") is greater than interval ("+ mon.Interval.String() +")")
	}

	if mon.ComponentID == 0 && mon.MetricID == 0 {
		errs = append(errs, "component_id & metric_id are unset")
	}

	if mon.HistorySize <= 0 {
		mon.HistorySize = DefaultHistorySize
	}

	if mon.Threshold <= 0 {
		mon.Threshold = 0
	}

	if mon.Threshold > 0 && mon.Threshold > mon.HistorySize {
		mon.Threshold = mon.HistorySize
	}

	if mon.CriticalThreshold <= 0 {
		mon.CriticalThreshold = 0
	}

	if mon.CriticalThreshold > 0 && mon.CriticalThreshold > mon.HistorySize {
		mon.CriticalThreshold = mon.HistorySize
	}

	if mon.PartialThreshold <= 0 {
		mon.PartialThreshold = 0
	}

	if mon.PartialThreshold > 0 && mon.PartialThreshold > mon.HistorySize {
		mon.PartialThreshold = mon.HistorySize
	}

	if mon.Threshold == 0 && mon.CriticalThreshold == 0 && mon.PartialThreshold == 0 && mon.ThresholdCount == 0 && mon.CriticalThresholdCount == 0 && mon.PartialThresholdCount == 0 {
		mon.Threshold = 100
	}

	if err := mon.Template.Fixed.Compile(); err != nil {
		errs = append(errs, "Could not compile \"fixed\" template: "+err.Error())
	}
	if err := mon.Template.Investigating.Compile(); err != nil {
		errs = append(errs, "Could not compile \"investigating\" template: "+err.Error())
	}

	return errs
}
func (mon *AbstractMonitor) GetMonitor() *AbstractMonitor {
	return mon
}
func (mon *AbstractMonitor) Describe() []string {
	features := []string{"Type: " + mon.Type}

	if len(mon.Name) > 0 {
		features = append(features, "Name: "+mon.Name)
	}
	if len(mon.Target) > 0 {
		features = append(features, "Target: "+mon.Target)
	} else {
		features = append(features, "Target: <mock>")
	}
	features = append(features, "Availability count metrics: "+strconv.Itoa(len(mon.Metrics.Availability)))
	features = append(features, "Incident count metrics: "+strconv.Itoa(len(mon.Metrics.IncidentCount)))
	features = append(features, "Response time metrics: "+strconv.Itoa(len(mon.Metrics.ResponseTime)))
	if mon.Resync > 0 {
		features = append(features, "Resyncs cycle: " + strconv.Itoa(mon.Resync))
	}
	if len(mon.ShellHookOnSuccess) > 0 {
		features = append(features, "Has a 'on_success' shellhook")
	}
	if len(mon.ShellHookOnFailure) > 0 {
		features = append(features, "Has a 'on_failure' shellhook")
	}

	return features
}

func (mon *AbstractMonitor) ReloadCachetData() {
	compInfo := mon.config.API.GetComponentData(mon.ComponentID)

	logrus.Infof("Current CachetHQ ID: %d", compInfo.ID)
	logrus.Infof("Current CachetHQ name: %s", compInfo.Name)
	logrus.Infof("Current CachetHQ state: %t", compInfo.Enabled)
	logrus.Infof("Current CachetHQ status: %d", compInfo.Status)
	if mon.ThresholdCount > 0 || mon.Threshold > 0 {
		if mon.ThresholdCount > 0 {
			logrus.Infof("Threshold (count): %d", mon.ThresholdCount)
		} else {
			logrus.Infof("Threshold (percent): %d", mon.Threshold)
		}
	} else {
		if mon.CriticalThresholdCount > 0 || mon.CriticalThreshold > 0 {
			if mon.CriticalThresholdCount > 0 {
				logrus.Infof("Critical threshold (count): %d", mon.CriticalThresholdCount)
			} else {
				logrus.Infof("Critical threshold (percent): %d", mon.CriticalThreshold)
			}
		}
		if mon.PartialThresholdCount > 0 || mon.PartialThreshold > 0 {
			if mon.PartialThresholdCount > 0 {
				logrus.Infof("Partial threshold (count): %d", mon.PartialThresholdCount)
			} else {
				logrus.Infof("Partial threshold (percent): %d", mon.PartialThreshold)
			}
		}
	}

	mon.currentStatus = compInfo.Status
	mon.Enabled = compInfo.Enabled

	mon.incident,_ = compInfo.LoadCurrentIncident(mon.config)

	if mon.incident != nil {
		logrus.Infof("Current incident ID: %v", mon.incident.ID)
	} else {
		logrus.Infof("No current incident")
	}
}

func (mon *AbstractMonitor) Init(cfg *CachetMonitor) bool {
	mon.config = cfg

	IsValid := true

	mon.ReloadCachetData()

	if mon.ComponentID == 0 {
		logrus.Infof("ComponentID couldn't be retreived")
		IsValid = false
	}

	mon.history = append(mon.history, mon.isUp())

	return IsValid
}

func (mon *AbstractMonitor) triggerShellHook(l *logrus.Entry, hooktype string, hook string, data string) {
	if len(hook) == 0 {
		return
	}
	l.Infof("Sending '%s' shellhook", hooktype)
	l.Debugf("Data: %s", data)

	out, err := exec.Command(hook, mon.Name, strconv.Itoa(mon.ComponentID), mon.Target, hooktype, data, strconv.Itoa(mon.currentStatus), strconv.Itoa(mon.currentUpCount), strconv.Itoa(mon.currentDownCount)).Output()
	if err != nil {
	    l.Warnf("Error when processing shellhook '%s': %s", hooktype, err)
	    l.Warnf("Command output: %s", out)
	}
}

func (mon *AbstractMonitor) ClockStart(cfg *CachetMonitor, iface MonitorInterface, wg *sync.WaitGroup) {
	wg.Add(1)

	mon.stopC = make(chan bool)

	if cfg.Immediate {
		mon.tick(iface)
	}

	ticker := time.NewTicker(mon.Interval)
	for {
		select {
		case <-ticker.C:
			mon.tick(iface)
		case <-mon.stopC:
			wg.Done()
			return
		}
	}
}

func (mon *AbstractMonitor) ClockStop() {
	select {
	case <-mon.stopC:
		return
	default:
		close(mon.stopC)
	}
}

func (mon *AbstractMonitor) isUp() bool {
	return (mon.currentStatus == 1)
}

func (mon *AbstractMonitor) isPartial() bool {
	return (mon.currentStatus == 3)
}

func (mon *AbstractMonitor) isCritical() bool {
	return (mon.currentStatus == 4)
}

func (mon *AbstractMonitor) test(l *logrus.Entry) bool { return false }

func (mon *AbstractMonitor) tick(iface MonitorInterface) {
	l := logrus.WithFields(logrus.Fields{ "monitor": mon.Name })

	if(! mon.Enabled) {
		l.Printf("monitor is disabled")
		return
	}

	reqStart := getMs()
	isUp := true
	isUp = iface.test(l)
	lag := getMs() - reqStart

	if len(mon.history) == mon.HistorySize-1 {
		l.Debugf("monitor %v is now fully operational", mon.Name)
	}

	if len(mon.history) >= mon.HistorySize {
		mon.history = mon.history[len(mon.history)-(mon.HistorySize-1):]
	}
	mon.history = append(mon.history, isUp)

	mon.AnalyseData(l)

	// Will trigger shellhook 'on_failure' as this isn't done in implementations
	if ! isUp {
		mon.triggerShellHook(l, "on_failure", mon.ShellHookOnFailure, "")
	}

	// report lag
	if mon.MetricID > 0 {
		go mon.config.API.SendMetric(l, mon.MetricID, lag)
	}
	go mon.config.API.SendMetrics(l, "response time", mon.Metrics.ResponseTime, lag)

	if(mon.Resync > 0) {
		mon.resyncMod = (mon.resyncMod+1) % mon.Resync
		if(mon.resyncMod == 0) {
			l.Debugf("Reloading component's data")
			mon.ReloadCachetData()
		} else {
			l.Debugf("Resync progressbar: %d/%d", mon.resyncMod, mon.Resync)
		}
	}
}

// TODO: test
// AnalyseData decides if the monitor is statistically up or down and creates / resolves an incident
func (mon *AbstractMonitor) AnalyseData(l *logrus.Entry) {
	// look at the past few incidents
	numDown := 0
	mon.currentUpCount = 0
	mon.currentDownCount = 0
	for _, wasUp := range mon.history {
		if wasUp == false {
			numDown++
			mon.currentDownCount++
		} else {
			mon.currentUpCount++
		}
	}

	t := (float32(numDown) / float32(len(mon.history))) * 100
	if numDown == 0 {
		l.Printf("monitor is fully up")
		go mon.config.API.SendMetrics(l, "availability", mon.Metrics.Availability, 1)
	}

	if len(mon.history) != mon.HistorySize {
		// not yet saturated
		l.Debugf("Component's history has not been yet saturated (stack: %d/%d)", len(mon.history), mon.HistorySize)
		return
	}

	triggered := false
	criticalTriggered := false
	partialTriggered := false

	if numDown > 0 {
		if mon.ThresholdCount > 0 || mon.Threshold > 0 {
			if mon.ThresholdCount > 0 {
				triggered = (numDown >= mon.ThresholdCount)
				l.Printf("monitor down (down count=%d, threshold=%d)", numDown, mon.Threshold)
			} else {
				triggered = (int(t) > mon.Threshold)
				l.Printf("monitor down (down percentage=%.2f%%, threshold=%d%%)", t, mon.Threshold)
			}
		} else {
			if mon.CriticalThresholdCount > 0 || mon.CriticalThreshold > 0 {
				if mon.CriticalThresholdCount > 0 {
					criticalTriggered = (numDown >= mon.CriticalThresholdCount)
				} else {
					criticalTriggered = (int(t) > mon.CriticalThreshold)
				}
			}
			if ! criticalTriggered {
				if mon.PartialThresholdCount > 0 || mon.PartialThreshold > 0 {
					partialTriggered = (mon.PartialThresholdCount > 0 && numDown >= mon.PartialThresholdCount) || (mon.PartialThreshold > 0 && int(t) > mon.PartialThreshold)
				}
			}
			if mon.CriticalThresholdCount > 0 || mon.PartialThresholdCount > 0 {
				l.Printf("monitor down (down count=%d, partial threshold=%d, critical threshold=%d)", numDown, mon.PartialThresholdCount, mon.CriticalThresholdCount)
			}
			if mon.CriticalThreshold > 0 || mon.PartialThreshold > 0 {
				l.Printf("monitor down (down percentage=%.2f%%, partial threshold=%d%%, critical threshold=%d%%)", t, mon.PartialThreshold, mon.CriticalThreshold)
			}
		}
		l.Debugf("Down count: %d, history: %d, percentage: %.2f%%", numDown, len(mon.history), t)
		l.Debugf("Is triggered: %t", triggered)
		l.Debugf("Is critically Triggered: %t", criticalTriggered)
		l.Debugf("Is partially Triggered: %t", partialTriggered)
		l.Debugf("Monitor's current incident: %v", mon.incident)

		if triggered || criticalTriggered || partialTriggered {
			// Process metric
			go mon.config.API.SendMetrics(l, "incident count", mon.Metrics.IncidentCount, 1)

			if mon.incident == nil {
				// create incident
				mon.currentStatus = 2
				tplData := getTemplateData(mon)
				tplData["FailReason"] = mon.lastFailReason

				subject, message := mon.Template.Investigating.Exec(tplData)
				incidentForceComponentStatus := 4
				if partialTriggered {
					incidentForceComponentStatus = 3
				}
				mon.incident = &Incident{
					Name:        subject,
					ComponentID: mon.ComponentID,
					Message:     message,
					Notify:      true,
					ComponentStatus: incidentForceComponentStatus,
				}

				// is down, create an incident
				l.Warnf("creating incident. Monitor is down: %v", mon.lastFailReason)
				// set investigating status
				mon.incident.SetInvestigating()
				// create incident 
				if err := mon.incident.Send(mon.config); err != nil {
					l.Printf("Error sending incident: %v", err)
				}
			}
			if triggered || criticalTriggered {
				if (! mon.isCritical()) {
					mon.config.API.SetComponentStatus(mon, 4)
				}
			}
			if partialTriggered {
				if (! mon.isPartial()) {
					mon.config.API.SetComponentStatus(mon, 3)
				}
			}
			return
		}
	}

	// we are up to normal

	// global status seems incorrect though we couldn't fid any prior incident
	if ! mon.isUp() && mon.incident == nil {
		l.Info("Reseting component's status")
		mon.lastFailReason = ""
		mon.incident = nil
		mon.config.API.SetComponentStatus(mon, 1)
		return
	}

	if mon.incident == nil {
		return
	}

	// was down, created an incident, its now ok, make it resolved.
	l.Infof("Resolving incident %d", mon.incident.ID)

	// resolve incident
	tplData := getTemplateData(mon)
	tplData["incident"] = mon.incident

	subject, message := mon.Template.Fixed.Exec(tplData)
	mon.incident.Name = subject
	mon.incident.Message = message
	mon.incident.SetFixed()
	if err := mon.incident.Send(mon.config); err != nil {
		l.Warnf("Error updating sending incident: %v", err)
	}

	mon.lastFailReason = ""
	mon.incident = nil
	mon.currentStatus = 1
}
