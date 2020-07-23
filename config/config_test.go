package config

import (
	"testing"
)

func TestConfig_GetOptions(t *testing.T) {
	ParseConfig("testconf", "./")
	options := GetOptions()
	if options.LogLevel != "info" {
		t.Error("Config not loaded")
	}
}
