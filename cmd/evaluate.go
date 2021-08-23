package cmd

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

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
		return fmt.Errorf("failed to calculate video metrics: %w", err)
	}

	ssimTable, err := getMetricTable("ssim.log", ' ', ssimValueGetter)
	if err != nil {
		return fmt.Errorf("failed to get ssim metrics table: %w", err)
	}

	psnrTable, err := getMetricTable("psnr.log", ' ', psnrValueGetter)
	if err != nil {
		return fmt.Errorf("failed to get psnr metrics table: %w", err)
	}

	g := rtpValueGetter{valueColumn: 8}
	sentRTPTable, err := getMetricTable("sender_logs/rtp/rtp_out.log", '\t', g.get)
	if err != nil {
		return fmt.Errorf("failed to get rtp out metrics table: %w", err)
	}
	//g = rtpValueGetter{}
	//receivedRTCPTable, err := getMetricTable("sender_logs/rtp/rtcp_in.log", '\t', g.get)
	//if err != nil {
	//	return err
	//}
	g = rtpValueGetter{valueColumn: 8}
	receivedRTPTable, err := getMetricTable("receiver_logs/rtp/rtp_in.log", '\t', g.get)
	if err != nil {
		return fmt.Errorf("failed to get rtp in metrics table: %w", err)
	}
	//g = rtpValueGetter{}
	//sentRTCPTable, err := getMetricTable("receiver_logs/rtp/rtcp_out.log", '\t', g.get)
	//if err != nil {
	//	return err
	//}

	var config Config
	err = parseJSONFile("config.json", &config)
	if err != nil {
		return err
	}

	return saveToJSONFile(outFilename, &Result{
		Config: config,
		TestCases: map[string]*TestCase{
			"simple-p2p": {
				AverageSSIM:  math.Round(averageMapValues(ssimTable)*100) / 100,
				AveragePSNR:  math.Round(averageMapValues(psnrTable)*100) / 100,
				PerFrameSSIM: ssimTable,
				PerFramePSNR: psnrTable,

				SentRTP: binToSeconds(sentRTPTable),
				//ReceivedRTCP: receivedRTCPTable,

				ReceivedRTP: binToSeconds(receivedRTPTable),
				//SentRTCP:    sentRTCPTable,
			},
		},
	})
}

const (
	ssimValueColumn = 4
	psnrValueColumn = 5
)

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

func ssimValueGetter(i int, row []string) IntToFloat64 {
	vStr := strings.Split(row[ssimValueColumn], ":")[1]
	v, err := strconv.ParseFloat(vStr, 64)
	if err != nil {
		panic(err)
	}
	return IntToFloat64{
		Key:   i,
		Value: v,
	}
}

func psnrValueGetter(i int, row []string) IntToFloat64 {
	vStr := strings.Split(row[psnrValueColumn], ":")[1]
	v, err := parseAndBound(vStr, 64)
	if err != nil {
		panic(err)
	}
	return IntToFloat64{
		Key:   i,
		Value: v,
	}
}

type rtpValueGetter struct {
	startTime   time.Time
	valueColumn int
}

func (g *rtpValueGetter) get(i int, row []string) IntToFloat64 {
	k, err := strconv.ParseInt(row[2], 10, 64)
	if err != nil {
		panic(err)
	}
	ts := time.Unix(0, k)
	if g.startTime.IsZero() {
		g.startTime = ts
	}
	key := ts.Sub(g.startTime)
	v, err := strconv.ParseFloat(row[g.valueColumn], 64)
	if err != nil {
		panic(err)
	}
	x := IntToFloat64{
		Key:   int(key.Milliseconds()),
		Value: v,
	}
	return x
}

func binToSeconds(table []IntToFloat64) []IntToFloat64 {
	bins := int(math.Ceil(float64(table[len(table)-1].Key) / 1000.0))
	result := make([]IntToFloat64, bins)
	for _, v := range table {
		bin := int(math.Floor(float64(v.Key) / 1000.0))
		result[bin].Key = bin
		result[bin].Value += v.Value
	}
	return result
}

func getMetricTable(filename string, comma rune, valueGetter func(i int, row []string) IntToFloat64) ([]IntToFloat64, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	r := csv.NewReader(file)
	r.Comma = comma
	r.TrimLeadingSpace = true

	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	table := make([]IntToFloat64, len(rows))
	for i, r := range rows {
		table[i] = valueGetter(i, r)
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
