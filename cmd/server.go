// Copyright © 2017 Douglas Chimento <dchimento@gmail.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"encoding/json"
	log "github.com/Sirupsen/logrus"
	"github.com/Sirupsen/logrus/hooks/syslog"
	"github.com/gorilla/mux"
	"github.com/spf13/cobra"
	"io/ioutil"
	"log/syslog"
	"net/http"
	"runtime"
	"strings"
	"time"
	"github.com/gocraft/health"
	"fmt"
)

type server struct {
	auditClient eventRecorder
}

const (
	auditEventURL = "/api/v1/event"
)

func handlers(s *server) *mux.Router {
	router := mux.NewRouter()
	router.HandleFunc(auditEventURL, s.handleEvent).
		Methods("POST").
		HeadersRegexp("Content-Type", "application/json")

	return router
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "",
	Long:  "",
	Run:   run,
}

func run(cmd *cobra.Command, args []string) {
	var err error
	if config.Debug {
		log.SetLevel(log.DebugLevel)
	}
	if config.Threads > 0 {
		runtime.GOMAXPROCS(config.Threads)
	}
	defaultAuditClient := &eventClient{
		db:        loadDSN(config.Dsn),
		geoClient: geoClientTransporter(geoClient),
	}
	log.Debugf("Running %s with %s", cmd.Name(), args)
	s := &server{
		auditClient: defaultAuditClient,
	}
	srv := &http.Server{
		Handler:      handlers(s),
		Addr:         config.BindAddr,
		WriteTimeout: 3 * time.Second,
		ReadTimeout:  3 * time.Second,
	}
	if config.Syslog != "" {
		if syslogHook, err = logrus_syslog.NewSyslogHook("tcp", config.Syslog, syslog.LOG_LOCAL0, "ssh-password-pot") ; err != nil {
			log.Error("Unable to connect to local syslog daemon")
		} else {
			log.AddHook(syslogHook)
		}
	}
	defaultDbEventLogger.Debug = config.Debug
	healthMonitor(cmd.Name())
	log.Infof("Listing on %s", config.BindAddr)
	log.Fatal(srv.ListenAndServe())
}

func (s *server) handleEvent(w http.ResponseWriter, r *http.Request) {
	job := stream.NewJob(fmt.Sprintf("%s", auditEventURL))
	var event SSHEvent
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Error(err)
		job.EventErr("handle_event_invalid_body", err)
		job.Complete(health.ValidationError)
		return
	}
	if err = json.Unmarshal(b, &event); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Errorf("Error reading %s", err)
		job.EventErr("handle_event_invalid_json", err)
		job.Complete(health.ValidationError)
		return
	}

	if event.OriginAddr == "" {
		if r.Header.Get("X-Forwarded-For") != "" {
			log.Debug("Using RemoteAddr from  X-Forwarded-For")
			event.OriginAddr = r.Header.Get("X-Forwarded-For")
		} else {
			//IP:Port
			log.Debugf("Using RemoteAddr as OriginAddr %s", r.RemoteAddr)
			event.OriginAddr = strings.Split(r.RemoteAddr, ":")[0]
		}

	}

	err = s.auditClient.recordEvent(&event)
	go s.auditClient.resolveGeoEvent(&event)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorf("Error writing %+v %s", &event, err)
		job.EventErr("handle_event_event_error", err)
		job.Complete(health.Error)
		return
	}
	job.Complete(health.Success)
	j, _ := json.Marshal(event)
	w.WriteHeader(http.StatusAccepted)
	w.Header().Add("Content-type", "application/json")
	w.Write(j)
}

func init() {
	RootCmd.AddCommand(serverCmd)
}
