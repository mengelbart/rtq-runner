package cmd

import (
	"encoding/json"
	"html/template"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gonum.org/v1/plot"
)

var (
	resultsFilename string
	templateDir     string
	outputDir       string
)

func init() {
	buildCmd.Flags().StringVarP(&resultsFilename, "result", "r", "results.json", "filename of the results JSON file")
	buildCmd.Flags().StringVarP(&templateDir, "templates", "t", "templates", "Template directory containing HTML template files")
	buildCmd.Flags().StringVarP(&outputDir, "output", "o", "html", "Output directory for generated HTML files")

	rootCmd.AddCommand(buildCmd)
}

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "build",
	RunE: func(cmd *cobra.Command, args []string) error {
		return buildHTML(resultsFilename, templateDir, outputDir)
	},
}

var templates *template.Template

func buildHTML(inputFilename, templateDirname, outputDirname string) error {
	var result AggregatedResults
	err := parseJSONFile(inputFilename, &result)
	if err != nil {
		return err
	}

	templates = template.Must(template.ParseGlob(filepath.Join(templateDir, "*.html")))

	for i, t := range result {
		for tc, r := range t {
			detailLink := filepath.Join(i, tc)
			detailDir := filepath.Join(outputDirname, detailLink)
			r.Config.DetailsLink = detailLink
			err := buildResultDetailPage(r.Config, &r.Metrics, detailDir)
			if err != nil {
				return err
			}
		}
	}

	return buildHomePage(&result, outputDirname)
}

func buildHomePage(input *AggregatedResults, outDir string) error {
	if _, err := os.Stat(outDir); os.IsNotExist(err) {
		err = os.Mkdir(outDir, os.ModePerm)
		if err != nil {
			return err
		}
	}

	index, err := os.Create(filepath.Join(outDir, "index.html"))
	if err != nil {
		return err
	}
	defer index.Close()

	var htmlInput IndexInput
	htmlInput.TableHeaders = input.getTableHeaders()
	htmlInput.TableRows = input.getTableRows(htmlInput.valueHeaders())

	return templates.ExecuteTemplate(index, "index.html", htmlInput)
}

type IndexInput struct {
	TableHeaders []IndexTableHeader
	TableRows    []IndexTableRow
}

func (i IndexInput) valueHeaders() []string {
	h := make([]string, len(i.TableHeaders))
	for x := 0; x < len(i.TableHeaders); x++ {
		h[x] = i.TableHeaders[x].Header
	}
	return h[1:]
}

type IndexTableHeader struct {
	Header  string
	Tooltip string
}

type IndexTableRow struct {
	Implementation Implementation
	Metrics        []*IndexMetric
}

type IndexMetric struct {
	Link    string
	Metrics Metrics
}

type detailsInput struct {
	ConfigJSON string

	AverageSSIM          float64
	AveragePSNR          float64
	AverageTargetBitrate float64

	SSIMPlotSVG template.HTML
	PSNRPlotSVG template.HTML

	RTPOutPlotSVG  template.HTML
	RTPInPlotSVG   template.HTML
	RTCPOutPlotSVG template.HTML
	RTCPInPlotSVG  template.HTML

	QLOGSenderPacketsSent       template.HTML
	QLOGSenderPacketsReceived   template.HTML
	QLOGReceiverPacketsSent     template.HTML
	QLOGReceiverPacketsReceived template.HTML

	QLOGCongestionWindow template.HTML

	CCTargetBitrate template.HTML
	CCSRTT          template.HTML
}

func buildResultDetailPage(config Config, input *Metrics, outDir string) error {
	err := os.MkdirAll(outDir, os.ModePerm)
	if err != nil {
		return err
	}

	index, err := os.Create(filepath.Join(outDir, "index.html"))
	if err != nil {
		return err
	}
	defer index.Close()

	configJSON, err := json.Marshal(config)
	if err != nil {
		return err
	}

	ssim, err := input.plotPerFrameVideoMetric("SSIM")
	if err != nil {
		return err
	}
	psnr, err := input.plotPerFrameVideoMetric("PSNR")
	if err != nil {
		return err
	}

	rtpOut, err := plotMetric("Sent RTP bytes", plot.DefaultTicks{}, input.SentRTP)
	if err != nil {
		return err
	}
	rtpIn, err := plotMetric("Received RTP bytes", plot.DefaultTicks{}, input.ReceivedRTP)
	if err != nil {
		return err
	}

	var rtcpOut template.HTML
	if len(input.SentRTCP) > 0 {
		rtcpOut, err = plotMetric("Sent RTCP bytes", plot.DefaultTicks{}, input.SentRTCP)
		if err != nil {
			return err
		}
	}

	var rtcpIn template.HTML
	if len(input.ReceivedRTCP) > 0 {
		rtcpIn, err = plotMetric("Received RTCP bytes", plot.DefaultTicks{}, input.ReceivedRTCP)
		if err != nil {
			return err
		}
	}

	var qsps, qspr, qrps, qrpr, qcc template.HTML
	if len(input.QLOGSenderPacketsSent) > 0 {
		qsps, err = plotMetric("QLOG bytes sent", plot.DefaultTicks{}, input.QLOGSenderPacketsSent)
		if err != nil {
			return err
		}
	}
	if len(input.QLOGSenderPacketsReceived) > 0 {
		qspr, err = plotMetric("QLOG bytes received", plot.DefaultTicks{}, input.QLOGSenderPacketsReceived)
		if err != nil {
			return err
		}
	}
	if len(input.QLOGReceiverPacketsSent) > 0 {
		qrps, err = plotMetric("QLOG bytes sent", plot.DefaultTicks{}, input.QLOGReceiverPacketsSent)
		if err != nil {
			return err
		}
	}
	if len(input.QLOGReceiverPacketsReceived) > 0 {
		qrpr, err = plotMetric("QLOG bytes received", plot.DefaultTicks{}, input.QLOGReceiverPacketsReceived)
		if err != nil {
			return err
		}
	}

	if len(input.QLOGCongestionWindow) > 1 {
		qcc, err = plotMetric("QLOG Congestion Window", secondsTicker{}, input.QLOGCongestionWindow)
		if err != nil {
			return err
		}
	}

	var cc template.HTML
	if len(input.CCTargetBitrate) > 0 {
		cc, err = input.plotCCBitrate()
		if err != nil {
			return err
		}
	}
	var ccrtt template.HTML
	if len(input.CCSRTT) > 0 {
		ccrtt, err = input.plotSRTT()
		if err != nil {
			return err
		}
	}

	details := detailsInput{
		ConfigJSON: string(configJSON),

		AverageSSIM:                 input.AverageSSIM,
		AveragePSNR:                 input.AveragePSNR,
		AverageTargetBitrate:        input.AverageTargetBitrate,
		SSIMPlotSVG:                 ssim,
		PSNRPlotSVG:                 psnr,
		RTPOutPlotSVG:               rtpOut,
		RTPInPlotSVG:                rtpIn,
		RTCPOutPlotSVG:              rtcpOut,
		RTCPInPlotSVG:               rtcpIn,
		QLOGSenderPacketsSent:       qsps,
		QLOGSenderPacketsReceived:   qspr,
		QLOGReceiverPacketsSent:     qrps,
		QLOGReceiverPacketsReceived: qrpr,
		QLOGCongestionWindow:        qcc,
		CCTargetBitrate:             cc,
		CCSRTT:                      ccrtt,
	}

	return templates.ExecuteTemplate(index, "detail.html", details)
}
