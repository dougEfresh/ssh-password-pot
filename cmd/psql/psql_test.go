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

package psql

import (
	"database/sql"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dougEfresh/passwd-pot/api"
	"github.com/dougEfresh/passwd-pot/cmd/work"
	"github.com/dougEfresh/passwd-pot/log"
	_ "github.com/lib/pq"
)

var submittedEvent *api.Event

func init() {
	logger = log.DefaultLogger(os.Stdout)
}

type mockQueue struct {
}

func (mq *mockQueue) Send(e *api.Event) {
	logger.Infof("Sent %s", e)
	submittedEvent = e
}

func TestServerRequest(t *testing.T) {
	mc := &mockQueue{}
	var wg sync.WaitGroup
	w := work.Worker{
		Addr:       "localhost:5430",
		EventQueue: mc,
		Wg:         &wg,
	}
	go Run(w, logger)
	time.Sleep(500 * time.Millisecond)

	conn, err := sql.Open("postgres", "postgres://postgres:test@127.0.0.1:5430/?sslmode=disable")
	if err != nil {
		t.Fatalf("%s", err)
	}
	conn.Ping()
	time.Sleep(250 * time.Millisecond)

	if submittedEvent == nil {
		t.Fatal("Event not sent")
	}
	if submittedEvent.User != "postgres" {
		t.Fatalf("Wrong event sent %s", submittedEvent)
	}

	if submittedEvent.Passwd != "test" {
		t.Fatalf("Wrong event sent %s", submittedEvent)
	}

	if submittedEvent.RemoteVersion == "" {
		t.Fatalf("Wrong event sent %s", submittedEvent)
	}

	if !strings.Contains(submittedEvent.RemoteAddr, "127.0.0.1") {
		t.Fatalf("Wrong event sent %s", submittedEvent)
	}

	if !strings.Contains(submittedEvent.Protocol, "psql") {
		t.Fatalf("Wrong event sent %s", submittedEvent)
	}

	if !strings.Contains(submittedEvent.Application, "psql-passwd-pot") {
		t.Fatalf("Wrong event sent %s", submittedEvent)
	}

	if submittedEvent.RemotePort == 0 {
		t.Fatalf("Wrong event sent %s", submittedEvent)
	}

	if submittedEvent.RemoteName == "" {
		t.Fatalf("Wrong event sent %s", submittedEvent)
	}
	submittedEvent = nil
}
