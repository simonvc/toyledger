# Miniledger — Product Reference Document

Miniledger is a double-entry accounting ledger designed for a Georgian bank (Pave Bank). It enforces IFRS chart-of-accounts conventions, multi-currency support with GEL reporting, and regulatory ratio monitoring.

---

## Core Principles

1. **Double-entry accounting** — every transaction must have at least two entries. For each currency involved, the sum of debits and credits must equal zero. This is enforced at both the application layer and the database layer via SQL triggers.

2. **Amounts as integers** — all monetary amounts are stored as `int64` minor units (e.g. cents). 1000 USD = `100000` (100000 cents). This eliminates floating-point rounding errors. Positive values are debits, negative values are credits.

3. **Per-currency balancing** — a transaction involving USD must have its USD entries sum to zero. A transaction involving EUR must have its EUR entries sum to zero. Multi-currency transactions (like FX swaps) contain entries in multiple currencies, each independently balanced.

4. **Immutability** — once a transaction is finalized, its entries cannot be inserted, updated, or deleted. Corrections are made by posting new reversing transactions.

---

## Schema

### Accounts

```sql
CREATE TABLE accounts (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    code       INTEGER NOT NULL,
    category   TEXT NOT NULL CHECK (category IN ('assets','liabilities','equity','revenue','expenses')),
    currency   TEXT NOT NULL DEFAULT 'USD',
    is_system  INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX idx_accounts_category ON accounts(category);
CREATE INDEX idx_accounts_code ON accounts(code);
```

### Transactions

```sql
CREATE TABLE transactions (
    id          TEXT PRIMARY KEY,
    description TEXT NOT NULL,
    finalized   INTEGER NOT NULL DEFAULT 0,
    posted_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX idx_transactions_posted ON transactions(posted_at);
```

### Entries

```sql
CREATE TABLE entries (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    transaction_id TEXT NOT NULL REFERENCES transactions(id),
    account_id     TEXT NOT NULL REFERENCES accounts(id),
    amount         INTEGER NOT NULL,
    currency       TEXT NOT NULL,
    created_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX idx_entries_txn ON entries(transaction_id);
CREATE INDEX idx_entries_account ON entries(account_id);
```

---

## SQL Triggers

Five triggers enforce ledger integrity at the database level. These are the last line of defense — even if the application has a bug, the database will reject invalid state.

### 1. Balance check on finalization

When a transaction's `finalized` flag is set to `1`, this trigger checks that entries sum to zero **per currency**. If any currency is unbalanced, the entire update is aborted.

```sql
CREATE TRIGGER trg_check_balance
BEFORE UPDATE OF finalized ON transactions
WHEN NEW.finalized = 1
BEGIN
    SELECT CASE
        WHEN EXISTS (
            SELECT currency, SUM(amount) as total
            FROM entries
            WHERE transaction_id = NEW.id
            GROUP BY currency
            HAVING total != 0
        )
        THEN RAISE(ABORT, 'transaction entries do not balance: per-currency sum != 0')
    END;
END;
```

### 2–4. Immutability of finalized transactions

Once finalized, no entries can be added, removed, or modified:

```sql
CREATE TRIGGER trg_immutable_entries_insert
BEFORE INSERT ON entries
WHEN (SELECT finalized FROM transactions WHERE id = NEW.transaction_id) = 1
BEGIN
    SELECT RAISE(ABORT, 'cannot add entries to a finalized transaction');
END;

CREATE TRIGGER trg_immutable_entries_delete
BEFORE DELETE ON entries
WHEN (SELECT finalized FROM transactions WHERE id = OLD.transaction_id) = 1
BEGIN
    SELECT RAISE(ABORT, 'cannot remove entries from a finalized transaction');
END;

CREATE TRIGGER trg_immutable_entries_update
BEFORE UPDATE ON entries
WHEN (SELECT finalized FROM transactions WHERE id = OLD.transaction_id) = 1
BEGIN
    SELECT RAISE(ABORT, 'cannot modify entries of a finalized transaction');
END;
```

### 5. Currency match on entry insert

Every entry's currency must match its account's currency. The sole exception is accounts with currency `*` (wildcard), which accept entries in any currency. Only the `~fx` account uses this.

```sql
CREATE TRIGGER trg_entry_currency_match
BEFORE INSERT ON entries
WHEN (SELECT currency FROM accounts WHERE id = NEW.account_id) != '*'
    AND NEW.currency != (SELECT currency FROM accounts WHERE id = NEW.account_id)
BEGIN
    SELECT RAISE(ABORT, 'entry currency does not match account currency');
END;
```

---

## Two-Phase Transaction Commit

Transactions are created in two phases within a single database transaction:

1. **Insert** — the transaction row is created with `finalized = 0`, then all entry rows are inserted. At this point, triggers allow the inserts because the transaction is not yet finalized.

2. **Finalize** — `UPDATE transactions SET finalized = 1 WHERE id = ?` fires `trg_check_balance`. If any currency's entries don't sum to zero, the trigger aborts and the entire database transaction rolls back. Nothing is persisted.

This means it is impossible for the database to contain a finalized transaction with unbalanced entries.

Transaction IDs are UUID v7 (time-sortable).

---

## IFRS Chart of Accounts

Account codes follow IFRS conventions using 4-digit codes. The first digit determines the category:

| Range | Category | Normal Balance | Description |
|-------|----------|---------------|-------------|
| 1000–1999 | Assets | Debit | What the bank owns |
| 2000–2999 | Liabilities | Credit | What the bank owes |
| 3000–3999 | Equity | Credit | Owner's stake |
| 4000–4999 | Revenue | Credit | Income earned |
| 5000–5999 | Expenses | Debit | Costs incurred |

"Normal balance" means the side that increases the account. A debit to an asset account increases it; a credit to a liability account increases it.

### Predefined Accounts

| Code | Name | Category | Description |
|------|------|----------|-------------|
| 1010 | Cash and Cash Equivalents | Assets | Cash on hand and in bank accounts |
| 1020 | Accounts Receivable | Assets | Amounts owed to the entity by customers |
| 1030 | Inventory | Assets | Goods held for sale |
| 1040 | Prepaid Expenses | Assets | Payments made in advance for future expenses |
| 1050 | Property, Plant & Equipment | Assets | Long-term tangible assets |
| 1060 | Restricted Cash / Regulatory Reserves | Assets | Cash held at regulators or under restrictions |
| 2010 | Accounts Payable | Liabilities | Amounts owed to suppliers |
| 2020 | Customer Accounts | Liabilities | Customer deposit and balance accounts |
| 2030 | Accrued Expenses | Liabilities | Expenses incurred but not yet paid |
| 2040 | Loans Payable | Liabilities | Outstanding loan obligations |
| 3010 | Retained Earnings | Equity | Accumulated profits retained in the entity |
| 3020 | Common Stock | Equity | Equity shares issued |
| 4010 | Service Revenue | Revenue | Income from services rendered |
| 4020 | Interest Income | Revenue | Income earned from interest |
| 5010 | Operating Expenses | Expenses | General operating costs |
| 5020 | Cost of Goods Sold | Expenses | Direct costs of goods sold |
| 5030 | Salaries and Wages | Expenses | Employee compensation |
| 5040 | Depreciation | Expenses | Allocation of asset costs over useful life |

---

## System Accounts (~ prefix)

Accounts prefixed with `~` are internal/system accounts. They are auto-created on database initialization and serve specific operational purposes. The `~` prefix is enforced bidirectionally:

- Accounts with `is_system = 1` **must** have an ID starting with `~`
- Accounts with `is_system = 0` **cannot** have an ID starting with `~`

### System Account Registry

| Code | ID | Name | Category | Currency | Purpose |
|------|----|------|----------|----------|---------|
| 1097 | ~fx | FX Conversion | Assets | `*` (any) | Intermediary for cross-currency transactions |
| 1098 | ~settlement | Settlement | Assets | USD | Pending settlement with payment processors/banks |
| 1099 | ~suspense | Suspense Account | Assets | USD | Temporary holding for unclassified entries |
| 2098 | ~tax | Tax Collected | Liabilities | USD | Tax held on behalf of tax authorities |
| 2099 | ~escrow | Escrow | Liabilities | USD | Funds held on behalf of third parties |
| 3099 | ~capital | Capital | Equity | USD | Owner's capital contributions and withdrawals |
| 4099 | ~interest | Interest Income | Revenue | USD | Interest earned on customer balances or loans |
| 4090 | ~fees | Fee Income | Revenue | USD | Fee income from customer charges |
| 5091 | ~writeoff | Write-offs | Expenses | USD | Bad debt write-offs, failed payments |

### The ~fx Account (Wildcard Currency)

The `~fx` account is unique: its currency is `*`, meaning the `trg_entry_currency_match` trigger allows entries in **any** currency. This is essential for FX transactions where a single intermediary account must hold positions in multiple currencies simultaneously.

No other account uses `*`. All regular and other system accounts are denominated in a single currency.

---

## Multi-Currency Support

### Supported Currencies

| Code | Name | Exponent | GEL Rate |
|------|------|----------|----------|
| GEL | Georgian Lari | 2 | 1.00 |
| USD | US Dollar | 2 | 2.70 |
| EUR | Euro | 2 | 2.95 |
| GBP | Pound Sterling | 2 | 3.40 |

The **exponent** determines minor unit conversion: exponent 2 means 100 minor units per major unit (e.g. 100 cents = 1 dollar).

### Reporting Currency

**GEL (Georgian Lari)** is the reporting currency. All balance sheet totals and regulatory ratios are computed in GEL using the rate table above. Individual account balances are shown in their native currency with a GEL equivalent.

### Currency Constraints

- Each account is denominated in exactly one currency (except `~fx` which uses `*`)
- Each entry must specify a currency
- The entry's currency must match its account's currency (enforced by `trg_entry_currency_match`)
- Within a transaction, entries are balanced **per currency** — not across currencies

---

## How FX Works

Foreign exchange transactions use the `~fx` account as an intermediary. Because `~fx` has currency `*`, it can receive entries in any currency. The key insight: **each currency must balance independently within the transaction**.

### FX Transaction Structure (4 entries)

An FX swap always has 4 entries split into two groups:

**Group 0 — Source currency side:**
- DR (source account) — removes source currency from the account
- CR (~fx) — source currency enters the FX intermediary

**Group 1 — Destination currency side:**
- DR (~fx) — destination currency leaves the FX intermediary
- CR (destination account) — destination currency arrives at the account

### Example: Customer swaps 1,000 USD for 850 EUR

The customer has a USD account (`acc_1`) and a EUR account (`acc_2`).

| # | Type | Account | Amount | Currency | Group |
|---|------|---------|--------|----------|-------|
| 1 | DR | acc_1 (customer USD) | +100,000 | USD | 0 |
| 2 | CR | ~fx | -100,000 | USD | 0 |
| 3 | DR | ~fx | +85,000 | EUR | 1 |
| 4 | CR | acc_2 (customer EUR) | -85,000 | EUR | 1 |

Verification:
- USD entries: +100,000 + (-100,000) = **0** (balanced)
- EUR entries: +85,000 + (-85,000) = **0** (balanced)
- `trg_check_balance` passes

### FX PnL

The `~fx` account accumulates positions across currencies. After the transaction above, `~fx` holds:
- -100,000 USD (it paid out USD)
- +85,000 EUR (it received EUR)

Converting each position to GEL at the reporting rates:
- -100,000 USD x 2.70 = -270,000 GEL
- +85,000 EUR x 2.95 = +250,750 GEL
- **Net FX PnL = -19,250 GEL** (loss)

If the bank charges a spread (e.g. giving 840 EUR instead of 850), the net GEL position improves, representing FX dealing profit.

### Bank FX vs Customer FX

There are two FX templates:

**FX Conversion (Bank)** — the bank converts its own cash holdings:
- Source/dest accounts are cash accounts (code 1010)
- Used for treasury operations

**Customer FX Swap** — a customer swaps currencies:
- Source/dest accounts are customer liability accounts (code 2020)
- The customer's source currency balance decreases, destination currency balance increases
- The bank's `~fx` position changes accordingly

---

## Debit and Credit Conventions

All amounts in the `entries` table use a single signed integer:

| | Representation | Effect on Debit-Normal Account | Effect on Credit-Normal Account |
|---|---|---|---|
| **Debit** | Positive amount (+) | Increases balance | Decreases balance |
| **Credit** | Negative amount (-) | Decreases balance | Increases balance |

Examples:
- Customer deposits 1,000 USD → DR Cash +100,000, CR Customer -100,000
- Customer liability (code 2020) is credit-normal, so -100,000 means the balance **increases** (we owe them more)
- Cash (code 1010) is debit-normal, so +100,000 means the balance **increases** (we have more cash)

---

## Balance Sheet

The balance sheet query aggregates finalized entry amounts per account, grouped by category:

```sql
SELECT a.id, a.name, a.category, a.currency, COALESCE(SUM(e.amount), 0) as balance
FROM accounts a
LEFT JOIN entries e ON e.account_id = a.id
LEFT JOIN transactions t ON t.id = e.transaction_id AND t.finalized = 1
GROUP BY a.id
HAVING balance != 0
ORDER BY a.code
```

The fundamental accounting equation holds:

> **Assets + Liabilities + Equity = 0**

In this system, asset balances are positive (net debits) and liability/equity balances are negative (net credits). The balance sheet is balanced when `TotalAssets + TotalLiabilities + TotalEquity = 0`.

For display, each line shows its native currency amount alongside a GEL equivalent. Section totals and the overall total are computed in GEL (the reporting currency).

---

## Regulatory Ratios

Three prudential ratios are computed from the balance sheet data:

```sql
SELECT a.category, a.code, COALESCE(SUM(e.amount), 0) as balance
FROM accounts a
LEFT JOIN entries e ON e.account_id = a.id
LEFT JOIN transactions t ON t.id = e.transaction_id AND t.finalized = 1
GROUP BY a.category, a.code
HAVING balance != 0
```

### Capital Adequacy Ratio (CAR)

```
CAR = Equity / Total Assets
```

- Equity is the negated sum of all equity-category account balances (credit-normal → negate to get positive)
- Total Assets is the sum of all asset-category account balances
- **Minimum: 8%** — below this the bank is undercapitalized

### Leverage Ratio

```
Leverage = Equity / Total Assets (unweighted)
```

In this simplified model, the leverage ratio equals the CAR because all assets are unweighted. In a full implementation, the denominator would be risk-weighted.

- **Minimum: 3%**

### Reserve Ratio

```
Reserve Ratio = Reserves (code 1060) / Customer Deposits (code 2020)
```

- Reserves are the sum of balances on accounts with code 1060
- Customer Deposits are the negated sum of balances on accounts with code 2020 (credit-normal)
- **Minimum: 10%** — ensures sufficient liquidity to cover customer withdrawals

### Ratio Impact Preview

Before posting any transaction, the system projects how the proposed entries would change each ratio. This is computed by:

1. Fetching current ratios from the database
2. For each proposed entry, adjusting the relevant bucket based on the target account's category/code:
   - Asset account entry → adjusts Total Assets
   - Equity account entry → adjusts Equity (negated, since equity is credit-normal)
   - Liability account with code 2020 → adjusts Customer Deposits
   - Account with code 1060 → adjusts Reserves
3. Recomputing ratios from the adjusted buckets

---

## Open Currency Position (OCP)

The Open Currency Position report shows the bank's net foreign exchange exposure per currency. It answers: "for each currency, how much do we hold in real accounts versus how much do we owe?"

### Calculation

For each currency, aggregate balances from all accounts denominated in that currency:

- **Assets (Long)**: sum of balances from asset-category accounts (debit-normal, positive = bank holds)
- **Liabilities (Short)**: absolute sum of balances from liability-category accounts (credit-normal, negate for display)
- **Equity**: absolute sum of balances from equity-category accounts (credit-normal, negate for display)
- **Net Position**: raw sum of all account balances in that currency (assets + liabilities + equity in their signed form)
- **GEL Equivalent**: `ToGEL(Net, Currency)` — net position converted to reporting currency

The **Total Open Position** is the sum of all per-currency GEL equivalents.

### Excluding ~fx

The `~fx` account (currency `*`) is **excluded** from OCP. It is a booking intermediary that records FX conversions, not a real cash holding. Including it would double-count: when a customer FX swap is booked, `~fx` receives entries that exactly mirror the customer liability, making the net appear zero even when the bank has genuine currency risk.

The `~fx` account's own per-currency position (viewable on its account detail page) shows the bank's **FX dealing book** — the accumulated conversions the bank has facilitated. This is a complementary view but distinct from OCP.

### Interpreting OCP

| Net Position | Meaning | Risk |
|---|---|---|
| Positive (long) | Bank holds more assets than liabilities in this currency | If the currency depreciates, the bank loses value |
| Negative (short) | Bank owes more than it holds in this currency | The bank must acquire this currency to meet obligations; if the currency appreciates, costs increase |
| Zero | Matched book — assets equal liabilities | No FX risk in this currency |

### Example

Given these accounts:

| Account | Category | Currency | Balance |
|---------|----------|----------|---------|
| ~suspense | Assets | USD | +1,000.00 |
| acc_1 | Liabilities | USD | -455.56 |
| acc_2 | Liabilities | EUR | -491.30 |

The OCP is:

| CCY | Assets | Liabilities | Net | GEL Equiv |
|-----|--------|-------------|-----|-----------|
| EUR | 0.00 | 491.30 | -491.30 | -1,449.34 |
| USD | 1,000.00 | 455.56 | +544.44 | +1,470.00 |
| **Total** | | | | **+20.66 GEL** |

The bank is **short EUR** (owes EUR it doesn't hold) and **long USD** (holds more USD than it owes). The total GEL exposure is slightly positive, meaning the bank's FX risk is roughly balanced in aggregate but exposed to EUR/USD relative movements.

---

## Transaction Templates

Templates define reusable journal entry patterns. Each template has a name, description, and a list of entry slots with:

- **CoACode** — suggested IFRS chart of accounts code (determines the default account)
- **Role** — human-readable label explaining this entry's purpose
- **IsDebit** — whether this entry is a debit (true) or credit (false)
- **Group** — for multi-currency templates: 0 = source currency side, 1 = destination currency side

### Available Templates

| Template | Entries | Description |
|----------|---------|-------------|
| Capital Injection | DR 1060, CR 3099 | Regulatory reserves increase, capital equity increases |
| Customer Deposit | DR 1010, CR 2020 | Cash (nostro) increases, customer liability increases |
| Customer Withdrawal | DR 2020, CR 1010 | Customer liability decreases, cash decreases |
| Record Service Revenue | DR 1020, CR 4010 | Receivable increases, revenue recorded |
| Receive Payment | DR 1010, CR 1020 | Cash increases, receivable settled |
| Pay Supplier | DR 2010, CR 1010 | Payable settled, cash decreases |
| Pay Operating Expense | DR 5010, CR 1010 | Expense recorded, cash decreases |
| Pay Salaries | DR 5030, CR 1010 | Salary expense recorded, cash decreases |
| Collect Tax | DR 1010, CR 2098 | Cash increases, tax liability increases |
| Charge Customer Fee | DR 2020, CR 4090 | Customer balance decreases, fee revenue increases |
| Write Off Bad Debt | DR 5091, CR 1020 | Write-off expense recorded, receivable removed |
| Earn Interest | DR 1010, CR 4099 | Cash increases, interest revenue recorded |
| FX Conversion (Bank) | DR 1097, CR 1010 / DR 1010, CR 1097 | Bank converts own cash between currencies via ~fx |
| Customer FX Swap | DR 2020, CR 1097 / DR 1097, CR 2020 | Customer swaps currencies via ~fx |

---

## Per-Code Constraints

Beyond the universal rules (balancing, immutability, currency matching), the ledger supports optional constraints that can be enabled per chart-of-accounts code. All accounts sharing a code share the same constraint settings. Constraints are enforced at both the application layer and the database layer.

### Block Inverted Balance

When enabled for a code, this constraint prevents any transaction from pushing an account's balance to the "wrong" side of zero:

- **Debit-normal accounts** (assets, expenses) — balance cannot go negative. A negative balance would mean the account holds more credits than debits, which is abnormal (e.g. cash going below zero = overdraft).
- **Credit-normal accounts** (liabilities, equity, revenue) — balance cannot go positive. A positive balance would mean more debits than credits, which is abnormal (e.g. a customer deposit account flipping into receivable territory).

The check is performed against the **projected** balance: the account's existing finalized balance plus all entries in the pending transaction. If the projected balance would invert, the transaction is rejected before finalization.

#### When to use it

| Category | Rationale |
|----------|-----------|
| Assets (1xxx) | Prevents overdraw — an asset account going negative means the entity has less than nothing |
| Liabilities (2xxx) | Prevents liability accounts from flipping to a debit balance, which would imply customers owe the bank when the bank should owe them |
| Equity (3xxx) | Prevents equity erosion below zero — protects against technical insolvency |
| Revenue (4xxx) | Prevents revenue from reversing past zero, which would create a phantom expense |
| Expenses (5xxx) | Prevents expense accounts from going negative, which would imply unrecorded income |

### Entry Direction

Controls which entry directions (debit or credit) are allowed for accounts at a given code. Three modes:

| Mode | Positive (Debit) | Negative (Credit) | Use case |
|------|:-:|:-:|----------|
| **BOTH** (default) | Allowed | Allowed | Most accounts — normal operations include both increases and decreases |
| **DEBIT_ONLY** | Allowed | Rejected | Accounts that should only receive, never disburse (e.g. regulatory reserves at 1060, expense accounts) |
| **CREDIT_ONLY** | Rejected | Allowed | Accounts that should only accumulate on the credit side (e.g. customer deposit accounts at 2020, revenue accounts) |

Each entry is checked individually before insertion. If a credit entry is posted to a DEBIT_ONLY code (or vice versa), the transaction is rejected immediately.

#### Interaction with Block Inverted Balance

When both constraints are active, certain direction modes become contradictory:

- A **debit-normal** account with Block Inverted cannot be set to **CREDIT_ONLY** — that would only allow credits, guaranteeing the balance goes negative.
- A **credit-normal** account with Block Inverted cannot be set to **DEBIT_ONLY** — that would only allow debits, guaranteeing the balance goes positive.

The system automatically corrects these invalid combinations: enabling Block Inverted on a code with a contradictory direction will reset the direction to the safe default (DEBIT_ONLY for debit-normal, CREDIT_ONLY for credit-normal).

### Suggested Configurations

| Code | Name | Suggested | Rationale |
|------|------|-----------|-----------|
| 1060 | Restricted Cash | Block Inverted + DEBIT_ONLY | Regulatory reserves should only receive funds and never go negative |
| 2020 | Customer Accounts | Block Inverted + CREDIT_ONLY | Customer deposits should only accumulate and never flip to debit |
| 4xxx | Revenue codes | Block Inverted + CREDIT_ONLY | Revenue should only be recognized (credited), not directly debited |
| 5xxx | Expense codes | Block Inverted + DEBIT_ONLY | Expenses should only be incurred (debited), not directly credited |
| 3xxx | Equity codes | Block Inverted + BOTH | Equity accepts both contributions and distributions but should not go negative |

Constraints are optional. By default, all codes have Block Inverted off and Entry Direction set to BOTH, preserving the ledger's traditional flexibility. Constraints are most valuable for operational accounts where accidental sign errors could have regulatory or customer-facing consequences.

---

## Account Deletion Constraints

An account can only be deleted if it has **zero entries**. This prevents orphaning historical transaction data. The check is:

```sql
SELECT COUNT(*) FROM entries WHERE account_id = ?
```

If count > 0, deletion is refused. System accounts can be deleted under the same rule, though in practice they will accumulate entries quickly.

---

## Concurrency Model

SQLite is configured with:

- **WAL mode** (`journal_mode=WAL`) — allows concurrent readers while a writer is active
- **Foreign keys enabled** (`foreign_keys=ON`)
- **Busy timeout** (`busy_timeout=5000`) — waits up to 5 seconds for a lock
- **Synchronous normal** (`synchronous=NORMAL`) — balanced durability/performance

Two connection pools are maintained:

- **Writer pool**: `MaxOpenConns = 1` — serializes all writes (SQLite limitation)
- **Reader pool**: `MaxOpenConns = NumCPU` — parallel reads via WAL

All write operations (create transaction, create account, delete account, rename account) go through the writer. All read operations (balance sheet, trial balance, list accounts, ratios) go through the reader pool.
