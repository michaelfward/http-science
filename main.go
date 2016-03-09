package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
)

// Science is an http.Handler that forwards requests it receives to two places, logging any
// difference in response.
type Science struct {
	ControlDial    string
	ExperimentDial string
	DiffLog        *log.Logger
}

// TODO: Comment explaning this...
type myReader struct {
	*bytes.Buffer
}

func (m myReader) Close() error {
	return nil
}

// forwardRequest forwards a request to an http server and returns the raw HTTP response.
// It also removes the Date header from the returned response data so you can diff it against other
// responses.
func forwardRequest(r *http.Request, addr string) (string, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return "", fmt.Errorf("error establishing tcp connection to %s: %s", addr, err)
	}
	defer conn.Close()
	read := bufio.NewReader(conn)
	if err = r.WriteProxy(conn); err != nil {
		return "", fmt.Errorf("error initializing write proxy to %s: %s", addr, err)
	}
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return "", err
	}
	toUse := myReader{bytes.NewBuffer(buf)}
	toSave := myReader{bytes.NewBuffer(buf)}
	r.Body = toUse
	res, err := http.ReadResponse(read, r)
	r.Body = toSave
	if err != nil {
		return "", fmt.Errorf("error reading response from %s: %s", addr, err)
	}
	defer res.Body.Close()
	res.Header.Del("Date")
	// Remove the Transfer-Encoding and Content-Length headers. We've seen some false positives where the control
	// returns one and the experiment returns the other, but the return the same actual body. Since we're already
	// matching the bodies and the way the data is sent over the wire doesn't matter, let's ignore these.
	res.Header.Del("Transfer-Encoding")
	res.Header.Del("Content-Length")

	resDump, err := httputil.DumpResponse(res, true)
	if err != nil {
		return "", fmt.Errorf("error dumping response from %s: %s", addr, err)
	}
	return string(resDump), nil
}

func (s Science) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// save request for potential diff logging
	reqDump, err := httputil.DumpRequest(r, true)
	if err != nil {
		log.Printf("error dumping request: %s", err)
	}
	req := string(reqDump)

	// forward requests to control and experiment, diff response
	var resControl string
	var resExperiment string
	if resControl, err = forwardRequest(r, s.ControlDial); err != nil {
		log.Printf("error forwarding request to control: %s", err)
		return
	}
	if resExperiment, err = forwardRequest(r, s.ExperimentDial); err != nil {
		log.Printf("error forwarding request to experiment: %s", err)
		return
	}

	if resControl != resExperiment {
		s.DiffLog.Printf(`=== diff ===
%s
---
%s
---
%s
============
`, req, resControl, resExperiment)
	}

	// return 200 no matter what
	fmt.Fprintf(w, "OK")
}

func main() {
	for _, env := range []string{"CONTROL", "EXPERIMENT"} {
		if os.Getenv(env) == "" {
			log.Fatalf("%s required", env)
		}
	}
	log.Fatal(http.ListenAndServe(":80", Science{
		ControlDial:    os.Getenv("CONTROL"),
		ExperimentDial: os.Getenv("EXPERIMENT"),
		DiffLog:        log.New(os.Stdout, "", 0),
	}))
}
