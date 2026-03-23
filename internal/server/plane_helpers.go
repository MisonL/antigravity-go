package server

import "fmt"

func summarizePlaneCollection(payload map[string]interface{}, preferredKeys []string, idKeys []string, timeKeys []string) planeSnapshot {
	records := extractPlaneRecords(payload, preferredKeys)
	snapshot := planeSnapshot{Count: len(records)}
	for _, record := range records {
		if snapshot.LatestID == "" {
			snapshot.LatestID = pickPlaneString(record, idKeys...)
		}
		if snapshot.LatestUpdatedAt == "" {
			snapshot.LatestUpdatedAt = pickPlaneString(record, timeKeys...)
		}
		if snapshot.LatestID != "" && snapshot.LatestUpdatedAt != "" {
			break
		}
	}
	return snapshot
}

func extractPlaneRecords(value interface{}, preferredKeys []string) []map[string]interface{} {
	if records := toPlaneRecordSlice(value); len(records) > 0 {
		return records
	}

	record, ok := value.(map[string]interface{})
	if !ok {
		return nil
	}

	for _, key := range preferredKeys {
		if records := toPlaneRecordSlice(record[key]); len(records) > 0 {
			return records
		}
	}

	return nil
}

func toPlaneRecordSlice(value interface{}) []map[string]interface{} {
	switch typed := value.(type) {
	case []interface{}:
		out := make([]map[string]interface{}, 0, len(typed))
		for _, item := range typed {
			record, ok := item.(map[string]interface{})
			if !ok {
				return nil
			}
			out = append(out, record)
		}
		return out
	case map[string]interface{}:
		if len(typed) == 0 {
			return nil
		}

		out := make([]map[string]interface{}, 0, len(typed))
		for _, item := range typed {
			record, ok := item.(map[string]interface{})
			if !ok {
				return nil
			}
			out = append(out, record)
		}
		return out
	default:
		return nil
	}
}

func pickPlaneString(record map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		value, ok := record[key]
		if !ok {
			continue
		}

		switch typed := value.(type) {
		case string:
			if typed != "" {
				return typed
			}
		case float64, bool, int, int64:
			return fmt.Sprintf("%v", typed)
		}
	}
	return ""
}
