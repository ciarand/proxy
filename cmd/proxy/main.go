package main

import (
	"flag"
	"net/http"
	"os"

	"github.com/ciarand/proxy"

	log "github.com/Sirupsen/logrus"
)

var from, to *string

func init() {
	from = flag.String("from", "localhost:8080", "the address to serve from")
	to = flag.String("to", "", "the fully qualified address to proxy requests to")
}

func main() {
	if err := run(); err != nil {
		proxy.LogIfErr(err)
		os.Exit(1)
	}
}

func run() error {
	flag.Parse()

	proxy, err := proxy.NewProxy(*from, *to)
	if err != nil {
		return err
	}

	http.Handle("/", proxy)

	log.WithFields(log.Fields{
		"from": proxy.From,
		"to":   proxy.To,
	}).Info("beginning to listen")

	return http.ListenAndServe(proxy.To.Host, nil)
}
