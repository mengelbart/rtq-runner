package cmd

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"

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

	ssimSVG, err := plotTimeSeriesSVG(input.PerFrameSSIM, "SSIM", "Frames", "SSIM")
	if err != nil {
		return err
	}
	psnrSVG, err := plotTimeSeriesSVG(input.PerFramePSNR, "PSNR", "Frames", "PSNR")
	if err != nil {
		return err
	}
	sentRTP, err := plotTimeSeriesSVG(input.SentRTP, "RTP Out", "s", "Bytes")
	if err != nil {
		return err
	}
	receivedRTP, err := plotTimeSeriesSVG(input.ReceivedRTP, "RTP In", "s", "Bytes")
	if err != nil {
		return err
	}
	//sentRTCP, err := plotTimeSeriesSVG(input.SentRTCP, "RTCP Out", "s", "Bytes")
	//if err != nil {
	//	return err
	//}
	//receivedRTCP, err := plotTimeSeriesSVG(input.ReceivedRTCP, "RTCP In", "s", "Bytes")
	//if err != nil {
	//	return err
	//}

	details := detailsInput{
		AverageSSIM:   input.AverageSSIM,
		AveragePSNR:   input.AveragePSNR,
		SSIMPlotSVG:   template.HTML(ssimSVG),
		PSNRPlotSVG:   template.HTML(psnrSVG),
		RTPOutPlotSVG: template.HTML(sentRTP),
		RTPInPlotSVG:  template.HTML(receivedRTP),
		//RTCPOutPlotSVG: template.HTML(sentRTCP),
		//RTCPInPlotSVG:  template.HTML(receivedRTCP),
	}

	return templates.ExecuteTemplate(index, "detail.html", details)
}

func plotTimeSeriesSVG(data []IntToFloat64, title, xLabel, yLabel string) (string, error) {
	p := plot.New()
	p.Title.Text = title
	p.X.Label.Text = xLabel
	p.Y.Label.Text = yLabel

	s, err := plotter.NewLine(getPoints(data))
	if err != nil {
		return "", err
	}

	p.Add(s)

	writerTo, err := p.WriterTo(4*vg.Inch, 2*vg.Inch, "svg")
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	_, err = writerTo.WriteTo(buf)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func getPoints(table []IntToFloat64) plotter.XYs {
	pts := make(plotter.XYs, len(table))
	for i, r := range table {
		pts[i].X = float64(r.Key)
		pts[i].Y = r.Value
	}
	return pts
}
