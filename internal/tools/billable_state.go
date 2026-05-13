package tools

import "strings"

const (
	billableStateBillable    = "BILLABLE"
	billableStateNonBillable = "NONBILLABLE"
	billableStateUnset       = "UNSET"
)

func billableStateFromPresence(value bool, present bool) string {
	if !present {
		return billableStateUnset
	}
	if value {
		return billableStateBillable
	}
	return billableStateNonBillable
}

func billableStateFromRaw(value any) (string, bool) {
	switch v := value.(type) {
	case nil:
		return billableStateUnset, false
	case bool:
		return billableStateFromPresence(v, true), true
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "billable", "yes", "1":
			return billableStateBillable, true
		case "false", "nonbillable", "non_billable", "non-billable", "no", "0":
			return billableStateNonBillable, true
		default:
			return billableStateUnset, false
		}
	default:
		return billableStateUnset, false
	}
}
