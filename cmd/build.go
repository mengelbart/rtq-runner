package cmd

import (
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
	buildCmd.Flags().StringVarP(&templateDir, "templates", "t", "web", "Template directory containing HTML template files")
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

func buildHTML(inputFilename, templateDirname, outputDirname string) error {
	var result AggregatedResults
	err := parseJSONFile(inputFilename, &result)
	if err != nil {
		return err
	}
	return buildHomePage(&result, templateDirname, outputDirname)
}

func buildHomePage(input *AggregatedResults, templateDir, outDir string) error {
	templates, err := template.ParseFiles(filepath.Join(templateDir, "index.html"))
	if err != nil {
		return err
	}

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

	return templates.Execute(index, input)
}
