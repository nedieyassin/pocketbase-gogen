package cmd

import (
	"log"
	"os"
	"path/filepath"

	"github.com/nedieyassin/pocketbase-gogen/generator"
	"github.com/pocketbase/pocketbase/core"
	"github.com/spf13/cobra"
)

var (
	directFlag    bool
	packageName   string
	generateUtils bool
	generateHooks bool

	generateCmd = &cobra.Command{
		Use:   "generate [input path] [output path]",
		Short: "Generate Data Proxies from a Template File",
		Long: `The generated code provides type-safe read and write access to pocketbase records.

Arguments:
	The input path goes to a *.go template file that was generated first by the template command.
	
	Use the --direct flag to skip the templating step.
	In this case the input path goes to the PB data directory (usually /pb_data) or a *.json file of the exported PB schema.

	The output path specifies the *.go file name where the generated code will be saved. The package name will be derived from the directory name.
	Use the --package flag to override the package name.`,
		Run: runGenerate,
	}
)

func init() {
	generateCmd.Flags().BoolVarP(&directFlag, "direct", "d", false, "Skip the template and generate directly from the PB schema")
	generateCmd.Flags().StringVarP(&packageName, "package", "p", "", "Override the output directory name with a chosen package name")
	generateCmd.Flags().BoolVarP(&generateUtils, "utils", "u", false, "Additionally generate utils.go next to the output file")
	generateCmd.Flags().BoolVarP(&generateHooks, "hooks", "j", false, "Additionally generate proxy_events.go and proxy_hooks.go next to the output file (auto-enables --utils)")
}

func runGenerate(cmd *cobra.Command, args []string) {
	if len(args) != 2 {
		log.Fatal("Two path arguments required. Use --help for more information.")
	}

	var collections []*core.Collection
	var templateSource []byte
	if directFlag {
		collections = importSchema(args[0])
	} else {
		templateSource = readTemplate(args[0])
	}

	outDir := filepath.Dir(args[1])
	err := os.MkdirAll(outDir, os.ModePerm)
	errCheck(err)

	if packageName == "" {
		packageName = dirNameFromFilePath(args[1])
	}

	if directFlag {
		templateSource, err = generator.Template(collections, args[1], packageName)
		errCheck(err)
	}

	parser, err := generator.NewTemplateParser(templateSource)
	errCheck(err)
	sourceCode, err := generator.Generate(parser, args[1], packageName)
	errCheck(err)

	proxyFile, err := os.Create(args[1])
	errCheck(err)
	defer proxyFile.Close()
	_, err = proxyFile.Write(sourceCode)
	errCheck(err)

	log.Printf("Saved the generated code to %v", args[1])

	if !generateUtils && !generateHooks {
		return
	}

	utilsPath := generatedFilePath(args[1], "utils.go")
	sourceCode, err = generator.GenerateUtils(parser, utilsPath, packageName)
	errCheck(err)

	utilsFile, err := os.Create(utilsPath)
	errCheck(err)
	defer utilsFile.Close()
	_, err = utilsFile.Write(sourceCode)
	errCheck(err)

	log.Printf("Saved the generated utils code to %v", utilsPath)

	if !generateHooks {
		return
	}

	eventsPath := generatedFilePath(args[1], "proxy_events.go")
	sourceCode, err = generator.GenerateProxyEvents(eventsPath, packageName)
	errCheck(err)

	eventsFile, err := os.Create(eventsPath)
	errCheck(err)
	defer eventsFile.Close()
	_, err = eventsFile.Write(sourceCode)
	errCheck(err)

	log.Printf("Saved the generated events code to %v", eventsPath)

	hooksPath := generatedFilePath(args[1], "proxy_hooks.go")
	sourceCode, err = generator.GenerateProxyHooks(parser, hooksPath, packageName)
	errCheck(err)

	hooksFile, err := os.Create(hooksPath)
	errCheck(err)
	defer hooksFile.Close()
	_, err = hooksFile.Write(sourceCode)
	errCheck(err)

	log.Printf("Saved the generated hooks code to %v", hooksPath)
}

func readTemplate(filename string) []byte {
	if filepath.Ext(filename) != ".go" {
		log.Fatal(
			`The input file is not a *.go file.
Use the --direct flag if you want to generate directly from PB schema or use the template command to get a PocketBase go template first.`,
		)
	}
	source, err := os.ReadFile(filename)
	errCheck(err)

	return source
}

func generatedFilePath(proxyPath, fileName string) string {
	dirPath := filepath.Dir(proxyPath)
	return filepath.Join(dirPath, fileName)
}
