package ledger

// ChartEntry represents a predefined entry in the IFRS chart of accounts.
type ChartEntry struct {
	Code        int      `json:"code"`
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Category    Category `json:"category"`
	Description string   `json:"description"`
	IsSystem    bool     `json:"is_system"`
}

// PredefinedAccounts is the minimal IFRS chart of accounts.
var PredefinedAccounts = []ChartEntry{
	// Assets (1xxx)
	{Code: 1010, ID: "1010", Name: "Nostro Accounts", Category: CategoryAssets, Description: "Our accounts at correspondent banks"},
	{Code: 1020, ID: "1020", Name: "Accounts Receivable", Category: CategoryAssets, Description: "Amounts owed to the entity by customers"},
	{Code: 1030, ID: "1030", Name: "Inventory", Category: CategoryAssets, Description: "Goods held for sale"},
	{Code: 1040, ID: "1040", Name: "Prepaid Expenses", Category: CategoryAssets, Description: "Payments made in advance for future expenses"},
	{Code: 1050, ID: "1050", Name: "Property, Plant & Equipment", Category: CategoryAssets, Description: "Long-term tangible assets"},
	{Code: 1060, ID: "1060", Name: "Restricted Cash / Regulatory Reserves", Category: CategoryAssets, Description: "Cash held at regulators or under restrictions"},

	// Liabilities (2xxx)
	{Code: 2010, ID: "2010", Name: "Vostro Accounts", Category: CategoryLiabilities, Description: "Correspondent bank accounts held at us"},
	{Code: 2020, ID: "2020", Name: "Customer Accounts", Category: CategoryLiabilities, Description: "Customer deposit and balance accounts"},
	{Code: 2030, ID: "2030", Name: "Accrued Expenses", Category: CategoryLiabilities, Description: "Expenses incurred but not yet paid"},
	{Code: 2040, ID: "2040", Name: "Loans Payable", Category: CategoryLiabilities, Description: "Outstanding loan obligations"},

	// Equity (3xxx)
	{Code: 3010, ID: "3010", Name: "Retained Earnings", Category: CategoryEquity, Description: "Accumulated profits retained in the entity"},
	{Code: 3020, ID: "3020", Name: "Common Stock", Category: CategoryEquity, Description: "Equity shares issued"},

	// Revenue (4xxx)
	{Code: 4010, ID: "4010", Name: "Service Revenue", Category: CategoryRevenue, Description: "Income from services rendered"},
	{Code: 4020, ID: "4020", Name: "Interest Income", Category: CategoryRevenue, Description: "Income earned from interest"},

	// Expenses (5xxx)
	{Code: 5010, ID: "5010", Name: "Operating Expenses", Category: CategoryExpenses, Description: "General operating costs"},
	{Code: 5020, ID: "5020", Name: "Cost of Goods Sold", Category: CategoryExpenses, Description: "Direct costs of goods sold"},
	{Code: 5030, ID: "5030", Name: "Salaries and Wages", Category: CategoryExpenses, Description: "Employee compensation"},
	{Code: 5040, ID: "5040", Name: "Depreciation", Category: CategoryExpenses, Description: "Allocation of asset costs over useful life"},
}

// SystemAccounts are internal accounts created automatically.
var SystemAccounts = []ChartEntry{
	{Code: 1097, ID: "~fx", Name: "FX Conversion", Category: CategoryAssets, IsSystem: true, Description: "Intermediary for cross-currency transactions"},
	{Code: 1098, ID: "~settlement", Name: "Settlement", Category: CategoryAssets, IsSystem: true, Description: "Pending settlement with payment processors/banks"},
	{Code: 1099, ID: "~suspense", Name: "Suspense Account", Category: CategoryAssets, IsSystem: true, Description: "Temporary holding for unclassified entries"},
	{Code: 2098, ID: "~tax", Name: "Tax Collected", Category: CategoryLiabilities, IsSystem: true, Description: "Tax held on behalf of tax authorities (VAT/GST/sales tax)"},
	{Code: 2099, ID: "~escrow", Name: "Escrow", Category: CategoryLiabilities, IsSystem: true, Description: "Funds held on behalf of third parties pending a condition"},
	{Code: 3099, ID: "~capital", Name: "Capital", Category: CategoryEquity, IsSystem: true, Description: "Owner's capital contributions and withdrawals"},
	{Code: 4099, ID: "~interest", Name: "Interest Income", Category: CategoryRevenue, IsSystem: true, Description: "Interest earned on customer balances or loans"},
	{Code: 4090, ID: "~fees", Name: "Fee Income", Category: CategoryRevenue, IsSystem: true, Description: "Fee income from customer charges"},
	{Code: 5091, ID: "~writeoff", Name: "Write-offs", Category: CategoryExpenses, IsSystem: true, Description: "Bad debt write-offs, failed payments, irrecoverable amounts"},
}

// LookupChartEntry finds a chart entry by code (checks predefined and system accounts).
func LookupChartEntry(code int) *ChartEntry {
	for i := range PredefinedAccounts {
		if PredefinedAccounts[i].Code == code {
			return &PredefinedAccounts[i]
		}
	}
	for i := range SystemAccounts {
		if SystemAccounts[i].Code == code {
			return &SystemAccounts[i]
		}
	}
	return nil
}

// AllChartEntries returns predefined + system accounts combined.
func AllChartEntries() []ChartEntry {
	all := make([]ChartEntry, 0, len(PredefinedAccounts)+len(SystemAccounts))
	all = append(all, PredefinedAccounts...)
	all = append(all, SystemAccounts...)
	return all
}
