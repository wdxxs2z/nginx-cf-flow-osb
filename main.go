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
	"strconv"
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

	prepareEnvironment(config)

	logger.Debug("enable debug mode", lager.Data{
		"listen": "127.0.0.1:9999",
	})
	go func() {
		log.Println(http.ListenAndServe("localhost:9999", nil))
	}()

	broker := broker.New(config.ServiceConfig, logger)
	broker.Run(":" + port)
}

func prepareEnvironment(config *Config) {
	if os.Getenv("CF_API") == "" {
		if config.CloudFoundryApi != "" {
			os.Setenv("CF_API", config.CloudFoundryApi)
		} else {
			log.Fatal("Error set cloud foundry api,config and env('CF_API') not found the value")
		}
	}
	if os.Getenv("CF_USERNAME") == "" {
		if config.CloudFoundryUsername != "" {
			os.Setenv("CF_USERNAME", config.CloudFoundryUsername)
		} else {
			log.Fatal("Error set cloud foundry username,config and env('CF_USERNAME') not found the value")
		}
	}
	if os.Getenv("CF_PASSWORD") == "" {
		if config.CloudFoundryPassword != "" {
			os.Setenv("CF_PASSWORD", config.CloudFoundryPassword)
		} else {
			log.Fatal("Error set cloud foundry api,config and env('CF_PASSWORD') not found the value")
		}
	}
	if os.Getenv("DATABASE_NAME") == "" {
		if config.ServiceConfig.DatabaseConfig.DbName != "" {
			os.Setenv("DATABASE_NAME", config.ServiceConfig.DatabaseConfig.DbName)
		} else {
			log.Fatal("Error set service broker database name,config and env('DATABASE_NAME') not found the value")
		}
	}
	if os.Getenv("DATABASE_USERNAME") == "" {
		if config.ServiceConfig.DatabaseConfig.Username != "" {
			os.Setenv("DATABASE_USERNAME", config.ServiceConfig.DatabaseConfig.Username)
		} else {
			log.Fatal("Error set service broker database username,config and env('DATABASE_USERNAME') not found the value")
		}
	}
	if os.Getenv("DATABASE_HOST") == "" {
		if config.ServiceConfig.DatabaseConfig.Host != "" {
			os.Setenv("DATABASE_HOST", config.ServiceConfig.DatabaseConfig.Host)
		} else {
			log.Fatal("Error set service broker database host,config and env('DATABASE_HOST') not found the value")
		}
	}
	if os.Getenv("DATABASE_PORT") == "" {
		if config.ServiceConfig.DatabaseConfig.Port != 0 {
			os.Setenv("DATABASE_PORT", strconv.Itoa(config.ServiceConfig.DatabaseConfig.Port))
		} else {
			log.Fatal("Error set service broker database port,config and env('DATABASE_PORT') not found the value")
		}
	}
	if os.Getenv("DATABASE_PASSWORD") == "" {
		if config.ServiceConfig.DatabaseConfig.Password != "" {
			os.Setenv("DATABASE_PASSWORD", config.ServiceConfig.DatabaseConfig.Password)
		} else {
			log.Fatal("Error set service broker database password,config and env('DATABASE_PASSWORD') not found the value")
		}
	}
}