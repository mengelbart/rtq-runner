package cmd

import (
	"encoding/json"
	"io/fs"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

var (
	aggregateInputDirname   string
	aggregateOutputFilename string
	aggregationDate         int64
)

func init() {
	aggregateCmd.Flags().StringVarP(&aggregateInputDirname, "input", "i", "results", "Directory containing all results JSON files to aggregate")
	aggregateCmd.Flags().StringVarP(&aggregateOutputFilename, "output", "o", "results.json", "Output filename for aggregated results")
	aggregateCmd.Flags().Int64VarP(&aggregationDate, "date", "d", time.Now().Unix(), "Unix Timestamp in seconds since epoch")
	rootCmd.AddCommand(aggregateCmd)
}

var aggregateCmd = &cobra.Command{
	Use:   "aggregate",
	Short: "aggregate results",
	RunE: func(cmd *cobra.Command, args []string) error {
		parsedDate := time.Unix(aggregationDate, 0)
		return aggregate(aggregateInputDirname, aggregateOutputFilename, parsedDate)
	},
}

func aggregate(inputDirname, outputFilename string, date time.Time) error {
	aggregated := AggregatedResults{
		Date: date,
	}

	err := filepath.Walk(inputDirname, func(path string, info fs.FileInfo, err error) error {
		if !info.IsDir() {
			var result Result
			err := parseJSONFile(path, &result)
			if err != nil {
				return err
			}
			aggregated.Results = append(aggregated.Results, result)
		}
		return nil
	})
	if err != nil {
		return err
	}

	data, err := json.Marshal(aggregated)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(outputFilename, data, 0644)
}
