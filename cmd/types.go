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

type TestCase struct {
	Scenario string `json:"scenario"`

	AverageSSIM float64 `json:"average_ssim"`
	AveragePSNR float64 `json:"average_psnr"`
}

type Result struct {
	Config Config `json:"config"`

	Tests []TestCase `json:"tests"`

	AverageSSIM float64 `json:"average_ssim"`
	AveragePSNR float64 `json:"average_psnr"`
}

type AggregatedResults struct {
	Date time.Time `json:"date"`

	Results []Result `json:"results"`
}
