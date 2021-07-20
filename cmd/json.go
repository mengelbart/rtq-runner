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

func getImageURLFromImplementations(image string) (string, error) {
	var is Implementations
	err := parseJSONFile("implementations.json", &is)
	if err != nil {
		return "", err
	}

	for _, v := range is {
		if v.Receiver.Image == image {
			return v.Receiver.URL, nil
		}
		if v.Sender.Image == image {
			return v.Sender.URL, nil
		}
	}
	return image, nil
}
