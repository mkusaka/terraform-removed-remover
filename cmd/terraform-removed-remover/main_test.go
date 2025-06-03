package main

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFindTerraformFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "terraform-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFiles := []string{
		filepath.Join(tempDir, "main.tf"),
		filepath.Join(tempDir, "variables.tf"),
		filepath.Join(tempDir, "nested", "module.tf"),
		filepath.Join(tempDir, "nested", "deep", "resource.tf"),
		filepath.Join(tempDir, "not-terraform.txt"),
	}

	if err := os.MkdirAll(filepath.Join(tempDir, "nested", "deep"), 0755); err != nil {
		t.Fatalf("Failed to create nested directories: %v", err)
	}

	for _, file := range testFiles {
		dir := filepath.Dir(file)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatalf("Failed to create directory %s: %v", dir, err)
			}
		}
		if err := os.WriteFile(file, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", file, err)
		}
	}

	files, err := findTerraformFiles(tempDir)
	if err != nil {
		t.Fatalf("findTerraformFiles failed: %v", err)
	}

	if len(files) != 4 {
		t.Errorf("Expected to find 4 .tf files, but found %d", len(files))
	}

	_, err = findTerraformFiles("/non-existent-dir")
	if err == nil {
		t.Errorf("Expected error for non-existent directory, but got nil")
	}
}

func TestProcessFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "terraform-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "test.tf")
	content := `
resource "aws_instance" "web" {
  ami           = "ami-123456"
  instance_type = "t2.micro"
}

removed {
  from = aws_instance.old
  lifecycle {
    destroy = false
  }
}

resource "aws_s3_bucket" "data" {
  bucket = "my-bucket"
}

removed {
  from = aws_s3_bucket.logs
  lifecycle {
    destroy = true
  }
}
`
	err = os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	stats := Stats{
		StartTime: time.Now(),
	}
	err = processFile(testFile, &stats)
	if err != nil {
		t.Fatalf("processFile failed: %v", err)
	}

	if stats.FilesProcessed != 1 {
		t.Errorf("Expected FilesProcessed to be 1, but got %d", stats.FilesProcessed)
	}
	if stats.FilesModified != 1 {
		t.Errorf("Expected FilesModified to be 1, but got %d", stats.FilesModified)
	}
	if stats.RemovedBlocksRemoved != 2 {
		t.Errorf("Expected RemovedBlocksRemoved to be 2, but got %d", stats.RemovedBlocksRemoved)
	}

	modifiedContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read modified file: %v", err)
	}

	if string(modifiedContent) == content {
		t.Errorf("File content was not modified")
	}

	err = processFile("/non-existent-file.tf", &stats)
	if err == nil {
		t.Errorf("Expected error for non-existent file, but got nil")
	}

	invalidFile := filepath.Join(tempDir, "invalid.tf")
	err = os.WriteFile(invalidFile, []byte("this is not valid HCL"), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid file: %v", err)
	}

	err = processFile(invalidFile, &stats)
	if err == nil {
		t.Errorf("Expected error for invalid HCL, but got nil")
	}
	
	unformattedFile := filepath.Join(tempDir, "unformatted.tf")
	unformattedContent := `
resource "aws_instance" "web" {
ami = "ami-123456"
  instance_type   =     "t2.micro"
}
`
	err = os.WriteFile(unformattedFile, []byte(unformattedContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write unformatted file: %v", err)
	}

	err = processFile(unformattedFile, &stats)
	if err != nil {
		t.Fatalf("processFile failed for formatting test: %v", err)
	}

	formattedContent, err := os.ReadFile(unformattedFile)
	if err != nil {
		t.Fatalf("Failed to read formatted file: %v", err)
	}

	if string(formattedContent) == unformattedContent {
		t.Errorf("File was not formatted")
	}

	formattedString := string(formattedContent)
	t.Logf("Formatted content: %s", formattedString)
	
	if !strings.Contains(formattedString, "  ami") {
		t.Errorf("Formatting did not properly indent attributes")
	}
}

func TestMainFunction(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	tempDir, err := os.MkdirTemp("", "terraform-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "main.tf")
	content := `
resource "aws_instance" "web" {
  ami           = "ami-123456"
  instance_type = "t2.micro"
}

removed {
  from = aws_instance.old
  lifecycle {
    destroy = false
  }
}
`
	err = os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	os.Args = []string{"cmd", "-dry-run=false", tempDir}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	
	stats := Stats{
		StartTime: time.Now(),
	}
	
	files, err := findTerraformFiles(tempDir)
	if err != nil {
		t.Fatalf("findTerraformFiles failed: %v", err)
	}
	
	if len(files) != 1 {
		t.Errorf("Expected to find 1 .tf file, but found %d", len(files))
	}
	
	err = processFile(files[0], &stats)
	if err != nil {
		t.Fatalf("processFile failed: %v", err)
	}
	
	if stats.RemovedBlocksRemoved != 1 {
		t.Errorf("Expected RemovedBlocksRemoved to be 1, but got %d", stats.RemovedBlocksRemoved)
	}
}

func TestFlagHandling(t *testing.T) {
	oldArgs := os.Args
	oldFlagCommandLine := flag.CommandLine
	defer func() { 
		os.Args = oldArgs 
		flag.CommandLine = oldFlagCommandLine
	}()
	
	tempDir, err := os.MkdirTemp("", "terraform-flag-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	testFile := filepath.Join(tempDir, "test.tf")
	content := `
resource "aws_instance" "web" {
  ami           = "ami-123456"
  instance_type = "t2.micro"
}

removed {
  from = aws_instance.old
  lifecycle {
    destroy = false
  }
}
`
	err = os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	
	os.Args = []string{"cmd", "-dry-run", tempDir}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	
	stats := Stats{
		StartTime: time.Now(),
		DryRun:    true,
	}
	
	err = processFile(testFile, &stats)
	if err != nil {
		t.Fatalf("processFile failed: %v", err)
	}
	
	modifiedContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file after dry run: %v", err)
	}
	
	if string(modifiedContent) != content {
		t.Errorf("Dry run mode modified the file, but it shouldn't have")
	}
}

func TestConsecutiveRemovedBlocks(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "terraform-consecutive-removed-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "consecutive_removed.tf")
	content := `
resource "aws_instance" "web" {
  ami           = "ami-123456"
  instance_type = "t2.micro"
}

removed {
  from = aws_instance.old1
  lifecycle {
    destroy = false
  }
}

removed {
  from = aws_instance.old2
  lifecycle {
    destroy = true
  }
}

removed {
  from = aws_instance.old3
  lifecycle {
    destroy = false
  }
}

resource "aws_s3_bucket" "data" {
  bucket = "my-bucket"
}
`
	err = os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	stats := Stats{
		StartTime:           time.Now(),
		NormalizeWhitespace: true,
	}
	err = processFile(testFile, &stats)
	if err != nil {
		t.Fatalf("processFile failed: %v", err)
	}

	modifiedContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read modified file: %v", err)
	}

	t.Logf("Modified content: %s", string(modifiedContent))

	if strings.Contains(string(modifiedContent), "removed {") {
		t.Errorf("File still contains removed blocks after processing")
	}

	lines := strings.Split(string(modifiedContent), "\n")
	consecutiveEmptyLines := 0
	maxConsecutiveEmptyLines := 0
	
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			consecutiveEmptyLines++
		} else {
			if consecutiveEmptyLines > maxConsecutiveEmptyLines {
				maxConsecutiveEmptyLines = consecutiveEmptyLines
			}
			consecutiveEmptyLines = 0
		}
	}
	
	if maxConsecutiveEmptyLines > 2 {
		t.Errorf("File contains %d consecutive empty lines, expected at most 1", maxConsecutiveEmptyLines-1)
	}
}

func TestWhitespaceNormalizationFlag(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "terraform-whitespace-flag-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	content := `
resource "aws_instance" "web" {
  ami           = "ami-123456"
  instance_type = "t2.micro"
}

removed {
  from = aws_instance.old1
  lifecycle {
    destroy = false
  }
}

removed {
  from = aws_instance.old2
  lifecycle {
    destroy = true
  }
}

removed {
  from = aws_instance.old3
  lifecycle {
    destroy = false
  }
}

resource "aws_s3_bucket" "data" {
  bucket = "my-bucket"
}
`

	testFileDisabled := filepath.Join(tempDir, "normalization_disabled.tf")
	err = os.WriteFile(testFileDisabled, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	statsDisabled := Stats{
		StartTime:           time.Now(),
		NormalizeWhitespace: false,
	}
	err = processFile(testFileDisabled, &statsDisabled)
	if err != nil {
		t.Fatalf("processFile failed with normalization disabled: %v", err)
	}

	disabledContent, err := os.ReadFile(testFileDisabled)
	if err != nil {
		t.Fatalf("Failed to read modified file: %v", err)
	}

	t.Logf("Content with normalization disabled: %s", string(disabledContent))

	testFileEnabled := filepath.Join(tempDir, "normalization_enabled.tf")
	err = os.WriteFile(testFileEnabled, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	statsEnabled := Stats{
		StartTime:           time.Now(),
		NormalizeWhitespace: true,
	}
	err = processFile(testFileEnabled, &statsEnabled)
	if err != nil {
		t.Fatalf("processFile failed with normalization enabled: %v", err)
	}

	enabledContent, err := os.ReadFile(testFileEnabled)
	if err != nil {
		t.Fatalf("Failed to read modified file: %v", err)
	}

	t.Logf("Content with normalization enabled: %s", string(enabledContent))

	disabledLines := strings.Split(string(disabledContent), "\n")
	disabledConsecutiveEmptyLines := 0
	disabledMaxConsecutiveEmptyLines := 0
	
	for _, line := range disabledLines {
		if strings.TrimSpace(line) == "" {
			disabledConsecutiveEmptyLines++
		} else {
			if disabledConsecutiveEmptyLines > disabledMaxConsecutiveEmptyLines {
				disabledMaxConsecutiveEmptyLines = disabledConsecutiveEmptyLines
			}
			disabledConsecutiveEmptyLines = 0
		}
	}
	
	enabledLines := strings.Split(string(enabledContent), "\n")
	enabledConsecutiveEmptyLines := 0
	enabledMaxConsecutiveEmptyLines := 0
	
	for _, line := range enabledLines {
		if strings.TrimSpace(line) == "" {
			enabledConsecutiveEmptyLines++
		} else {
			if enabledConsecutiveEmptyLines > enabledMaxConsecutiveEmptyLines {
				enabledMaxConsecutiveEmptyLines = enabledConsecutiveEmptyLines
			}
			enabledConsecutiveEmptyLines = 0
		}
	}
	
	if disabledMaxConsecutiveEmptyLines <= enabledMaxConsecutiveEmptyLines {
		t.Errorf("Expected more consecutive empty lines with normalization disabled, but got %d (disabled) vs %d (enabled)",
			disabledMaxConsecutiveEmptyLines, enabledMaxConsecutiveEmptyLines)
	}
	
	if enabledMaxConsecutiveEmptyLines > 2 {
		t.Errorf("With normalization enabled, file contains %d consecutive empty lines, expected at most 1", 
			enabledMaxConsecutiveEmptyLines-1)
	}
}

func TestTrailingEmptyLines(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "terraform-trailing-empty-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "trailing_removed.tf")
	content := `
module "hoge" {
  source = "fuga"
}

removed {
  from = aws_instance.example
  lifecycle {
    destroy = false
  }
}
`
	err = os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	stats := Stats{
		StartTime:           time.Now(),
		NormalizeWhitespace: true,
	}
	err = processFile(testFile, &stats)
	if err != nil {
		t.Fatalf("processFile failed: %v", err)
	}

	modifiedContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read modified file: %v", err)
	}

	t.Logf("Modified content: %s", string(modifiedContent))

	if strings.Contains(string(modifiedContent), "removed {") {
		t.Errorf("File still contains removed blocks after processing")
	}

	lines := strings.Split(string(modifiedContent), "\n")
	
	trailingEmptyLines := 0
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) == "" {
			trailingEmptyLines++
		} else {
			break
		}
	}
	
	if trailingEmptyLines > 1 {
		t.Errorf("File contains %d trailing empty lines, expected at most 1", trailingEmptyLines)
	}
}
