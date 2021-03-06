package core

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"runtime"
	"time"

	// Import for side-effect of registering http handler
	_ "net/http/pprof"

	"github.com/signalfx/golib/datapoint"
	"github.com/signalfx/golib/sfxclient"
	"github.com/signalfx/signalfx-agent/internal/core/config"
	"github.com/signalfx/signalfx-agent/internal/utils"
	log "github.com/sirupsen/logrus"
)

// VersionLine should be populated by the startup logic to contain version
// information that can be reported in diagnostics.
var VersionLine string

// Serves the diagnostic status on the specified path
func (a *Agent) serveDiagnosticInfo(host string, port uint16) error {
	if a.diagnosticServer != nil {
		a.diagnosticServer.Close()
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(a.diagnosticTextHandler))
	mux.Handle("/metrics", http.HandlerFunc(a.internalMetricsHandler))

	a.diagnosticServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", host, port),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	go func() {
		log.Infof("Serving internal metrics at %s:%d", host, port)
		err := a.diagnosticServer.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.WithFields(log.Fields{
				"host":  host,
				"port":  port,
				"error": err,
			}).Error("Problem with diagnostic server")
		}
	}()

	return nil
}

func readStatusInfo(host string, port uint16) ([]byte, error) {
	resp, err := http.Get(fmt.Sprintf("http://%s:%d/", host, port))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return ioutil.ReadAll(resp.Body)
}

func (a *Agent) diagnosticTextHandler(rw http.ResponseWriter, req *http.Request) {
	rw.Write([]byte(a.DiagnosticText()))
}

// DiagnosticText returns a simple textual output of the agent's status
func (a *Agent) DiagnosticText() string {
	return fmt.Sprintf(
		"SignalFx Agent Status"+
			"\n=====================\n"+
			"\nVersion: %s"+
			"\nAgent Configuration:"+
			"\n%s\n\n"+
			"%s\n"+
			"%s\n"+
			"%s",
		VersionLine,
		utils.IndentLines(config.ToString(a.lastConfig), 2),
		a.writer.DiagnosticText(),
		a.observers.DiagnosticText(),
		a.monitors.DiagnosticText())

}

func (a *Agent) internalMetricsHandler(rw http.ResponseWriter, req *http.Request) {
	jsonOut, err := json.Marshal(a.InternalMetrics())
	if err != nil {
		log.WithError(err).Error("Could not serialize internal metrics to JSON")
		rw.WriteHeader(500)
		rw.Write([]byte(err.Error()))
		return
	}

	rw.Header().Add("Content-Type", "application/json")
	rw.WriteHeader(200)

	rw.Write(jsonOut)
}

// InternalMetrics aggregates internal metrics from subcomponents and returns a
// list of datapoints that represent the instaneous state of the agent
func (a *Agent) InternalMetrics() []*datapoint.Datapoint {
	out := make([]*datapoint.Datapoint, 0)

	mstat := runtime.MemStats{}
	runtime.ReadMemStats(&mstat)
	out = append(out, []*datapoint.Datapoint{
		sfxclient.Cumulative("sfxagent.go_total_alloc", nil, int64(mstat.TotalAlloc)),
		sfxclient.Gauge("sfxagent.go_sys", nil, int64(mstat.Sys)),
		sfxclient.Cumulative("sfxagent.go_mallocs", nil, int64(mstat.Mallocs)),
		sfxclient.Cumulative("sfxagent.go_frees", nil, int64(mstat.Frees)),
		sfxclient.Gauge("sfxagent.go_heap_alloc", nil, int64(mstat.HeapAlloc)),
		sfxclient.Gauge("sfxagent.go_heap_sys", nil, int64(mstat.HeapSys)),
		sfxclient.Gauge("sfxagent.go_heap_idle", nil, int64(mstat.HeapIdle)),
		sfxclient.Gauge("sfxagent.go_heap_inuse", nil, int64(mstat.HeapInuse)),
		sfxclient.Gauge("sfxagent.go_heap_released", nil, int64(mstat.HeapReleased)),
		sfxclient.Gauge("sfxagent.go_stack_inuse", nil, int64(mstat.StackInuse)),
		sfxclient.Gauge("sfxagent.go_next_gc", nil, int64(mstat.NextGC)),
		sfxclient.Cumulative("sfxagent.go_pause_total_ns", nil, int64(mstat.PauseTotalNs)),
		sfxclient.Cumulative("sfxagent.go_gc_cpu_fraction", nil, int64(mstat.GCCPUFraction)),
		sfxclient.Gauge("sfxagent.go_num_gc", nil, int64(mstat.NumGC)),
		sfxclient.Gauge("sfxagent.go_gomaxprocs", nil, int64(runtime.GOMAXPROCS(0))),
		sfxclient.Gauge("sfxagent.go_num_goroutine", nil, int64(runtime.NumGoroutine())),
	}...)

	out = append(out, a.writer.InternalMetrics()...)
	out = append(out, a.observers.InternalMetrics()...)
	out = append(out, a.monitors.InternalMetrics()...)

	for i := range out {
		if out[i].Dimensions == nil {
			out[i].Dimensions = make(map[string]string)
		}

		out[i].Dimensions["host"] = a.lastConfig.Hostname
		out[i].Timestamp = time.Now()
	}
	return out
}

func (a *Agent) ensureProfileServerRunning() {
	if !a.profileServerRunning {
		// We don't use that much memory so the default mem sampling rate is
		// too small to be very useful. Setting to 1 profiles ALL allocations
		runtime.MemProfileRate = 1
		// Crank up CPU profile rate too since our CPU usage tends to be pretty
		// bursty around read cycles.
		runtime.SetCPUProfileRate(-1)
		runtime.SetCPUProfileRate(2000)

		go func() {
			a.profileServerRunning = true
			// This is very difficult to access from the host on mac without
			// exposing it on all interfaces
			log.Println(http.ListenAndServe("0.0.0.0:6060", nil))
		}()
	}
}
