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

package ftp

import (
	log "github.com/Sirupsen/logrus"
	"strings"

	"time"

	"bufio"
	"github.com/dougEfresh/passwd-pot/api"
	"github.com/dougEfresh/passwd-pot/cmd/queue"
	"github.com/dougEfresh/passwd-pot/cmd/work"
	"io"
	"net"
	"strconv"
)

var unAuthorized []byte = []byte("530 Login authentication failed\r\n")
var userOk []byte = []byte("331 User OK\r\n")

func (p *potHandler) sendEvent(user string, password string, remoteAddrPair []string) {
	log.Debugf("processing request %s %s", user, password)
	remotePort, err := strconv.Atoi(remoteAddrPair[1])
	if err != nil {
		remotePort = 0
	}
	e := &api.Event{
		Time:        api.EventTime(time.Now().UTC()),
		User:        user,
		Passwd:      password,
		RemoteAddr:  remoteAddrPair[0],
		RemoteName:  remoteAddrPair[0],
		RemotePort:  remotePort,
		Application: "ftp-passwd-pot",
		Protocol:    "ftp",
	}

	if names, err := net.LookupAddr(e.RemoteAddr); err == nil && len(names) > 0 {
		e.RemoteName = names[0]
	}
	p.eventQueue.Send(e)
}

type potHandler struct {
	eventQueue queue.EventQueue
}

func (p *potHandler) handleNewConnection(conn net.Conn) {
	if _, err := conn.Write([]byte("220 This is a private system - No anonymous login\r\n")); err != nil {
		log.Errorf("Error sending 220 %s", err)
		conn.Close()
		return
	}
	p.handleConnection(conn)
}
func getCommand(line string) (string, []string) {
	line = strings.Trim(line, "\r \n")
	cmd := strings.Split(line, " ")
	return cmd[0], cmd[1:]
}

func getSafeArg(args []string, nr int) (string, error) {
	if nr < len(args) {
		return args[nr], nil
	}
	log.Error("Out of range")
	return "", nil
}

func (p *potHandler) handleConnection(conn net.Conn) {
	defer conn.Close()
	var user string
	var pass string
	remoteAddrPair := strings.Split(conn.RemoteAddr().String(), ":")
	for {
		cmd, args, err := readCommand(conn)
		if err == io.EOF {
			return
		}
		if err != nil {
			log.Errorf("Error reading cmd %s", err)
			return
		}
		if cmd == "USER" {
			if _, err := conn.Write(userOk); err != nil {
				log.Error("Error writing 331 User")
				return
			}
			user = args[0]
			continue
		}

		if cmd == "PASS" {
			pass = args[0]
			go p.sendEvent(user, pass, remoteAddrPair)
			if conn.Write(unAuthorized); err != nil {
				log.Errorf("Error sending unauthorized")
				return
			}
			continue
		}
		if cmd == "QUIT" {
			return
		}
		log.Errorf("Unknown command! %s %s", cmd, args)
	}
}

func readCommand(conn net.Conn) (string, []string, error) {
	c, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return "", nil, err
	}
	cmd, args := getCommand(c)
	return cmd, args, nil
}

func Run(worker work.Worker) {
	defer worker.Wg.Done()
	if worker.Addr == "" {
		log.Warn("Not starting ftp pot")
		return
	}
	ln, err := net.Listen("tcp", worker.Addr)
	if err != nil {
		log.Errorf("Cannot bind to %s %s", worker.Addr, err)
		return
	}
	log.Infof("Started ftp pot on %s", worker.Addr)
	p := &potHandler{
		eventQueue: worker.EventQueue,
	}
	for {
		conn, _ := ln.Accept()
		go p.handleNewConnection(conn)
	}
}