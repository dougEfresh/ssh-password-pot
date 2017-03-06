package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"encoding/json"
	"io/ioutil"
	"time"
)

const (
	requestBody       = `{"time": 1487973301661, "user": "admin", "passwd": "12345678", "remoteAddr": "1.2.3.4", "remotePort": 63185, "remoteName": "203.116.142.113", "remoteVersion": "SSH-2.0-JSCH-0.1.51" }`
	requestBodyOrigin = `{"time": 1487973301661, "user": "admin", "passwd": "12345678", "remoteAddr": "192.168.1.1", "remotePort": 63185, "remoteName": "203.116.142.113", "remoteVersion": "SSH-2.0-JSCH-0.1.51" , "originAddr" : "10.0.0.1"}`
)

func init() {
	defaultEventClient = testEventClient
}

func TestServerRequest(t *testing.T) {

	ts := httptest.NewServer(handlers())
	defer ts.Close()
	t.Log(fmt.Sprintf("%s%s", ts.URL, eventUrl))

	res, err := http.Post(fmt.Sprintf("%s%s", ts.URL, eventUrl),
		"application/json",
		strings.NewReader(requestBody))

	if err != nil {
		t.Error(err)
	}
	defer res.Body.Close()
	b, err := ioutil.ReadAll(res.Body)

	if res.StatusCode != http.StatusAccepted {
		t.Fatalf("Status code not 202 (%d)\n%s", res.StatusCode, string(b))
	}

	if res.ContentLength == 0 {
		t.Fatal("No Body")
	}

	var event SSHEvent
	if err != nil {
		t.Fatalf("Error reading body %s", err)
	}

	err = json.Unmarshal(b, &event)
	if err != nil {
		t.Fatalf("%s", err)
	}

	if event.ID == 0 {
		t.Fatalf("Event has id of 0 %+v", event)
	}

	time.Sleep(1 * time.Second)

	eventGeo := testEventClient.get(event.ID)
	if eventGeo == nil {
		t.Fatalf("Not not find id %d", event.ID)
	}

	if eventGeo.OriginCountry == "" {
		t.Fatal("Origin Country is null")
	}

	if eventGeo.RemoteCountry == "" {
		t.Fatal("Remote Country is null")
	}
}

func TestServerRequestWithOrigin(t *testing.T) {
	ts := httptest.NewServer(handlers())
	defer ts.Close()

	res, err := http.Post(fmt.Sprintf("%s%s", ts.URL, eventUrl),
		"application/json",
		strings.NewReader(requestBodyOrigin))

	if err != nil {
		t.Error(err)
	}

	var event SSHEvent
	b, _ := ioutil.ReadAll(res.Body)
	err = json.Unmarshal(b, &event)
	if err != nil {
		t.Fatalf("%s %s", string(b), err)
	}

	if event.ID == 0 {
		t.Fatalf("Event has id of 0 %+v", event)
	}

	time.Sleep(1 * time.Second)

	eventGeo := testEventClient.get(event.ID)
	if eventGeo == nil {
		t.Fatalf("Not not find id %d", event.ID)
	}

	if eventGeo.OriginCountry != "ZX" {
		t.Fatalf("Origin Country is not ZZ (%s)", eventGeo.OriginCountry)
	}

	if eventGeo.RemoteCountry == "" {
		t.Fatal("Remote Country is null")
	}
}
