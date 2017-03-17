package ftp

import (
	"bufio"
	"github.com/Sirupsen/logrus"
	"github.com/dougEfresh/passwd-pot/api"
	"github.com/dougEfresh/passwd-pot/cmd/work"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

var submittedEvent *api.Event

func init() {
	logrus.SetLevel(logrus.DebugLevel)
}

type mockQueue struct {
}

func (mq *mockQueue) Send(e *api.Event) {
	submittedEvent = e
}

func TestServerRequest(t *testing.T) {
	mc := &mockQueue{}
	var wg sync.WaitGroup
	w := &work.Worker{
		Addr:       "localhost:2121",
		EventQueue: mc,
		Wg:         &wg,
	}
	go Run(w)
	time.Sleep(500 * time.Millisecond)
	conn, err := net.Dial("tcp", "localhost:2121")
	if err != nil {
		t.Fatalf("Error! %s", err)
	}
	msg, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("Error! %s", err)
	}
	defer conn.Close()
	msg = strings.Replace(msg, "\r", "", 1)

	if !strings.Contains(msg, "220 This is a private") {
		t.Fatalf("220 not there (%s)", msg)
	}
	conn.Write([]byte("USER blah\r\n"))
	msg, err = bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("Error! %s", err)
	}
	msg = strings.Replace(msg, "\r", "", 1)
	if !strings.Contains(msg, "331 User") {
		t.Fatalf("331 not there (%s)", msg)
	}
	conn.Write([]byte("PASS ugh\r\n"))
	msg, err = bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("Error! %s", err)
	}
	msg = strings.Replace(msg, "\r\n", "", 1)
	if !strings.Contains(msg, "530 Login authentication failed") {
		t.Fatalf("530 not there (%s)", msg)
	}
	conn.Write([]byte("QUIT\r\n"))
	time.Sleep(200 * time.Millisecond)
	if submittedEvent == nil {
		t.Fatal("Submitted event is null")
	}
	if submittedEvent.User != "blah" {
		t.Fatalf("Wrong event sent %s", submittedEvent)
	}

	if submittedEvent.Passwd != "ugh" {
		t.Fatalf("Wrong event sent %s", submittedEvent)
	}

	if !strings.Contains(submittedEvent.RemoteVersion, "") {
		t.Fatalf("Wrong event sent %s", submittedEvent)
	}

	if !strings.Contains(submittedEvent.RemoteAddr, "127.0.0.1") {
		t.Fatalf("Wrong event sent %s", submittedEvent)
	}

	if !strings.Contains(submittedEvent.Protocol, "ftp") {
		t.Fatalf("Wrong event sent %s", submittedEvent)
	}

	if !strings.Contains(submittedEvent.Application, "ftp-passwd-pot") {
		t.Fatalf("Wrong event sent %s", submittedEvent)
	}

	if submittedEvent.RemotePort == 0 {
		t.Fatalf("Wrong event sent %s", submittedEvent)
	}

	if submittedEvent.RemoteName == "" {
		t.Fatalf("Wrong event sent %s", submittedEvent)
	}

}