package gommm_test

import (
	"testing"

	"github.com/wxio/gommm/internal/gommm"
)

func Test_LoadConfig(t *testing.T) {
	config, err := gommm.LoadConfig("test_fixtures/config.json")

	expect(t, err, nil)
	expect(t, config.Port, 5678)
	expect(t, config.ProxyTo, "http://localhost:3000")
}

func Test_LoadConfig_WithNonExistantFile(t *testing.T) {
	_, err := gommm.LoadConfig("im/not/here.json")

	refute(t, err, nil)
	expect(t, err.Error(), "Unable to read configuration file im/not/here.json")
}

func Test_LoadConfig_WithMalformedFile(t *testing.T) {
	_, err := gommm.LoadConfig("test_fixtures/bad_config.json")

	refute(t, err, nil)
	expect(t, err.Error(), "Unable to parse configuration file test_fixtures/bad_config.json")
}
