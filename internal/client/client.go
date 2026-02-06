package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/simonvc/miniledger/internal/ledger"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) CreateAccount(ctx context.Context, acct *ledger.Account) (*ledger.Account, error) {
	body := map[string]any{
		"id":        acct.ID,
		"name":      acct.Name,
		"code":      acct.Code,
		"currency":  acct.Currency,
		"category":  acct.Category,
		"is_system": acct.IsSystem,
	}
	var result ledger.Account
	if err := c.post(ctx, "/api/v1/accounts", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ListAccounts(ctx context.Context, category string, system *bool) ([]ledger.Account, error) {
	params := url.Values{}
	if category != "" {
		params.Set("category", category)
	}
	if system != nil {
		if *system {
			params.Set("system", "true")
		} else {
			params.Set("system", "false")
		}
	}
	var result []ledger.Account
	if err := c.get(ctx, "/api/v1/accounts?"+params.Encode(), &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) GetAccount(ctx context.Context, id string) (*ledger.Account, error) {
	var result ledger.Account
	if err := c.get(ctx, "/api/v1/accounts/"+url.PathEscape(id), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

type BalanceResponse struct {
	AccountID string `json:"account_id"`
	Balance   int64  `json:"balance"`
	Currency  string `json:"currency"`
	Formatted string `json:"formatted"`
}

func (c *Client) GetAccountBalance(ctx context.Context, id string) (*BalanceResponse, error) {
	var result BalanceResponse
	if err := c.get(ctx, "/api/v1/accounts/"+url.PathEscape(id)+"/balance", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ListAccountEntries(ctx context.Context, id string) ([]ledger.Entry, error) {
	var result []ledger.Entry
	if err := c.get(ctx, "/api/v1/accounts/"+url.PathEscape(id)+"/entries", &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) RenameAccount(ctx context.Context, id, newName string) (*ledger.Account, error) {
	body := map[string]any{"name": newName}
	var result ledger.Account
	if err := c.patch(ctx, "/api/v1/accounts/"+url.PathEscape(id), body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteAccount(ctx context.Context, id string) error {
	return c.del(ctx, "/api/v1/accounts/"+url.PathEscape(id))
}

func (c *Client) CreateTransaction(ctx context.Context, txn *ledger.Transaction) (*ledger.Transaction, error) {
	type entryReq struct {
		AccountID string `json:"account_id"`
		Amount    int64  `json:"amount"`
		Currency  string `json:"currency"`
	}
	entries := make([]entryReq, len(txn.Entries))
	for i, e := range txn.Entries {
		entries[i] = entryReq{AccountID: e.AccountID, Amount: e.Amount, Currency: e.Currency}
	}
	body := map[string]any{
		"description": txn.Description,
		"entries":     entries,
	}
	var result ledger.Transaction
	if err := c.post(ctx, "/api/v1/transactions", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ListTransactions(ctx context.Context, accountID string) ([]ledger.Transaction, error) {
	params := url.Values{}
	if accountID != "" {
		params.Set("account_id", accountID)
	}
	var result []ledger.Transaction
	if err := c.get(ctx, "/api/v1/transactions?"+params.Encode(), &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) GetTransaction(ctx context.Context, id string) (*ledger.Transaction, error) {
	var result ledger.Transaction
	if err := c.get(ctx, "/api/v1/transactions/"+id, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) BalanceSheet(ctx context.Context) (*ledger.BalanceSheet, error) {
	var result ledger.BalanceSheet
	if err := c.get(ctx, "/api/v1/reports/balance-sheet", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) TrialBalance(ctx context.Context) (*ledger.TrialBalance, error) {
	var result ledger.TrialBalance
	if err := c.get(ctx, "/api/v1/reports/trial-balance", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) RegulatoryRatios(ctx context.Context) (*ledger.RegulatoryRatios, error) {
	var result ledger.RegulatoryRatios
	if err := c.get(ctx, "/api/v1/reports/ratios", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetChart(ctx context.Context) ([]ledger.ChartEntry, error) {
	var result []ledger.ChartEntry
	if err := c.get(ctx, "/api/v1/chart", &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Ping checks if the server is reachable.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/v1/chart", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) get(ctx context.Context, path string, result any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	return c.doRequest(req, result)
}

func (c *Client) del(ctx context.Context, path string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		var apiErr apiError
		if json.Unmarshal(bodyBytes, &apiErr) == nil && apiErr.Error != "" {
			return fmt.Errorf("server error (%d): %s", resp.StatusCode, apiErr.Error)
		}
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

func (c *Client) patch(ctx context.Context, path string, body any, result any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "PATCH", c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return c.doRequest(req, result)
}

func (c *Client) post(ctx context.Context, path string, body any, result any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return c.doRequest(req, result)
}

type apiError struct {
	Error string `json:"error"`
}

func (c *Client) doRequest(req *http.Request, result any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var apiErr apiError
		if json.Unmarshal(bodyBytes, &apiErr) == nil && apiErr.Error != "" {
			return fmt.Errorf("server error (%d): %s", resp.StatusCode, apiErr.Error)
		}
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	if result != nil {
		if err := json.Unmarshal(bodyBytes, result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}
