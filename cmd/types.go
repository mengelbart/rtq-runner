package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"sort"
	"strconv"
	"time"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/font"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

const (
	width  = 2.54 * 4 * vg.Centimeter
	height = 3 * 1 * vg.Centimeter
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
	Image  string `json:"image"`
	URL    string `json:"url"`
	Params string `json:"params"`
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

	LinkCapacity plotter.XYs `json:"link_capacity"`

	SentRTP  plotter.XYs `json:"sent_rtp"`
	SentRTCP plotter.XYs `json:"sent_rtcp"`

	ReceivedRTP  plotter.XYs `json:"received_rtp"`
	ReceivedRTCP plotter.XYs `json:"received_rtcp"`

	QLOGSenderPacketsSent     plotter.XYs `json:"qlog_sender_packets_sent"`
	QLOGSenderPacketsReceived plotter.XYs `json:"qlog_sender_packets_received"`

	QLOGReceiverPacketsSent     plotter.XYs `json:"qlog_receiver_packets_sent"`
	QLOGReceiverPacketsReceived plotter.XYs `json:"qlog_receiver_packets_received"`

	QLOGCongestionWindow plotter.XYs `json:"qlog_congestion_window"`

	CCTargetBitrate   plotter.XYs `json:"cc_target_bitrate"`
	CCRateTransmitted plotter.XYs `json:"cc_rate_transmitted"`
	CCSRTT            plotter.XYs `json:"cc_srtt"`
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

func (t *Metrics) plotPerFrameVideoMetric(name string, data plotter.XYs) (*plot.Plot, error) {
	p := plot.New()
	p.Add(plotter.NewGrid())
	p.Title.Text = fmt.Sprintf("%s per Frame", name)
	p.X.Label.Text = "Frames"
	p.Y.Label.Text = name

	l, err := plotter.NewLine(data)
	if err != nil {
		return nil, err
	}
	p.Add(l)

	return p, nil
}

func (t *Metrics) plotCCBitrate() (*plot.Plot, error) {
	p := plot.New()
	p.Add(plotter.NewGrid())
	p.Title.Text = "CC Target Bitrate"
	p.X.Label.Text = "s"
	p.Y.Label.Text = "kbit/s"
	p.X.Tick.Marker = fixedSecondsTicker{seconds: []int{0, 60, 120}}
	p.Legend.TextStyle.Font = font.From(plot.DefaultFont, 6)
	p.Legend.ThumbnailWidth = 0.4 * vg.Centimeter
	p.Legend.YOffs = -0.25 * vg.Centimeter

	if t.CCRateTransmitted != nil {
		rateTransmittedLine, err := plotter.NewLine(t.CCRateTransmitted)
		if err != nil {
			return nil, err
		}
		rateTransmittedLine.Color = color.RGBA{B: 255, A: 255}
		p.Add(rateTransmittedLine)
		p.Legend.Add("Transmitted", rateTransmittedLine)
	}

	targetBitrateLine, err := plotter.NewLine(t.CCTargetBitrate)
	if err != nil {
		return nil, err
	}
	targetBitrateLine.Color = color.RGBA{R: 255, A: 255}
	p.Add(targetBitrateLine)
	p.Legend.Add("Target", targetBitrateLine)

	capacityLine, err := plotter.NewLine(t.LinkCapacity)
	if err != nil {
		return nil, err
	}
	p.Add(capacityLine)
	p.Legend.Add("Link Capacity", capacityLine)

	p.Y.Max = 1400 // TODO: Hack to set height for paper output, remove for later experiments

	return p, nil
}

func (t *Metrics) plotSRTT() (*plot.Plot, error) {
	p := plot.New()
	p.Add(plotter.NewGrid())
	p.Title.Text = "CC RTT"
	p.X.Label.Text = "s"
	p.Y.Label.Text = "s"
	p.X.Tick.Marker = secondsTicker{}

	l1, err := plotter.NewLine(t.CCSRTT)
	if err != nil {
		return nil, err
	}
	p.Add(l1)

	return p, nil
}

func plotMetric(title string, ticker plot.Ticker, data plotter.XYs) (*plot.Plot, error) {
	p := plot.New()
	p.Add(plotter.NewGrid())
	p.Title.Text = title
	p.X.Label.Text = "s"
	p.Y.Label.Text = "Bytes"
	p.Legend.Top = false
	p.Legend.TextStyle.Font = font.From(plot.DefaultFont, 8)
	p.Legend.ThumbnailWidth = 0.4 * vg.Centimeter
	p.X.Tick.Marker = ticker

	var l *plotter.Line
	var err error

	l, err = plotter.NewLine(data)
	if err != nil {
		return nil, err
	}
	p.Add(l)
	//p.Legend.Add(title, l)

	_, _, ymin, ymax := plotter.XYRange(data)

	if ymax <= ymin {
		ymin = 0
	}
	p.Y.Min = ymin
	p.Y.Max = ymax

	return p, nil
}

type fixedSecondsTicker struct {
	seconds []int
}

func (f fixedSecondsTicker) Ticks(min, max float64) []plot.Tick {
	var result []plot.Tick
	for _, s := range f.seconds {
		result = append(result, plot.Tick{
			Value: float64(1000 * s),
			Label: fmt.Sprintf("%v", s),
		})
	}
	return result
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
