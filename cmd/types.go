package cmd

import "time"

type Implementation struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type Config struct {
	Sender    *Implementation `json:"sender"`
	Receiver  *Implementation `json:"receiver"`
	VideoFile string          `json:"videoFile"`
	Timeout   time.Duration   `json:"timeout"`
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
