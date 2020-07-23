package config

import (
	"testing"
)

func TestConfig_GetOptions(t *testing.T) {
	ParseConfig("testconf.yaml", "./config/")
	options := GetOptions()
	if options.LogLevel != "info" {
		t.Error("Config not loaded")
	}
}
