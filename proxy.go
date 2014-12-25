package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
)

// LogIfErr takes an error and, if the error is non-nil, logs it to stdout.
func LogIfErr(err error) {
	if err != nil {
		log.WithField("error", err).Error("an error occurred")
	}
}

// Proxy is a wrapper around the httputil.ReverseProxy that does some extra
// logging of helpful information.
type Proxy struct {
	From     *url.URL
	To       *url.URL
	embedded *httputil.ReverseProxy
}

// NewProxy takes two strings, one that determines what to rewrite redirect
// hosts to (i.e. localhost:8080) and one that determines the location of the
// server we're proxying for.
func NewProxy(from, to string) (*Proxy, error) {
	toURL, err := url.Parse(to)
	if err != nil {
		return nil, err
	}

	fromURL, err := url.Parse(from)
	if err != nil {
		return nil, err
	}

	proxy := &Proxy{
		embedded: httputil.NewSingleHostReverseProxy(fromURL),
		From:     fromURL,
		To:       toURL,
	}

	proxy.embedded.Director = proxy.director

	return proxy, nil
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.WithFields(log.Fields{
		"method":         r.Method,
		"uri":            r.RequestURI,
		"client_address": r.RemoteAddr,
	}).Info("request started")

	rec := httptest.NewRecorder()

	start := time.Now()
	p.embedded.ServeHTTP(rec, r)
	elapsed := time.Since(start)

	// if it's a redirect
	if rec.Code >= 300 && rec.Code < 400 {
		if err := p.rewriteRedirect(rec.Header()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			LogIfErr(err)
		}
	}

	copyHeader(w.Header(), rec.Header())
	w.WriteHeader(rec.Code)
	_, err := io.Copy(w, rec.Body)
	LogIfErr(err)

	log.WithFields(log.Fields{
		"elapsed": elapsed,
		"status":  rec.Code,
	}).Info("request complete")
}

func (p *Proxy) rewriteRedirect(h http.Header) error {
	loc := h.Get("Location")

	u, err := url.Parse(loc)
	if err != nil {
		return err
	}

	if !u.IsAbs() {
		return nil
	}

	u.Host = p.To.Host

	log.WithFields(log.Fields{
		"new": u.RequestURI(),
		"old": loc,
	}).Info("rewriting absolute redirect to relative")

	h.Set("Location", u.RequestURI())

	return nil
}

func (p *Proxy) director(req *http.Request) {
	target := p.From
	targetQuery := target.RawQuery

	req.URL.Scheme = target.Scheme
	req.URL.Host = target.Host
	req.Host = target.Host
	req.URL.Path = singleJoiningSlash(target.Path, req.URL.Path)

	if targetQuery == "" || req.URL.RawQuery == "" {
		req.URL.RawQuery = targetQuery + req.URL.RawQuery
	} else {
		req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
	}
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
