package science

import (
	"encoding/json"
	"net/http"
	"reflect"

	"github.com/Clever/http-science/config"
)

// cleanupHeaders removes headers that can be different for inconsequential reasons
func cleanupHeaders(res *http.Response, cleanup []string) {
	for _, val := range cleanup {
		delete(res.Header, val)
		if val == "Content-Length" {
			res.ContentLength = 0
		} else if val == "Transfer-Encoding" {
			res.TransferEncoding = nil
		}
	}
}

func codesAreEqual(control, experiment int) bool {
	return control == experiment
}

func headersAreEqual(control, experiment http.Header) bool {
	return reflect.DeepEqual(control, experiment)
}

func bodiesAreEqual(control, experiment []byte) bool {
	if isSimplyEqual(control, experiment) {
		return true
	}
	return isJSONEqual(control, experiment)
}

// isSimplyEqual returns true if the two strings are equal, false otherwise
func isSimplyEqual(dumpControl, dumpExperiment []byte) bool {
	return (string(dumpControl) == string(dumpExperiment))
}

// isJSONEqual compares two responses and returns true if they are equivalent
// it ignores ordering of keys and elements in maps and arrays in the body
// it will be fairly inefficient for large objects (n^2 on arrays)
func isJSONEqual(resControl, resExperiment []byte) bool {
	var controlJSON, expJSON map[string]interface{}
	// Return false if they can't be parsed as JSON
	if err := json.Unmarshal(resControl, &controlJSON); err != nil {
		return false
	}
	if err := json.Unmarshal(resExperiment, &expJSON); err != nil {
		return false
	}

	if config.WeakCompare {
		return msiAreEqual(controlJSON, expJSON)
	}
	return reflect.DeepEqual(controlJSON, expJSON)
}

func msiAreEqual(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}

	areEqual := true
	for k, v := range a {
		switch v := v.(type) {
		case []interface{}:
			bv, ok := b[k].([]interface{})
			if !ok {
				return false
			}
			areEqual = areEqual && sliceAreEqual(v, bv)
		case map[string]interface{}:
			bv, ok := b[k].(map[string]interface{})
			if !ok {
				return false
			}
			areEqual = areEqual && msiAreEqual(v, bv)
		default:
			bv, ok := b[k]
			if !ok {
				return false
			}
			areEqual = areEqual && reflect.DeepEqual(v, bv)
		}
		// Early exit if we find a diff
		if !areEqual {
			return false
		}
	}
	return true
}

func sliceAreEqual(a, b []interface{}) bool {
	if len(a) != len(b) {
		return false
	}

	for _, v := range a {
		areEqual := false
		for _, bv := range b {

			switch v := v.(type) {
			case []interface{}:
				bv, ok := bv.([]interface{})
				if !ok {
					continue
				}
				areEqual = sliceAreEqual(v, bv)
			case map[string]interface{}:
				bv, ok := bv.(map[string]interface{})
				if !ok {
					continue
				}
				areEqual = msiAreEqual(v, bv)
			default:
				areEqual = reflect.DeepEqual(v, bv)
			}
			// Early exit inner loop once we find a match for this item
			if areEqual {
				break
			}
		}
		// Early exit outer loop if we didn't find a match for this item
		if !areEqual {
			return false
		}
	}
	return true
}
