package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

// Version represents the current version of the terraform-removed-remover tool
const Version = "0.0.1"

// Stats holds statistics about the processing operation
type Stats struct {
	FilesProcessed       int
	FilesModified        int
	RemovedBlocksRemoved int
	StartTime            time.Time
	EndTime              time.Time
	DryRun               bool
	NormalizeWhitespace  bool
}

func findTerraformFiles(rootDir string) ([]string, error) {
	var files []string

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error accessing path %s: %w", path, err)
		}

		if !info.IsDir() && strings.HasSuffix(path, ".tf") {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

func processFile(filePath string, stats *Stats) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("error reading file %s: %w", filePath, err)
	}

	// Parse with hclsyntax to get block ranges that exclude leading comments
	syntaxFile, diags := hclsyntax.ParseConfig(content, filePath, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return fmt.Errorf("error parsing %s: %s", filePath, diags.Error())
	}

	syntaxBody, ok := syntaxFile.Body.(*hclsyntax.Body)
	if !ok {
		return fmt.Errorf("unexpected body type in %s", filePath)
	}

	// Collect byte ranges of removed blocks (SrcRange excludes leading comments)
	type byteRange struct {
		start, end int
	}
	var removedRanges []byteRange
	for _, block := range syntaxBody.Blocks {
		if block.Type == "removed" {
			r := block.Range()
			removedRanges = append(removedRanges, byteRange{start: r.Start.Byte, end: r.End.Byte})
		}
	}

	removedBlocksCount := len(removedRanges)
	fileModified := removedBlocksCount > 0

	stats.FilesProcessed++

	if !stats.DryRun {
		resultContent := content
		if fileModified {
			// Remove blocks from content in reverse order to preserve byte offsets
			result := make([]byte, len(content))
			copy(result, content)

			for i := len(removedRanges) - 1; i >= 0; i-- {
				r := removedRanges[i]
				start := r.start
				end := r.end

				// Consume leading whitespace on the same line as `removed`
				for start > 0 && (result[start-1] == ' ' || result[start-1] == '\t') {
					start--
				}

				// Consume trailing newline after closing brace
				for end < len(result) && (result[end] == '\r' || result[end] == '\n') {
					end++
					if result[end-1] == '\n' {
						break
					}
				}

				result = append(result[:start], result[end:]...)
			}
			resultContent = result
		}

		formattedContent := hclwrite.Format(resultContent)

		if fileModified && stats.NormalizeWhitespace {
			formattedContent = normalizeConsecutiveNewlines(formattedContent)
		}

		if fileModified || !bytes.Equal(formattedContent, content) {
			stats.FilesModified++

			if fileModified {
				stats.RemovedBlocksRemoved += removedBlocksCount
			}

			err = os.WriteFile(filePath, formattedContent, 0600)
			if err != nil {
				return fmt.Errorf("error writing file %s: %w", filePath, err)
			}
		}
	} else if fileModified {
		stats.FilesModified++
		stats.RemovedBlocksRemoved += removedBlocksCount
	}

	return nil
}

func normalizeConsecutiveNewlines(content []byte) []byte {
	contentStr := string(content)

	re := strings.NewReplacer("\n\n\n", "\n\n", "\r\n\r\n\r\n", "\r\n\r\n")

	for {
		newContent := re.Replace(contentStr)
		if newContent == contentStr {
			break
		}
		contentStr = newContent
	}

	contentStr = strings.ReplaceAll(contentStr, "\r\n", "\n")

	contentStr = strings.TrimRight(contentStr, "\n") + "\n"

	if bytes.Contains(content, []byte("\r\n")) {
		contentStr = strings.ReplaceAll(contentStr, "\n", "\r\n")
	}

	return []byte(contentStr)
}

func printUsage() {
	fmt.Println("Terraform Removed Block Remover")
	fmt.Println("-------------------------------")
	fmt.Println("This tool recursively scans Terraform files, removes all 'removed' blocks,")
	fmt.Println("and applies standard Terraform formatting to the files.")
	fmt.Println()
	fmt.Println("Usage: terraform-removed-remover [options] [directory]")
	fmt.Println("       If directory is not specified, the current directory will be used.")
	fmt.Println()
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println()
}

func main() {
	helpFlag := flag.Bool("help", false, "Display help information")
	versionFlag := flag.Bool("version", false, "Display version information")
	dryRunFlag := flag.Bool("dry-run", false, "Run without modifying files")
	verboseFlag := flag.Bool("verbose", false, "Enable verbose output")
	normalizeFlag := flag.Bool("normalize-whitespace", false, "Normalize whitespace after removing removed blocks")

	flag.Usage = printUsage

	flag.Parse()

	if *helpFlag {
		printUsage()
		os.Exit(0)
	}

	if *versionFlag {
		fmt.Printf("Terraform Removed Block Remover v%s\n", Version)
		os.Exit(0)
	}

	args := flag.Args()
	rootDir := "."

	if len(args) > 0 {
		rootDir = args[0]
	}

	info, err := os.Stat(rootDir)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}

	if !info.IsDir() {
		fmt.Printf("Error: %s is not a directory\n", rootDir)
		os.Exit(1)
	}

	stats := Stats{
		StartTime:           time.Now(),
		DryRun:              *dryRunFlag,
		NormalizeWhitespace: *normalizeFlag,
	}

	fmt.Printf("Scanning directory: %s\n", rootDir)
	files, err := findTerraformFiles(rootDir)
	if err != nil {
		fmt.Printf("Error finding Terraform files: %s\n", err)
		os.Exit(1)
	}
	fmt.Printf("Found %d Terraform files\n", len(files))

	for _, file := range files {
		if *verboseFlag {
			fmt.Printf("Processing: %s\n", file)
		}
		err := processFile(file, &stats)
		if err != nil {
			fmt.Printf("Error processing %s: %s\n", file, err)
		}
	}

	stats.EndTime = time.Now()
	duration := stats.EndTime.Sub(stats.StartTime)

	fmt.Printf("\nStatistics:\n")
	if stats.DryRun {
		fmt.Println("DRY RUN MODE: No files were modified")
	}
	fmt.Printf("Files processed: %d\n", stats.FilesProcessed)
	fmt.Printf("Files modified: %d\n", stats.FilesModified)
	fmt.Printf("Removed blocks removed: %d\n", stats.RemovedBlocksRemoved)
	fmt.Printf("Processing time: %v\n", duration)
}
