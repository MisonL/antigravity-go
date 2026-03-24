package corecap

import (
	"github.com/mison/antigravity-go/internal/rpc"
)

// BrowserManager provides a stable wrapper around Core's browser-related RPCs.
type BrowserManager struct {
	client *rpc.Client
}

func NewBrowserManager(client *rpc.Client) *BrowserManager {
	return &BrowserManager{client: client}
}

// List returns all active browser pages from Core.
func (m *BrowserManager) List() (map[string]interface{}, error) {
	return withManagerClient("browser manager", m, func(client *rpc.Client) (map[string]interface{}, error) {
		return client.ListPages()
	})
}

// Open opens a new URL in the managed browser.
func (m *BrowserManager) Open(url string) (map[string]interface{}, error) {
	if err := requireNonEmpty(url, "url"); err != nil {
		return nil, err
	}
	return withManagerClient("browser manager", m, func(client *rpc.Client) (map[string]interface{}, error) {
		return client.OpenUrl(url)
	})
}

// Screenshot captures a screenshot for the target page.
func (m *BrowserManager) Screenshot(pageID string) (map[string]interface{}, error) {
	if err := requireNonEmpty(pageID, "page_id"); err != nil {
		return nil, err
	}
	return withManagerClient("browser manager", m, func(client *rpc.Client) (map[string]interface{}, error) {
		return client.CaptureScreenshot(pageID)
	})
}

// Focus brings the target page to the foreground in the managed browser.
func (m *BrowserManager) Focus(pageID string) (map[string]interface{}, error) {
	if err := requireNonEmpty(pageID, "page_id"); err != nil {
		return nil, err
	}
	return withManagerClient("browser manager", m, func(client *rpc.Client) (map[string]interface{}, error) {
		return client.FocusUserPage(pageID)
	})
}

// Click clicks the first element matching selector on the target page.
func (m *BrowserManager) Click(pageID, selector string) (map[string]interface{}, error) {
	if err := requireNonEmpty(pageID, "page_id"); err != nil {
		return nil, err
	}
	if err := requireNonEmpty(selector, "selector"); err != nil {
		return nil, err
	}
	return withManagerClient("browser manager", m, func(client *rpc.Client) (map[string]interface{}, error) {
		return client.ClickElement(pageID, selector)
	})
}

// Type enters text into the first element matching selector on the target page.
func (m *BrowserManager) Type(pageID, selector, text string) (map[string]interface{}, error) {
	if err := requireNonEmpty(pageID, "page_id"); err != nil {
		return nil, err
	}
	if err := requireNonEmpty(selector, "selector"); err != nil {
		return nil, err
	}
	return withManagerClient("browser manager", m, func(client *rpc.Client) (map[string]interface{}, error) {
		return client.TypeText(pageID, selector, text)
	})
}

// Scroll scrolls the target page by the given delta.
func (m *BrowserManager) Scroll(pageID string, deltaX, deltaY int) (map[string]interface{}, error) {
	if err := requireNonEmpty(pageID, "page_id"); err != nil {
		return nil, err
	}
	return withManagerClient("browser manager", m, func(client *rpc.Client) (map[string]interface{}, error) {
		return client.ScrollPage(pageID, deltaX, deltaY)
	})
}

func (m *BrowserManager) getClient() *rpc.Client {
	if m == nil {
		return nil
	}
	return m.client
}
