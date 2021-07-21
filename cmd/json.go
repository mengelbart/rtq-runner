package cmd

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

func parseJSONFile(filename string, result interface{}) error {
	jsonFile, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer jsonFile.Close()

	data, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, result)
}

func saveToJSONFile(filename string, input interface{}) error {
	data, err := json.Marshal(input)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, data, 0644)
}
