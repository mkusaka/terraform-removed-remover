// Package main provides integration tests for the terraform-removed-remover tool
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIntegrationBasicUsage(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "terraform-integration-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			_ = removeErr // Ignore cleanup errors in tests
		}
	}()

	mainTf := filepath.Join(tempDir, "main.tf")
	mainContent := `
provider "aws" {
  region = "us-west-2"
}

resource "aws_instance" "web" {
  ami           = "ami-123456"
  instance_type = "t2.micro"
}

removed {
  from = aws_instance.old_web
  lifecycle {
    destroy = false
  }
}

resource "aws_s3_bucket" "data" {
  bucket = "my-bucket"
}
`
	err = os.WriteFile(mainTf, []byte(mainContent), 0600)
	if err != nil {
		t.Fatalf("Failed to write main.tf: %v", err)
	}

	moduleDir := filepath.Join(tempDir, "modules", "networking")
	err = os.MkdirAll(moduleDir, 0750)
	if err != nil {
		t.Fatalf("Failed to create module directory: %v", err)
	}

	vpcTf := filepath.Join(moduleDir, "vpc.tf")
	vpcContent := `
resource "aws_vpc" "main" {
  cidr_block = "10.0.0.0/16"
}

removed {
  from = aws_vpc.old_main
  lifecycle {
    destroy = true
  }
}

removed {
  from = aws_subnet.old_subnet
  lifecycle {
    destroy = false
  }
}
`
	err = os.WriteFile(vpcTf, []byte(vpcContent), 0600)
	if err != nil {
		t.Fatalf("Failed to write vpc.tf: %v", err)
	}

	stats := Stats{
		StartTime:           time.Now(),
		NormalizeWhitespace: true,
	}

	files, err := findTerraformFiles(tempDir)
	if err != nil {
		t.Fatalf("findTerraformFiles failed: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("Expected to find 2 .tf files, but found %d", len(files))
	}

	for _, file := range files {
		processErr := processFile(file, &stats)
		if processErr != nil {
			t.Fatalf("processFile failed for %s: %v", file, processErr)
		}
	}

	if stats.FilesProcessed != 2 {
		t.Errorf("Expected FilesProcessed to be 2, but got %d", stats.FilesProcessed)
	}
	if stats.FilesModified != 2 {
		t.Errorf("Expected FilesModified to be 2, but got %d", stats.FilesModified)
	}
	if stats.RemovedBlocksRemoved != 3 {
		t.Errorf("Expected RemovedBlocksRemoved to be 3, but got %d", stats.RemovedBlocksRemoved)
	}

	mainModified, err := os.ReadFile(mainTf)
	if err != nil {
		t.Fatalf("Failed to read modified main.tf: %v", err)
	}

	if strings.Contains(string(mainModified), "removed {") {
		t.Errorf("main.tf still contains removed blocks after processing")
	}

	vpcModified, err := os.ReadFile(vpcTf)
	if err != nil {
		t.Fatalf("Failed to read modified vpc.tf: %v", err)
	}

	if strings.Contains(string(vpcModified), "removed {") {
		t.Errorf("vpc.tf still contains removed blocks after processing")
	}
}

// TestLeadingCommentsPreserved tests that comments preceding removed blocks
// are NOT removed along with the removed block.
func TestLeadingCommentsPreserved(t *testing.T) {
	testDir, err := os.MkdirTemp("", "terraform-comment-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(testDir); removeErr != nil {
			_ = removeErr // Ignore cleanup errors in tests
		}
	}()

	// Case 1: Comment directly before removed block (no blank line)
	t.Run("comment_directly_before_removed_block", func(t *testing.T) {
		filePath := filepath.Join(testDir, "case1.tf")
		input := `resource "aws_instance" "web" {
  ami           = "ami-123456"
  instance_type = "t2.micro"
}

# This comment describes the resource removal
removed {
  from = aws_instance.old
  lifecycle {
    destroy = false
  }
}
`
		if err := os.WriteFile(filePath, []byte(input), 0600); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		stats := Stats{}
		if err := processFile(filePath, &stats); err != nil {
			t.Fatalf("processFile failed: %v", err)
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("Failed to read modified file: %v", err)
		}

		result := string(content)
		t.Logf("Case 1 output:\n%s", result)

		if strings.Contains(result, "removed {") {
			t.Error("removed block should have been removed")
		}
		if !strings.Contains(result, "# This comment describes the resource removal") {
			t.Error("Leading comment was removed along with the removed block — this is the bug")
		}
	})

	// Case 2: Multiple comment lines directly before removed block
	t.Run("multiple_comments_before_removed_block", func(t *testing.T) {
		filePath := filepath.Join(testDir, "case2.tf")
		input := `resource "aws_instance" "web" {
  ami           = "ami-123456"
  instance_type = "t2.micro"
}

# Description of the removal
# reason: no longer needed
removed {
  from = aws_instance.old
  lifecycle {
    destroy = false
  }
}
`
		if err := os.WriteFile(filePath, []byte(input), 0600); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		stats := Stats{}
		if err := processFile(filePath, &stats); err != nil {
			t.Fatalf("processFile failed: %v", err)
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("Failed to read modified file: %v", err)
		}

		result := string(content)
		t.Logf("Case 2 output:\n%s", result)

		if !strings.Contains(result, "# Description of the removal") {
			t.Error("First comment line was removed along with the removed block")
		}
		if !strings.Contains(result, "# reason: no longer needed") {
			t.Error("Second comment line was removed along with the removed block")
		}
	})

	// Case 3: Blank line separates comment from removed block
	t.Run("comment_separated_by_blank_line", func(t *testing.T) {
		filePath := filepath.Join(testDir, "case3.tf")
		input := `resource "aws_instance" "web" {
  ami           = "ami-123456"
  instance_type = "t2.micro"
}

# This comment is separated by a blank line

removed {
  from = aws_instance.old
  lifecycle {
    destroy = false
  }
}
`
		if err := os.WriteFile(filePath, []byte(input), 0600); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		stats := Stats{}
		if err := processFile(filePath, &stats); err != nil {
			t.Fatalf("processFile failed: %v", err)
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("Failed to read modified file: %v", err)
		}

		result := string(content)
		t.Logf("Case 3 output:\n%s", result)

		if !strings.Contains(result, "# This comment is separated by a blank line") {
			t.Error("Comment separated by blank line was unexpectedly removed")
		}
	})

	// Case 4: Comment belongs to the NEXT resource, not the removed block
	t.Run("comment_between_removed_and_resource", func(t *testing.T) {
		filePath := filepath.Join(testDir, "case4.tf")
		input := `resource "aws_instance" "web" {
  ami           = "ami-123456"
  instance_type = "t2.micro"
}

# Describes the S3 bucket below
removed {
  from = aws_instance.old
  lifecycle {
    destroy = false
  }
}

resource "aws_s3_bucket" "data" {
  bucket = "my-data-bucket"
}
`
		if err := os.WriteFile(filePath, []byte(input), 0600); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		stats := Stats{}
		if err := processFile(filePath, &stats); err != nil {
			t.Fatalf("processFile failed: %v", err)
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("Failed to read modified file: %v", err)
		}

		result := string(content)
		t.Logf("Case 4 output:\n%s", result)

		if !strings.Contains(result, "# Describes the S3 bucket below") {
			t.Error("Comment that semantically belongs to another resource was removed with the removed block")
		}
	})
}

func TestIntegrationDryRun(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "terraform-dry-run-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			_ = removeErr // Ignore cleanup errors in tests
		}
	}()

	testFile := filepath.Join(tempDir, "test.tf")
	originalContent := `
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
	err = os.WriteFile(testFile, []byte(originalContent), 0600)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	stats := Stats{
		StartTime: time.Now(),
		DryRun:    true,
	}

	files, err := findTerraformFiles(tempDir)
	if err != nil {
		t.Fatalf("findTerraformFiles failed: %v", err)
	}

	for _, file := range files {
		processErr := processFile(file, &stats)
		if processErr != nil {
			t.Fatalf("processFile failed for %s: %v", file, processErr)
		}
	}

	if stats.RemovedBlocksRemoved != 1 {
		t.Errorf("Expected RemovedBlocksRemoved to be 1, but got %d", stats.RemovedBlocksRemoved)
	}

	modifiedContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file after dry run: %v", err)
	}

	if string(modifiedContent) != originalContent {
		t.Errorf("Dry run mode modified the file, but it shouldn't have")
	}
}

func TestIntegrationEmptyDirectory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "terraform-empty-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			_ = removeErr // Ignore cleanup errors in tests
		}
	}()

	stats := Stats{
		StartTime: time.Now(),
	}

	files, err := findTerraformFiles(tempDir)
	if err != nil {
		t.Fatalf("findTerraformFiles failed: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("Expected to find 0 .tf files in empty directory, but found %d", len(files))
	}

	for _, file := range files {
		processErr := processFile(file, &stats)
		if processErr != nil {
			t.Fatalf("processFile failed for %s: %v", file, processErr)
		}
	}

	if stats.FilesProcessed != 0 {
		t.Errorf("Expected FilesProcessed to be 0, but got %d", stats.FilesProcessed)
	}
	if stats.RemovedBlocksRemoved != 0 {
		t.Errorf("Expected RemovedBlocksRemoved to be 0, but got %d", stats.RemovedBlocksRemoved)
	}
}

func TestIntegrationNoRemovedBlocks(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "terraform-no-removed-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			_ = removeErr // Ignore cleanup errors in tests
		}
	}()

	testFile := filepath.Join(tempDir, "clean.tf")
	content := `
resource "aws_instance" "web" {
  ami           = "ami-123456"
  instance_type = "t2.micro"
}

resource "aws_s3_bucket" "data" {
  bucket = "my-bucket"
}
`
	err = os.WriteFile(testFile, []byte(content), 0600)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	stats := Stats{
		StartTime: time.Now(),
	}

	files, err := findTerraformFiles(tempDir)
	if err != nil {
		t.Fatalf("findTerraformFiles failed: %v", err)
	}

	for _, file := range files {
		processErr := processFile(file, &stats)
		if processErr != nil {
			t.Fatalf("processFile failed for %s: %v", file, processErr)
		}
	}

	if stats.FilesProcessed != 1 {
		t.Errorf("Expected FilesProcessed to be 1, but got %d", stats.FilesProcessed)
	}
	if stats.RemovedBlocksRemoved != 0 {
		t.Errorf("Expected RemovedBlocksRemoved to be 0, but got %d", stats.RemovedBlocksRemoved)
	}
}
