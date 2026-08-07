package collector

import (
	"archive/zip"
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

	zipWriter := zip.NewWriter(archiveFile)
	parent := filepath.Dir(sourceDir)

	walkErr := filepath.Walk(sourceDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relative, err := filepath.Rel(parent, path)
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relative)
		if info.IsDir() {
			header.Name += "/"
		}
		header.Method = zip.Deflate

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			_, err = writer.Write([]byte(linkTarget))
			return err
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(writer, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})

	zipErr := zipWriter.Close()
	fileErr := archiveFile.Close()
	if walkErr != nil {
		return walkErr
	}
	if zipErr != nil {
		return fmt.Errorf("cannot finalize zip archive: %w", zipErr)
	}
	return fileErr
}
