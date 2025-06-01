package cmd

import (
	"log"
	"os"
	"path/filepath"

	"github.com/nedieyassin/pocketbase-gogen/generator"
	"github.com/spf13/cobra"
)

var templateCmd = &cobra.Command{
	Use:   "template [input path] [output path]",
	Short: "Generate an Editable Schema as Code",
	Long: `The template is an intermediate between the PB schema and the proxy code.

Arguments:
  The input path goes to the PB data directory (usually /pb_data) or a *.json file of the exported PB schema.

  The template file will be written to the output path. The package name will be derived from the directory name.
  Use the --package flag to override the package name.


What is this template/schema as code for?

The template is a go source file that contains human readable struct definitions for every collection in the PB schema. The file is not for compilation but you should place it inside your go project in a separate package that is never imported. The template is there so it can be edited before using the generate command. This gives control over the naming of the generated proxies. You can also add methods to the template structs and they will be transferred into the generated code. All field assignments and accesses will be replaced by the proxy getters and setters.`,
	Run: runTemplate,
}

func init() {
	templateCmd.Flags().StringVarP(&packageName, "package", "p", "", "Override the output directory name with a chosen package name")
}

func runTemplate(cmd *cobra.Command, args []string) {
	if len(args) != 2 {
		log.Fatal("Two path arguments required. Use --help for more information.")
	}

	collections := importSchema(args[0])

	outDir := filepath.Dir(args[1])
	err := os.MkdirAll(outDir, os.ModePerm)
	errCheck(err)

	if packageName == "" {
		packageName = dirNameFromFilePath(args[1])
	}

	sourceCode, err := generator.Template(collections, args[1], packageName)
	errCheck(err)

	out, err := os.Create(args[1])
	errCheck(err)
	defer out.Close()
	_, err = out.Write(sourceCode)
	errCheck(err)

	log.Printf("Saved the template to %v", args[1])
}
