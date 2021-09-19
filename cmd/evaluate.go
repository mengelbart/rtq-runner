package cmd

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mengelbart/qlog"
	"github.com/spf13/cobra"
	"gonum.org/v1/plot/plotter"
)

const (
	ssimLogFile            = "ssim.log"
	psnrLogFile            = "psnr.log"
	senderRTPOutLogFile    = "sender_logs/rtp/rtp_out.log"
	receiverRTPInLogFile   = "receiver_logs/rtp/rtp_in.log"
	senderRTCPInLogFile    = "sender_logs/rtp/rtcp_in.log"
	receiverRTCPOutLogFile = "receiver_logs/rtp/rtcp_out.log"
	receiverQLOGFileGLOB   = "receiver_logs/qlog/*.qlog"
	senderQLOGFileGLOB     = "sender_logs/qlog/*.qlog"
	senderLogCCFile        = "sender_logs/cc.log"
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
	result := &Result{}

	err := parseJSONFile("config.json", &result.Config)
	if err != nil {
		return err
	}

	if err = calculateVideoMetrics(
		fmt.Sprintf("input/%v", result.Config.TestCase.VideoFile.Name),
		"output/out.mkv",
	); err != nil {
		log.Printf("failed to calculate video metrics: %v\n", err)
	}

	if _, err := os.Stat(ssimLogFile); err == nil {
		result.Metrics.PerFrameSSIM, err = getXYsFromCSV(ssimLogFile, ' ', ssimValueGetter)
		if err != nil {
			return fmt.Errorf("failed to get ssim metrics table: %w", err)
		}
		result.Metrics.AverageSSIM = math.Round(averageMapValues(result.Metrics.PerFrameSSIM)*100) / 100
	} else if os.IsNotExist(err) {
		fmt.Printf("%v not found: %v\n", ssimLogFile, err)
	} else {
		return fmt.Errorf("failed to stat %v: %w", ssimLogFile, err)
	}

	if _, err := os.Stat(psnrLogFile); err == nil {
		result.Metrics.PerFramePSNR, err = getXYsFromCSV(psnrLogFile, ' ', psnrValueGetter)
		if err != nil {
			return fmt.Errorf("failed to get psnr metrics table: %w", err)
		}
		result.Metrics.AveragePSNR = math.Round(averageMapValues(result.Metrics.PerFramePSNR)*100) / 100
	} else if os.IsNotExist(err) {
		fmt.Printf("%v not found: %v\n", psnrLogFile, err)
	} else {
		return fmt.Errorf("failed to stat %v: %w", psnrLogFile, err)
	}

	if _, err := os.Stat(senderRTPOutLogFile); err == nil {
		g := csvValueGetter{timeColumn: 2, valueColumn: 8}
		sentRTPTable, err := getXYsFromCSV(senderRTPOutLogFile, '\t', g.get)
		if err != nil {
			return fmt.Errorf("failed to get rtp out metrics table: %w", err)
		}
		result.Metrics.SentRTP = binToSeconds(sentRTPTable)
	} else if os.IsNotExist(err) {
		fmt.Printf("%v not found: %v\n", senderRTPOutLogFile, err)
	} else {
		return fmt.Errorf("failed to stat %v: %w", senderRTPOutLogFile, err)
	}

	if _, err := os.Stat(receiverRTPInLogFile); err == nil {
		g := csvValueGetter{timeColumn: 2, valueColumn: 8}
		receivedRTPTable, err := getXYsFromCSV(receiverRTPInLogFile, '\t', g.get)
		if err != nil {
			return fmt.Errorf("failed to get rtp in metrics table: %w", err)
		}
		result.Metrics.ReceivedRTP = binToSeconds(receivedRTPTable)
	} else if os.IsNotExist(err) {
		fmt.Printf("%v not found: %v\n", receiverRTPInLogFile, err)
	} else {
		return fmt.Errorf("failed to stat %v: %w", receiverRTPInLogFile, err)
	}

	if _, err := os.Stat(receiverRTCPOutLogFile); err == nil {
		g := csvValueGetter{timeColumn: 2, valueColumn: 3}
		sentRTCPTable, err := getXYsFromCSV(receiverRTCPOutLogFile, '\t', g.get)
		if err != nil {
			return err
		}
		result.Metrics.SentRTCP = binToSeconds(sentRTCPTable)
	} else if os.IsNotExist(err) {
		fmt.Printf("%v not found: %v\n", receiverRTCPOutLogFile, err)
	} else {
		return fmt.Errorf("failed to stat %v: %w", receiverRTCPOutLogFile, err)
	}

	if _, err := os.Stat(senderRTCPInLogFile); err == nil {
		g := csvValueGetter{timeColumn: 2, valueColumn: 3}
		receivedRTCPTable, err := getXYsFromCSV(senderRTCPInLogFile, '\t', g.get)
		if err != nil {
			return err
		}
		result.Metrics.ReceivedRTCP = binToSeconds(receivedRTCPTable)
	} else if os.IsNotExist(err) {
		fmt.Printf("%v not found: %v\n", senderRTCPInLogFile, err)
	} else {
		return fmt.Errorf("failed to stat %v: %w", senderRTCPInLogFile, err)
	}

	files, err := filepath.Glob(senderQLOGFileGLOB)
	if err != nil {
		return fmt.Errorf("failed to GLOB files: %v, %w", senderQLOGFileGLOB, err)
	}
	if len(files) > 0 {
		if len(files) != 1 {
			return fmt.Errorf("found invalid number of qlog files: %v", len(files))
		}
		q := qlogDataGetter{path: files[0], metric: qlogPacketSentEventName}
		qlogSenderPacketsSent, err := q.get()
		if err != nil {
			return fmt.Errorf("failed to read QLOG File %v: %w", files[0], err)
		}
		result.Metrics.QLOGSenderPacketsSent = binToSeconds(qlogSenderPacketsSent)

		q = qlogDataGetter{path: files[0], metric: qlogPacketReceivedEventName}
		qlogSenderPacketsReceived, err := q.get()
		if err != nil {
			return fmt.Errorf("failed to read QLOG File %v: %w", files[0], err)
		}
		result.Metrics.QLOGSenderPacketsReceived = binToSeconds(qlogSenderPacketsReceived)

		q = qlogDataGetter{path: files[0], metric: qlogMetricsUpdatedEventName}
		qlogCongestionWindow, err := q.get()
		if err != nil {
			return fmt.Errorf("failed to read QLOG File %v: %w", files[0], err)
		}
		result.Metrics.QLOGCongestionWindow = rect(qlogCongestionWindow)
	}

	files, err = filepath.Glob(receiverQLOGFileGLOB)
	if err != nil {
		return fmt.Errorf("failed to GLOB files: %v, %w", receiverQLOGFileGLOB, err)
	}
	if len(files) > 0 {
		if len(files) != 1 {
			return fmt.Errorf("found invalid number of qlog files: %v", len(files))
		}
		q := qlogDataGetter{path: files[0], metric: qlogPacketSentEventName}
		qlogReceiverPacketsSent, err := q.get()
		if err != nil {
			return fmt.Errorf("failed to read QLOG File %v: %w", files[0], err)
		}
		result.Metrics.QLOGReceiverPacketsSent = binToSeconds(qlogReceiverPacketsSent)

		q = qlogDataGetter{path: files[0], metric: qlogPacketReceivedEventName}
		qlogReceiverPacketsReceived, err := q.get()
		if err != nil {
			return fmt.Errorf("failed to read QLOG File %v: %w", files[0], err)
		}
		result.Metrics.QLOGReceiverPacketsReceived = binToSeconds(qlogReceiverPacketsReceived)
	}

	if _, err = os.Stat(senderLogCCFile); err == nil {
		g := csvValueGetter{timeColumn: 0, valueColumn: 1}
		ccTargetBitrateTable, err := getXYsFromCSV("sender_logs/cc.log", ',', g.get)
		if err != nil {
			return err
		}
		result.Metrics.CCTargetBitrate = ccTargetBitrateTable
		result.Metrics.AverageTargetBitrate = math.Round(averageMapValues(ccTargetBitrateTable)*100) / 100

		g = csvValueGetter{timeColumn: 0, valueColumn: 13}
		ccRateTransmitted, err := getXYsFromCSV("sender_logs/cc.log", ',', g.get)
		if err != nil {
			return err
		}
		result.Metrics.CCRateTransmitted = ccRateTransmitted

		g = csvValueGetter{timeColumn: 0, valueColumn: 5}
		ccSRTT, err := getXYsFromCSV("sender_logs/cc.log", ',', g.get)
		if err != nil {
			return err
		}
		result.Metrics.CCSRTT = ccSRTT

	} else if os.IsNotExist(err) {
		fmt.Printf("%v not found: %v\n", senderLogCCFile, err)
	} else {
		return fmt.Errorf("failed to stat %v: %w", senderLogCCFile, err)
	}

	return saveToJSONFile(outFilename, result)
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

func ssimValueGetter(i int, row []string) (plotter.XY, error) {
	vStr := strings.Split(row[ssimValueColumn], ":")[1]
	v, err := strconv.ParseFloat(vStr, 64)
	if err != nil {
		return plotter.XY{}, err
	}
	return plotter.XY{
		X: float64(i),
		Y: v,
	}, nil
}

func psnrValueGetter(i int, row []string) (plotter.XY, error) {
	vStr := strings.Split(row[psnrValueColumn], ":")[1]
	v, err := parseAndBound(vStr, 64)
	if err != nil {
		return plotter.XY{}, err
	}
	return plotter.XY{
		X: float64(i),
		Y: v,
	}, nil
}

type csvValueGetter struct {
	timeColumn  int
	valueColumn int
}

func (g *csvValueGetter) get(i int, row []string) (plotter.XY, error) {
	k, err := strconv.ParseInt(row[g.timeColumn], 10, 64)
	if err != nil {
		return plotter.XY{}, err
	}
	ts := time.Duration(k) * time.Millisecond
	v, err := strconv.ParseFloat(row[g.valueColumn], 64)
	if err != nil {
		return plotter.XY{}, err
	}
	return plotter.XY{
		X: float64(ts.Milliseconds()),
		Y: v,
	}, nil
}

func rect(table plotter.XYs) (result plotter.XYs) {
	if len(table) <= 0 {
		return table
	}
	if len(table) <= 1 {
		x0 := plotter.XY{
			X: 0,
			Y: table[0].Y,
		}
		result = append(result, x0)
		result = append(result, table[0])
		return result
	}
	for i := 0; i < len(table)-1; i++ {
		result = append(result, table[i])
		result = append(result, plotter.XY{
			X: table[i+1].X,
			Y: table[i].Y,
		})
	}
	return result
}

func binToSeconds(table plotter.XYs) plotter.XYs {
	if len(table) <= 0 {
		return table
	}
	bins := int(math.Ceil(float64(table[len(table)-1].X) / 1000.0))
	result := make(plotter.XYs, bins)

	for i := 0; i < bins; i++ {
		result[i].X = float64(i)
	}

	for _, v := range table {
		bin := int(math.Floor(float64(v.X) / 1000.0))
		result[bin].Y += v.Y
	}
	return result
}

type valueGetter func(i int, row []string) (plotter.XY, error)

func getXYsFromCSV(filename string, comma rune, get valueGetter) (plotter.XYs, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	r := csv.NewReader(file)
	r.Comma = comma
	r.TrimLeadingSpace = true

	var xys plotter.XYs
	for i := 0; ; i++ {
		row, err := r.Read()
		if err != nil {
			if err == io.EOF || errors.Is(err, csv.ErrFieldCount) {
				return xys, nil
			}
			return xys, err
		}

		value, err := get(i, row)
		if err != nil {
			log.Printf("WARNING: failed to read value from CSV file '%v', assuming file was cut: %v\n", filename, err)
			return xys, nil
		}
		xys = append(xys, value)
	}
}

func averageMapValues(table plotter.XYs) float64 {
	if len(table) <= 0 {
		return float64(0)
	}
	sum := 0.0
	for _, x := range table {
		sum += x.Y
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

func (q *qlogDataGetter) get() (plotter.XYs, error) {
	qlogFile, err := os.Open(q.path)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(qlogFile)
	scanner.Split(bufio.ScanLines)
	var bs []byte
	for scanner.Scan() {
		b := scanner.Bytes()
		x := map[string]interface{}{}
		if err := json.Unmarshal(b, &x); err == nil {
			bs = append(bs, append(b, []byte("\n")...)...)
		}
	}

	var qlogData qlog.QLOGFileNDJSON
	err = qlogData.UnmarshalNDJSON(bs)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal qlog file: %w", err)
	}

	var table plotter.XYs
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

var qlogGetters = map[string]func(r qlog.EventWrapper) (plotter.XY, error){
	qlogPacketSentEventName:     qlogPacketSentGetter,
	qlogPacketReceivedEventName: qlogPacketReceivedGetter,
	qlogMetricsUpdatedEventName: qlogCongestionWindowGetter,
}

func qlogPacketSentGetter(r qlog.EventWrapper) (plotter.XY, error) {
	return plotter.XY{
		X: r.RelativeTime,
		Y: float64(r.Data.PacketSent.Raw.Length),
	}, nil
}

func qlogPacketReceivedGetter(r qlog.EventWrapper) (plotter.XY, error) {
	return plotter.XY{
		X: r.RelativeTime,
		Y: float64(r.Data.PacketReceived.Raw.Length),
	}, nil
}

func qlogCongestionWindowGetter(r qlog.EventWrapper) (plotter.XY, error) {
	if r.Data.MetricsUpdated.CongestionWindow <= 0 {
		return plotter.XY{}, fmt.Errorf("value not given")
	}
	return plotter.XY{
		X: r.RelativeTime,
		Y: float64(r.Data.MetricsUpdated.CongestionWindow),
	}, nil
}
