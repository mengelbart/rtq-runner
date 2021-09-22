package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/font"
)

const (
	ssimPlotFileName = "%v-%v-ssim.svg"
	psnrPlotFileName = "%v%v-psnr.svg"

	rtpOutPlotFileName                      = "%v-%v-rtp-out.svg"
	rtpInPlotFileName                       = "%v-%v-rtp-in.svg"
	rtcpOutPlotFileName                     = "%v-%v-rtcp-out.svg"
	rtcpInPlotFileName                      = "%v-%v-rtcp-in.svg"
	qlogSenderPacketSentsPlotFileName       = "%v-%v-qlog-sender-sent.svg"
	qlogSenderPacketsReceivedPlotFileName   = "%v-%v-qlog-sender-received.svg"
	qlogReceiverPacketsSentPlotFileName     = "%v-%v-qlog-receiver-sent.svg"
	qlogReceiverPacketsReceivedPlotFileName = "%v-%v-qlog-receiver-received.svg"
	qlogCongestionWindowPlotFileName        = "%v-%v-qlog-cc-window.svg"
	ccTargetBitratePlotFileName             = "%v-%v-cc-target-bitrate.svg"
	ccSRTTPlotFileName                      = "%v-%v-cc-srtt.svg"
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
			r.Config.DetailsLink = detailLink
			err := buildResultDetailPage(r.Config, &r.Metrics, outputDirname, detailLink)
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

	SSIMPlotSVG string

	PSNRPlotSVG string

	RTPOutPlotSVG  string
	RTPInPlotSVG   string
	RTCPOutPlotSVG string
	RTCPInPlotSVG  string

	QLOGSenderPacketsSent       string
	QLOGSenderPacketsReceived   string
	QLOGReceiverPacketsSent     string
	QLOGReceiverPacketsReceived string

	QLOGCongestionWindow string

	CCTargetBitrate string
	CCSRTT          string
}

func writeToHTML(p *plot.Plot, w, h font.Length) (template.HTML, error) {
	buf, err := writePlot(p, "svg", w, h)
	if err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}

func writePlot(p *plot.Plot, format string, w, h font.Length) (*bytes.Buffer, error) {
	writerTo, err := p.WriterTo(w, h, format)
	if err != nil {
		return nil, err
	}
	buf := new(bytes.Buffer)
	_, err = writerTo.WriteTo(buf)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func buildResultDetailPage(config Config, input *Metrics, outDir, link string) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return err
	}

	details := detailsInput{
		ConfigJSON: string(configJSON),

		AverageSSIM:          input.AverageSSIM,
		AveragePSNR:          input.AveragePSNR,
		AverageTargetBitrate: input.AverageTargetBitrate,
		SSIMPlotSVG:          fmt.Sprintf(ssimPlotFileName, config.Implementation.Name, config.TestCase.Name),
		PSNRPlotSVG:          fmt.Sprintf(psnrPlotFileName, config.Implementation.Name, config.TestCase.Name),
		RTPOutPlotSVG:        fmt.Sprintf(rtpOutPlotFileName, config.Implementation.Name, config.TestCase.Name),
		RTPInPlotSVG:         fmt.Sprintf(rtpInPlotFileName, config.Implementation.Name, config.TestCase.Name),
	}

	err = os.MkdirAll(filepath.Join(outDir, link), os.ModePerm)
	if err != nil {
		return err
	}

	index, err := os.Create(filepath.Join(outDir, link, "index.html"))
	if err != nil {
		return err
	}
	defer index.Close()

	ssimPlot, err := input.plotPerFrameVideoMetric("SSIM", input.PerFrameSSIM)
	if err != nil {
		return err
	}
	ssimPlot.Save(width, height, filepath.Join(outDir, link, details.SSIMPlotSVG))

	psnrPlot, err := input.plotPerFrameVideoMetric("PSNR", input.PerFramePSNR)
	if err != nil {
		return err
	}
	psnrPlot.Save(width, height, filepath.Join(outDir, link, details.PSNRPlotSVG))

	rtpOutPlot, err := plotMetric("Sent RTP bytes", plot.DefaultTicks{}, input.SentRTP)
	if err != nil {
		return err
	}
	rtpOutPlot.Save(width, height, filepath.Join(outDir, link, details.RTPOutPlotSVG))

	rtpInPlot, err := plotMetric("Received RTP bytes", plot.DefaultTicks{}, input.ReceivedRTP)
	if err != nil {
		return err
	}
	rtpInPlot.Save(width, height, filepath.Join(outDir, link, details.RTPInPlotSVG))

	if len(input.SentRTCP) > 0 {
		rtcpOut, err := plotMetric("Sent RTCP bytes", plot.DefaultTicks{}, input.SentRTCP)
		if err != nil {
			return err
		}
		details.RTCPOutPlotSVG = fmt.Sprintf(rtcpOutPlotFileName, config.Implementation.Name, config.TestCase.Name)
		rtcpOut.Save(width, height, filepath.Join(outDir, link, details.RTCPOutPlotSVG))
	}

	if len(input.ReceivedRTCP) > 0 {
		rtcpInPlot, err := plotMetric("Received RTCP bytes", plot.DefaultTicks{}, input.ReceivedRTCP)
		if err != nil {
			return err
		}
		details.RTCPInPlotSVG = fmt.Sprintf(rtcpInPlotFileName, config.Implementation.Name, config.TestCase.Name)
		rtcpInPlot.Save(width, height, filepath.Join(outDir, link, details.RTCPInPlotSVG))
	}

	if len(input.QLOGSenderPacketsSent) > 0 {
		qspsPlot, err := plotMetric("QLOG bytes sent", plot.DefaultTicks{}, input.QLOGSenderPacketsSent)
		if err != nil {
			return err
		}
		details.QLOGSenderPacketsSent = fmt.Sprintf(qlogSenderPacketSentsPlotFileName, config.Implementation.Name, config.TestCase.Name)
		qspsPlot.Save(width, height, filepath.Join(outDir, link, details.QLOGSenderPacketsSent))
	}
	if len(input.QLOGSenderPacketsReceived) > 0 {
		qsprPlot, err := plotMetric("QLOG bytes received", plot.DefaultTicks{}, input.QLOGSenderPacketsReceived)
		if err != nil {
			return err
		}
		details.QLOGSenderPacketsReceived = fmt.Sprintf(qlogSenderPacketsReceivedPlotFileName, config.Implementation.Name, config.TestCase.Name)
		qsprPlot.Save(width, height, filepath.Join(outDir, link, details.QLOGSenderPacketsReceived))
	}
	if len(input.QLOGReceiverPacketsSent) > 0 {
		qrpsPlot, err := plotMetric("QLOG bytes sent", plot.DefaultTicks{}, input.QLOGReceiverPacketsSent)
		if err != nil {
			return err
		}
		details.QLOGReceiverPacketsSent = fmt.Sprintf(qlogReceiverPacketsSentPlotFileName, config.Implementation.Name, config.TestCase.Name)
		qrpsPlot.Save(width, height, filepath.Join(outDir, link, details.QLOGReceiverPacketsSent))
	}
	if len(input.QLOGReceiverPacketsReceived) > 0 {
		qrprPlot, err := plotMetric("QLOG bytes received", plot.DefaultTicks{}, input.QLOGReceiverPacketsReceived)
		if err != nil {
			return err
		}
		details.QLOGReceiverPacketsReceived = fmt.Sprintf(qlogReceiverPacketsReceivedPlotFileName, config.Implementation.Name, config.TestCase.Name)
		qrprPlot.Save(width, height, filepath.Join(outDir, link, details.QLOGReceiverPacketsReceived))
	}

	if len(input.QLOGCongestionWindow) > 1 {
		qccPlot, err := plotMetric("QLOG Congestion Window", secondsTicker{}, input.QLOGCongestionWindow)
		if err != nil {
			return err
		}
		details.QLOGCongestionWindow = fmt.Sprintf(qlogCongestionWindowPlotFileName, config.Implementation.Name, config.TestCase.Name)
		qccPlot.Save(width, height, filepath.Join(outDir, link, details.QLOGCongestionWindow))
	}

	if len(input.CCTargetBitrate) > 0 {
		maxX := input.CCTargetBitrate[len(input.CCTargetBitrate)-1].X
		linkCapacity := getCapacityFromConfig(config, maxX)
		input.LinkCapacity = rect(linkCapacity)
		ccPlot, err := input.plotCCBitrate()
		if err != nil {
			return err
		}
		details.CCTargetBitrate = fmt.Sprintf(ccTargetBitratePlotFileName, config.Implementation.Name, config.TestCase.Name)
		ccPlot.Save(width, height, filepath.Join(outDir, link, details.CCTargetBitrate))
	}
	if len(input.CCSRTT) > 0 {
		ccrttPlot, err := input.plotSRTT()
		if err != nil {
			return err
		}
		details.CCSRTT = fmt.Sprintf(ccSRTTPlotFileName, config.Implementation.Name, config.TestCase.Name)
		ccrttPlot.Save(width, height, filepath.Join(outDir, link, details.CCSRTT))
	}

	return templates.ExecuteTemplate(index, "detail.html", details)
}
