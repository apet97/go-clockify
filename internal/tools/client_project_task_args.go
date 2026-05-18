package tools

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
)

func optionalBoolArg(args map[string]any, key string) (bool, bool) {
	v, ok := args[key].(bool)
	return v, ok
}

func numberArg(args map[string]any, key string) (float64, bool) {
	v, ok := args[key]
	if !ok {
		return 0, false
	}
	return numberFromAny(v)
}

func int64Arg(args map[string]any, key string) (int64, bool) {
	v, ok := args[key]
	if !ok {
		return 0, false
	}
	f, ok := numberFromAny(v)
	if !ok || math.IsNaN(f) || math.IsInf(f, 0) || f < math.MinInt64 || f > math.MaxInt64 {
		return 0, false
	}
	return int64(f), true
}

func strictStringSliceArg(args map[string]any, key string) ([]string, bool, error) {
	raw, ok := args[key]
	if !ok {
		return nil, false, nil
	}
	switch x := raw.(type) {
	case []string:
		out := make([]string, 0, len(x))
		for _, item := range x {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
		return out, true, nil
	case []any:
		out := make([]string, 0, len(x))
		for _, rawItem := range x {
			item, ok := rawItem.(string)
			if !ok {
				return nil, true, fmt.Errorf("%s must contain only strings", key)
			}
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
		return out, true, nil
	default:
		return nil, true, fmt.Errorf("%s must be an array of strings", key)
	}
}

func addStringQuery(query map[string]string, args map[string]any, argKey, queryKey string) {
	if v := strings.TrimSpace(stringArg(args, argKey)); v != "" {
		query[queryKey] = v
	}
}

func addBoolQuery(query map[string]string, args map[string]any, argKey, queryKey string) {
	if v, ok := optionalBoolArg(args, argKey); ok {
		query[queryKey] = strconv.FormatBool(v)
	}
}

func addIntQuery(query map[string]string, args map[string]any, argKey, queryKey string) {
	if v, ok := int64Arg(args, argKey); ok {
		query[queryKey] = strconv.FormatInt(v, 10)
	}
}

func valuesFromQueryMap(query map[string]string) url.Values {
	values := url.Values{}
	for k, v := range query {
		if v != "" {
			values.Add(k, v)
		}
	}
	return values
}

func addRepeatedStringQuery(values url.Values, args map[string]any, argKey, queryKey string) error {
	items, ok, err := strictStringSliceArg(args, argKey)
	if err != nil || !ok {
		return err
	}
	for _, item := range items {
		values.Add(queryKey, item)
	}
	return nil
}

func setIfString(body map[string]any, args map[string]any, argKey, bodyKey string) {
	if v := strings.TrimSpace(stringArg(args, argKey)); v != "" {
		body[bodyKey] = v
	}
}

func setIfBool(body map[string]any, args map[string]any, argKey, bodyKey string) {
	if v, ok := optionalBoolArg(args, argKey); ok {
		body[bodyKey] = v
	}
}

func setIfInt64(body map[string]any, args map[string]any, argKey, bodyKey string) {
	if v, ok := int64Arg(args, argKey); ok {
		body[bodyKey] = v
	}
}

func setIfStringSlice(body map[string]any, args map[string]any, argKey, bodyKey string) error {
	items, ok, err := strictStringSliceArg(args, argKey)
	if err != nil || !ok {
		return err
	}
	body[bodyKey] = items
	return nil
}

func mapArg(args map[string]any, key string) (map[string]any, bool, error) {
	raw, ok := args[key]
	if !ok {
		return nil, false, nil
	}
	out, ok := raw.(map[string]any)
	if !ok {
		return nil, true, fmt.Errorf("%s must be an object", key)
	}
	return out, true, nil
}

func mapSliceArg(args map[string]any, key string) ([]map[string]any, bool, error) {
	raw, ok := args[key]
	if !ok {
		return nil, false, nil
	}
	switch items := raw.(type) {
	case []map[string]any:
		return items, true, nil
	case []any:
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			m, ok := item.(map[string]any)
			if !ok {
				return nil, true, fmt.Errorf("%s must contain only objects", key)
			}
			out = append(out, m)
		}
		return out, true, nil
	default:
		return nil, true, fmt.Errorf("%s must be an array of objects", key)
	}
}

func normalizeRateRequest(raw map[string]any) map[string]any {
	out := map[string]any{}
	if amount, ok := numberFromMap(raw, "amount"); ok {
		out["amount"] = amount
	}
	if since := stringFromMap(raw, "since"); since != "" {
		out["since"] = since
	}
	return out
}

func normalizeEstimateRequest(raw map[string]any) map[string]any {
	out := map[string]any{}
	if estimate := stringFromMap(raw, "estimate"); estimate != "" {
		out["estimate"] = estimate
	}
	if typ := stringFromMap(raw, "type"); typ != "" {
		out["type"] = typ
	}
	return out
}

func normalizeEstimateWithOptionsRequest(raw map[string]any) map[string]any {
	out := map[string]any{}
	if v, ok := boolFromMap(raw, "active"); ok {
		out["active"] = v
	}
	if v, ok := numberFromMap(raw, "estimate"); ok {
		out["estimate"] = v
	}
	if v, ok := boolFromMap(raw, "include_expenses"); ok {
		out["includeExpenses"] = v
	} else if v, ok := boolFromMap(raw, "includeExpenses"); ok {
		out["includeExpenses"] = v
	}
	if v := stringFromMap(raw, "reset_option"); v != "" {
		out["resetOption"] = v
	} else if v := stringFromMap(raw, "resetOption"); v != "" {
		out["resetOption"] = v
	}
	if v := stringFromMap(raw, "type"); v != "" {
		out["type"] = v
	}
	return out
}

func normalizeTimeEstimateRequest(raw map[string]any) map[string]any {
	out := map[string]any{}
	if v, ok := boolFromMap(raw, "active"); ok {
		out["active"] = v
	}
	if v := stringFromMap(raw, "estimate"); v != "" {
		out["estimate"] = v
	}
	if v, ok := boolFromMap(raw, "include_non_billable"); ok {
		out["includeNonBillable"] = v
	} else if v, ok := boolFromMap(raw, "includeNonBillable"); ok {
		out["includeNonBillable"] = v
	}
	if v := stringFromMap(raw, "reset_option"); v != "" {
		out["resetOption"] = v
	} else if v := stringFromMap(raw, "resetOption"); v != "" {
		out["resetOption"] = v
	}
	if v := stringFromMap(raw, "type"); v != "" {
		out["type"] = v
	}
	return out
}

func normalizeEstimateResetRequest(raw map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"active", "is_active"} {
		if v, ok := boolFromMap(raw, key); ok {
			if key == "is_active" {
				out["isActive"] = v
			} else {
				out[key] = v
			}
		}
	}
	if v, ok := intFromMap(raw, "day_of_month"); ok {
		out["dayOfMonth"] = v
	} else if v, ok := intFromMap(raw, "dayOfMonth"); ok {
		out["dayOfMonth"] = v
	}
	if v := stringFromMap(raw, "day_of_week"); v != "" {
		out["dayOfWeek"] = v
	} else if v := stringFromMap(raw, "dayOfWeek"); v != "" {
		out["dayOfWeek"] = v
	}
	if v, ok := intFromMap(raw, "hour"); ok {
		out["hour"] = v
	}
	if v := stringFromMap(raw, "interval"); v != "" {
		out["interval"] = v
	}
	if v := stringFromMap(raw, "month"); v != "" {
		out["month"] = v
	}
	return out
}

func normalizeUserGroupsRequest(raw map[string]any) map[string]any {
	out := map[string]any{}
	if v := stringFromMap(raw, "contains"); v != "" {
		out["contains"] = v
	}
	if ids, ok, err := stringSliceFromMap(raw, "ids"); err == nil && ok {
		out["ids"] = ids
	}
	if v := stringFromMap(raw, "status"); v != "" {
		out["status"] = v
	}
	return out
}

func normalizeMembershipRequest(raw map[string]any) map[string]any {
	out := map[string]any{}
	if v := stringFromMap(raw, "user_id"); v != "" {
		out["userId"] = v
	} else if v := stringFromMap(raw, "userId"); v != "" {
		out["userId"] = v
	}
	if v := stringFromMap(raw, "membership_status"); v != "" {
		out["membershipStatus"] = v
	} else if v := stringFromMap(raw, "membershipStatus"); v != "" {
		out["membershipStatus"] = v
	}
	if v := stringFromMap(raw, "membership_type"); v != "" {
		out["membershipType"] = v
	} else if v := stringFromMap(raw, "membershipType"); v != "" {
		out["membershipType"] = v
	}
	if rate, ok := nestedMap(raw, "hourly_rate", "hourlyRate"); ok {
		out["hourlyRate"] = normalizeRateRequest(rate)
	}
	if rate, ok := nestedMap(raw, "cost_rate", "costRate"); ok {
		out["costRate"] = normalizeRateRequest(rate)
	}
	return out
}

func normalizeTaskRequest(raw map[string]any) map[string]any {
	out := map[string]any{}
	if v := stringFromMap(raw, "assignee_id"); v != "" {
		out["assigneeId"] = v
	} else if v := stringFromMap(raw, "assigneeId"); v != "" {
		out["assigneeId"] = v
	}
	if ids, ok, err := stringSliceFromMap(raw, "assignee_ids"); err == nil && ok {
		out["assigneeIds"] = ids
	} else if ids, ok, err := stringSliceFromMap(raw, "assigneeIds"); err == nil && ok {
		out["assigneeIds"] = ids
	}
	if v, ok := boolFromMap(raw, "billable"); ok {
		out["billable"] = v
	}
	if v, ok := intFromMap(raw, "budget_estimate"); ok {
		out["budgetEstimate"] = v
	} else if v, ok := intFromMap(raw, "budgetEstimate"); ok {
		out["budgetEstimate"] = v
	}
	if v := stringFromMap(raw, "estimate"); v != "" {
		out["estimate"] = v
	}
	if v := stringFromMap(raw, "name"); v != "" {
		out["name"] = v
	}
	if v := stringFromMap(raw, "status"); v != "" {
		out["status"] = v
	}
	if ids, ok, err := stringSliceFromMap(raw, "user_group_ids"); err == nil && ok {
		out["userGroupIds"] = ids
	} else if ids, ok, err := stringSliceFromMap(raw, "userGroupIds"); err == nil && ok {
		out["userGroupIds"] = ids
	}
	return out
}

func nestedMap(raw map[string]any, snakeKey, camelKey string) (map[string]any, bool) {
	if v, ok := raw[snakeKey].(map[string]any); ok {
		return v, true
	}
	if v, ok := raw[camelKey].(map[string]any); ok {
		return v, true
	}
	return nil, false
}

func stringFromMap(raw map[string]any, key string) string {
	v, _ := raw[key].(string)
	return strings.TrimSpace(v)
}

func boolFromMap(raw map[string]any, key string) (bool, bool) {
	v, ok := raw[key].(bool)
	return v, ok
}

func numberFromMap(raw map[string]any, key string) (float64, bool) {
	v, ok := raw[key]
	if !ok {
		return 0, false
	}
	return numberFromAny(v)
}

func intFromMap(raw map[string]any, key string) (int64, bool) {
	n, ok := numberFromMap(raw, key)
	return int64(n), ok
}

func stringSliceFromMap(raw map[string]any, key string) ([]string, bool, error) {
	return strictStringSliceArg(raw, key)
}
