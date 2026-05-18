package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func assertJSONMatchesContractFixture(t *testing.T, actual []byte, fixtureName string) {
	t.Helper()
	fixturePath := filepath.Join("..", "..", "web", "src", "test", "contracts", fixtureName)
	expected, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read contract fixture %s: %v", fixtureName, err)
	}
	var expectedValue any
	if err := json.Unmarshal(expected, &expectedValue); err != nil {
		t.Fatalf("decode contract fixture %s: %v", fixtureName, err)
	}
	var actualValue any
	if err := json.Unmarshal(actual, &actualValue); err != nil {
		t.Fatalf("decode actual contract JSON: %v\n%s", err, string(actual))
	}
	if !reflect.DeepEqual(actualValue, expectedValue) {
		expectedPretty, _ := json.MarshalIndent(expectedValue, "", "  ")
		actualPretty, _ := json.MarshalIndent(actualValue, "", "  ")
		t.Fatalf("contract fixture %s mismatch\nexpected:\n%s\nactual:\n%s", fixtureName, expectedPretty, actualPretty)
	}
}
