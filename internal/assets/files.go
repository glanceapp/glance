package assets

import (
	"crypto/md5"
	"embed"
	"encoding/hex"
	"io"
	"io/fs"
	"log/slog"
	"strconv"
	"time"
)

//go:embed static
var _publicFS embed.FS

//go:embed templates
var _templateFS embed.FS

var PublicFS, _ = fs.Sub(_publicFS, "static")
var TemplateFS, _ = fs.Sub(_templateFS, "templates")

func getFSHash(files fs.FS) string {
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

	slog.Warn("Could not compute assets cache", "err", err)
	return strconv.FormatInt(time.Now().Unix(), 10)
}

var PublicFSHash = getFSHash(PublicFS)
