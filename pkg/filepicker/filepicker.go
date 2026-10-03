package filepicker

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/annenpolka/cclog/internal/domain"
	"github.com/annenpolka/cclog/internal/formatter"
	"github.com/annenpolka/cclog/internal/parser"
)

type FileInfo struct {
	Name              string
	Path              string
	IsDir             bool
	Size              int64
	ModTime           time.Time
	ConversationTitle string
	ProjectName       string
}

func (f FileInfo) FilterValue() string {
	return f.Name
}

func (f FileInfo) Title() string {
	if f.IsDir {
		return f.Name + "/"
	}

	// For JSONL files, display "date [project] title" format
	if filepath.Ext(f.Name) == ".jsonl" {
		dateStr := f.ModTime.Format("2006-01-02 15:04")

		// Add project name if available
		var projectPart string
		if f.ProjectName != "" {
			projectPart = " [" + f.ProjectName + "]"
		}

		// Add conversation title if available
		if f.ConversationTitle != "" {
			return dateStr + projectPart + " " + f.ConversationTitle
		}

		// If no title but has project name, show date [project]
		if f.ProjectName != "" {
			return dateStr + projectPart
		}

		return dateStr
	}

	return f.Name
}

func (f FileInfo) Description() string {
	// Return empty string for clean display - date is shown in Title for JSONL files
	return ""
}

func GetFiles(dir string) ([]FileInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []FileInfo

	// Add parent directory entry if not at root
	absDir, err := filepath.Abs(dir)
	if err == nil {
		parentDir := filepath.Dir(absDir)
		// Only add ".." if not at root and parent is different
		if parentDir != absDir && parentDir != "." {
			// Get actual modification time for parent directory
			var parentModTime time.Time
			if parentStat, err := os.Stat(parentDir); err == nil {
				parentModTime = parentStat.ModTime()
			}

			parentInfo := FileInfo{
				Name:    "..",
				Path:    parentDir,
				IsDir:   true,
				Size:    0,
				ModTime: parentModTime,
			}
			files = append(files, parentInfo)
		}
	}

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		fileInfo := FileInfo{
			Name:    entry.Name(),
			Path:    filepath.Join(dir, entry.Name()),
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		}

		// Extract conversation title and project name for JSONL files
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".jsonl" {
			title, projectName := extractConversationInfo(fileInfo.Path)
			// Skip empty files (when title extraction fails due to empty file)
			if title == "" {
				continue
			}
			fileInfo.ConversationTitle = title
			fileInfo.ProjectName = projectName
		}
		files = append(files, fileInfo)
	}

	// Sort files by modification time (newest first)
	// Keep parent directory at the beginning if it exists
	var parentDir *FileInfo
	var regularFiles []FileInfo

	for i, file := range files {
		if file.Name == ".." {
			parentDir = &files[i]
		} else {
			regularFiles = append(regularFiles, file)
		}
	}

	// Sort regular files by modification time (newest first)
	sort.Slice(regularFiles, func(i, j int) bool {
		return regularFiles[i].ModTime.After(regularFiles[j].ModTime)
	})

	// Rebuild files slice with parent directory first (if exists)
	var sortedFiles []FileInfo
	if parentDir != nil {
		sortedFiles = append(sortedFiles, *parentDir)
	}
	sortedFiles = append(sortedFiles, regularFiles...)

	return sortedFiles, nil
}

// extractConversationInfo extracts title and project name from JSONL conversation file
func extractConversationInfo(filePath string) (string, string) {
	// Parse the JSONL file to extract conversation information
	log, err := parser.ParseJSONLFile(filePath)
	if err != nil {
		return "", ""
	}

	// Skip empty files - return empty string to indicate this file should be filtered out
	if len(log.Messages) == 0 {
		return "", ""
	}

	// Extract project name from CWD field of the first message that has one
	var projectName string
	for _, msg := range log.Messages {
		if msg.CWD != "" {
			projectName = extractProjectName(msg.CWD)
			break
		}
	}

	// Apply filtering using shared formatter API
	filteredLog := formatter.FilterConversationLog(log, true)

	// Skip files with no meaningful messages after filtering
	if len(filteredLog.Messages) == 0 {
		return "", ""
	}

	// Prefer a user-assigned custom title (dropped by filtering, so read from the raw log)
	title := domain.ExtractCustomTitle(log)
	if title == "" {
		title = domain.ExtractTitle(filteredLog)
	}
	return title, projectName
}

// extractConversationTitle extracts title from JSONL conversation file (backward compatibility)
func extractConversationTitle(filePath string) string {
	title, _ := extractConversationInfo(filePath)
	return title
}

// (removed) extractMessageContent: prefer formatter.ExtractMessageContent

// GetFilesRecursive recursively collects all .jsonl files from a directory and its subdirectories
func GetFilesRecursive(rootDir string) ([]FileInfo, error) {
	var allFiles []FileInfo

	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Only include .jsonl files
		if filepath.Ext(d.Name()) != ".jsonl" {
			return nil
		}

		// Get file info for modification time
		info, err := d.Info()
		if err != nil {
			return err
		}

		fileInfo := FileInfo{
			Name:    d.Name(),
			Path:    path,
			IsDir:   false,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		}

		// Extract conversation title and project name for JSONL files
		title, projectName := extractConversationInfo(path)
		// Skip empty files (when title extraction fails due to empty file)
		if title == "" {
			return nil
		}
		fileInfo.ConversationTitle = title
		fileInfo.ProjectName = projectName

		allFiles = append(allFiles, fileInfo)
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort by modification time (newest first)
	sort.Slice(allFiles, func(i, j int) bool {
		return allFiles[i].ModTime.After(allFiles[j].ModTime)
	})

	return allFiles, nil
}

// extractProjectName extracts project name from cwd path
func extractProjectName(cwd string) string {
	if cwd == "" || cwd == "/" {
		return ""
	}

	// Clean the path and get the base name
	cleanPath := filepath.Clean(cwd)
	projectName := filepath.Base(cleanPath)

	// Return empty string if it's root or dot
	if projectName == "/" || projectName == "." {
		return ""
	}

	return projectName
}

// SearchInConversation はメッセージ内容から検索クエリをマッチングする
func SearchInConversation(query string, messages []domain.Message) bool {
	if query == "" {
		return true
	}

	lowerQuery := strings.ToLower(query)

	for _, msg := range messages {
		// Only search within the textual message content, not metadata
		content := formatter.ExtractMessageContent(msg.Message)
		if strings.Contains(strings.ToLower(content), lowerQuery) {
			return true
		}
	}

	return false
}

// FilterFilesBySearch は検索クエリに基づいてファイルをフィルタリングする
func FilterFilesBySearch(query string, files []FileInfo) []FileInfo {
	if query == "" {
		return files
	}

	var result []FileInfo

	for _, file := range files {
		// ディレクトリは常に含める
		if file.IsDir {
			result = append(result, file)
			continue
		}

		// JSONLファイル以外はスキップ
		if !strings.HasSuffix(file.Name, ".jsonl") {
			continue
		}

		// JSONLファイルを解析して検索
		conversationLog, err := parser.ParseJSONLFile(file.Path)
		if err != nil {
			continue
		}

		if SearchInConversation(query, conversationLog.Messages) {
			result = append(result, file)
		}
	}

	return result
}
