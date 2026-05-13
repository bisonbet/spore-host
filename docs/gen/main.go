//go:build ignore

// Command gen generates Markdown command reference docs from Cobra CLI definitions.
// Run via: go run docs/gen/main.go
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	spawnCmd "github.com/spore-host/spore-host/spawn/cmd"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func main() {
	outDir := "docs/tools/reference"
	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Fatal(err)
	}

	tools := []struct {
		name string
		root *cobra.Command
	}{
		{"spawn", spawnCmd.RootCmd()},
	}

	for _, t := range tools {
		dir := filepath.Join(outDir, t.name+"-gen")
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatal(err)
		}

		if err := doc.GenMarkdownTree(t.root, dir); err != nil {
			log.Fatalf("gen %s: %v", t.name, err)
		}

		if err := mergeIntoSinglePage(t.name, dir, filepath.Join(outDir, t.name+".md")); err != nil {
			log.Fatalf("merge %s: %v", t.name, err)
		}

		os.RemoveAll(dir)
		fmt.Printf("Generated %s reference\n", t.name)
	}
}

func mergeIntoSinglePage(tool, srcDir, outFile string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s command reference\n\n", tool))
	sb.WriteString("::: info Auto-generated\n")
	sb.WriteString(fmt.Sprintf("This reference is generated from the `%s` source. Run `make docs` to update.\n", tool))
	sb.WriteString(":::\n\n")

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(srcDir, e.Name()))
		if err != nil {
			return err
		}
		sb.Write(data)
		sb.WriteString("\n---\n\n")
	}

	return os.WriteFile(outFile, []byte(sb.String()), 0644)
}
