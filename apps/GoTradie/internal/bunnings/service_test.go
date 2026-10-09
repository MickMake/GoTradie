package bunnings

import (
	"testing"

	"github.com/MickMake/GoTradie/internal/config"
)

func TestNewUsesHierarchicalBunningsSettings(t *testing.T) {
	service, err := New(config.Config{Providers: map[string]config.ProviderConfig{
		"bunnings": {
			Environment:  "test",
			ClientID:     "client-id",
			ClientSecret: "client-secret",
			Scopes:       []string{"item.read", "pricing.read"},
			Country:      "NZ",
			Location:     "9473",
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if service.country != "NZ" || service.location != "9473" {
		t.Fatalf("country=%q location=%q", service.country, service.location)
	}
	if service.client.UserAgent != "GoTradie/v0.5.3" {
		t.Fatalf("user agent = %q", service.client.UserAgent)
	}
	if got := service.client.BaseURLs.Item; got != "https://item.stg.api.bunnings.com.au/item" {
		t.Fatalf("test environment Item URL = %q", got)
	}
}
