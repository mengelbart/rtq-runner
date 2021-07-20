package cmd

import (
	"encoding/csv"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var (
	resultsOutputFilename string
)

func init() {
	evalCmd.Flags().StringVarP(&resultsOutputFilename, "output", "o", "result.json", "Results output filename")

	rootCmd.AddCommand(evalCmd)
}

var evalCmd = &cobra.Command{
	Use:   "eval",
	Short: "Evaluate results of a previous test run",
	RunE: func(cmd *cobra.Command, args []string) error {
		return eval(resultsOutputFilename)
	},
}

func eval(outFilename string) error {
	err := calculateVideoMetrics("input/sintel_trailer.mkv", "output/out.mkv")
	if err != nil {
		return err
	}

	ssimTable, err := getVideoMetricTable("ssim.log", ssimValueColumn, strconv.ParseFloat)
	if err != nil {
		return err
	}

	psnrTable, err := getVideoMetricTable("psnr.log", psnrValueColumn, parseAndBound)
	if err != nil {
		return err
	}

	var config Config
	err = parseJSONFile("config.json", &config)
	if err != nil {
		return err
	}

	return saveToJSONFile(outFilename, &Result{
		Config:       config,
		AverageSSIM:  math.Round(averageMapValues(ssimTable)*100) / 100,
		AveragePSNR:  math.Round(averageMapValues(psnrTable)*100) / 100,
		PerFrameSSIM: ssimTable,
		PerFramePSNR: psnrTable,
	})
}

const (
	ssimValueColumn = 4
	psnrValueColumn = 5
)

type floatParser func(string, int) (float64, error)

func parseAndBound(n string, bitSize int) (float64, error) {
	float, err := strconv.ParseFloat(n, bitSize)
	if err != nil {
		return 0, err
	}
	if math.IsInf(float, 0) {
		return 1, nil
	}
	return float / (1 + float), nil
}

func getVideoMetricTable(filename string, valueColumn int, parse floatParser) ([]IntToFloat64, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	r := csv.NewReader(file)
	r.Comma = ' '
	r.TrimLeadingSpace = true

	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	table := make([]IntToFloat64, len(rows))
	for i, r := range rows {
		vStr := strings.Split(r[valueColumn], ":")[1]
		v, err := parse(vStr, 64)
		if err != nil {
			return nil, err
		}
		table[i] = IntToFloat64{
			Key:   i,
			Value: v,
		}
	}

	return table, nil
}

func averageMapValues(table []IntToFloat64) float64 {
	sum := 0.0
	for _, x := range table {
		sum += x.Value
	}
	return sum / float64(len(table))
}

func calculateVideoMetrics(inputFile, outputFile string) error {
	ffmpegLog := "ffmpeg.log"
	ffmpegLogFile, err := os.Create(ffmpegLog)
	if err != nil {
		return err
	}
	ffmpeg := exec.Command(
		"ffmpeg",
		"-i",
		inputFile,
		"-i",
		outputFile,
		"-lavfi",
		"ssim=ssim.log;[0:v][1:v]psnr=psnr.log",
		"-f",
		"null",
		"-",
	)
	ffmpeg.Stdout = ffmpegLogFile
	ffmpeg.Stderr = ffmpegLogFile
	return ffmpeg.Run()
}
