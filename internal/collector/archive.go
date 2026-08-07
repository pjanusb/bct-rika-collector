package collector

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func createArchive(sourceDir string, archivePath string) error {
	archiveFile, err := os.Create(archivePath)
	if err != nil {
		return err
	}

	gzipWriter := gzip.NewWriter(archiveFile)
	tarWriter := tar.NewWriter(gzipWriter)
	parent := filepath.Dir(sourceDir)

	walkErr := filepath.Walk(sourceDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relative, err := filepath.Rel(parent, path)
		if err != nil {
			return err
		}

		linkTarget := ""
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}

		header, err := tar.FileInfoHeader(info, linkTarget)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relative)
		if info.IsDir() {
			header.Name += "/"
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tarWriter, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})

	tarErr := tarWriter.Close()
	gzipErr := gzipWriter.Close()
	fileErr := archiveFile.Close()
	if walkErr != nil {
		return walkErr
	}
	if tarErr != nil {
		return fmt.Errorf("cannot finalize tar archive: %w", tarErr)
	}
	if gzipErr != nil {
		return fmt.Errorf("cannot finalize gzip archive: %w", gzipErr)
	}
	return fileErr
}
