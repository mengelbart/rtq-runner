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

	PerFrameSSIM []IntToFloat64 `json:"per_frame_ssim"`
	PerFramePSNR []IntToFloat64 `json:"per_frame_psnr"`

	SentRTP  []IntToFloat64 `json:"sent_rtp"`
	SentRTCP []IntToFloat64 `json:"sent_rtcp"`

	ReceivedRTP  []IntToFloat64 `json:"received_rtp"`
	ReceivedRTCP []IntToFloat64 `json:"received_rtcp"`

	QLOGSenderPacketsSent     []Float64ToFloat64 `json:"qlog_sender_packets_sent"`
	QLOGSenderPacketsReceived []Float64ToFloat64 `json:"qlog_sender_packets_received"`

	QLOGReceiverPacketsSent     []Float64ToFloat64 `json:"qlog_receiver_packets_sent"`
	QLOGReceiverPacketsReceived []Float64ToFloat64 `json:"qlog_receiver_packets_received"`

	QLOGCongestionWindow []Float64ToFloat64 `json:"qlog_congestion_window"`

	CCTargetBitrate []IntToFloat64 `json:"cc_target_bitrate"`
}

func (t *TestCase) getVideoMetric(m string) []IntToFloat64 {
	switch m {
	case "PSNR":
		return t.PerFramePSNR
	case "SSIM":
		return t.PerFrameSSIM
	}
	panic(fmt.Errorf("unknown video metric %s", m))
}

func (t *TestCase) getRTPMetric(m string) []IntToFloat64 {
	switch m {
	case "SentRTP":
		return t.SentRTP
	case "ReceivedRTP":
		return t.ReceivedRTP
	case "SentRTCP":
		return t.SentRTCP
	case "ReceivedRTCP":
		return t.ReceivedRTCP
	}
	panic(fmt.Errorf("unknown rtp metric %s", m))
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

	l, err := plotter.NewLine(getXYs(t.PerFramePSNR))
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

	l, err := plotter.NewLine(getXYs(t.CCTargetBitrate))
	if err != nil {
		return "", err
	}
	p.Add(l)

	return writePlot(p, 4*vg.Inch, 2*vg.Inch)
}

func (t *TestCase) plotQLOGMetric(title string, table []Float64ToFloat64) (template.HTML, error) {
	p := plot.New()
	p.Add(plotter.NewGrid())
	p.Title.Text = title
	p.X.Label.Text = "ms"
	p.Y.Label.Text = "Bytes"
	p.Legend.Top = false

	var l *plotter.Line
	var err error

	data := getXYsFloat64(table)
	l, err = plotter.NewLine(data)
	if err != nil {
		return "", err
	}
	p.Add(l)
	p.Legend.Add(title, l)

	p.Y.Max = data.max
	p.Y.Min = data.min

	return writePlot(p, 4*vg.Inch, 2*vg.Inch)
}

func (t *TestCase) plotRTPMetric(title string, table []IntToFloat64) (template.HTML, error) {
	p := plot.New()
	p.Add(plotter.NewGrid())
	p.Title.Text = title
	p.X.Label.Text = "s"
	p.Y.Label.Text = "Bytes"
	p.Legend.Top = false

	var l *plotter.Line
	var err error

	//l, err := plotter.NewLine(getXYs([]IntToFloat64{
	//	{Key: 0, Value: 1.25e+6},
	//	{Key: table[len(table)-1].Key, Value: 1.25e+6},
	//}))
	//if err != nil {
	//	return "", err
	//}
	////l.FillColor = color.RGBA{R: 255, A: 255}
	//l.Color = color.RGBA{R: 255, A: 255}
	//p.Add(l)
	//p.Legend.Add("Bandwidth", l)

	data := getXYs(table)
	l, err = plotter.NewLine(data)
	if err != nil {
		return "", err
	}
	p.Add(l)
	p.Legend.Add(title, l)

	p.Y.Max = data.max
	p.Y.Min = data.min

	return writePlot(p, 4*vg.Inch, 2*vg.Inch)
}

type maxXYs struct {
	plotter.XYs
	min, max float64
}

// TODO: Refactor XY getters and remove redundant code

func getXYs(table []IntToFloat64) maxXYs {
	max, min := float64(0), float64(0)
	pts := make(plotter.XYs, len(table))
	for i, r := range table {
		pts[i].X = float64(r.Key)
		pts[i].Y = r.Value
		if r.Value > max {
			max = r.Value
		}
		if r.Value < min {
			min = r.Value
		}
	}
	return maxXYs{
		XYs: pts,
		max: max,
		min: min,
	}
}

func getXYsFloat64(table []Float64ToFloat64) maxXYs {
	max, min := float64(0), float64(0)
	pts := make(plotter.XYs, len(table))
	for i, r := range table {
		pts[i].X = r.Key
		pts[i].Y = r.Value
		if r.Value > max {
			max = r.Value
		}
		if r.Value < min {
			min = r.Value
		}
	}
	return maxXYs{
		XYs: pts,
		max: max,
		min: min,
	}
}
