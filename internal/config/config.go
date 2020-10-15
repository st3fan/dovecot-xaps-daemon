package config

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

var conf Config

type (
	Config struct {
		loaded        bool
		LogLevel      string
		DatabaseFile  string
		SocketPath    string
		CheckInterval uint
		Delay         uint
		KeyFile       string
		CertFile      string
	}
)

func ParseConfig(configName, configPath string) {
	viper.SetConfigType("yaml")
	viper.SetConfigName("xapsd")
	viper.SetConfigName(configName)
	viper.AddConfigPath("/etc/xapsd/")
	viper.AddConfigPath(configPath)
	
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatal(err)
	}
	err = viper.Unmarshal(&conf)
	if err != nil {
		log.Fatal(err)
	}
	conf.loaded = true
}

func GetOptions() Config {
	if !conf.loaded {
		ParseConfig("", "")
	}
	return conf
}
