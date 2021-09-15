package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"sort"
	"strconv"
	"time"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/font"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

type Duration struct {
	time.Duration
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		d.Duration = time.Duration(value)
		return nil
	case string:
		var err error
		d.Duration, err = time.ParseDuration(value)
		if err != nil {
			return err
		}
		return nil
	default:
		return errors.New("invalid duration")
	}
}

type Endpoint struct {
	Image string `json:"image"`
	URL   string `json:"url"`
}

type Implementations map[string]Implementation

type Implementation struct {
	Sender   Endpoint `json:"sender"`
	Receiver Endpoint `json:"receiver"`
	Name     string   `json:"name"`
}

type TestCases map[string]TestCase

type TestCase struct {
	Name      string    `json:"name"`
	VideoFile VideoFile `json:"videofile"`

	Phases []tcPhase `json:"phases"`
}

type VideoFile struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

type Config struct {
	Date           time.Time      `json:"date"`
	DetailsLink    string         `json:"details_link"`
	Implementation Implementation `json:"implementation"`
	TestCase       TestCase       `json:"testcase"`
	Timeout        time.Duration  `json:"timeout"`
}

type Metrics struct {
	AverageSSIM          float64 `json:"average_ssim"`
	AveragePSNR          float64 `json:"average_psnr"`
	AverageTargetBitrate float64 `json:"average_cc_target_bitrate"`

	PerFrameSSIM plotter.XYs `json:"per_frame_ssim"`
	PerFramePSNR plotter.XYs `json:"per_frame_psnr"`

	SentRTP  plotter.XYs `json:"sent_rtp"`
	SentRTCP plotter.XYs `json:"sent_rtcp"`

	ReceivedRTP  plotter.XYs `json:"received_rtp"`
	ReceivedRTCP plotter.XYs `json:"received_rtcp"`

	QLOGSenderPacketsSent     plotter.XYs `json:"qlog_sender_packets_sent"`
	QLOGSenderPacketsReceived plotter.XYs `json:"qlog_sender_packets_received"`

	QLOGReceiverPacketsSent     plotter.XYs `json:"qlog_receiver_packets_sent"`
	QLOGReceiverPacketsReceived plotter.XYs `json:"qlog_receiver_packets_received"`

	QLOGCongestionWindow plotter.XYs `json:"qlog_congestion_window"`

	CCTargetBitrate plotter.XYs `json:"cc_target_bitrate"`
}

type Float64ToFloat64 struct {
	Key   float64 `json:"key"`
	Value float64 `json:"value"`
}

type IntToFloat64 struct {
	Key   int     `json:"key"`
	Value float64 `json:"value"`
}

// map: "implementation" -> "testcase" -> Result
type AggregatedResults map[string]map[string]*Result

func (r AggregatedResults) getTableHeaders() []IndexTableHeader {
	result := []IndexTableHeader{}
	dupMap := map[string]bool{}
	for _, t := range r {
		for name, r := range t {
			if _, ok := dupMap[name]; !ok {
				result = append(result, IndexTableHeader{
					Header:  name,
					Tooltip: r.Config.TestCase.Name,
				})
				dupMap[name] = true
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Header < result[j].Header
	})
	result = append([]IndexTableHeader{
		{
			Header:  "Implementation",
			Tooltip: "Implementation used in the experiment",
		},
	}, result...)
	return result
}

func (r AggregatedResults) getTableRows(header []string) []IndexTableRow {
	result := []IndexTableRow{}
	for _, t := range r {
		var impl Implementation
		metrics := make([]*IndexMetric, len(header))
		for i, h := range header {
			if run, ok := t[h]; ok {
				metrics[i] = &IndexMetric{
					Link:    run.Config.DetailsLink,
					Metrics: run.Metrics,
				}
				impl = run.Config.Implementation
			}
		}
		next := IndexTableRow{
			Implementation: impl,
			Metrics:        metrics,
		}
		result = append(result, next)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Implementation.Name < result[j].Implementation.Name
	})

	return result
}

type Result struct {
	Config  Config  `json:"config"`
	Metrics Metrics `json:"metrics"`
}

func writePlot(p *plot.Plot, w, h font.Length) (template.HTML, error) {
	writerTo, err := p.WriterTo(w, h, "svg")
	if err != nil {
		return "", err
	}
	buf := new(bytes.Buffer)
	_, err = writerTo.WriteTo(buf)
	if err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}

func (t *Metrics) plotPerFrameVideoMetric(metric string) (template.HTML, error) {
	p := plot.New()
	p.Add(plotter.NewGrid())
	p.Title.Text = fmt.Sprintf("%s per Frame", metric)
	p.X.Label.Text = "Frames"
	p.Y.Label.Text = metric

	l, err := plotter.NewLine(t.PerFramePSNR)
	if err != nil {
		return "", err
	}
	p.Add(l)

	return writePlot(p, 4*vg.Inch, 2*vg.Inch)
}

func (t *Metrics) plotCCBitrate() (template.HTML, error) {
	p := plot.New()
	p.Add(plotter.NewGrid())
	p.Title.Text = "CC Target Bitrate"
	p.X.Label.Text = "s"
	p.Y.Label.Text = "bit/s"
	p.X.Tick.Marker = secondsTicker{}

	l, err := plotter.NewLine(t.CCTargetBitrate)
	if err != nil {
		return "", err
	}
	p.Add(l)

	return writePlot(p, 4*vg.Inch, 2*vg.Inch)
}

func (t *Metrics) plotMetric(title string, ticker plot.Ticker, data plotter.XYs) (template.HTML, error) {
	p := plot.New()
	p.Add(plotter.NewGrid())
	p.Title.Text = title
	p.X.Label.Text = "s"
	p.Y.Label.Text = "Bytes"
	p.Legend.Top = false
	p.X.Tick.Marker = ticker

	var l *plotter.Line
	var err error

	l, err = plotter.NewLine(data)
	if err != nil {
		return "", err
	}
	p.Add(l)
	p.Legend.Add(title, l)

	_, _, ymax, ymin := plotter.XYRange(data)

	p.Y.Max = ymax
	p.Y.Min = ymin

	return writePlot(p, 4*vg.Inch, 2*vg.Inch)
}

type secondsTicker struct{}

func (secondsTicker) Ticks(min, max float64) []plot.Tick {
	tks := plot.DefaultTicks{}.Ticks(min, max)
	for i, t := range tks {
		if t.Label == "" { // Skip minor ticks, they are fine.
			continue
		}
		l, err := strconv.ParseFloat(t.Label, 64)
		if err != nil {
			panic(err)
		}
		tks[i].Label = fmt.Sprintf("%.2f", l/1000.0)
	}
	return tks

}
