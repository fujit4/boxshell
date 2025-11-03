package storage

import (
	"boxshell/internal/boxapi"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FindLatestDownloadedFile はダウンロードキャッシュから指定された名前の最新のファイルを検索します。
func FindLatestDownloadedFile(name string) (string, error) {
	baseDir := os.Getenv("XDG_DATA_HOME")
	if baseDir == "" {
		baseDir = os.Getenv("LOCALAPPDATA")
		if baseDir == "" {
			return "", fmt.Errorf("neither XDG_DATA_HOME nor LOCALAPPDATA are set")
		}
	}
	downloadDir := filepath.Join(baseDir, "boxshell")

	var latestFile string
	var latestTimestamp string

	err := filepath.Walk(downloadDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Name() == name {
			parts := strings.Split(filepath.ToSlash(path), "/")
			if len(parts) >= 3 {
				timestamp := parts[len(parts)-2]
				if latestTimestamp == "" || timestamp > latestTimestamp {
					latestTimestamp = timestamp
					latestFile = path
				}
			}
		}
		return nil
	})

	if err != nil {
		return "", err
	}

	if latestFile == "" {
		return "", fmt.Errorf("file not found in download cache: %s", name)
	}

	return latestFile, nil
}

// Download は Box からファイルをダウンロードして、ローカルストレージに保存します。
func Download(ctx context.Context, client *boxapi.Client, fileID, fileName, localDir string) (string, error) {
	var destPath string

	if localDir != "" {
		// -l が指定されている場合、そのディレクトリに直接ファイルを保存
		if err := os.MkdirAll(localDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create destination directory: %w", err)
		}
		destPath = filepath.Join(localDir, fileName)
	} else {
		// -l がない場合、従来のXDG/LOCALAPPDATAパスを使用
		baseDir := os.Getenv("XDG_DATA_HOME")
		if baseDir == "" {
			baseDir = os.Getenv("LOCALAPPDATA")
			if baseDir == "" {
				return "", fmt.Errorf("neither XDG_DATA_HOME nor LOCALAPPDATA are set")
			}
		}
		timestamp := strings.Replace(time.Now().Format("20060102150405.000"), ".", "", 1)
		destDir := filepath.Join(baseDir, "boxshell", fileID, timestamp)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create destination directory: %w", err)
		}
		destPath = filepath.Join(destDir, fileName)
	}

	// Box からファイルをダウンロード
	body, err := client.DownloadFile(ctx, fileID)
	if err != nil {
		return "", fmt.Errorf("failed to download file from box: %w", err)
	}
	defer body.Close()

	// ファイルをローカルに保存
	file, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("failed to create local file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, body); err != nil {
		return "", fmt.Errorf("failed to save file content: %w", err)
	}

	return destPath, nil
}
