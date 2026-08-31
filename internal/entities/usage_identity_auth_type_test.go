package entities

import "testing"

func TestUsageIdentityAuthTypeCanonicalMapping(t *testing.T) {
	tests := []struct {
		name      string
		authType  UsageIdentityAuthType
		canonical string
	}{
		{name: "auth file", authType: UsageIdentityAuthTypeAuthFile, canonical: UsageIdentityAuthTypeNameOAuth},
		{name: "ai provider", authType: UsageIdentityAuthTypeAIProvider, canonical: UsageIdentityAuthTypeNameAPIKey},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if !test.authType.Valid() {
				t.Fatalf("expected %d to be valid", test.authType)
			}
			name, ok := test.authType.CanonicalName()
			if !ok || name != test.canonical {
				t.Fatalf("canonical name = %q, %t; want %q, true", name, ok, test.canonical)
			}
			parsed, ok := ParseUsageIdentityAuthType(name)
			if !ok || parsed != test.authType {
				t.Fatalf("parsed auth type = %d, %t; want %d, true", parsed, ok, test.authType)
			}
		})
	}
}

func TestUsageIdentityAuthTypeRejectsUnknownMappings(t *testing.T) {
	if UsageIdentityAuthType(0).Valid() || UsageIdentityAuthType(3).Valid() {
		t.Fatal("unknown numeric identity kinds must be invalid")
	}
	if name, ok := UsageIdentityAuthType(99).CanonicalName(); ok || name != "" {
		t.Fatalf("unknown canonical name = %q, %t; want empty, false", name, ok)
	}
	for _, name := range []string{"", "api_key", "OAuth", "unknown"} {
		if parsed, ok := ParseUsageIdentityAuthType(name); ok || parsed != 0 {
			t.Fatalf("ParseUsageIdentityAuthType(%q) = %d, %t; want zero, false", name, parsed, ok)
		}
	}
}
