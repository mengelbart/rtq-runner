package cmd

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
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
	for i, r := range result.Results {
		for k, v := range r.TestCases {
			detailLink := filepath.Join(fmt.Sprintf("%v", result.Date.Unix()), r.Config.Implementation.Name, k)
			detailDir := filepath.Join(outputDirname, detailLink)
			result.Results[i].Config.DetailsLink = detailLink
			err := buildResultDetailPage(v, detailDir)
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

	return templates.ExecuteTemplate(index, "index.html", input)
}

type detailsInput struct {
	AverageSSIM float64
	AveragePSNR float64

	SSIMPlotSVG template.HTML
	PSNRPlotSVG template.HTML

	RTPOutPlotSVG  template.HTML
	RTPInPlotSVG   template.HTML
	RTCPOutPlotSVG template.HTML
	RTCPInPlotSVG  template.HTML

	CC template.HTML
}

func buildResultDetailPage(input *TestCase, outDir string) error {
	err := os.MkdirAll(outDir, os.ModePerm)
	if err != nil {
		return err
	}

	index, err := os.Create(filepath.Join(outDir, "index.html"))
	if err != nil {
		return err
	}
	defer index.Close()

	ssim, err := input.plotPerFrameVideoMetric("SSIM")
	if err != nil {
		return err
	}
	psnr, err := input.plotPerFrameVideoMetric("PSNR")
	if err != nil {
		return err
	}

	rtpOut, err := input.plotRTPMetric("Sent RTP bytes", input.SentRTP)
	if err != nil {
		return err
	}
	rtpIn, err := input.plotRTPMetric("Received RTP bytes", input.ReceivedRTP)
	if err != nil {
		return err
	}

	var rtcpOut template.HTML
	if len(input.SentRTCP) > 0 {
		rtcpOut, err = input.plotRTPMetric("Sent RTCP bytes", input.SentRTCP)
		if err != nil {
			return err
		}
	}

	var rtcpIn template.HTML
	if len(input.ReceivedRTCP) > 0 {
		rtcpIn, err = input.plotRTPMetric("Received RTCP bytes", input.ReceivedRTCP)
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

	details := detailsInput{
		AverageSSIM:    input.AverageSSIM,
		AveragePSNR:    input.AveragePSNR,
		SSIMPlotSVG:    ssim,
		PSNRPlotSVG:    psnr,
		RTPOutPlotSVG:  rtpOut,
		RTPInPlotSVG:   rtpIn,
		RTCPOutPlotSVG: rtcpOut,
		RTCPInPlotSVG:  rtcpIn,
		CC:             cc,
	}

	return templates.ExecuteTemplate(index, "detail.html", details)
}
