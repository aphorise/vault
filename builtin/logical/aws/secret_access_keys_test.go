package aws

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/vault/sdk/logical"
	"github.com/stretchr/testify/require"
)

func TestNormalizeDisplayName_NormRequired(t *testing.T) {
	invalidNames := map[string]string{
		"^#$test name\nshould be normalized)(*": "___test_name_should_be_normalized___",
		"^#$test name1 should be normalized)(*": "___test_name1_should_be_normalized___",
		"^#$test name  should be normalized)(*": "___test_name__should_be_normalized___",
		"^#$test name__should be normalized)(*": "___test_name__should_be_normalized___",
	}

	for k, v := range invalidNames {
		normalizedName := normalizeDisplayName(k)
		if normalizedName != v {
			t.Fatalf(
				"normalizeDisplayName does not normalize AWS name correctly: %s should resolve to %s",
				k,
				normalizedName)
		}
	}
}

func TestNormalizeDisplayName_NormNotRequired(t *testing.T) {
	validNames := []string{
		"test_name_should_normalize_to_itself@example.com",
		"test1_name_should_normalize_to_itself@example.com",
		"UPPERlower0123456789-_,.@example.com",
	}

	for _, n := range validNames {
		normalizedName := normalizeDisplayName(n)
		if normalizedName != n {
			t.Fatalf(
				"normalizeDisplayName erroneously normalizes valid names: expected %s but normalized to %s",
				n,
				normalizedName)
		}
	}
}

func TestGenUsername(t *testing.T) {
	type testCase struct {
		name             string
		policy           string
		userType         string
		UsernameTemplate string
		expectedError    string
		expectedRegex    string
		expectedLength   int
	}

	tests := map[string]testCase{
		"Truncated to 64. No warnings expected": {
			name:             "name1",
			policy:           "policy1",
			userType:         "iam_user",
			UsernameTemplate: defaultUserNameTemplate,
			expectedError:    "",
			expectedRegex:    `^vault-name1-policy1-[0-9]+-[a-zA-Z0-9]+`,
			expectedLength:   64,
		},
		"Truncated to 32. No warnings expected": {
			name:             "name1",
			policy:           "policy1",
			userType:         "sts",
			UsernameTemplate: defaultUserNameTemplate,
			expectedError:    "",
			expectedRegex:    `^vault-[0-9]+-[a-zA-Z0-9]+`,
			expectedLength:   32,
		},
		"Too long. Error expected — IAM": {
			name:             "this---is---a---very---long---name",
			policy:           "long------policy------name",
			userType:         "assume_role",
			UsernameTemplate: `{{ if (eq .Type "IAM") }}{{ printf "%s-%s-%s-%s" (.DisplayName) (.PolicyName) (unix_time) (random 20) }}{{ end }}`,
			expectedError:    "the username generated by the template exceeds the IAM username length limits of 64 chars",
			expectedRegex:    "",
			expectedLength:   64,
		},
		"Too long. Error expected — STS": {
			name:             "this---is---a---very---long---name",
			policy:           "long------policy------name",
			userType:         "sts",
			UsernameTemplate: `{{ if (eq .Type "STS") }}{{ printf "%s-%s-%s-%s" (.DisplayName) (.PolicyName) (unix_time) (random 20) }}{{ end }}`,
			expectedError:    "the username generated by the template exceeds the STS username length limits of 32 chars",
			expectedRegex:    "",
			expectedLength:   32,
		},
	}

	for testDescription, testCase := range tests {
		t.Run(testDescription, func(t *testing.T) {
			testUsername, err := genUsername(testCase.name, testCase.policy, testCase.userType, testCase.UsernameTemplate)
			if err != nil && !strings.Contains(err.Error(), testCase.expectedError) {
				t.Fatalf("expected an error %s; instead received %s", testCase.expectedError, err)
			}

			if err == nil {
				require.Regexp(t, testCase.expectedRegex, testUsername)

				if len(testUsername) > testCase.expectedLength {
					t.Fatalf("expected username to be of length %d, got %d", testCase.expectedLength, len(testUsername))
				}
			}
		})
	}
}

func TestReadConfig_DefaultTemplate(t *testing.T) {
	config := logical.TestBackendConfig()
	config.StorageView = &logical.InmemStorage{}
	b := Backend()
	if err := b.Setup(context.Background(), config); err != nil {
		t.Fatal(err)
	}

	testTemplate := ""
	configData := map[string]interface{}{
		"connection_uri":    "test_uri",
		"username":          "guest",
		"password":          "guest",
		"username_template": testTemplate,
	}
	configReq := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "config/root",
		Storage:   config.StorageView,
		Data:      configData,
	}
	resp, err := b.HandleRequest(context.Background(), configReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v\nerr:%s", resp, err)
	}
	if resp != nil {
		t.Fatal("expected a nil response")
	}

	configResult, err := readConfig(context.Background(), config.StorageView)
	if err != nil {
		t.Fatalf("expected err to be nil; got %s", err)
	}

	// No template provided, config set to defaultUsernameTemplate
	if configResult.UsernameTemplate != defaultUserNameTemplate {
		t.Fatalf(
			"expected template %s; got %s",
			defaultUserNameTemplate,
			configResult.UsernameTemplate,
		)
	}
}

func TestReadConfig_CustomTemplate(t *testing.T) {
	config := logical.TestBackendConfig()
	config.StorageView = &logical.InmemStorage{}
	b := Backend()
	if err := b.Setup(context.Background(), config); err != nil {
		t.Fatal(err)
	}

	testTemplate := "`foo-{{ .DisplayName }}`"
	configData := map[string]interface{}{
		"connection_uri":    "test_uri",
		"username":          "guest",
		"password":          "guest",
		"username_template": testTemplate,
	}
	configReq := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "config/root",
		Storage:   config.StorageView,
		Data:      configData,
	}
	resp, err := b.HandleRequest(context.Background(), configReq)
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("bad: resp: %#v\nerr:%s", resp, err)
	}
	if resp != nil {
		t.Fatal("expected a nil response")
	}

	configResult, err := readConfig(context.Background(), config.StorageView)
	if err != nil {
		t.Fatalf("expected err to be nil; got %s", err)
	}

	if configResult.UsernameTemplate != testTemplate {
		t.Fatalf(
			"expected template %s; got %s",
			testTemplate,
			configResult.UsernameTemplate,
		)
	}
}
