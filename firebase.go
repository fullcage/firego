/*
Package firego is a REST client for Firebase (https://firebase.com).
*/
package firego

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net"
	"net/http"
	_url "net/url"
	"strings"
	"sync"
	"time"
)

// TimeoutDuration is the length of time any request will have to establish
// a connection and receive headers from Firebase before returning
// an ErrTimeout error
var TimeoutDuration = 30 * time.Second

// ErrTimeout is an error type is that is returned if a request
// exceeds the TimeoutDuration configured
type ErrTimeout struct {
	error
}

// query parameter constants
const (
	authParam    = "auth"
	formatParam  = "format"
	shallowParam = "shallow"
	formatVal    = "export"
)

// Firebase represents a location in the cloud
type Firebase struct {
	url    string
	params _url.Values
	client *http.Client

	watchMtx     sync.Mutex
	watching     bool
	stopWatching chan struct{}
}

func sanitizeURL(url string) string {
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		url = "https://" + url
	}

	if strings.HasSuffix(url, "/") {
		url = url[:len(url)-1]
	}

	return url
}

// New creates a new Firebase reference
func New(url string) *Firebase {
	return &Firebase{
		url:          sanitizeURL(url),
		params:       _url.Values{},
		client:       newClient(),
		stopWatching: make(chan struct{}),
	}
}

// newClient returns a *http.Client configured with TimeoutDuration and
// a http.RoundTripper formed exactly like http.DefaultTransport
func newClient() *http.Client {
	return &http.Client{
		Timeout: TimeoutDuration,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			Dial: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
}

// String returns the string representation of the
// Firebase reference
func (fb *Firebase) String() string {
	return fb.url
}

// Child creates a new Firebase reference for the requested
// child with the same configuration as the parent
func (fb *Firebase) Child(child string) *Firebase {
	c := &Firebase{
		url:          fb.url + "/" + child,
		params:       _url.Values{},
		client:       fb.client,
		stopWatching: make(chan struct{}),
	}

	// making sure to manually copy the map items into a new
	// map to avoid modifying the map reference.
	for k, v := range fb.params {
		c.params[k] = v
	}
	return c
}

// Shallow limits the depth of the data returned when calling Value.
// If the data at the location is a JSON primitive (string, number or boolean),
// its value will be returned. If the data is a JSON object, the values
// for each key will be truncated to true.
//
// Reference https://www.firebase.com/docs/rest/api/#section-param-shallow
func (fb *Firebase) Shallow(v bool) {
	if v {
		fb.params.Set(shallowParam, "true")
	} else {
		fb.params.Del(shallowParam)
	}
}

// IncludePriority determines whether or not to ask Firebase
// for the values priority. By default, the priority is not returned
//
// Reference https://www.firebase.com/docs/rest/api/#section-param-format
func (fb *Firebase) IncludePriority(v bool) {
	if v {
		fb.params.Set(formatParam, formatVal)
	} else {
		fb.params.Del(formatParam)
	}
}

func (fb *Firebase) makeRequest(method string, body []byte) (*http.Request, error) {
	path := fb.url + "/.json"

	if len(fb.params) > 0 {
		path += "?" + fb.params.Encode()
	}
	return http.NewRequest(method, path, bytes.NewReader(body))
}

func (fb *Firebase) doRequest(method string, body []byte) ([]byte, error) {
	req, err := fb.makeRequest(method, body)
	if err != nil {
		return nil, err
	}

	resp, err := fb.client.Do(req)
	switch err := err.(type) {
	default:
		return nil, err
	case nil:
		// carry on

	case *_url.Error:
		// `http.Client.Do` will return a `url.Error` that wraps a `net.Error`
		// when exceeding it's `Transport`'s `ResponseHeadersTimeout` or when
		// `net.Dial` timed out
		if e1, ok := err.Err.(net.Error); ok && e1.Timeout() {
			return nil, ErrTimeout{err}
		}
		return nil, err
	}

	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/200 != 1 {
		return nil, errors.New(string(respBody))
	}
	return respBody, nil
}
