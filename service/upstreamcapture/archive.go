package upstreamcapture

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
)

// Local layout under capture_setting.ResolvedLocalDir():
//
//	raw/<day>/<model>/<newapiReqId>_<reqId>.json   records still accumulating
//	packages/<day>/<model>.zip                     one package per day and model
//	<day>/channel_<id>/...                         legacy layout (uploaded in real time)
const (
	rawDirName      = "raw"
	packagesDirName = "packages"
	dayLayout       = "2006-01-02"
	zipExt          = ".zip"
)

// captureZone defines the capture "day" (Beijing time). A fixed offset keeps it
// independent of the container TZ and of tzdata availability.
var captureZone = time.FixedZone("UTC+8", 8*60*60)

func captureDay(t time.Time) string {
	return t.In(captureZone).Format(dayLayout)
}

// modelDirName turns an upstream model name into a single safe path segment.
func modelDirName(name string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), ".")
	if out == "" {
		return "unknown"
	}
	return out
}

func isDayName(name string) bool {
	_, err := time.ParseInLocation(dayLayout, name, captureZone)
	return err == nil
}

// isPackableDate reports whether the raw records of day may be packed at now.
// day is a capture day ("2006-01-02", Beijing time). Records are filed by the
// day they are written (see recordDir), so nothing new lands in a day once it
// has ended and it can be packed right after midnight.
func isPackableDate(day string, now time.Time) bool {
	start, err := time.ParseInLocation(dayLayout, day, captureZone)
	if err != nil {
		return false
	}
	end := start.AddDate(0, 0, 1)
	return !now.Before(end)
}

type PackResult struct {
	Packages int `json:"packages"`
	Records  int `json:"records"`
}

type rawGroup struct {
	day, model, dir string
}

// PackEndedDays packs every raw <day>/<model> directory of a finished day into
// packages/<day>/<model>.zip and removes the packed raw files. A package that
// already exists (records that finished after the day was packed) is rebuilt
// with its previous entries plus the new files.
func PackEndedDays(ctx context.Context, root string, now time.Time, progress func(done, total int)) (PackResult, error) {
	var result PackResult
	var groups []rawGroup
	rawRoot := filepath.Join(root, rawDirName)
	days, err := os.ReadDir(rawRoot)
	if errors.Is(err, fs.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	for _, day := range days {
		if !day.IsDir() || !isDayName(day.Name()) || !isPackableDate(day.Name(), now) {
			continue
		}
		models, err := os.ReadDir(filepath.Join(rawRoot, day.Name()))
		if err != nil {
			return result, err
		}
		for _, m := range models {
			if m.IsDir() {
				groups = append(groups, rawGroup{day: day.Name(), model: m.Name(), dir: filepath.Join(rawRoot, day.Name(), m.Name())})
			}
		}
	}

	for i, g := range groups {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		packed, err := packGroup(root, g)
		if err != nil {
			return result, fmt.Errorf("pack %s/%s: %w", g.day, g.model, err)
		}
		if packed > 0 {
			result.Packages++
			result.Records += packed
		}
		// Remove directories left empty; a non-empty one (late write) is kept.
		_ = os.Remove(g.dir)
		_ = os.Remove(filepath.Dir(g.dir))
		if progress != nil {
			progress(i+1, len(groups))
		}
	}
	return result, nil
}

// packGroup writes one package and returns how many raw records went into it.
func packGroup(root string, g rawGroup) (int, error) {
	entries, err := os.ReadDir(g.dir)
	if err != nil {
		return 0, err
	}
	var names []string
	for _, e := range entries {
		if e.Type().IsRegular() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		return 0, nil
	}
	// File names start with the time-ordered new-api request id.
	sort.Strings(names)

	zipPath := filepath.Join(root, packagesDirName, g.day, g.model+zipExt)
	if err := os.MkdirAll(filepath.Dir(zipPath), 0o755); err != nil {
		return 0, err
	}
	tmpPath := zipPath + ".tmp"
	if err := writePackage(tmpPath, zipPath, g.dir, names); err != nil {
		_ = os.Remove(tmpPath)
		return 0, err
	}
	if err := os.Rename(tmpPath, zipPath); err != nil {
		_ = os.Remove(tmpPath)
		return 0, err
	}
	for _, name := range names {
		if err := os.Remove(filepath.Join(g.dir, name)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return len(names), err
		}
	}
	return len(names), nil
}

func writePackage(tmpPath, existingPath, rawDir string, names []string) error {
	out, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	zw := zip.NewWriter(out)

	existing := map[string]struct{}{}
	if prev, err := zip.OpenReader(existingPath); err == nil {
		defer prev.Close()
		for _, f := range prev.File {
			if err := zw.Copy(f); err != nil {
				return err
			}
			existing[f.Name] = struct{}{}
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	for _, name := range names {
		if _, dup := existing[name]; dup {
			continue
		}
		if err := addPackageEntry(zw, filepath.Join(rawDir, name), name); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return out.Sync()
}

func addPackageEntry(zw *zip.Writer, path, name string) error {
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: info.ModTime()})
	if err != nil {
		return err
	}
	_, err = io.Copy(w, in)
	return err
}

type localPackage struct {
	day, model, path string
	size, mtime      int64
}

func (p localPackage) uploadKey() string {
	return fmt.Sprintf("%s|%s|%d|%d", p.day, p.model, p.size, p.mtime)
}

func uploadRecordKey(r *model.CaptureUploadRecord) string {
	return fmt.Sprintf("%s|%s|%d|%d", r.CaptureDate, r.ModelName, r.ZipSize, r.ZipMtime)
}

func listPackages(root string) ([]localPackage, error) {
	var pkgs []localPackage
	pkgRoot := filepath.Join(root, packagesDirName)
	days, err := os.ReadDir(pkgRoot)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	for _, day := range days {
		if !day.IsDir() || !isDayName(day.Name()) {
			continue
		}
		files, err := os.ReadDir(filepath.Join(pkgRoot, day.Name()))
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			if !f.Type().IsRegular() || !strings.HasSuffix(f.Name(), zipExt) {
				continue
			}
			info, err := f.Info()
			if err != nil {
				continue
			}
			pkgs = append(pkgs, localPackage{
				day:   day.Name(),
				model: strings.TrimSuffix(f.Name(), zipExt),
				path:  filepath.Join(pkgRoot, day.Name(), f.Name()),
				size:  info.Size(),
				mtime: info.ModTime().UnixNano(),
			})
		}
	}
	return pkgs, nil
}

func countPackageEntries(path string) int {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return 0
	}
	defer zr.Close()
	return len(zr.File)
}

type LocalStatsItem struct {
	Day          string `json:"day"`
	Model        string `json:"model"`
	RawFiles     int    `json:"raw_files"`
	RawBytes     int64  `json:"raw_bytes"`
	PackageFiles int    `json:"package_files"`
	PackageBytes int64  `json:"package_bytes"`
	Packaged     bool   `json:"packaged"`
	Uploaded     bool   `json:"uploaded"`
	UploadedAt   int64  `json:"uploaded_at"`
}

type LocalStats struct {
	Root               string           `json:"root"`
	TotalBytes         int64            `json:"total_bytes"`
	RawFiles           int              `json:"raw_files"`
	RawBytes           int64            `json:"raw_bytes"`
	PackageCount       int              `json:"package_count"`
	PackageBytes       int64            `json:"package_bytes"`
	UploadedCount      int              `json:"uploaded_count"`
	UploadedBytes      int64            `json:"uploaded_bytes"`
	PendingUploadBytes int64            `json:"pending_upload_bytes"`
	LegacyFiles        int              `json:"legacy_files"`
	LegacyBytes        int64            `json:"legacy_bytes"`
	Items              []LocalStatsItem `json:"items"`
}

// CollectLocalStats sizes everything stored locally. A package counts as
// uploaded only when a successful upload matches its current size and mtime.
func CollectLocalStats(root string, uploads []*model.CaptureUploadRecord) (*LocalStats, error) {
	stats := &LocalStats{Root: root, Items: []LocalStatsItem{}}
	items := map[string]*LocalStatsItem{}
	item := func(day, modelName string) *LocalStatsItem {
		key := day + "/" + modelName
		if it, ok := items[key]; ok {
			return it
		}
		it := &LocalStatsItem{Day: day, Model: modelName}
		items[key] = it
		return it
	}

	rawRoot := filepath.Join(root, rawDirName)
	err := filepath.WalkDir(rawRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if !d.Type().IsRegular() || !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(rawRoot, path)
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) != 3 {
			return nil
		}
		it := item(parts[0], parts[1])
		it.RawFiles++
		it.RawBytes += info.Size()
		stats.RawFiles++
		stats.RawBytes += info.Size()
		return nil
	})
	if err != nil {
		return nil, err
	}

	uploadedAt := make(map[string]int64, len(uploads))
	for _, u := range uploads {
		key := uploadRecordKey(u)
		if u.FinishedAt > uploadedAt[key] {
			uploadedAt[key] = u.FinishedAt
		}
	}
	pkgs, err := listPackages(root)
	if err != nil {
		return nil, err
	}
	for _, p := range pkgs {
		it := item(p.day, p.model)
		it.Packaged = true
		it.PackageBytes = p.size
		it.PackageFiles = countPackageEntries(p.path)
		stats.PackageCount++
		stats.PackageBytes += p.size
		if at, ok := uploadedAt[p.uploadKey()]; ok {
			it.Uploaded = true
			it.UploadedAt = at
			stats.UploadedCount++
			stats.UploadedBytes += p.size
		}
	}

	for _, dir := range legacyDirs(root) {
		_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
			if err != nil || !d.Type().IsRegular() {
				return nil
			}
			if info, err := d.Info(); err == nil {
				stats.LegacyFiles++
				stats.LegacyBytes += info.Size()
			}
			return nil
		})
	}

	stats.PendingUploadBytes = stats.RawBytes + stats.PackageBytes - stats.UploadedBytes
	stats.TotalBytes = stats.RawBytes + stats.PackageBytes + stats.LegacyBytes
	for _, it := range items {
		stats.Items = append(stats.Items, *it)
	}
	sort.Slice(stats.Items, func(i, j int) bool {
		if stats.Items[i].Day != stats.Items[j].Day {
			return stats.Items[i].Day > stats.Items[j].Day
		}
		return stats.Items[i].Model < stats.Items[j].Model
	})
	return stats, nil
}

// legacyDirs lists <root>/<day> directories written by the old real-time layout.
func legacyDirs(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && isDayName(e.Name()) {
			dirs = append(dirs, filepath.Join(root, e.Name()))
		}
	}
	return dirs
}

// DeleteAllLocal removes raw records, packages and legacy day directories. It
// never removes root itself or anything else in it, because local_dir is
// operator-configurable and may point at a shared directory.
func DeleteAllLocal(root string) error {
	targets := append([]string{
		filepath.Join(root, rawDirName),
		filepath.Join(root, packagesDirName),
	}, legacyDirs(root)...)
	for _, dir := range targets {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	return nil
}
