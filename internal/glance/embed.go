package glance

import (
	"crypto/md5"
	"embed"
	"encoding/hex"
	"io"
	"io/fs"
	"log"
	"strconv"
	"time"
)

//go:embed static
var _staticFS embed.FS

//go:embed templates
var _templateFS embed.FS

var staticFS, _ = fs.Sub(_staticFS, "static")
var templateFS, _ = fs.Sub(_templateFS, "templates")

var staticFSHash = computeFSHash(staticFS)

func computeFSHash(files fs.FS) string {
	hash := md5.New()

	err := fs.WalkDir(files, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		file, err := files.Open(path)
		if err != nil {
			return err
		}

		if _, err := io.Copy(hash, file); err != nil {
			return err
		}

		return nil
	})

	if err == nil {
		return hex.EncodeToString(hash.Sum(nil))[:10]
	}

	log.Printf("Could not compute assets cache: %v", err)
	return strconv.FormatInt(time.Now().Unix(), 10)
}
