package storage

import (
	"boxshell/internal/boxapi"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

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
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
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
