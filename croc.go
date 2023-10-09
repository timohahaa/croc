package croc

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"

	"golang.org/x/net/publicsuffix"
)

type Request *http.Request
type Response *http.Response

const (
	GET     = "GET"
	POST    = "POST"
	PUT     = "PUT"
	DELETE  = "DELETE"
	HEAD    = "HEAD"
	PATCH   = "PATCH"
	OPTIONS = "OPTIONS"
)

type BasicAuth struct {
	Username string
	Password string
}

// proxy type to use in http.Transport
type Proxy func(r *http.Request) (*url.URL, error)

func emptyCookieJar() http.CookieJar {
	cookieJarOpts := cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	}
	cookieJar, _ := cookiejar.New(&cookieJarOpts)
	return cookieJar
}

type CrocClient struct {
	url       string
	method    string
	headers   http.Header
	client    *http.Client
	transport *http.Transport
	cookies   []*http.Cookie
	basicAuth BasicAuth
	rawBody   []byte
	proxy     Proxy
	err       error
	//response handling
	lastRequest    Request
	lastResponse   Response
	respStatusCode int
	respHeaders    http.Header
	rawRespBody    []byte
	contentLength  int64
}

func New() *CrocClient {
	cookieJar := emptyCookieJar()
	cc := &CrocClient{
		url:            "",
		method:         "",
		headers:        http.Header{},
		client:         &http.Client{Jar: cookieJar},
		transport:      &http.Transport{DisableKeepAlives: true},
		cookies:        make([]*http.Cookie, 0),
		basicAuth:      BasicAuth{},
		rawBody:        make([]byte, 0),
		proxy:          nil,
		err:            nil,
		lastRequest:    nil,
		lastResponse:   nil,
		respStatusCode: 0,
		respHeaders:    http.Header{},
		rawRespBody:    make([]byte, 0),
		contentLength:  0,
	}
	return cc
}

// function Error() returns THE FIRST error that occured during client calls
func (cc *CrocClient) Error() error {
	return cc.err
}

// function ClearRequestData clears internal fields
// such as url, method, headers, basicAuth and rawBody
// NOTE: ClearRequestData() not clear cookies and proxy
// to clear cookies/proxy use ClearCookies() and ClearProxy()
func (cc *CrocClient) ClearRequestData() *CrocClient {
	cc.url = ""
	cc.method = ""
	cc.headers = http.Header{}
	cc.basicAuth = BasicAuth{}
	cc.rawBody = make([]byte, 0)
	return cc
}

// function ClearCookies() clears all of the cookies to be used with next request
// it does not clear http.Client's cookieJar
func (cc *CrocClient) ClearCookies() *CrocClient {
	cc.cookies = make([]*http.Cookie, 0)
	return cc
}

// function ClearProxy() clears a proxy to be used with next request
func (cc *CrocClient) ClearProxy() *CrocClient {
	cc.proxy = nil
	return cc
}

func (cc *CrocClient) Get(targetUrl string) *CrocClient {
	cc.ClearRequestData()
	cc.method = GET
	cc.url = targetUrl
	cc.err = nil
	return cc
}

func (cc *CrocClient) Post(targetUrl string) *CrocClient {
	cc.ClearRequestData()
	cc.method = POST
	cc.url = targetUrl
	cc.err = nil
	return cc
}

func (cc *CrocClient) Put(targetUrl string) *CrocClient {
	cc.ClearRequestData()
	cc.method = PUT
	cc.url = targetUrl
	cc.err = nil
	return cc
}

func (cc *CrocClient) Delete(targetUrl string) *CrocClient {
	cc.ClearRequestData()
	cc.method = DELETE
	cc.url = targetUrl
	cc.err = nil
	return cc
}

func (cc *CrocClient) Head(targetUrl string) *CrocClient {
	cc.ClearRequestData()
	cc.method = HEAD
	cc.url = targetUrl
	cc.err = nil
	return cc
}

func (cc *CrocClient) Patch(targetUrl string) *CrocClient {
	cc.ClearRequestData()
	cc.method = PATCH
	cc.url = targetUrl
	cc.err = nil
	return cc
}

func (cc *CrocClient) Options(targetUrl string) *CrocClient {
	cc.ClearRequestData()
	cc.method = OPTIONS
	cc.url = targetUrl
	cc.err = nil
	return cc
}

// function AddCookies() adds a cookies to a current request
func (cc *CrocClient) AddCookies(cks []*http.Cookie) *CrocClient {
	cc.cookies = append(cc.cookies, cks...)
	return cc
}

// function SetHeader() sets header fields with single values
// it overwrites any existing header values corresponding to the same key
func (cc *CrocClient) SetHeader(key, value string) *CrocClient {
	cc.headers.Set(key, value)
	return cc
}

// function AppendHeader() sets header fields with multiple values
// it does not overwrite any existing values, but instead appends to them
func (cc *CrocClient) AppendHeader(key, value string) *CrocClient {
	cc.headers.Add(key, value)
	return cc
}

// sets basic auth to use with a request
func (cc *CrocClient) SetBasicAuth(username, password string) *CrocClient {
	cc.basicAuth = BasicAuth{Username: username, Password: password}
	return cc
}

// function Proxy() is used to set a proxy to use with a request
func (cc *CrocClient) Proxy(proxyUrl string) *CrocClient {
	parsedUrl, err := url.Parse(proxyUrl)
	if err != nil {
		cc.err = err
		return cc
	}
	cc.proxy = http.ProxyURL(parsedUrl)
	return cc
}

// function Payload() is used to add marshaled body to the request
func (cc *CrocClient) Payload(data []byte) *CrocClient {
	cc.rawBody = data
	return cc
}

func (cc *CrocClient) makeRequest() (Request, error) {
	if cc.method == "" {
		return nil, errors.New("no method specified")
	}
	if cc.url == "" {
		return nil, errors.New("no url specified")
	}
	// create a request object
	bodyReader := bytes.NewReader(cc.rawBody)
	req, err := http.NewRequest(cc.method, cc.url, bodyReader)
	if err != nil {
		return nil, err
	}
	// populate it with header data
	for key, values := range cc.headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	// add basic auth
	emptyAuth := BasicAuth{}
	if cc.basicAuth != emptyAuth {
		req.SetBasicAuth(cc.basicAuth.Username, cc.basicAuth.Password)
	}
	// add cookies
	for _, cookie := range cc.cookies {
		req.AddCookie(cookie)
	}

	return req, nil
}

// function End() ends the call-chain and makes a request
func (cc *CrocClient) End() error {
	if cc.err != nil {
		return cc.err
	}
	req, err := cc.makeRequest()
	if err != nil {
		cc.err = err
		return err
	}
	cc.lastRequest = req
	// set proxy to transport
	cc.transport.Proxy = cc.proxy
	// set transport
	cc.client.Transport = cc.transport
	// now make a request
	resp, err := cc.client.Do(req)
	if err != nil {
		cc.err = err
		return err
	}
	cc.lastResponse = resp
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		cc.err = err
		return err
	}
	cc.rawRespBody = body
	cc.respStatusCode = resp.StatusCode
	cc.respHeaders = resp.Header
	cc.contentLength = resp.ContentLength
	return nil
}

// function Request() returns the last request made (even if it returned an error)
func (cc *CrocClient) Request() Request {
	return cc.lastRequest
}

// function Response() returns the last succesfully recieved response
func (cc *CrocClient) Response() Response {
	return cc.lastResponse
}

// function RespStatus() last responses status code
func (cc *CrocClient) RespStatus() int {
	return cc.respStatusCode
}

// function RespHeaders() returns last responses headers
func (cc *CrocClient) RespHeaders() http.Header {
	return cc.respHeaders
}

// function RespLength() returns last responses content length
func (cc *CrocClient) RespLength() int64 {
	return cc.contentLength
}

// function RawRespBody() returns last responses body as raw bytes
func (cc *CrocClient) RawRespBody() []byte {
	return cc.rawRespBody
}

// function Do() just does a provided request WITH A SET PROXY
// and returns the response object with response-body bytes and an error
// NOTE: Do() does not save the request and response fields and objects, it does the minimal needed job
// so you can call:
//
// client := croc.New()
// client.Proxy("1.2.3.4:1337")
// resp, body, err := client.Do(myPremadeRequest)
func (cc *CrocClient) Do(req Request) (Response, []byte, error) {
	// set proxy to transport
	cc.transport.Proxy = cc.proxy
	// set transport
	cc.client.Transport = cc.transport
	// now make a request
	resp, err := cc.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	// if could not read body, but request was succesfully made - return request, nil body and an error
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, nil, err
	}
	return resp, body, err
}

/*
type CrocClient struct {
	url       string
	method    string
	headers   http.Header
	client    *http.Client
	transport *http.Transport
	cookies   []*http.Cookie
	basicAuth BasicAuth
	rawBody   []byte
	proxy     Proxy
	err       error
	//response handling
	lastRequest    Request
	lastResponse   Response
	respStatusCode int
	respHeaders    http.Header
	rawRespBody    []byte
	contentLength  int64
}
*/
