package discovery

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

type File struct {
	Index     int
	RelPath   string
	AbsPath   string
	SizeBytes int64
}

func Discover(inputRoot string) ([]File, error) {
	absRoot, err := filepath.Abs(inputRoot)
	if err != nil {
		return nil, err
	}

	type pair struct {
		rel  string
		abs  string
		size int64
	}
	var found []pair

	err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(d.Name()), ".gpx") {
			rel, relErr := filepath.Rel(absRoot, path)
			if relErr != nil {
				return relErr
			}
			info, infoErr := d.Info()
			if infoErr != nil {
				return infoErr
			}
			found = append(found, pair{rel: filepath.ToSlash(rel), abs: path, size: info.Size()})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(found, func(i, j int) bool {
		return found[i].rel < found[j].rel
	})

	files := make([]File, 0, len(found))
	for i, f := range found {
		files = append(files, File{Index: i, RelPath: f.rel, AbsPath: f.abs, SizeBytes: f.size})
	}

	return files, nil
}
