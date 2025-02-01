package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/pocketbase/pocketbase/core"
	"github.com/snonky/pocketbase-gogen/generator"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "pocketbase-gogen",
	Short: "Code Generator for PocketBase",
	Long: `Creates type safe proxies for PocketBase
	
This tool is for go developers who use PocketBase as their backend framework and would like type safe access to their data.
It takes in a PocketBase schema and generates a proxy struct for each collection with getters and setters for all fields.

Run this command from inside your PocketBase project so it has access to the same packages as the rest of your source code, most importantly the PocketBase package itself.

Start by invoking the template command, inspect your template and continue from there with the generate command.`,
	CompletionOptions: cobra.CompletionOptions{
		DisableDefaultCmd: true,
	},
}

func init() {
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})
	rootCmd.AddCommand(templateCmd)
	rootCmd.AddCommand(generateCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func errCheck(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func dirNameFromFilePath(path string) string {
	absPath, err := filepath.Abs(path)
	errCheck(err)
	dirPath := filepath.Dir(absPath)
	dirName := filepath.Base(dirPath)
	return dirName
}

// Returns true when the import path goes to a directory
// with *.db files. False when its a .json file.
func checkSchemaImportPath(path string) bool {
	inFileInfo, err := os.Stat(path)
	errCheck(err)
	isDir := inFileInfo.IsDir()

	if !isDir && filepath.Ext(path) != ".json" {
		log.Fatal("The input path leads to a file but it is not a *.json file.")
	}

	if isDir {
		files, err := os.ReadDir(path)
		errCheck(err)

		dbPresent := false
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			if f.Name() == "data.db" {
				dbPresent = true
				break
			}
		}

		if !dbPresent {
			log.Fatal("The input directory path does not contain the data.db file of PocketBase")
		}
	}

	return isDir
}

func importSchema(dataSourcePath string) []*core.Collection {
	viaPB := checkSchemaImportPath(dataSourcePath)
	var collections []*core.Collection
	if viaPB {
		collections = generator.QuerySchema(dataSourcePath, false)
	} else {
		collections = generator.ParseSchemaJson(dataSourcePath, false)
	}
	return collections
}
