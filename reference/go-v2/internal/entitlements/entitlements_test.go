package entitlements_test

import (
	"testing"

	"github.com/its-aryansingh/qrit/services/internal/entitlements"
)

func TestPlanLimits(t *testing.T) {
	freeLimits := entitlements.GetLimits("free")
	if freeLimits.DynamicCodes != 3 {
		t.Errorf("Free dynamic codes = %d; want 3", freeLimits.DynamicCodes)
	}

	proLimits := entitlements.GetLimits("pro")
	if proLimits.DynamicCodes != 100 || proLimits.CustomDomains != 1 {
		t.Errorf("Pro limits incorrect: %+v", proLimits)
	}

	bizLimits := entitlements.GetLimits("business")
	if bizLimits.DynamicCodes != 1000 || bizLimits.APIRequestsPerMin != 600 {
		t.Errorf("Business limits incorrect: %+v", bizLimits)
	}

	entLimits := entitlements.GetLimits("enterprise")
	if entLimits.DynamicCodes != 100000 || entLimits.CustomDomains != 50 {
		t.Errorf("Enterprise limits incorrect: %+v", entLimits)
	}

	unknownLimits := entitlements.GetLimits("unknown_plan")
	if unknownLimits.DynamicCodes != 3 {
		t.Errorf("Unknown plan should fallback to Free (3 codes), got %d", unknownLimits.DynamicCodes)
	}
}

func TestFeatures(t *testing.T) {
	// Free has no advanced features
	if entitlements.HasFeature("free", entitlements.FeatureScheduling) {
		t.Errorf("Free should not have scheduling")
	}

	// Pro has scheduling, templates, remove_branding
	if !entitlements.HasFeature("pro", entitlements.FeatureScheduling) {
		t.Errorf("Pro should have scheduling")
	}
	if !entitlements.HasFeature("pro", entitlements.FeatureTemplates) {
		t.Errorf("Pro should have templates")
	}
	if entitlements.HasFeature("pro", entitlements.FeatureAPI) {
		t.Errorf("Pro should not have API")
	}

	// Business has Pro features + rules, campaigns, webhooks, api
	if !entitlements.HasFeature("business", entitlements.FeatureScheduling) {
		t.Errorf("Business should inherit Pro scheduling")
	}
	if !entitlements.HasFeature("business", entitlements.FeatureAPI) {
		t.Errorf("Business should have API")
	}
	if entitlements.HasFeature("business", entitlements.FeatureSSO) {
		t.Errorf("Business should not have SSO")
	}

	// Enterprise has all features
	if !entitlements.HasFeature("enterprise", entitlements.FeatureSSO) {
		t.Errorf("Enterprise should have SSO")
	}
	if !entitlements.HasFeature("enterprise", entitlements.FeatureAPI) {
		t.Errorf("Enterprise should have API")
	}
}

func TestDynamicCodeLimit(t *testing.T) {
	if err := entitlements.CheckDynamicCodeLimit("free", 2); err != nil {
		t.Errorf("unexpected error at 2 codes: %v", err)
	}

	err := entitlements.CheckDynamicCodeLimit("free", 3)
	if err == nil {
		t.Fatalf("expected error at 3 codes on free plan")
	}
	entErr, ok := err.(*entitlements.EntitlementError)
	if !ok || entErr.Code != "limit_reached" || entErr.RequiredPlan != entitlements.PlanPro {
		t.Errorf("unexpected error details: %+v", entErr)
	}
}
