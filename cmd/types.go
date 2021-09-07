package cmd

import (
	"bytes"
	"fmt"
	"html/template"
	"time"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/font"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

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

type Config struct {
	Date           time.Time      `json:"date"`
	DetailsLink    string         `json:"details_link"`
	Implementation Implementation `json:"implementation"`
	VideoFile      string         `json:"video_file"`
	Timeout        time.Duration  `json:"timeout"`
}

type TestCase struct {
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

type AggregatedResults struct {
	Date time.Time `json:"date"`

	Results []Result `json:"results"`
}

type Result struct {
	Config Config `json:"config"`

	TestCases map[string]*TestCase `json:"test_cases"`
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

func (t *TestCase) plotPerFrameVideoMetric(metric string) (template.HTML, error) {
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

func (t *TestCase) plotCCBitrate() (template.HTML, error) {
	p := plot.New()
	p.Add(plotter.NewGrid())
	p.Title.Text = "CC Target Bitrate"
	p.X.Label.Text = "ms"
	p.Y.Label.Text = "bit/s"

	l, err := plotter.NewLine(t.CCTargetBitrate)
	if err != nil {
		return "", err
	}
	p.Add(l)

	return writePlot(p, 4*vg.Inch, 2*vg.Inch)
}

func (t *TestCase) plotMetric(title string, data plotter.XYs) (template.HTML, error) {
	p := plot.New()
	p.Add(plotter.NewGrid())
	p.Title.Text = title
	p.X.Label.Text = "s"
	p.Y.Label.Text = "Bytes"
	p.Legend.Top = false

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

func (t *TestCase) plotMetric1(title string, data plotter.XYs) (template.HTML, error) {
	p := plot.New()
	p.Add(plotter.NewGrid())
	p.Title.Text = title
	p.X.Label.Text = "s"
	p.Y.Label.Text = "Bytes"
	p.Legend.Top = false

	p.X.Tick.Marker = secondsTicker{}

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
		tks[i].Label = t.Label[:2]
	}
	return tks

}
