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
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/dougEfresh/passwd-pot/api"
	"github.com/dougEfresh/passwd-pot/cmd/listen"
	"github.com/dougEfresh/passwd-pot/cmd/work"
	"github.com/dougEfresh/passwd-pot/log"
)

func sendEvent(worker work.Worker, user string, password string, params []byte, remoteAddrPair []string) {
	logger.Debugf("processing request %s %s", user, password)
	remotePort, err := strconv.Atoi(remoteAddrPair[1])
	if err != nil {
		remotePort = 0
	}
	e := &api.Event{
		Time:          api.EventTime(time.Now().UTC()),
		User:          user,
		Passwd:        password,
		RemoteAddr:    remoteAddrPair[0],
		RemoteName:    remoteAddrPair[0],
		RemotePort:    remotePort,
		Application:   "psql-passwd-pot",
		Protocol:      "psql",
		RemoteVersion: strings.Join(nullTermToStrings(params), "|"),
	}

	if names, err := net.LookupAddr(e.RemoteAddr); err == nil && len(names) > 0 {
		e.RemoteName = names[0]
	}
	worker.EventQueue.Send(e)
}

func nullTermToStrings(b []byte) (s []string) {
	for {
		i := bytes.IndexByte(b, 0)
		if i == -1 {
			break
		}
		s = append(s, string(b[0:i]))
		b = b[i+1:]
	}
	return
}

type handler struct {
	c       net.Conn
	buf     *bufio.Reader
	namei   int
	scratch [512]byte
	work    work.Worker
}

func (cn *handler) recvMessage(r *readBuf) (byte, error) {
	x := make([]byte, 5)
	_, err := io.ReadFull(cn.buf, x)
	if err != nil {
		return 0, err
	}

	// read the type and length of the message that follows
	t := x[0]
	logger.Debugf("recvMessage %c", t)
	n := int(binary.BigEndian.Uint32(x[1:])) - 4
	var y []byte
	if n <= len(cn.scratch) {
		y = cn.scratch[:n]
	} else {
		y = make([]byte, n)
	}
	_, err = io.ReadFull(cn.buf, y)
	if err != nil {
		return 0, err
	}
	*r = y
	return t, nil
}

func (cn *handler) recvStartMessage(r *readBuf) error {
	x := cn.scratch[0:4]
	_, err := io.ReadFull(cn.buf, x)
	if err != nil {
		return err
	}
	logger.Debugf("recvStartMessage read %s", x)
	// read the type and length of the message that follows

	n := int(binary.BigEndian.Uint32(x)) - 8
	logger.Debugf("recvStartMessage size is %d", n)
	x = make([]byte, 4)
	_, err = io.ReadFull(cn.buf, x)
	if err != nil {
		return err
	}
	version := int(binary.BigEndian.Uint32(x))
	logger.Debugf("recvStartMessage got protocol %d", version)
	if version == 80877103 {
		logger.Info("recvStartMessage got ssl request")
		_, err := cn.c.Write([]byte{'N'})
		if err != nil {
			return err
		}
		return cn.recvStartMessage(r)
	} else {
		logger.Info("recvStartMessage got non-ssl request")
	}
	var y []byte
	y = make([]byte, n)
	_, err = io.ReadFull(cn.buf, y)
	if err != nil {
		return err
	}
	*r = y
	return nil
}

func (cn *handler) recv() (t byte, r *readBuf, err error) {
	for {
		r = &readBuf{}
		t, err = cn.recvMessage(r)
		if err != nil {
			return
		}

		switch t {
		case 'E':
			err = errors.New("Got E back")
			return
		case 'N':
			// ignore
		default:
			return
		}
	}
}

func (cn *handler) writeBuf(b byte) *writeBuf {
	cn.scratch[0] = b
	return &writeBuf{
		buf: cn.scratch[:5],
		pos: 1,
	}
}
func (cn *handler) send(m *writeBuf) (int, error) {
	return cn.c.Write(m.wrap())
}

func (cn *handler) handleClient() {
	// read start packet
	defer cn.c.Close()
	remoteAddrPair := strings.Split(cn.c.RemoteAddr().String(), ":")
	var user string
	var pass string
	r := &readBuf{}
	err := cn.recvStartMessage(r)
	logger.Infof("handleClient startUp %+s %s", r, err)
	if err != nil {
		logger.Errorf("handleClient can't get startup packet %s", err)
		return
	}
	params := bytes.Split(*r, []byte{0})
	for n, v := range params {
		if string(v) == "user" {
			user = string(params[n+1])
		}
	}

	w := cn.writeBuf('R')
	w.int32(3)
	n, err := cn.send(w)
	if err == io.EOF {
		logger.Errorf("Error sending R (%d) %s", n, *r)
		go sendEvent(cn.work, user, pass, *r, remoteAddrPair)
		return
	}
	t, msg, err := cn.recv()
	if err != nil {
		logger.Errorf("handleClient error reading %s", err)
		go sendEvent(cn.work, user, pass, *r, remoteAddrPair)
		return
	}
	logger.Debugf("handleClient got %+s %s", t, msg)
	// strip \0
	pass = string([]byte(*msg)[:len(*msg)-1])
	go sendEvent(cn.work, user, pass, *r, remoteAddrPair)
	w = cn.writeBuf('E')
	w.string("SFATAL")
	w.string("C28P01")
	w.string(fmt.Sprintf("Mpassword authentication failed for user \"%s\"", user))
	w.string("Fauth.c")
	w.string("L288")
	w.string("Rauth_failed")
	w.string("")
	cn.send(w)
}

type server struct {
	work.Worker
}

func (s server) HandleConnection(conn net.Conn) {
	cn := &handler{
		c:    conn,
		buf:  bufio.NewReader(conn),
		work: s.Worker,
	}
	cn.handleClient()
}

func Run(worker work.Worker, l log.FieldLogger) {
	logger = l
	listen.Run(worker, server{worker}, l)
}

var logger log.FieldLogger
