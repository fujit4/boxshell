package shell

import (
	"bufio"
	"boxshell/internal/boxapi"
	"context"
	"fmt"
	"os"
	"strings"
)

// Shell は REPL の状態を管理します。
type Shell struct {
	boxClient       *boxapi.Client
	currentBoxDirID string
	currentBoxPath  string
}

// Run は REPL を開始します。
func Run(ctx context.Context, boxClient *boxapi.Client) error {
	sh := &Shell{
		boxClient:       boxClient,
		currentBoxDirID: "0", // ルートから開始
		currentBoxPath:  "/",
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
			} else {
				for _, item := range items {
					fmt.Printf("[%s] %s\n", item.Type, item.Name)
				}
			}
		case "cd":
			if len(args) == 0 {
				// cd の引数がない場合は何もしない（またはルートに戻るなど仕様による）
				continue
			}
			if err := sh.changeBoxDir(ctx, args[0]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
		default:
			fmt.Printf("Unknown command: %s\n", command)
		}
		_ = args // argsを一時的に使用済みとしてマーク
	}

	return scanner.Err()
}

func (sh *Shell) updateCurrentBoxDirInfo(ctx context.Context) error {
	if sh.currentBoxDirID == "0" {
		sh.currentBoxPath = "/"
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

	sh.currentBoxPath = "/" + strings.Join(pathParts, "/")
	return nil
}

func (sh *Shell) changeBoxDir(ctx context.Context, target string) error {
	originalDirID := sh.currentBoxDirID
	currentDirID := sh.currentBoxDirID

	path := target
	if strings.HasPrefix(path, "/") {
		currentDirID = "0"
		path = strings.TrimPrefix(path, "/")
	}

	// パスが空文字列の場合（"cd /" や "cd" の後の空パス部分）、何もしない
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
				continue // ルートより上には行けない
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
		sh.currentBoxDirID = originalDirID // 失敗したら元に戻す
		return err
	}

	return nil
}
