package export

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
)

func ZipFiles(zipPath string, files map[string]string) error {
	out, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer out.Close()
	zw := zip.NewWriter(out)
	defer zw.Close()
	for name, path := range files {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		if _, err := io.Copy(w, f); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}
	return nil
}

func EnsureDir(dir string) error    { return os.MkdirAll(dir, 0755) }
func Join(base, name string) string { return filepath.Join(base, name) }
