package util

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	log "github.com/sirupsen/logrus"
)

// ParseJSONFile reads a file and parses it as JSON, using the provided object.
func ParseJSONFile(destination interface{}, path string) bool {
	log.WithFields(log.Fields{
		"datatype": fmt.Sprintf("%t", destination),
		"path":     path,
	}).Trace("Parsing JSON file")

	dat, err := ioutil.ReadFile(path)
	if err != nil {
		log.WithError(err).Fatal("Failed to read file")
		return false
	}
	if err := json.Unmarshal(dat, destination); err != nil {
		log.WithError(err).Fatal("Failed to parse file")
		return false
	}

	return true
}
