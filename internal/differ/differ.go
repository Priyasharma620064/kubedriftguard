// Package differ provides comparison engines to detect configuration drift between
// the desired state (Git) and the actual state (live cluster).
package differ

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var arrayIndexRegex = regexp.MustCompile(`\[\d+\]`)

// DiffField represents a single field that has drifted.
type DiffField struct {
	Path     string      `json:"path"`     // JSON path of the field (e.g., "spec.replicas" or "spec.template.spec.containers[0].image")
	Expected interface{} `json:"expected"` // Value declared in Git (desired state)
	Actual   interface{} `json:"actual"`   // Value present in cluster (live state)
}

// Differ performs comparisons between desired and live Kubernetes objects.
type Differ struct {
	// IgnoredPaths is a list of JSON paths that should be bypassed during comparison.
	IgnoredPaths []string
}

// NewDiffer creates a new Differ instance with standard ignored paths.
func NewDiffer(ignoredPaths []string) *Differ {
	// Add default metadata fields that are dynamically added/modified by K8s
	defaults := []string{
		"metadata.resourceVersion",
		"metadata.uid",
		"metadata.generation",
		"metadata.creationTimestamp",
		"metadata.selfLink",
		"metadata.managedFields",
		"status",
	}
	
	merged := make([]string, 0, len(defaults)+len(ignoredPaths))
	merged = append(merged, defaults...)
	
	// Normalize and append user-defined ignored paths
	for _, p := range ignoredPaths {
		merged = append(merged, normalizePath(p))
	}

	return &Differ{
		IgnoredPaths: merged,
	}
}

// Compare checks if the live object has drifted from the desired object.
// It recursively compares fields defined in the desired object.
// If a field is defined in 'desired', it must exist and match in 'live'.
// Fields in 'live' that are not defined in 'desired' are ignored (defaults/status).
func (d *Differ) Compare(desired, live *unstructured.Unstructured) ([]DiffField, error) {
	if desired == nil || live == nil {
		return nil, fmt.Errorf("desired and live objects must not be nil")
	}

	var diffs []DiffField
	err := d.compareMaps(desired.Object, live.Object, "", &diffs)
	if err != nil {
		return nil, err
	}

	return diffs, nil
}

// compareMaps recursively compares two maps based on the keys present in the desired map.
func (d *Differ) compareMaps(desired, live map[string]interface{}, currentPath string, diffs *[]DiffField) error {
	for key, desiredVal := range desired {
		path := key
		if currentPath != "" {
			path = currentPath + "." + key
		}

		if d.isIgnored(path) {
			continue
		}

		liveVal, exists := live[key]
		if !exists {
			// Field is declared in Git but missing in the cluster
			*diffs = append(*diffs, DiffField{
				Path:     path,
				Expected: desiredVal,
				Actual:   nil,
			})
			continue
		}

		if err := d.compareValues(desiredVal, liveVal, path, diffs); err != nil {
			return err
		}
	}
	return nil
}

// compareValues compares two values of arbitrary type.
func (d *Differ) compareValues(desired, live interface{}, path string, diffs *[]DiffField) error {
	if desired == nil && live == nil {
		return nil
	}

	// Retrieve underlying values to resolve any pointer or interface wrapping
	dVal := reflect.ValueOf(desired)
	lVal := reflect.ValueOf(live)

	// If one is nil and other is not
	if desired == nil || live == nil {
		*diffs = append(*diffs, DiffField{
			Path:     path,
			Expected: desired,
			Actual:   live,
		})
		return nil
	}

	// Handle maps
	if dVal.Kind() == reflect.Map && lVal.Kind() == reflect.Map {
		dMap, ok1 := desired.(map[string]interface{})
		lMap, ok2 := live.(map[string]interface{})
		if ok1 && ok2 {
			return d.compareMaps(dMap, lMap, path, diffs)
		}
		// Fallback for map types that aren't map[string]interface{}
		return d.compareReflectMaps(dVal, lVal, path, diffs)
	}

	// Handle slices/arrays
	if (dVal.Kind() == reflect.Slice || dVal.Kind() == reflect.Array) &&
		(lVal.Kind() == reflect.Slice || lVal.Kind() == reflect.Array) {
		return d.compareSlices(dVal, lVal, path, diffs)
	}

	// For primitive values, check equality.
	// Kubernetes API server can return numeric values as float64 from JSON parsing.
	// We handle type conversions gracefully (e.g. float64 vs int64).
	equal := valuesAreEqual(desired, live)
	if !equal {
		*diffs = append(*diffs, DiffField{
			Path:     path,
			Expected: desired,
			Actual:   live,
		})
	}

	return nil
}

// compareReflectMaps handles map comparison using reflection for edge cases.
func (d *Differ) compareReflectMaps(desired, live reflect.Value, path string, diffs *[]DiffField) error {
	for _, key := range desired.MapKeys() {
		keyStr := fmt.Sprintf("%v", key.Interface())
		fieldPath := path + "." + keyStr

		if d.isIgnored(fieldPath) {
			continue
		}

		desiredVal := desired.MapIndex(key).Interface()
		liveVal := live.MapIndex(key)

		if !liveVal.IsValid() {
			*diffs = append(*diffs, DiffField{
				Path:     fieldPath,
				Expected: desiredVal,
				Actual:   nil,
			})
			continue
		}

		if err := d.compareValues(desiredVal, liveVal.Interface(), fieldPath, diffs); err != nil {
			return err
		}
	}
	return nil
}

// compareSlices recursively compares elements in a slice/array.
func (d *Differ) compareSlices(desired, live reflect.Value, path string, diffs *[]DiffField) error {
	dLen := desired.Len()
	lLen := live.Len()

	// If lengths are different, it's an immediate drift
	if dLen != lLen {
		*diffs = append(*diffs, DiffField{
			Path:     path,
			Expected: desired.Interface(),
			Actual:   live.Interface(),
		})
		return nil
	}

	for i := 0; i < dLen; i++ {
		indexPath := fmt.Sprintf("%s[%d]", path, i)
		if err := d.compareValues(desired.Index(i).Interface(), live.Index(i).Interface(), indexPath, diffs); err != nil {
			return err
		}
	}

	return nil
}

// isIgnored checks if the current path matches any ignored path pattern.
func (d *Differ) isIgnored(path string) bool {
	normPath := normalizePath(path)
	for _, ip := range d.IgnoredPaths {
		// Exact match or matches a wildcard array notation like spec.containers[*].image
		if normPath == ip || matchWildcard(normPath, ip) {
			return true
		}
	}
	return false
}

// normalizePath replaces bracketed array indexes with wildcard or cleans path format.
func normalizePath(path string) string {
	return arrayIndexRegex.ReplaceAllString(strings.TrimSpace(path), "[*]")
}

// matchWildcard supports matching spec.containers[0].image with spec.containers[*].image
func matchWildcard(path, pattern string) bool {
	if !strings.Contains(pattern, "[*]") {
		return false
	}
	
	// Convert array indices to [*] in path to compare against pattern
	normalized := arrayIndexRegex.ReplaceAllString(path, "[*]")
	return normalized == pattern
}

// valuesAreEqual handles fuzzy numeric comparisons and string representation matches
func valuesAreEqual(a, b interface{}) bool {
	if reflect.DeepEqual(a, b) {
		return true
	}

	// Try numeric conversions
	aNum, okA := toFloat64(a)
	bNum, okB := toFloat64(b)
	if okA && okB {
		return aNum == bNum
	}

	// Fallback to string comparison for primitive types
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// toFloat64 converts generic interface to float64 if possible
func toFloat64(val interface{}) (float64, bool) {
	switch v := val.(type) {
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case float32:
		return float64(v), true
	case float64:
		return v, true
	}
	return 0, false
}
