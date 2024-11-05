package utils

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	harness "github.com/kudobuilder/kuttl/pkg/apis/testharness/v1beta1"
)

func TestNamespaced(t *testing.T) {
	fake := FakeDiscoveryClient()

	for _, test := range []struct {
		testName    string
		resource    runtime.Object
		namespace   string
		shouldError bool
	}{
		{
			testName:  "namespaced resource",
			resource:  NewPod("hello", ""),
			namespace: "set-the-namespace",
		},
		{
			testName:  "namespace already set",
			resource:  NewPod("hello", "other"),
			namespace: "other",
		},
		{
			testName:  "not-namespaced resource",
			resource:  NewResource("v1", "Namespace", "hello", ""),
			namespace: "",
		},
		{
			testName:    "non-existent resource",
			resource:    NewResource("v1", "Blah", "hello", ""),
			shouldError: true,
		},
	} {
		test := test

		t.Run(test.testName, func(t *testing.T) {
			m, _ := meta.Accessor(test.resource)

			actualName, actualNamespace, err := Namespaced(fake, test.resource, "set-the-namespace")

			if test.shouldError {
				assert.NotNil(t, err)
				assert.Equal(t, "", actualName)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, m.GetName(), actualName)
			}

			assert.Equal(t, test.namespace, actualNamespace)
			assert.Equal(t, test.namespace, m.GetNamespace())
		})
	}
}

func TestGETAPIResource(t *testing.T) {
	fake := FakeDiscoveryClient()

	apiResource, err := GetAPIResource(fake, schema.GroupVersionKind{
		Kind:    "Pod",
		Version: "v1",
	})
	assert.Nil(t, err)
	assert.Equal(t, apiResource.Kind, "Pod")

	_, err = GetAPIResource(fake, schema.GroupVersionKind{
		Kind:    "NonExistentResourceType",
		Version: "v1",
	})
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "resource type not found")
}

func TestRetry(t *testing.T) {
	index := 0

	assert.Nil(t, Retry(context.TODO(), func(context.Context) error {
		index++
		if index == 1 {
			return errors.New("ignore this error")
		}
		return nil
	}, func(err error) bool { return false }, func(err error) bool {
		return err.Error() == "ignore this error"
	}))

	assert.Equal(t, 2, index)
}

func TestRetryWithUnexpectedError(t *testing.T) {
	index := 0

	assert.Equal(t, errors.New("bad error"), Retry(context.TODO(), func(context.Context) error {
		index++
		if index == 1 {
			return errors.New("bad error")
		}
		return nil
	}, func(err error) bool { return false }, func(err error) bool {
		return err.Error() == "ignore this error"
	}))
	assert.Equal(t, 1, index)
}

func TestKubeconfigPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		override string
		expected string
	}{
		{name: "no-override", path: "foo", expected: "foo/kubeconfig"},
		{name: "override-relative", path: "foo", override: "bar/kubeconfig", expected: "foo/bar/kubeconfig"},
		{name: "override-abs", path: "foo", override: "/bar/kubeconfig", expected: "/bar/kubeconfig"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			result := kubeconfigPath(tt.path, tt.override)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRetryWithNil(t *testing.T) {
	assert.Equal(t, nil, Retry(context.TODO(), nil, IsJSONSyntaxError))
}

func TestRetryWithNilFromFn(t *testing.T) {
	assert.Equal(t, nil, Retry(context.TODO(), func(ctx context.Context) error {
		return nil
	}, IsJSONSyntaxError))
}

func TestRetryWithNilInFn(t *testing.T) {
	c := RetryClient{}
	var list client.ObjectList
	assert.Error(t, Retry(context.TODO(), func(ctx context.Context) error {
		return c.Client.List(ctx, list)
	}, IsJSONSyntaxError))
}

func TestRetryWithTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	assert.Equal(t, errors.New("error"), Retry(ctx, func(context.Context) error {
		return errors.New("error")
	}, func(err error) bool { return true }))
}

func TestLoadYAML(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "test.yaml")
	assert.Nil(t, err)
	defer tmpfile.Close()

	err = os.WriteFile(tmpfile.Name(), []byte(`
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: nginx
spec:
  containers:
  - name: nginx
    image: nginx:1.7.9
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: nginx
  name: hello
spec:
  containers:
  - name: nginx
    image: nginx:1.7.9
`), 0600)
	if err != nil {
		t.Fatal(err)
	}

	objs, err := LoadYAMLFromFile(tmpfile.Name())
	assert.Nil(t, err)

	assert.Equal(t, &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					"app": "nginx",
				},
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"image": "nginx:1.7.9",
						"name":  "nginx",
					},
				},
			},
		},
	}, objs[0])

	assert.Equal(t, &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					"app": "nginx",
				},
				"name": "hello",
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"image": "nginx:1.7.9",
						"name":  "nginx",
					},
				},
			},
		},
	}, objs[1])
}

func TestMatchesKind(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "test.yaml")
	assert.Nil(t, err)
	defer tmpfile.Close()

	err = os.WriteFile(tmpfile.Name(), []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: hello
spec:
  containers:
  - name: nginx
    image: nginx:1.7.9
---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: hello
`), 0600)
	if err != nil {
		t.Fatal(err)
	}

	objs, err := LoadYAMLFromFile(tmpfile.Name())
	assert.Nil(t, err)

	crd := NewResource("apiextensions.k8s.io/v1beta1", "CustomResourceDefinition", "", "")
	pod := NewResource("v1", "Pod", "", "")
	svc := NewResource("v1", "Service", "", "")

	assert.False(t, MatchesKind(objs[0], crd))
	assert.True(t, MatchesKind(objs[0], pod))
	assert.True(t, MatchesKind(objs[0], pod, crd))
	assert.True(t, MatchesKind(objs[0], crd, pod))
	assert.False(t, MatchesKind(objs[0], crd, svc))

	assert.True(t, MatchesKind(objs[1], crd))
	assert.False(t, MatchesKind(objs[1], pod))
	assert.True(t, MatchesKind(objs[1], pod, crd))
	assert.True(t, MatchesKind(objs[1], crd, pod))
	assert.False(t, MatchesKind(objs[1], svc, pod))
}

func TestGetKubectlArgs(t *testing.T) {
	for _, test := range []struct {
		testName  string
		namespace string
		args      string
		env       map[string]string
		expected  []string
	}{
		{
			testName:  "namespace long, combined already set at end is not modified",
			namespace: "default",
			args:      "kubectl kuttl test --namespace=test-canary",
			expected: []string{
				"kubectl", "kuttl", "test", "--namespace=test-canary",
			},
		},
		{
			testName:  "namespace long already set at end is not modified",
			namespace: "default",
			args:      "kubectl kuttl test --namespace test-canary",
			expected: []string{
				"kubectl", "kuttl", "test", "--namespace", "test-canary",
			},
		},
		{
			testName:  "namespace short, combined already set at end is not modified",
			namespace: "default",
			args:      "kubectl kuttl test -n=test-canary",
			expected: []string{
				"kubectl", "kuttl", "test", "-n=test-canary",
			},
		},
		{
			testName:  "namespace short already set at end is not modified",
			namespace: "default",
			args:      "kubectl kuttl test -n test-canary",
			expected: []string{
				"kubectl", "kuttl", "test", "-n", "test-canary",
			},
		},
		{
			testName:  "namespace long, combined already set in middle is not modified",
			namespace: "default",
			args:      "kubectl kuttl --namespace=test-canary test",
			expected: []string{
				"kubectl", "kuttl", "--namespace=test-canary", "test",
			},
		},
		{
			testName:  "namespace long already set in middle is not modified",
			namespace: "default",
			args:      "kubectl kuttl --namespace test-canary test",
			expected: []string{
				"kubectl", "kuttl", "--namespace", "test-canary", "test",
			},
		},
		{
			testName:  "namespace short, combined already set in middle is not modified",
			namespace: "default",
			args:      "kubectl kuttl -n=test-canary test",
			expected: []string{
				"kubectl", "kuttl", "-n=test-canary", "test",
			},
		},
		{
			testName:  "namespace short already set in middle is not modified",
			namespace: "default",
			args:      "kubectl kuttl -n test-canary test",
			expected: []string{
				"kubectl", "kuttl", "-n", "test-canary", "test",
			},
		},
		{
			testName:  "namespace not set is appended",
			namespace: "default",
			args:      "kubectl kuttl test",
			expected: []string{
				"kubectl", "kuttl", "test", "--namespace", "default",
			},
		},
		{
			testName:  "unknown arguments do not break parsing with namespace is not set",
			namespace: "default",
			args:      "kubectl kuttl test --config kuttl-test.yaml",
			expected: []string{
				"kubectl", "kuttl", "test", "--config", "kuttl-test.yaml", "--namespace", "default",
			},
		},
		{
			testName:  "unknown arguments do not break parsing if namespace is set at beginning",
			namespace: "default",
			args:      "kubectl --namespace=test-canary kuttl test --config kuttl-test.yaml",
			expected: []string{
				"kubectl", "--namespace=test-canary", "kuttl", "test", "--config", "kuttl-test.yaml",
			},
		},
		{
			testName:  "unknown arguments do not break parsing if namespace is set at middle",
			namespace: "default",
			args:      "kubectl kuttl --namespace=test-canary test --config kuttl-test.yaml",
			expected: []string{
				"kubectl", "kuttl", "--namespace=test-canary", "test", "--config", "kuttl-test.yaml",
			},
		},
		{
			testName:  "unknown arguments do not break parsing if namespace is set at end",
			namespace: "default",
			args:      "kubectl kuttl test --config kuttl-test.yaml --namespace=test-canary",
			expected: []string{
				"kubectl", "kuttl", "test", "--config", "kuttl-test.yaml", "--namespace=test-canary",
			},
		},
		{
			testName:  "quotes are respected when parsing",
			namespace: "default",
			args:      "kubectl kuttl \"test quoted\"",
			expected: []string{
				"kubectl", "kuttl", "test quoted", "--namespace", "default",
			},
		},
		{
			testName:  "os ENV are expanded",
			namespace: "default",
			args:      "kubectl kuttl $TEST_FOO ${TEST_FOO}",
			env:       map[string]string{"TEST_FOO": "test"},
			expected: []string{
				"kubectl", "kuttl", "test", "test", "--namespace", "default",
			},
		},
		{
			testName:  "kubectl is not pre-pended if it is already present",
			namespace: "default",
			args:      "kubectl kuttl test",
			expected: []string{
				"kubectl", "kuttl", "test", "--namespace", "default",
			},
		},
	} {
		test := test

		t.Run(test.testName, func(t *testing.T) {
			if test.env != nil || len(test.env) > 0 {
				for key, value := range test.env {
					os.Setenv(key, value)
				}
				defer func() {
					for key := range test.env {
						os.Unsetenv(key)
					}
				}()
			}
			cmd, err := GetArgs(context.TODO(), harness.Command{
				Command:    test.args,
				Namespaced: true,
			}, test.namespace, nil)
			assert.Nil(t, err)
			assert.Equal(t, test.expected, cmd.Args)
		})
	}
}

func TestRunScript(t *testing.T) {
	tests := []struct {
		name           string
		command        string
		script         string
		wantedErr      bool
		expectedStdout bool
	}{
		{
			name:           `no script and no command`,
			command:        "",
			script:         "",
			wantedErr:      true,
			expectedStdout: false,
		},
		{
			name:           `script AND command`,
			command:        "echo 'hello'",
			script:         "for i in {1..5}; do echo $NAMESPACE; done",
			wantedErr:      true,
			expectedStdout: false,
		},
		// failure for script command as a command (reason we need a script script option)
		{
			name:           `command has a failing script command`,
			command:        "for i in {1..5}; do echo $NAMESPACE; done",
			script:         "",
			wantedErr:      true,
			expectedStdout: false,
		},
		{
			name:           `working script command`,
			command:        "",
			script:         "for i in {1..5}; do echo $NAMESPACE; done",
			wantedErr:      false,
			expectedStdout: true,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			hcmd := harness.Command{
				Command: tt.command,
				Script:  tt.script,
			}

			logger := NewTestLogger(t, "")
			// script runs with output
			_, err := RunCommand(context.TODO(), "", hcmd, "", stdout, stderr, logger, 0, "")

			if tt.wantedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if tt.expectedStdout {
				assert.True(t, stdout.Len() > 0)
			} else {
				assert.True(t, stdout.Len() == 0)
			}
		})
	}
}

func TestPrettyDiff(t *testing.T) {
	actual, err := LoadYAMLFromFile("test_data/prettydiff-actual.yaml")
	assert.NoError(t, err)
	assert.Len(t, actual, 1)
	expected, err := LoadYAMLFromFile("test_data/prettydiff-expected.yaml")
	assert.NoError(t, err)
	assert.Len(t, expected, 1)

	result, err := PrettyDiff(expected[0].(*unstructured.Unstructured), actual[0].(*unstructured.Unstructured))
	assert.NoError(t, err)
	assert.Equal(t, "\n\033[31m--- Deployment:/central\033[0m\n"+
		"\033[32m+++ Deployment:kuttl-test-thorough-hermit/central\033[0m\n"+
		"@@ -1,7 +1,35 @@\n"+
		" apiVersion: apps/v1\n"+
		" kind: Deployment\n"+
		" metadata:\n"+
		"\033[32m+  annotations:\033[0m\n"+
		"\033[32m+    email: support@stackrox.com\033[0m\n"+
		"\033[32m+    meta.helm.sh/release-name: stackrox-central-services\033[0m\n"+
		"\033[32m+    meta.helm.sh/release-namespace: kuttl-test-thorough-hermit\033[0m\n"+
		"\033[32m+    owner: stackrox\033[0m\n"+
		"\033[32m+  labels:\033[0m\n"+
		"\033[32m+    app: central\033[0m\n"+
		"\033[32m+    app.kubernetes.io/component: central\033[0m\n"+
		"\033[32m+    app.kubernetes.io/instance: stackrox-central-services\033[0m\n"+
		"\033[32m+    app.kubernetes.io/managed-by: Helm\033[0m\n"+
		"\033[32m+    app.kubernetes.io/name: stackrox\033[0m\n"+
		"\033[32m+    app.kubernetes.io/part-of: stackrox-central-services\033[0m\n"+
		"\033[32m+    app.kubernetes.io/version: 4.3.x-160-g465d734c11\033[0m\n"+
		"\033[32m+    helm.sh/chart: stackrox-central-services-400.3.0-160-g465d734c11\033[0m\n"+
		"\033[32m+  managedFields: '[... elided field over 10 lines long ...]'\033[0m\n"+
		"   name: central\n"+
		"\033[32m+  namespace: kuttl-test-thorough-hermit\033[0m\n"+
		"\033[32m+  ownerReferences:\033[0m\n"+
		"\033[32m+  - apiVersion: platform.stackrox.io/v1alpha1\033[0m\n"+
		"\033[32m+    blockOwnerDeletion: true\033[0m\n"+
		"\033[32m+    controller: true\033[0m\n"+
		"\033[32m+    kind: Central\033[0m\n"+
		"\033[32m+    name: stackrox-central-services\033[0m\n"+
		"\033[32m+    uid: ff834d91-0853-42b3-9460-7ebf1c659f8a\033[0m\n"+
		"\033[32m+spec: '[... elided field over 10 lines long ...]'\033[0m\n"+
		" status:\n"+
		"\033[31m-  availableReplicas: 1\033[0m\n"+
		"\033[32m+  conditions: '[... elided field over 10 lines long ...]'\033[0m\n"+
		"\033[32m+  observedGeneration: 2\033[0m\n"+
		"\033[32m+  replicas: 1\033[0m\n"+
		"\033[32m+  unavailableReplicas: 1\033[0m\n"+
		"\033[32m+  updatedReplicas: 1\033[0m\n \n", result)
}
