package shell

import (
	"bufio"
	"boxshell/internal/boxapi"
	"boxshell/internal/config"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Shell は REPL の状態を管理します。
type Shell struct {
	boxClient        *boxapi.Client
	config           *config.Config
	currentBoxDirID  string
	currentBoxPath   string // 表示用のフォーマット済みパス
	canonicalBoxPath string // 内部処理用の正規パス (例: /foo/bar)
	lastLsItems      []boxapi.Item
}

// formatPath は正規パスを現在のパスモードに合わせてフォーマットします。
func (sh *Shell) formatPath(canonicalPath string) string {
	if sh.config.PathMode == config.ModeWindows {
		if canonicalPath == "/" {
			return "Z:\\"
		}
		// "/" を "\\" に置換し、先頭に "Z:" を付ける
		return "Z:" + strings.ReplaceAll(canonicalPath, "/", "\\")
	}
	// linuxモード
	return canonicalPath
}

// Run は REPL を開始します。
func Run(ctx context.Context, boxClient *boxapi.Client) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	sh := &Shell{
		boxClient:       boxClient,
		config:          cfg,
		currentBoxDirID: "0", // ルートから開始
	}

	// 初期カレントディレクトリの情報を取得
	if err := sh.updateCurrentBoxDirInfo(ctx); err != nil {
		return err
	}

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Printf("box:%s> ", sh.currentBoxPath)

		if !scanner.Scan() {
			break
		}

		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		command := parts[0]
		args := parts[1:]

		switch command {
		case "exit":
			return nil
		case "pwd":
			fmt.Println(sh.currentBoxPath)
		case "ls":
			items, err := sh.boxClient.GetFolderItems(ctx, sh.currentBoxDirID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				sh.lastLsItems = nil // エラー時は前回結果をクリア
			} else {
				sh.lastLsItems = items // 結果を保存
				for i, item := range items {
					icon := "[f]"
					if item.Type == "folder" {
						icon = "[d]"
					}
					fmt.Printf("%d %s %s\n", i+1, icon, item.Name)
				}
			}
		case "cd":
			if len(args) == 0 {
				continue
			}

			// `cd -n <number>` のロジック
			if len(args) == 2 && args[0] == "-n" {
				num, err := strconv.Atoi(args[1])
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: Invalid number format\n")
					continue
				}
				if sh.lastLsItems == nil {
					fmt.Fprintf(os.Stderr, "Error: 'ls' must be run first to use numbered navigation\n")
					continue
				}
				if num < 1 || num > len(sh.lastLsItems) {
					fmt.Fprintf(os.Stderr, "Error: Number out of range\n")
					continue
				}

				targetItem := sh.lastLsItems[num-1]
				if targetItem.Type != "folder" {
					fmt.Fprintf(os.Stderr, "Error: Item %d is not a directory\n", num)
					continue
				}

				if err := sh.changeBoxDir(ctx, targetItem.Name); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				}
				continue // このコマンドの処理は完了
			}

			// 既存のパス指定のcdロジック
			targetPath := args[0]
			// Windowsモードの場合、パスを正規化する
			if sh.config.PathMode == config.ModeWindows {
				targetPath = strings.ReplaceAll(targetPath, "\\", "/")
				// ドライブ文字を削除 (例: Z:/foo -> /foo)
				if len(targetPath) >= 2 && targetPath[1] == ':' {
					targetPath = targetPath[2:]
				}
			}

			if err := sh.changeBoxDir(ctx, targetPath); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
		case "pathmode":
			if len(args) == 0 {
				fmt.Printf("Current path mode: %s\n", sh.config.PathMode)
				continue
			}
			mode := strings.ToLower(args[0])
			if mode == config.ModeLinux || mode == config.ModeWindows {
				sh.config.PathMode = mode
				if err := sh.config.Save(); err != nil {
					fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
				} else {
					fmt.Printf("Path mode switched to %s\n", mode)
					// プロンプト表示を更新
					sh.currentBoxPath = sh.formatPath(sh.canonicalBoxPath)
				}
			} else {
				fmt.Fprintf(os.Stderr, "Invalid mode. Use 'linux' or 'windows'.\n")
			}
		default:
			fmt.Printf("Unknown command: %s\n", command)
		}
	}

	return scanner.Err()
}

func (sh *Shell) updateCurrentBoxDirInfo(ctx context.Context) error {
	if sh.currentBoxDirID == "0" {
		sh.canonicalBoxPath = "/"
		sh.currentBoxPath = sh.formatPath(sh.canonicalBoxPath)
		return nil
	}

	folder, err := sh.boxClient.GetFolder(ctx, sh.currentBoxDirID)
	if err != nil {
		return fmt.Errorf("failed to get current folder info: %w", err)
	}

	var pathParts []string
	for _, entry := range folder.PathCollection.Entries {
		if entry.ID == "0" { // ルートフォルダはスキップ
			continue
		}
		pathParts = append(pathParts, entry.Name)
	}
	pathParts = append(pathParts, folder.Name)

	sh.canonicalBoxPath = "/" + strings.Join(pathParts, "/")
	sh.currentBoxPath = sh.formatPath(sh.canonicalBoxPath)
	return nil
}

// changeBoxDir は正規パス（/区切り）を元にディレクトリを変更します。
func (sh *Shell) changeBoxDir(ctx context.Context, target string) error {
	originalDirID := sh.currentBoxDirID
	currentDirID := sh.currentBoxDirID

	path := target
	if strings.HasPrefix(path, "/") {
		currentDirID = "0"
		path = strings.TrimPrefix(path, "/")
	}

	if path == "" {
		sh.currentBoxDirID = currentDirID
		if err := sh.updateCurrentBoxDirInfo(ctx); err != nil {
			sh.currentBoxDirID = originalDirID
			return err
		}
		return nil
	}

	parts := strings.Split(path, "/")

	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}

		if part == ".." {
			if currentDirID == "0" {
				continue
			}
			folder, err := sh.boxClient.GetFolder(ctx, currentDirID)
			if err != nil {
				return err
			}
			if len(folder.PathCollection.Entries) > 1 {
				parent := folder.PathCollection.Entries[len(folder.PathCollection.Entries)-2]
				currentDirID = parent.ID
			} else {
				currentDirID = "0"
			}
			continue
		}

		items, err := sh.boxClient.GetFolderItems(ctx, currentDirID)
		if err != nil {
			return err
		}

		found := false
		for _, item := range items {
			if item.Type == "folder" && item.Name == part {
				currentDirID = item.ID
				found = true

				break
			}
		}

		if !found {
			return fmt.Errorf("directory not found: %s", part)
		}
	}

	sh.currentBoxDirID = currentDirID
	if err := sh.updateCurrentBoxDirInfo(ctx); err != nil {
		sh.currentBoxDirID = originalDirID
		return err
	}

	return nil
}
