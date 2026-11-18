package upstreamcapture

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
)

// Local layout under capture_setting.ResolvedLocalDir():
//
//	raw/<day>/<model>/<newapiReqId>_<reqId>.json   records still accumulating
//	packages/<day>/<model>.jsonl                   one package per day and model, one record per line
//	packages/<day>/<model>.jsonl.count             number of records in the package
//	packages/<day>/<model>.zip                     legacy compressed package (stats/delete only)
//	<day>/channel_<id>/...                         legacy layout (uploaded in real time)
const (
	rawDirName      = "raw"
	packagesDirName = "packages"
	dayLayout       = "2006-01-02"
	jsonlExt        = ".jsonl"
	legacyZipExt    = ".zip"
	countFileSuffix = ".count"
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

// PackEndedDays appends every raw <day>/<model> directory of a finished day to
// packages/<day>/<model>.jsonl and removes the packed raw files. Records that
// finish after the day was packed are appended to the same package.
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

// packGroup appends one raw directory to its package and returns how many
// records went into it. Packages grow to tens of GB uncompressed, so they are
// appended in place instead of rebuilt; on any failure the package is cut back
// to its previous size so it never ends with a partial line.
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

	pkgPath := filepath.Join(root, packagesDirName, g.day, g.model+jsonlExt)
	if err := os.MkdirAll(filepath.Dir(pkgPath), 0o755); err != nil {
		return 0, err
	}
	out, err := os.OpenFile(pkgPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, err
	}
	defer out.Close()
	info, err := out.Stat()
	if err != nil {
		return 0, err
	}
	previousSize := info.Size()
	previousCount := readPackageCount(pkgPath)

	err = appendRecords(out, g.dir, names)
	if err == nil {
		err = writePackageCount(pkgPath, previousCount+len(names))
	}
	if err != nil {
		_ = out.Truncate(previousSize)
		return 0, err
	}
	for _, name := range names {
		if err := os.Remove(filepath.Join(g.dir, name)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return len(names), err
		}
	}
	return len(names), nil
}

// appendRecords writes each raw record as one JSONL line. Raw newlines in valid
// JSON can only be insignificant whitespace (newlines inside strings are
// escaped), so replacing them with spaces keeps the record intact.
func appendRecords(out *os.File, rawDir string, names []string) error {
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(rawDir, name))
		if err != nil {
			return err
		}
		data = bytes.TrimSpace(data)
		if len(data) == 0 {
			continue
		}
		data = bytes.ReplaceAll(data, []byte("\r"), []byte(" "))
		data = bytes.ReplaceAll(data, []byte("\n"), []byte(" "))
		if _, err := out.Write(append(data, '\n')); err != nil {
			return err
		}
	}
	return out.Sync()
}

func readPackageCount(pkgPath string) int {
	data, err := os.ReadFile(pkgPath + countFileSuffix)
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func writePackageCount(pkgPath string, n int) error {
	countPath := pkgPath + countFileSuffix
	tmpPath := countPath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(strconv.Itoa(n)), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, countPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

type localPackage struct {
	day, model, path, ext string
	size, mtime           int64
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
			ext := filepath.Ext(f.Name())
			if !f.Type().IsRegular() || (ext != jsonlExt && ext != legacyZipExt) {
				continue
			}
			info, err := f.Info()
			if err != nil {
				continue
			}
			pkgs = append(pkgs, localPackage{
				day:   day.Name(),
				model: strings.TrimSuffix(f.Name(), ext),
				path:  filepath.Join(pkgRoot, day.Name(), f.Name()),
				ext:   ext,
				size:  info.Size(),
				mtime: info.ModTime().UnixNano(),
			})
		}
	}
	return pkgs, nil
}

// countPackageRecords returns how many records a package holds: the count
// sidecar for JSONL packages, the entry count for legacy zip packages.
func countPackageRecords(p localPackage) int {
	if p.ext == jsonlExt {
		return readPackageCount(p.path)
	}
	zr, err := zip.OpenReader(p.path)
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
	// A day may hold both a legacy zip and a JSONL package of late records; the
	// item is uploaded only when every one of its packages is.
	for _, p := range pkgs {
		it := item(p.day, p.model)
		firstPackage := !it.Packaged
		it.Packaged = true
		it.PackageBytes += p.size
		it.PackageFiles += countPackageRecords(p)
		stats.PackageCount++
		stats.PackageBytes += p.size
		at, ok := uploadedAt[p.uploadKey()]
		if !ok {
			it.Uploaded = false
			it.UploadedAt = 0
			continue
		}
		stats.UploadedCount++
		stats.UploadedBytes += p.size
		if firstPackage || it.Uploaded {
			it.Uploaded = true
			it.UploadedAt = max(it.UploadedAt, at)
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
