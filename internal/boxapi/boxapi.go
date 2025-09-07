package boxapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	apiURL = "https://api.box.com/2.0"
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
	ID   string `json:"id"`
	Name string `json:"name"`
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
