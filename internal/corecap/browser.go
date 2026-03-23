package corecap

import (
	"fmt"

	"github.com/mison/antigravity-go/internal/rpc"
)

// BrowserManager provides a stable wrapper around Core's browser-related RPCs.
type BrowserManager struct {
	client *rpc.Client
}

func NewBrowserManager(client *rpc.Client) *BrowserManager {
	return &BrowserManager{client: client}
}

func (m *BrowserManager) requireClient() error {
	if m == nil || m.client == nil {
		return fmt.Errorf("browser manager is not initialized")
	}
	return nil
}

// List returns all active browser pages from Core.
func (m *BrowserManager) List() (map[string]interface{}, error) {
	if err := m.requireClient(); err != nil {
		return nil, err
	}
	return m.client.ListPages()
}

// Open opens a new URL in the managed browser.
func (m *BrowserManager) Open(url string) (map[string]interface{}, error) {
	if err := m.requireClient(); err != nil {
		return nil, err
	}
	if url == "" {
		return nil, fmt.Errorf("url is required")
	}
	return m.client.OpenUrl(url)
}

// Screenshot captures a screenshot for the target page.
func (m *BrowserManager) Screenshot(pageID string) (map[string]interface{}, error) {
	if err := m.requireClient(); err != nil {
		return nil, err
	}
	if pageID == "" {
		return nil, fmt.Errorf("page_id is required")
	}
	return m.client.CaptureScreenshot(pageID)
}

// Focus brings the target page to the foreground in the managed browser.
func (m *BrowserManager) Focus(pageID string) (map[string]interface{}, error) {
	if err := m.requireClient(); err != nil {
		return nil, err
	}
	if pageID == "" {
		return nil, fmt.Errorf("page_id is required")
	}
	return m.client.FocusUserPage(pageID)
}

// Click clicks the first element matching selector on the target page.
func (m *BrowserManager) Click(pageID, selector string) (map[string]interface{}, error) {
	if err := m.requireClient(); err != nil {
		return nil, err
	}
	if pageID == "" {
		return nil, fmt.Errorf("page_id is required")
	}
	if selector == "" {
		return nil, fmt.Errorf("selector is required")
	}
	return m.client.ClickElement(pageID, selector)
}

// Type enters text into the first element matching selector on the target page.
func (m *BrowserManager) Type(pageID, selector, text string) (map[string]interface{}, error) {
	if err := m.requireClient(); err != nil {
		return nil, err
	}
	if pageID == "" {
		return nil, fmt.Errorf("page_id is required")
	}
	if selector == "" {
		return nil, fmt.Errorf("selector is required")
	}
	return m.client.TypeText(pageID, selector, text)
}

// Scroll scrolls the target page by the given delta.
func (m *BrowserManager) Scroll(pageID string, deltaX, deltaY int) (map[string]interface{}, error) {
	if err := m.requireClient(); err != nil {
		return nil, err
	}
	if pageID == "" {
		return nil, fmt.Errorf("page_id is required")
	}
	return m.client.ScrollPage(pageID, deltaX, deltaY)
}
