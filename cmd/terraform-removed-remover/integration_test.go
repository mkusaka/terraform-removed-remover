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

func TestIntegrationDryRun(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "terraform-dry-run-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
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
