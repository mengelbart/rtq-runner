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
}

type Config struct {
	Date           time.Time      `json:"date"`
	Implementation Implementation `json:"implementation"`
	VideoFile      string         `json:"videoFile"`
	Timeout        time.Duration  `json:"timeout"`
}

type Result struct {
	Config Config `json:"config"`

	AverageSSIM float64 `json:"average_ssim"`
	AveragePSNR float64 `json:"average_psnr"`

	PerFrameSSIM []IntToFloat64 `json:"per_frame_ssim"`
	PerFramePSNR []IntToFloat64 `json:"per_frame_psnr"`
}

type IntToFloat64 struct {
	Key   int     `json:"key"`
	Value float64 `json:"value"`
}

type AggregatedResults struct {
	Date time.Time `json:"date"`

	Results []Result `json:"results"`
}
