package cmd

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mengelbart/qlog"
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

	g := csvValueGetter{timeColumn: 2, valueColumn: 8}
	sentRTPTable, err := getMetricTable("sender_logs/rtp/rtp_out.log", '\t', g.get)
	if err != nil {
		return fmt.Errorf("failed to get rtp out metrics table: %w", err)
	}
	g = csvValueGetter{timeColumn: 2, valueColumn: 3}
	receivedRTCPTable, err := getMetricTable("sender_logs/rtp/rtcp_in.log", '\t', g.get)
	if err != nil {
		return err
	}
	g = csvValueGetter{timeColumn: 2, valueColumn: 8}
	receivedRTPTable, err := getMetricTable("receiver_logs/rtp/rtp_in.log", '\t', g.get)
	if err != nil {
		return fmt.Errorf("failed to get rtp in metrics table: %w", err)
	}
	g = csvValueGetter{timeColumn: 2, valueColumn: 3}
	sentRTCPTable, err := getMetricTable("receiver_logs/rtp/rtcp_out.log", '\t', g.get)
	if err != nil {
		return err
	}

	var qlogSenderPacketsSent []Float64ToFloat64
	var qlogSenderPacketsReceived []Float64ToFloat64
	var qlogCongestionWindow []Float64ToFloat64
	files, err := filepath.Glob("sender_logs/qlog/*.qlog")
	if err != nil {
		return err
	}
	if len(files) > 0 {
		if len(files) != 1 {
			return fmt.Errorf("found invalid number of qlog files: %v", len(files))
		}
		q := qlogDataGetter{path: files[0], metric: qlogPacketSentEventName}
		qlogSenderPacketsSent, err = q.get()
		if err != nil {
			return fmt.Errorf("failed to read QLOG File %v: %w", files[0], err)
		}

		q = qlogDataGetter{path: files[0], metric: qlogPacketReceivedEventName}
		qlogSenderPacketsReceived, err = q.get()
		if err != nil {
			return fmt.Errorf("failed to read QLOG File %v: %w", files[0], err)
		}

		q = qlogDataGetter{path: files[0], metric: qlogMetricsUpdatedEventName}
		qlogCongestionWindow, err = q.get()
		if err != nil {
			return fmt.Errorf("failed to read QLOG File %v: %w", files[0], err)
		}
	}

	var qlogReceiverPacketsSent []Float64ToFloat64
	var qlogReceiverPacketsReceived []Float64ToFloat64
	files, err = filepath.Glob("receiver_logs/qlog/*.qlog")
	if err != nil {
		return err
	}
	if len(files) > 0 {
		if len(files) != 1 {
			return fmt.Errorf("found invalid number of qlog files: %v", len(files))
		}
		q := qlogDataGetter{path: files[0], metric: qlogPacketSentEventName}
		qlogReceiverPacketsSent, err = q.get()
		if err != nil {
			return fmt.Errorf("failed to read QLOG File %v: %w", files[0], err)
		}

		q = qlogDataGetter{path: files[0], metric: qlogPacketReceivedEventName}
		qlogReceiverPacketsReceived, err = q.get()
		if err != nil {
			return fmt.Errorf("failed to read QLOG File %v: %w", files[0], err)
		}
	}

	var ccTargetBitrateTable []IntToFloat64
	if _, err = os.Stat("sender_logs/cc.log"); err == nil {
		g = csvValueGetter{timeColumn: 0, valueColumn: 1}
		ccTargetBitrateTable, err = getMetricTable("sender_logs/cc.log", ',', g.get)
		if err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	var config Config
	err = parseJSONFile("config.json", &config)
	if err != nil {
		return err
	}

	return saveToJSONFile(outFilename, &Result{
		Config: config,
		TestCases: map[string]*TestCase{
			"simple-p2p": {
				AverageSSIM:          math.Round(averageMapValues(ssimTable)*100) / 100,
				AveragePSNR:          math.Round(averageMapValues(psnrTable)*100) / 100,
				AverageTargetBitrate: math.Round(averageMapValues(ccTargetBitrateTable)*100) / 100,

				PerFrameSSIM: ssimTable,
				PerFramePSNR: psnrTable,

				SentRTP:      binToSeconds(sentRTPTable),
				ReceivedRTCP: binToSeconds(receivedRTCPTable),

				ReceivedRTP: binToSeconds(receivedRTPTable),
				SentRTCP:    binToSeconds(sentRTCPTable),

				QLOGSenderPacketsSent:     binToSecondsFloat64(qlogSenderPacketsSent),
				QLOGSenderPacketsReceived: binToSecondsFloat64(qlogSenderPacketsReceived),

				QLOGReceiverPacketsSent:     binToSecondsFloat64(qlogReceiverPacketsSent),
				QLOGReceiverPacketsReceived: binToSecondsFloat64(qlogReceiverPacketsReceived),

				QLOGCongestionWindow: rect(qlogCongestionWindow),

				CCTargetBitrate: ccTargetBitrateTable,
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

type csvValueGetter struct {
	timeColumn  int
	valueColumn int
}

func (g *csvValueGetter) get(i int, row []string) IntToFloat64 {
	k, err := strconv.ParseInt(row[g.timeColumn], 10, 64)
	if err != nil {
		panic(err)
	}
	ts := time.Duration(k) * time.Millisecond
	v, err := strconv.ParseFloat(row[g.valueColumn], 64)
	if err != nil {
		panic(err)
	}
	return IntToFloat64{
		Key:   int(ts.Milliseconds()),
		Value: v,
	}
}

func binToSecondsFloat64(table []Float64ToFloat64) []Float64ToFloat64 {
	if len(table) <= 0 {
		return table
	}
	bins := int(math.Ceil(float64(table[len(table)-1].Key) / 1000.0))
	result := make([]Float64ToFloat64, bins)
	for _, v := range table {
		b := math.Floor(float64(v.Key) / 1000.0)
		bin := int(b)
		result[bin].Key = b
		result[bin].Value += v.Value
	}
	return result

}

func rect(table []Float64ToFloat64) []Float64ToFloat64 {
	result := make([]Float64ToFloat64, 2*len(table)-1)
	for i := 0; i < len(table)-1; i++ {
		result = append(result, table[i])
		result = append(result, Float64ToFloat64{
			Key:   table[i+1].Key,
			Value: table[i].Value,
		})
	}
	return result
}

func binToSeconds(table []IntToFloat64) []IntToFloat64 {
	if len(table) <= 0 {
		return table
	}
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

	var table []IntToFloat64
	for i := 0; ; i++ {
		row, err := r.Read()
		if err != nil {
			if err == io.EOF || errors.Is(err, csv.ErrFieldCount) { // Ignore parse ErrFieldCount errors, as logs might be cut
				return table, nil
			}
			return table, err
		}
		table = append(table, valueGetter(i, row))
	}
}

func averageMapValues(table []IntToFloat64) float64 {
	if len(table) <= 0 {
		return float64(0)
	}
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

type qlogDataGetter struct {
	path   string
	metric string
}

func (q *qlogDataGetter) get() ([]Float64ToFloat64, error) {
	qlogFile, err := os.Open(q.path)
	if err != nil {
		return nil, err
	}
	bs, err := ioutil.ReadAll(qlogFile)
	if err != nil {
		return nil, err
	}
	var qlogData qlog.QLOGFileNDJSON
	err = qlogData.UnmarshalNDJSON(bs)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal qlog file: %w", err)
	}

	var table []Float64ToFloat64
	for _, r := range qlogData.Trace.Events.Events {
		if r.Name == q.metric {
			v, err := qlogGetters[q.metric](r)
			if err == nil {
				table = append(table, v)
			}
		}
	}
	return table, nil
}

const (
	qlogPacketSentEventName     = "transport:packet_sent"
	qlogPacketReceivedEventName = "transport:packet_received"
	qlogMetricsUpdatedEventName = "recovery:metrics_updated"
)

var qlogGetters = map[string]func(r qlog.EventWrapper) (Float64ToFloat64, error){
	qlogPacketSentEventName:     qlogPacketSentGetter,
	qlogPacketReceivedEventName: qlogPacketReceivedGetter,
	qlogMetricsUpdatedEventName: qlogCongestionWindowGetter,
}

func qlogPacketSentGetter(r qlog.EventWrapper) (Float64ToFloat64, error) {
	return Float64ToFloat64{
		Key:   r.RelativeTime,
		Value: float64(r.Data.PacketSent.Raw.Length),
	}, nil
}

func qlogPacketReceivedGetter(r qlog.EventWrapper) (Float64ToFloat64, error) {
	return Float64ToFloat64{
		Key:   r.RelativeTime,
		Value: float64(r.Data.PacketReceived.Raw.Length),
	}, nil
}

func qlogCongestionWindowGetter(r qlog.EventWrapper) (Float64ToFloat64, error) {
	if r.Data.MetricsUpdated.CongestionWindow <= 0 {
		return Float64ToFloat64{}, fmt.Errorf("value not given")
	}
	return Float64ToFloat64{
		Key:   r.RelativeTime,
		Value: float64(r.Data.MetricsUpdated.CongestionWindow),
	}, nil
}
