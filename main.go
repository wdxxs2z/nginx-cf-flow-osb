package main

import (
	"os"
	"log"
	"flag"
	"strings"
	"net/http"
	_"net/http/pprof"

	"code.cloudfoundry.org/lager"

	"github.com/wdxxs2z/nginx-flow-osb/broker"
)

var (
	configpath 	string
	port           	string

	logLevels = map[string]lager.LogLevel{
		"DEBUG": lager.DEBUG,
		"INFO":  lager.INFO,
		"ERROR": lager.ERROR,
		"FATAL": lager.FATAL,
	}
)

func init() {
	flag.StringVar(&configpath, "config", "", "nginx flow control service broker config path")
	flag.StringVar(&port, "port", "8080", "listen port")
}

func buildLogger(logLevel string) lager.Logger {
	laggerLogLevel, ok := logLevels[strings.ToUpper(logLevel)]
	if !ok {
		log.Fatal("Invalid log level: ", logLevel)
	}

	logger := lager.NewLogger("nginx-flow-control-service-broker")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, laggerLogLevel))

	return logger
}

func main() {
	flag.Parse()
	config, err := LoadConfig(configpath)

	if err != nil {
		log.Fatalf("Error loading config file: %s", err)
	}

	logger := buildLogger(config.LogLevel)

	os.Setenv("USERNAME", config.Username)
	os.Setenv("PASSWORD", config.Password)

	os.Setenv("CF_API", config.CloudFoundryApi)
	os.Setenv("CF_USERNAME", config.CloudFoundryUsername)
	os.Setenv("CF_PASSWORD", config.CloudFoundryPassword)

	logger.Debug("enable debug mode", lager.Data{
		"listen": "127.0.0.1:9999",
	})
	go func() {
		log.Println(http.ListenAndServe("localhost:9999", nil))
	}()

	broker := broker.New(config.ServiceConfig, logger)
	broker.Run(":" + port)
}
