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

	avgSSIM, err := getAverageVideoMetric("ssim.log", ssimValueColumn, strconv.ParseFloat)
	if err != nil {
		return err
	}

	avgPSNR, err := getAverageVideoMetric("psnr.log", psnrValueColumn, parseAndBound)
	if err != nil {
		return err
	}

	var config Config
	err = parseJSONFile("config.json", &config)
	if err != nil {
		return err
	}

	return saveToJSONFile(outFilename, &Result{
		Config:      config,
		AverageSSIM: math.Round(avgSSIM*100) / 100,
		AveragePSNR: math.Round(avgPSNR*100) / 100,
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

func getAverageVideoMetric(filename string, valueColumn int, parse floatParser) (float64, error) {
	file, err := os.Open(filename)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	r := csv.NewReader(file)
	r.Comma = ' '
	r.TrimLeadingSpace = true

	rows, err := r.ReadAll()
	if err != nil {
		return 0, err
	}
	vals := make([]float64, len(rows))
	for i, r := range rows {
		vStr := strings.Split(r[valueColumn], ":")[1]
		v, err := parse(vStr, 64)
		if err != nil {
			return 0, err
		}
		vals[i] = v
	}

	return average(vals), nil
}

func average(xs []float64) float64 {
	sum := 0.0
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
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
