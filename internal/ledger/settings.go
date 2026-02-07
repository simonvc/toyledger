package ledger

// SettingName identifies a per-CoA-code setting.
type SettingName string

const (
	SettingBlockInverted SettingName = "BLOCK_NORMAL_INVERTED"
	SettingEntryDirection SettingName = "ENTRY_DIRECTION"
)

// EntryDirection controls which entry directions are allowed.
type EntryDirection string

const (
	DirectionBoth       EntryDirection = "BOTH"
	DirectionDebitOnly  EntryDirection = "DEBIT_ONLY"
	DirectionCreditOnly EntryDirection = "CREDIT_ONLY"
)

// CoASetting is a single setting row for a CoA code.
type CoASetting struct {
	Code    int         `json:"code"`
	Setting SettingName `json:"setting"`
	Value   string      `json:"value"`
}

// CodeSettings holds the resolved settings for a CoA code.
type CodeSettings struct {
	Code           int            `json:"code"`
	BlockInverted  bool           `json:"block_inverted"`
	EntryDirection EntryDirection `json:"entry_direction"`
}

// DefaultCodeSettings returns settings with default values for a given code.
func DefaultCodeSettings(code int) CodeSettings {
	return CodeSettings{
		Code:           code,
		BlockInverted:  false,
		EntryDirection: DirectionBoth,
	}
}
