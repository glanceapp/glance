package glance

import (
	"bytes"
	"crypto/md5"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

//go:embed static
var _staticFS embed.FS

//go:embed templates
var _templateFS embed.FS

var staticFS, _ = fs.Sub(_staticFS, "static")
var templateFS, _ = fs.Sub(_templateFS, "templates")

func readAllFromStaticFS(path string) ([]byte, error) {
	// For some reason fs.FS only works with forward slashes, so in case we're
	// running on Windows or pass paths with backslashes we need to replace them.
	path = strings.ReplaceAll(path, "\\", "/")

	file, err := staticFS.Open(path)
	if err != nil {
		return nil, err
	}

	return io.ReadAll(file)
}

var staticFSHash = func() string {
	hash, err := computeFSHash(staticFS)
	if err != nil {
		log.Printf("Could not compute static assets cache key: %v", err)
		return strconv.FormatInt(time.Now().Unix(), 10)
	}

	return hash
}()

func computeFSHash(files fs.FS) (string, error) {
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

	if err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil))[:10], nil
}

var cssImportPattern = regexp.MustCompile(`(?m)^@import "(.*?)";$`)
var cssSingleLineCommentPattern = regexp.MustCompile(`(?m)^\s*\/\*.*?\*\/$`)

// Yes, we bundle at runtime, give comptime pls
var bundledCSSContents = func() []byte {
	const mainFilePath = "css/main.css"

	var recursiveParseImports func(path string, depth int) ([]byte, error)
	recursiveParseImports = func(path string, depth int) ([]byte, error) {
		if depth > 20 {
			return nil, errors.New("maximum import depth reached, is one of your imports circular?")
		}

		mainFileContents, err := readAllFromStaticFS(path)
		if err != nil {
			return nil, err
		}

		// Normalize line endings, otherwise the \r's make the regex not match
		mainFileContents = bytes.ReplaceAll(mainFileContents, []byte("\r\n"), []byte("\n"))

		mainFileDir := filepath.Dir(path)
		var importLastErr error

		parsed := cssImportPattern.ReplaceAllFunc(mainFileContents, func(match []byte) []byte {
			if importLastErr != nil {
				return nil
			}

			matches := cssImportPattern.FindSubmatch(match)
			if len(matches) != 2 {
				importLastErr = fmt.Errorf(
					"import didn't return expected number of capture groups: %s, expected 2, got %d",
					match, len(matches),
				)
				return nil
			}

			importFilePath := filepath.Join(mainFileDir, string(matches[1]))
			importContents, err := recursiveParseImports(importFilePath, depth+1)
			if err != nil {
				importLastErr = err
				return nil
			}

			return importContents
		})

		if importLastErr != nil {
			return nil, importLastErr
		}

		return parsed, nil
	}

	contents, err := recursiveParseImports(mainFilePath, 0)
	if err != nil {
		panic(fmt.Sprintf("building CSS bundle: %v", err))
	}

	// We could strip a bunch more unnecessary characters, but the biggest
	// win comes from removing the whitespace at the beginning of lines
	// since that's at least 4 bytes per property, which yielded a ~20% reduction.
	contents = cssSingleLineCommentPattern.ReplaceAll(contents, nil)
	contents = whitespaceAtBeginningOfLinePattern.ReplaceAll(contents, nil)
	contents = bytes.ReplaceAll(contents, []byte("\n"), []byte(""))

	return contents
}()
