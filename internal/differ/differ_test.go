package differ

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNewDiffer(t *testing.T) {
	d := NewDiffer([]string{"spec.template.spec.containers[*].image"})
	if d == nil {
		t.Fatal("expected non-nil differ")
	}

	// Verify defaults are present
	foundResourceVersion := false
	for _, ip := range d.IgnoredPaths {
		if ip == "metadata.resourceVersion" {
			foundResourceVersion = true
		}
	}
	if !foundResourceVersion {
		t.Error("expected metadata.resourceVersion in default ignored paths")
	}
}

func TestCompareNoDrift(t *testing.T) {
	desired := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "nginx-deployment",
				"namespace": "default",
				"labels": map[string]interface{}{
					"app": "nginx",
				},
			},
			"spec": map[string]interface{}{
				"replicas": int64(3),
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": "nginx",
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app": "nginx",
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "nginx",
								"image": "nginx:1.14.2",
								"ports": []interface{}{
									map[string]interface{}{
										"containerPort": int64(80),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Live object has same values, plus dynamic K8s fields
	live := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":            "nginx-deployment",
				"namespace":       "default",
				"resourceVersion": "12345",
				"uid":             "abcdef-12345",
				"creationTimestamp": "2026-05-17T12:00:00Z",
				"labels": map[string]interface{}{
					"app": "nginx",
				},
			},
			"spec": map[string]interface{}{
				"replicas": float64(3), // parsed from json usually
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": "nginx",
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app": "nginx",
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "nginx",
								"image": "nginx:1.14.2",
								"ports": []interface{}{
									map[string]interface{}{
										"containerPort": float64(80),
									},
								},
							},
						},
					},
				},
			},
			"status": map[string]interface{}{
				"replicas": float64(3),
			},
		},
	}

	d := NewDiffer(nil)
	diffs, err := d.Compare(desired, live)
	if err != nil {
		t.Fatalf("unexpected error comparing: %v", err)
	}

	if len(diffs) > 0 {
		t.Errorf("expected 0 diffs, got %d: %v", len(diffs), diffs)
	}
}

func TestCompareWithDrift(t *testing.T) {
	desired := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"spec": map[string]interface{}{
				"replicas": int64(3),
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "nginx",
								"image": "nginx:1.14.2",
							},
						},
					},
				},
			},
		},
	}

	live := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"spec": map[string]interface{}{
				"replicas": int64(1), // Drift!
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "nginx",
								"image": "nginx:1.19.0", // Drift!
							},
						},
					},
				},
			},
		},
	}

	d := NewDiffer(nil)
	diffs, err := d.Compare(desired, live)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(diffs) != 2 {
		t.Fatalf("expected 2 diffs, got %d", len(diffs))
	}

	// Verify replica drift
	foundReplicas := false
	foundImage := false
	for _, diff := range diffs {
		if diff.Path == "spec.replicas" {
			foundReplicas = true
			if diff.Expected.(int64) != 3 || diff.Actual.(int64) != 1 {
				t.Errorf("incorrect replica diff: expected 3 vs 1, got %v vs %v", diff.Expected, diff.Actual)
			}
		}
		if diff.Path == "spec.template.spec.containers[0].image" {
			foundImage = true
			if diff.Expected != "nginx:1.14.2" || diff.Actual != "nginx:1.19.0" {
				t.Errorf("incorrect image diff: expected nginx:1.14.2 vs nginx:1.19.0, got %v vs %v", diff.Expected, diff.Actual)
			}
		}
	}

	if !foundReplicas {
		t.Error("expected drift in spec.replicas")
	}
	if !foundImage {
		t.Error("expected drift in spec.template.spec.containers[0].image")
	}
}

func TestCompareWithIgnoredPaths(t *testing.T) {
	desired := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"spec": map[string]interface{}{
				"replicas": int64(3),
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "nginx",
								"image": "nginx:1.14.2",
							},
						},
					},
				},
			},
		},
	}

	live := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"spec": map[string]interface{}{
				"replicas": int64(1), // Drift!
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "nginx",
								"image": "nginx:1.19.0", // Drift, but ignored!
							},
						},
					},
				},
			},
		},
	}

	// Ignore image drift using wildcard index
	d := NewDiffer([]string{"spec.template.spec.containers[*].image"})
	diffs, err := d.Compare(desired, live)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d: %v", len(diffs), diffs)
	}

	if diffs[0].Path != "spec.replicas" {
		t.Errorf("expected drift path to be 'spec.replicas', got %q", diffs[0].Path)
	}
}
