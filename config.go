package main

import (
	"github.com/wdxxs2z/nginx-flow-osb/config"
	"errors"
	"os"
	"io/ioutil"
	"fmt"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Username		string		`yaml:"username"`
	Password		string		`yaml:"password"`
	LogLevel		string		`yaml:"log_level"`
	CloudFoundryApi 	string          `yaml:"cf_api_url"`
	CloudFoundryUsername 	string          `yaml:"cf_username"`
	CloudFoundryPassword    string          `yaml:"cf_passwrod"`
	CloudFoundrySpace       string          `yaml:"cf_service_space"`
	ServiceConfig		config.Config   `yaml:"service_config"`
}

func LoadConfig(configFile string) (config *Config, err error) {
	if configFile == "" {
		return config, errors.New("Must provide a config file")
	}

	file, err := os.Open(configFile)
	if err != nil {
		return config, err
	}
	defer file.Close()

	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return config, err
	}

	if err = yaml.Unmarshal(bytes, &config); err != nil {
		return config, err
	}

	if err = config.Validate(); err != nil {
		return config, fmt.Errorf("Validating config contents: %s", err)
	}

	return config, nil
}

func (c Config) Validate() error {

	if c.LogLevel == "" {
		return errors.New("Must provide a non-empty LogLevel")
	}

	if c.Username == "" {
		return errors.New("Must provide a non-empty Username")
	}

	if c.Password == "" {
		return errors.New("Must provide a non-empty Password")
	}

	return nil
}
