package main

import "net/http"
import "strings"

var theHttpClient *http.Client
func httpClient() *http.Client {
	if theHttpClient == nil {
		t := &http.Transport{}
		t.RegisterProtocol("file", http.NewFileTransport(http.Dir("/")))
		theHttpClient = &http.Client{Transport: t}
	}
	return theHttpClient
}

// S3 needs plus signs escaped in URL path, even before query
// string. net/http (correctly) forces the right approach to
// escaping. This results in 404s and 403s. We need plus signs because
// omnibus and semver. We hack around it by getting an URL (as files
// themselves are public anyway), and processing the plus sign in the
// URL string.
func Get(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil { return nil, Err(err) }
	req.URL.Opaque = strings.Replace(req.URL.Path, "+", "%2B", -1)
	resp, err := httpClient().Do(req)
	if err != nil && resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, NewErrf("%v %v", resp.Proto, resp.Status)
	}
	return resp, nil
}
