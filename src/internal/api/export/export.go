package export

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"huepattl.de/unterlumen/internal/api/sse"
	"huepattl.de/unterlumen/internal/jobs"
	lib "huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/media"
	"huepattl.de/unterlumen/internal/pathguard"
)

// zipJobs holds temp ZIP files keyed by a random token.
// Entries are removed on download or after 10 minutes.
var (
	zipJobsMu sync.Mutex
	zipJobs   = make(map[string]string) // token → temp file path
)

type exportRequest struct {
	Files       []string           `json:"files"`
	Format      string             `json:"format"`
	Quality     int                `json:"quality"`
	Scale       media.ScaleOptions `json:"scale"`
	ExifMode    string             `json:"exifMode"`
	Destination string             `json:"destination"`
	SourcePath  string             `json:"sourcePath,omitempty"`
	// ZIP only: whole folders, and library photos by ID (filter results).
	Dirs   []string   `json:"dirs,omitempty"`
	Photos []photoRef `json:"photos,omitempty"`
}

type estimateRequest struct {
	Files      []string           `json:"files"`
	Format     string             `json:"format"`
	Quality    int                `json:"quality"`
	Scale      media.ScaleOptions `json:"scale"`
	Method     string             `json:"method"` // "heuristic" or "encode"
	SourcePath string             `json:"sourcePath,omitempty"`
}

type estimateEntry struct {
	File        string `json:"file"`
	InputBytes  int64  `json:"inputBytes"`
	OutputBytes int64  `json:"outputBytes"`
	OrigWidth   int    `json:"origWidth"`
	OrigHeight  int    `json:"origHeight"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Error       string `json:"error,omitempty"`
}

type estimateResponse struct {
	Estimates []estimateEntry `json:"estimates"`
}

type exportResult struct {
	File    string `json:"file"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type exportSaveResponse struct {
	Results []exportResult `json:"results"`
}

type zipStreamEvent struct {
	File     string `json:"file,omitempty"`
	Done     int    `json:"done"`
	Total    int    `json:"total"`
	Complete bool   `json:"complete,omitempty"`
	Token    string `json:"token,omitempty"`
	Error    string `json:"error,omitempty"`
}

// Handle registers all /api/export/* routes on mux.
// libs may be nil when library support is off.
func Handle(mux *http.ServeMux, root string, serverRole bool, reg *jobs.Registry, libs *lib.Manager) {
	mux.HandleFunc("/api/export/estimate", handleExportEstimate(root, serverRole))
	mux.HandleFunc("/api/export/zip", handleExportZip(root, serverRole, libs))
	mux.HandleFunc("/api/export/zip-stream", handleExportZipStream(root, serverRole, reg, libs))
	mux.HandleFunc("/api/export/zip-download", handleExportZipDownload())
	mux.HandleFunc("/api/export/save", handleExportSave(root, serverRole))
	if !serverRole {
		mux.HandleFunc("/api/export/folder-picker", handleFolderPicker())
	}
}

func handleExportEstimate(root string, serverRole bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req estimateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		opts := media.ExportOptions{Format: req.Format, Quality: req.Quality, Scale: req.Scale}
		eRoot := effectiveRoot(root, req.SourcePath, serverRole)
		var estimates []estimateEntry
		for _, relPath := range req.Files {
			absPath, ok := resolveFilePath(eRoot, serverRole, relPath)
			if !ok {
				estimates = append(estimates, estimateEntry{File: relPath})
				continue
			}
			var entry estimateEntry
			if req.Method == "encode" {
				entry = estimateEncode(relPath, absPath, opts)
			} else {
				entry = estimateHeuristic(relPath, absPath, opts)
			}
			estimates = append(estimates, entry)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(estimateResponse{Estimates: estimates})
	}
}

func estimateEncode(relPath, absPath string, opts media.ExportOptions) estimateEntry {
	data, err := media.ExportImage(absPath, opts)
	if err != nil {
		return estimateEntry{File: relPath, Error: err.Error()}
	}
	info, _ := os.Stat(absPath)
	var inputBytes int64
	if info != nil {
		inputBytes = info.Size()
	}
	origW, origH := media.GetSourceDims(absPath)
	_, _, _, _, outW, outH, _ := media.EstimateSize(absPath, opts)
	return estimateEntry{
		File:        relPath,
		InputBytes:  inputBytes,
		OutputBytes: int64(len(data)),
		OrigWidth:   origW,
		OrigHeight:  origH,
		Width:       outW,
		Height:      outH,
	}
}

func estimateHeuristic(relPath, absPath string, opts media.ExportOptions) estimateEntry {
	in, out, origW, origH, outW, outH, err := media.EstimateSize(absPath, opts)
	if err != nil {
		return estimateEntry{File: relPath, Error: err.Error()}
	}
	return estimateEntry{
		File:        relPath,
		InputBytes:  in,
		OutputBytes: out,
		OrigWidth:   origW,
		OrigHeight:  origH,
		Width:       outW,
		Height:      outH,
	}
}

func handleExportZip(root string, serverRole bool, libs *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req exportRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		opts := exportOpts(req)
		sources := zipSources{root: effectiveRoot(root, req.SourcePath, serverRole), serverRole: serverRole, libs: libs}
		items, _ := sources.collect(req)
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="export.zip"`)

		zw := zip.NewWriter(w)
		defer zw.Close()

		names := map[string]int{}
		for _, item := range items {
			addZipEntry(zw, item, req.Format, opts, names)
		}
	}
}

func handleExportSave(root string, serverRole bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req exportRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if req.Destination == "" {
			http.Error(w, "destination is required", http.StatusBadRequest)
			return
		}

		var destAbs string
		if filepath.IsAbs(req.Destination) {
			if serverRole {
				http.Error(w, "absolute destination paths not allowed in server mode", http.StatusBadRequest)
				return
			}
			info, err := os.Stat(req.Destination)
			if err != nil || !info.IsDir() {
				http.Error(w, "destination directory does not exist", http.StatusBadRequest)
				return
			}
			destAbs = req.Destination
		} else {
			var ok bool
			destAbs, ok = pathguard.SafePath(root, req.Destination)
			if !ok {
				http.Error(w, "invalid destination path", http.StatusBadRequest)
				return
			}
			info, err := os.Stat(destAbs)
			if err != nil || !info.IsDir() {
				http.Error(w, "destination directory does not exist", http.StatusBadRequest)
				return
			}
		}

		opts := exportOpts(req)
		results := processExportBatch(effectiveRoot(root, req.SourcePath, serverRole), req.Files, destAbs, req.Format, opts, serverRole)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(exportSaveResponse{Results: results})
	}
}

func processExportBatch(root string, files []string, dest, format string, opts media.ExportOptions, serverRole bool) []exportResult {
	var results []exportResult
	for _, relPath := range files {
		absPath, ok := resolveFilePath(root, serverRole, relPath)
		if !ok {
			results = append(results, exportResult{File: relPath, Error: "invalid path"})
			continue
		}
		data, err := media.ExportImage(absPath, opts)
		if err != nil {
			results = append(results, exportResult{File: relPath, Error: err.Error()})
			continue
		}
		outPath := filepath.Join(dest, media.ExportedName(filepath.Base(relPath), format))
		if err := os.WriteFile(outPath, data, 0644); err != nil {
			results = append(results, exportResult{File: relPath, Error: err.Error()})
			continue
		}
		results = append(results, exportResult{File: relPath, Success: true})
	}
	return results
}

func handleExportZipStream(root string, serverRole bool, reg *jobs.Registry, libs *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req exportRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		flusher, ok := sse.Start(w)
		if !ok {
			return
		}

		sources := zipSources{root: effectiveRoot(root, req.SourcePath, serverRole), serverRole: serverRole, libs: libs}
		items, refused := sources.collect(req)
		n := len(items)
		label := fmt.Sprintf("Exporting %d photo%s as a ZIP", n, plural(n))
		if req.Format == FormatOriginal {
			label = fmt.Sprintf("Packing %d original%s into a ZIP", n, plural(n))
		}
		job := reg.Start("export", label, "")
		// Ends as failed if the client goes away before the ZIP is ready.
		defer job.Finish(errors.New("it stopped before it finished"))
		send := reportingSSEWriter(sseWriter(w, flusher), job)
		opts := exportOpts(req)

		for _, name := range refused {
			send(zipStreamEvent{File: name, Total: n, Error: "not in a folder Unterlumen may read"})
		}
		tmpPath, err := buildZipFile(r.Context(), items, req.Format, opts, send)
		if err != nil {
			return // buildZipFile already sent the error event or client disconnected
		}

		token := generateToken()
		zipJobsMu.Lock()
		zipJobs[token] = tmpPath
		zipJobsMu.Unlock()

		scheduleZipExpiry(token, tmpPath)
		send(zipStreamEvent{Done: n, Total: n, Complete: true, Token: token})
	}
}

func buildZipFile(ctx context.Context, items []zipItem, format string, opts media.ExportOptions, send func(zipStreamEvent)) (string, error) {
	// An empty ZIP would look like success; say what happened instead.
	if len(items) == 0 {
		err := errors.New("none of the photos could be read")
		send(zipStreamEvent{Error: err.Error()})
		return "", err
	}
	tmpFile, err := os.CreateTemp("", "unterlumen-zip-*.zip")
	if err != nil {
		send(zipStreamEvent{Error: err.Error()})
		return "", err
	}
	tmpPath := tmpFile.Name()

	zw := zip.NewWriter(tmpFile)
	total := len(items)
	names := map[string]int{}

	for i, item := range items {
		select {
		case <-ctx.Done():
			zw.Close()
			tmpFile.Close()
			os.Remove(tmpPath)
			return "", ctx.Err()
		default:
		}

		send(zipStreamEvent{File: path.Base(item.name), Done: i, Total: total})
		if err := addZipEntry(zw, item, format, opts, names); err != nil {
			send(zipStreamEvent{File: path.Base(item.name), Done: i + 1, Total: total, Error: err.Error()})
		}
	}

	zw.Close()
	tmpFile.Close()
	return tmpPath, nil
}

// FormatOriginal packs the files as they are: no conversion, no scaling,
// every byte of metadata kept.
const FormatOriginal = "original"

// addZipEntry writes one photo into the ZIP: converted by the export
// options, or unchanged for FormatOriginal. names keeps entry names unique,
// since a selection from several folders can hold two IMG_0001.JPG.
func addZipEntry(zw *zip.Writer, item zipItem, format string, opts media.ExportOptions, names map[string]int) error {
	if format == FormatOriginal {
		return addOriginal(zw, item.abs, uniqueEntryName(names, item.name))
	}
	data, err := media.ExportImage(item.abs, opts)
	if err != nil {
		return err
	}
	fw, err := zw.Create(uniqueEntryName(names, media.ExportedName(item.name, format)))
	if err != nil {
		return err
	}
	_, err = fw.Write(data)
	return err
}

// addOriginal stores the file without compressing it again: photos are
// compressed already, and storing is fast.
func addOriginal(zw *zip.Writer, absPath, name string) error {
	f, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	fw, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store, Modified: info.ModTime()})
	if err != nil {
		return err
	}
	_, err = io.Copy(fw, f)
	return err
}

// uniqueEntryName returns name, or "name (2).ext" and so on when it is taken.
func uniqueEntryName(names map[string]int, name string) string {
	names[name]++
	n := names[name]
	if n == 1 {
		return name
	}
	ext := filepath.Ext(name)
	return uniqueEntryName(names, fmt.Sprintf("%s (%d)%s", strings.TrimSuffix(name, ext), n, ext))
}

func scheduleZipExpiry(token, path string) {
	go func() {
		time.Sleep(10 * time.Minute)
		zipJobsMu.Lock()
		if p, exists := zipJobs[token]; exists {
			os.Remove(p)
			delete(zipJobs, token)
		}
		zipJobsMu.Unlock()
	}()
}

func handleExportZipDownload() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "missing token", http.StatusBadRequest)
			return
		}

		zipJobsMu.Lock()
		path, ok := zipJobs[token]
		if ok {
			delete(zipJobs, token)
		}
		zipJobsMu.Unlock()

		if !ok {
			http.Error(w, "token not found or expired", http.StatusNotFound)
			return
		}
		defer os.Remove(path)

		f, err := os.Open(path)
		if err != nil {
			http.Error(w, "could not open ZIP", http.StatusInternalServerError)
			return
		}
		defer f.Close()

		if info, err := f.Stat(); err == nil {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": zipDownloadName(r)}))
		io.Copy(w, f)
	}
}

// zipDownloadName is the ?name= the page asked for — a plain file name
// ending in .zip — or export.zip.
func zipDownloadName(r *http.Request) string {
	name := filepath.Base(r.URL.Query().Get("name"))
	if name == "." || name == "/" || !strings.HasSuffix(strings.ToLower(name), ".zip") {
		return "export.zip"
	}
	return name
}

func handleFolderPicker() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path, err := openFolderPicker()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"path": path})
	}
}

// openFolderPicker invokes the OS-native folder chooser dialog and returns the
// selected path, or "" if the user cancelled. Returns an error only when no
// picker is available on the current platform.
func openFolderPicker() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("osascript", "-e", "POSIX path of (choose folder)").Output()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
				return "", nil // user cancelled
			}
			return "", err
		}
		return strings.TrimRight(string(out), "\n"), nil
	case "linux":
		for _, tool := range [][]string{
			{"zenity", "--file-selection", "--directory"},
			{"kdialog", "--getexistingdirectory"},
		} {
			if _, err := exec.LookPath(tool[0]); err != nil {
				continue
			}
			out, err := exec.Command(tool[0], tool[1:]...).Output()
			if err != nil {
				return "", nil // user cancelled
			}
			return strings.TrimRight(string(out), "\n"), nil
		}
		return "", fmt.Errorf("no folder picker available; install zenity or kdialog")
	default:
		return "", fmt.Errorf("folder picker not supported on %s", runtime.GOOS)
	}
}

// effectiveRoot returns sourcePath when it is a valid existing directory,
// falling back to browseRoot otherwise. This allows library-mode exports whose
// file paths are relative to the library source (which may differ from the
// browse root) to resolve correctly.
// effectiveRoot is the folder the request's relative paths start from: a
// library's source folder when one is named, else the browse root. In
// server mode the source folder must lie inside the browse root — the page
// sends it, and any folder it named would otherwise be readable.
func effectiveRoot(browseRoot, sourcePath string, serverRole bool) string {
	if sourcePath == "" {
		return browseRoot
	}
	info, err := os.Stat(sourcePath)
	if err != nil || !info.IsDir() {
		return browseRoot
	}
	if serverRole {
		if inside, ok := pathguard.Inside(browseRoot, sourcePath); ok {
			return inside
		}
		return browseRoot
	}
	return sourcePath
}

// resolveFilePath resolves a file for export. Relative paths are validated via
// pathguard against root. Absolute paths are permitted in non-server-role mode
// and validated by checking the file exists on disk.
func resolveFilePath(root string, serverRole bool, filePath string) (string, bool) {
	if filepath.IsAbs(filePath) {
		if serverRole {
			return "", false
		}
		if _, err := os.Stat(filePath); err != nil {
			return "", false
		}
		return filePath, true
	}
	return pathguard.SafePath(root, filePath)
}

func exportOpts(req exportRequest) media.ExportOptions {
	return media.ExportOptions{
		Format:   req.Format,
		Quality:  req.Quality,
		Scale:    req.Scale,
		ExifMode: req.ExifMode,
	}
}

func sseWriter(w http.ResponseWriter, flusher http.Flusher) func(zipStreamEvent) {
	return func(evt zipStreamEvent) {
		sse.Send(w, evt) //nolint:errcheck
		flusher.Flush()
	}
}

// reportingSSEWriter mirrors each export event in the status line.
func reportingSSEWriter(send func(zipStreamEvent), job *jobs.Handle) func(zipStreamEvent) {
	return func(evt zipStreamEvent) {
		send(evt)
		switch {
		case evt.Error != "":
			job.Finish(errors.New(evt.Error))
		case evt.Complete:
			job.Finish(nil)
		default:
			job.Progress(evt.Done, evt.Total, evt.File)
		}
	}
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func generateToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
