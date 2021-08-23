package cmd

import "time"

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
	AverageSSIM float64 `json:"average_ssim"`
	AveragePSNR float64 `json:"average_psnr"`

	PerFrameSSIM []IntToFloat64 `json:"per_frame_ssim"`
	PerFramePSNR []IntToFloat64 `json:"per_frame_psnr"`

	SentRTP  []IntToFloat64 `json:"sent_rtp"`
	SentRTCP []IntToFloat64 `json:"sent_rtcp"`

	ReceivedRTP  []IntToFloat64 `json:"received_rtp"`
	ReceivedRTCP []IntToFloat64 `json:"received_rtcp"`
}

type Result struct {
	Config Config `json:"config"`

	TestCases map[string]*TestCase `json:"test_cases"`
}

type IntToFloat64 struct {
	Key   int     `json:"key"`
	Value float64 `json:"value"`
}

type AggregatedResults struct {
	Date time.Time `json:"date"`

	Results []Result `json:"results"`
}
