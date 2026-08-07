package collector

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func (c *collector) copyRemoteTree(remoteDir string, localDir string) error {
	if _, err := c.ssh.CombinedOutput("[ -e "+shellQuote(remoteDir)+" ]", nil); err != nil {
		return fmt.Errorf("remote path does not exist: %s", remoteDir)
	}
	command := "cd " + shellQuote(remoteDir) + " || exit 1\ntar -cf - ."
	return c.copyRemoteTar(command, localDir)
}

func (c *collector) copyRemoteTar(command string, localDir string) error {
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		return err
	}

	stream, wait, closeSession, err := c.ssh.Stream(command)
	if err != nil {
		return err
	}
	defer closeSession()

	if err := extractTar(stream, localDir); err != nil {
		return err
	}
	return wait()
}

func extractTar(reader io.Reader, destination string) error {
	tarReader := tar.NewReader(reader)
	root, err := filepath.Abs(destination)
	if err != nil {
		return err
	}

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("cannot read remote tar stream: %w", err)
		}

		relative := filepath.Clean(filepath.FromSlash(header.Name))
		if relative == "." {
			continue
		}
		if filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("unsafe path in remote tar stream: %s", header.Name)
		}

		target := filepath.Join(root, relative)
		if err := ensureInsideRoot(root, target); err != nil {
			return err
		}
		if err := ensureNoSymlinkParents(root, target); err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
			applyMode(target, os.FileMode(header.Mode))
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if info, err := os.Lstat(target); err == nil && info.Mode()&os.ModeSymlink != 0 {
				if err := os.Remove(target); err != nil {
					return err
				}
			}
			file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(file, tarReader)
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
			applyMode(target, os.FileMode(header.Mode))
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.RemoveAll(target); err != nil {
				return err
			}
			if err := os.Symlink(header.Linkname, target); err != nil {
				return err
			}
		case tar.TypeLink:
			linkTarget := filepath.Join(root, filepath.Clean(filepath.FromSlash(header.Linkname)))
			if err := ensureInsideRoot(root, linkTarget); err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.Link(linkTarget, target); err != nil {
				return err
			}
		}
	}
}

func ensureInsideRoot(root string, path string) error {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path escapes output directory: %s", path)
	}
	return nil
}

func ensureNoSymlinkParents(root string, target string) error {
	relative, err := filepath.Rel(root, filepath.Dir(target))
	if err != nil {
		return err
	}
	current := root
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		if part == "." || part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to write through symlink: %s", current)
		}
	}
	return nil
}

func applyMode(path string, mode os.FileMode) {
	if runtime.GOOS != "windows" {
		_ = os.Chmod(path, mode)
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
