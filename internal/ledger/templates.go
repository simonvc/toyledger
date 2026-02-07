package ledger

import "fmt"

// TemplateEntry defines one side of a template transaction.
// CoACode is the suggested IFRS Chart of Accounts code (e.g. "1010").
// The user picks the actual account ID when executing the template.
type TemplateEntry struct {
	CoACode int    `json:"coa_code"`
	Role    string `json:"role"`     // human label like "Cash account", "Revenue account"
	IsDebit bool   `json:"is_debit"`
	Group   int    `json:"group"`    // 0 = default/source side, 1 = destination side (for multi-currency)
}

// Template defines a reusable transaction pattern for learning.
type Template struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Entries     []TemplateEntry `json:"entries"`
}

// Templates is the list of predefined transaction templates.
var Templates = []Template{
	{
		Name:        "Capital Injection",
		Description: "Owner puts money into the business. Regulatory reserves at the central bank increase (debit), Capital equity increases (credit).",
		Entries: []TemplateEntry{
			{CoACode: 1060, Role: "Reserves account (e.g. NBG Reserves)", IsDebit: true},
			{CoACode: 3099, Role: "Capital account", IsDebit: false},
		},
	},
	{
		Name:        "Customer Deposit",
		Description: "Customer deposits funds. Cash increases (debit), Customer liability increases — we owe them (credit).",
		Entries: []TemplateEntry{
			{CoACode: 1010, Role: "Cash account", IsDebit: true},
			{CoACode: 2020, Role: "Customer account", IsDebit: false},
		},
	},
	{
		Name:        "Customer Withdrawal",
		Description: "Customer withdraws funds. Customer liability decreases — we owe them less (debit), Cash decreases (credit).",
		Entries: []TemplateEntry{
			{CoACode: 2020, Role: "Customer account", IsDebit: true},
			{CoACode: 1010, Role: "Cash account", IsDebit: false},
		},
	},
	{
		Name:        "Record Service Revenue",
		Description: "Earn income from services. Receivable increases — they owe us (debit), Revenue increases (credit).",
		Entries: []TemplateEntry{
			{CoACode: 1020, Role: "Receivable account", IsDebit: true},
			{CoACode: 4010, Role: "Revenue account", IsDebit: false},
		},
	},
	{
		Name:        "Receive Payment",
		Description: "Customer pays an invoice. Cash increases (debit), Receivable decreases — debt settled (credit).",
		Entries: []TemplateEntry{
			{CoACode: 1010, Role: "Cash account", IsDebit: true},
			{CoACode: 1020, Role: "Receivable account", IsDebit: false},
		},
	},
	{
		Name:        "Pay Supplier",
		Description: "Pay a supplier invoice. Payable decreases — debt settled (debit), Cash decreases (credit).",
		Entries: []TemplateEntry{
			{CoACode: 2010, Role: "Payable account", IsDebit: true},
			{CoACode: 1010, Role: "Cash account", IsDebit: false},
		},
	},
	{
		Name:        "Pay Operating Expense",
		Description: "Pay a business expense. Expense increases (debit), Cash decreases (credit).",
		Entries: []TemplateEntry{
			{CoACode: 5010, Role: "Expense account", IsDebit: true},
			{CoACode: 1010, Role: "Cash account", IsDebit: false},
		},
	},
	{
		Name:        "Pay Salaries",
		Description: "Pay employee wages. Salary expense increases (debit), Cash decreases (credit).",
		Entries: []TemplateEntry{
			{CoACode: 5030, Role: "Salary expense account", IsDebit: true},
			{CoACode: 1010, Role: "Cash account", IsDebit: false},
		},
	},
	{
		Name:        "Collect Tax",
		Description: "Record tax collected from a sale. Cash increases (debit), Tax liability increases — we owe the authority (credit).",
		Entries: []TemplateEntry{
			{CoACode: 1010, Role: "Cash account", IsDebit: true},
			{CoACode: 2098, Role: "Tax liability account", IsDebit: false},
		},
	},
	{
		Name:        "Charge Customer Fee",
		Description: "Deduct a fee from customer balance. Customer liability decreases (debit), Fee recorded (credit).",
		Entries: []TemplateEntry{
			{CoACode: 2020, Role: "Customer account", IsDebit: true},
			{CoACode: 5090, Role: "Fee account", IsDebit: false},
		},
	},
	{
		Name:        "Write Off Bad Debt",
		Description: "Write off an uncollectible amount. Write-off expense increases (debit), Receivable decreases (credit).",
		Entries: []TemplateEntry{
			{CoACode: 5091, Role: "Write-off account", IsDebit: true},
			{CoACode: 1020, Role: "Receivable account", IsDebit: false},
		},
	},
	{
		Name:        "Earn Interest",
		Description: "Record interest earned. Cash increases (debit), Interest revenue increases (credit).",
		Entries: []TemplateEntry{
			{CoACode: 1010, Role: "Cash account", IsDebit: true},
			{CoACode: 4099, Role: "Interest income account", IsDebit: false},
		},
	},
	{
		Name:        "FX Conversion (Bank)",
		Description: "Convert the bank's own funds between currencies via ~fx. Source currency debits ~fx and credits the cash account; destination currency debits cash and credits ~fx. Each currency balances independently.",
		Entries: []TemplateEntry{
			{CoACode: 1097, Role: "FX intermediary (source)", IsDebit: true, Group: 0},
			{CoACode: 1010, Role: "Source cash account", IsDebit: false, Group: 0},
			{CoACode: 1010, Role: "Destination cash account", IsDebit: true, Group: 1},
			{CoACode: 1097, Role: "FX intermediary (dest)", IsDebit: false, Group: 1},
		},
	},
	{
		Name:        "Customer FX Swap",
		Description: "Customer swaps one currency for another. Source currency: customer liability decreases (debit), ~fx receives (credit). Destination currency: ~fx pays out (debit), customer liability increases (credit).",
		Entries: []TemplateEntry{
			{CoACode: 2020, Role: "Customer source currency account", IsDebit: true, Group: 0},
			{CoACode: 1097, Role: "FX intermediary (source)", IsDebit: false, Group: 0},
			{CoACode: 1097, Role: "FX intermediary (dest)", IsDebit: true, Group: 1},
			{CoACode: 2020, Role: "Customer dest currency account", IsDebit: false, Group: 1},
		},
	},
}

// IsMultiCurrency returns true if the template has entries in multiple groups.
func (t *Template) IsMultiCurrency() bool {
	for _, e := range t.Entries {
		if e.Group > 0 {
			return true
		}
	}
	return false
}

// DefaultAccountForCoA returns the default account ID for a given CoA code.
// System accounts use their ~ prefix ID, regular accounts use the code as string.
func DefaultAccountForCoA(code int) string {
	for _, sa := range SystemAccounts {
		if sa.Code == code {
			return sa.ID
		}
	}
	return fmt.Sprintf("%d", code)
}
