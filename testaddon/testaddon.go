package testaddon

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/bitrise-io/go-utils/v2/log"
)

const (
	// ResultDescriptorFileName is the name of the test result descriptor file.
	ResultDescriptorFileName = "test-info.json"
)

func generateTestInfoFile(dir string, data []byte) error {
	f, err := os.Create(filepath.Join(dir, ResultDescriptorFileName))
	if err != nil {
		return err
	}

	if _, err := f.Write(data); err != nil {
		return err
	}

	if err := f.Sync(); err != nil {
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	return nil
}

// ExportArtifact exports artifact found at path in directory uniqueDir,
// rooted at baseDir.
func ExportArtifact(path, baseDir, uniqueDir string, logger log.Logger) error {
	exportDir := filepath.Join(baseDir, uniqueDir)

	if err := os.MkdirAll(exportDir, os.ModePerm); err != nil {
		return fmt.Errorf("skipping artifact (%s): could not ensure unique export dir (%s): %s", path, exportDir, err)
	}

	if _, err := os.Stat(filepath.Join(exportDir, ResultDescriptorFileName)); os.IsNotExist(err) {
		m := map[string]string{"test-name": uniqueDir}
		data, err := json.Marshal(m)
		if err != nil {
			return fmt.Errorf("create test info descriptor: json marshal data (%s): %s", m, err)
		}
		if err := generateTestInfoFile(exportDir, data); err != nil {
			return fmt.Errorf("create test info descriptor: generate file: %s", err)
		}
	}

	name := filepath.Base(path)
	if err := copyFile(path, filepath.Join(exportDir, name), logger); err != nil {
		return fmt.Errorf("failed to export artifact (%s), error: %v", name, err)
	}
	return nil
}

func copyFile(src, dst string, logger log.Logger) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := srcFile.Close(); err != nil {
			logger.Warnf("Failed to close source file (%s): %v", src, err)
		}
	}()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if err := dstFile.Close(); err != nil {
			logger.Warnf("Failed to close destination file (%s): %v", dst, err)
		}
	}()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	return nil
}
