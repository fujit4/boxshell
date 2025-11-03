package boxapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

const (
	apiURL     = "https://api.box.com/2.0"
	uploadURL = "https://upload.box.com/api/2.0"
)

// Client は Box API との通信を行います。
type Client struct {
	httpClient *http.Client
}

// NewClient は新しい API クライアントを作成します。
func NewClient(httpClient *http.Client) *Client {
	return &Client{httpClient: httpClient}
}

// Folder は Box のフォルダ情報を表します。
type Folder struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	PathCollection struct {
		Entries []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"entries"`
	} `json:"path_collection"`
	ItemCollection struct {
		Entries []Item `json:"entries"`
	} `json:"item_collection"`
}

// Item はフォルダ内のアイテム（ファイルまたはフォルダ）を表します。
type Item struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
}

// UploadFile は新しいファイルを Box にアップロードします。
func (c *Client) UploadFile(ctx context.Context, folderID, localPath string) (*Item, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open local file: %w", err)
	}
	defer file.Close()

	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	// 属性パート
	attributes := map[string]interface{}{
		"name":   filepath.Base(localPath),
		"parent": map[string]string{"id": folderID},
	}
	attrJSON, err := json.Marshal(attributes)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal attributes: %w", err)
	}
	if err := w.WriteField("attributes", string(attrJSON)); err != nil {
		return nil, err
	}

	// ファイルパート
	fw, err := w.CreateFormFile("file", filepath.Base(localPath))
	if err != nil {
		return nil, err
	}
	if _, err = io.Copy(fw, file); err != nil {
		return nil, err
	}

	w.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/files/content", uploadURL), &b)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("failed to upload file: %s", resp.Status)
	}

	var result struct {
		Entries []Item `json:"entries"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Entries) == 0 {
		return nil, fmt.Errorf("no file info returned after upload")
	}

	return &result.Entries[0], nil
}

// UploadNewVersion は既存のファイルの新バージョンをアップロードします。
func (c *Client) UploadNewVersion(ctx context.Context, fileID, localPath string) (*Item, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open local file: %w", err)
	}
	defer file.Close()

	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	// 属性パート (空で良い)
	if err := w.WriteField("attributes", "{}"); err != nil {
		return nil, err
	}

	// ファイルパート
	fw, err := w.CreateFormFile("file", filepath.Base(localPath))
	if err != nil {
		return nil, err
	}
	if _, err = io.Copy(fw, file); err != nil {
		return nil, err
	}

	w.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/files/%s/content", uploadURL, fileID), &b)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("failed to upload new version: %s", resp.Status)
	}

	var result struct {
		Entries []Item `json:"entries"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Entries) == 0 {
		return nil, fmt.Errorf("no file info returned after upload")
	}

	return &result.Entries[0], nil
}

// DownloadFile は指定された fileID のファイルコンテンツを取得します。
func (c *Client) DownloadFile(ctx context.Context, fileID string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/files/%s/content", apiURL, fileID), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("failed to download file: %s", resp.Status)
	}

	return resp.Body, nil
}

// GetFolder は指定されたフォルダIDの詳細を取得します。
func (c *Client) GetFolder(ctx context.Context, folderID string) (*Folder, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/folders/%s", apiURL, folderID), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get folder: %s", resp.Status)
	}

	var folder Folder
	if err := json.NewDecoder(resp.Body).Decode(&folder); err != nil {
		return nil, err
	}

	return &folder, nil
}

// GetFolderItems は指定されたフォルダID配下のアイテム一覧を取得します。
func (c *Client) GetFolderItems(ctx context.Context, folderID string) ([]Item, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/folders/%s/items", apiURL, folderID), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get folder items: %s", resp.Status)
	}

	var itemCollection struct {
		Entries []Item `json:"entries"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&itemCollection); err != nil {
		return nil, err
	}

	return itemCollection.Entries, nil
}

// User は Box のユーザー情報を表します。
type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Login string `json:"login"`
}

// GetMe は現在のユーザー情報を取得します。
func (c *Client) GetMe(ctx context.Context) (*User, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/users/me", apiURL), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get user info: %s", resp.Status)
	}

	var user User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	return &user, nil
}
