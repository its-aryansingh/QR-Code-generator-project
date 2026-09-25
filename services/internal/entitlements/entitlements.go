package entitlements

import "fmt"

type Plan string

const (
	PlanFree       Plan = "free"
	PlanPro        Plan = "pro"
	PlanBusiness   Plan = "business"
	PlanEnterprise Plan = "enterprise"
)

type Limits struct {
	DynamicCodes         int `json:"dynamic_codes"`
	AnalyticsHistoryDays int `json:"analytics_history_days"`
	Seats                int `json:"seats"`
	OwnedWorkspaces      int `json:"owned_workspaces"`
	CustomDomains        int `json:"custom_domains"`
	BulkRowsPerJob       int `json:"bulk_rows_per_job"`
	APIRequestsPerMin    int `json:"api_requests_per_min"`
	Webhooks             int `json:"webhooks"`
	Templates            int `json:"templates"`
}

// Features supported across tiers
const (
	// Pro features
	FeatureScheduling     = "scheduling"
	FeatureExpiry         = "expiry"
	FeatureScanLimit      = "scan_limit"
	FeatureUTMAppend      = "utm_append"
	FeatureHostedPages    = "hosted_pages"
	FeatureTemplates      = "templates"
	FeatureCSVExport      = "csv_export"
	FeatureRemoveBranding = "remove_branding"

	// Business features
	FeatureRules           = "rules"
	FeatureCampaigns       = "campaigns"
	FeatureAPI             = "api"
	FeatureWebhooks        = "webhooks"
	FeatureRawScanLog      = "raw_scan_log"
	FeatureLockedTemplates = "locked_templates"
	FeatureAuditLog        = "audit_log"
	FeatureGS1             = "gs1"
	FeatureRoles           = "roles"

	// Enterprise features
	FeatureSSO             = "sso"
	FeatureSCIM            = "scim"
	FeatureSLA             = "sla"
	FeatureDedicatedDomain = "dedicated_domain"
)

var PlanLimits = map[Plan]Limits{
	PlanFree: {
		DynamicCodes:         3,
		AnalyticsHistoryDays: 30,
		Seats:                1,
		OwnedWorkspaces:      1,
		CustomDomains:        0,
		BulkRowsPerJob:       0,
		APIRequestsPerMin:    0,
		Webhooks:             0,
		Templates:            0,
	},
	PlanPro: {
		DynamicCodes:         100,
		AnalyticsHistoryDays: 365,
		Seats:                1,
		OwnedWorkspaces:      3,
		CustomDomains:        1,
		BulkRowsPerJob:       500,
		APIRequestsPerMin:    0,
		Webhooks:             0,
		Templates:            10,
	},
	PlanBusiness: {
		DynamicCodes:         1000,
		AnalyticsHistoryDays: 1095,
		Seats:                5,
		OwnedWorkspaces:      10,
		CustomDomains:        5,
		BulkRowsPerJob:       5000,
		APIRequestsPerMin:    600,
		Webhooks:             10,
		Templates:            100,
	},
	PlanEnterprise: {
		DynamicCodes:         100000,
		AnalyticsHistoryDays: 3650,
		Seats:                1000,
		OwnedWorkspaces:      100,
		CustomDomains:        50,
		BulkRowsPerJob:       50000,
		APIRequestsPerMin:    3000,
		Webhooks:             50,
		Templates:            100000,
	},
}

var proFeatures = map[string]struct{}{
	FeatureScheduling:     {},
	FeatureExpiry:         {},
	FeatureScanLimit:      {},
	FeatureUTMAppend:      {},
	FeatureHostedPages:    {},
	FeatureTemplates:      {},
	FeatureCSVExport:      {},
	FeatureRemoveBranding: {},
}

var businessFeatures = map[string]struct{}{
	FeatureRules:           {},
	FeatureCampaigns:       {},
	FeatureAPI:             {},
	FeatureWebhooks:        {},
	FeatureRawScanLog:      {},
	FeatureLockedTemplates: {},
	FeatureAuditLog:        {},
	FeatureGS1:             {},
	FeatureRoles:           {},
}

var enterpriseFeatures = map[string]struct{}{
	FeatureSSO:             {},
	FeatureSCIM:            {},
	FeatureSLA:             {},
	FeatureDedicatedDomain: {},
}

// EntitlementError represents an entitlement check failure (HTTP 402 Payment Required).
type EntitlementError struct {
	Code         string `json:"code"`
	Message      string `json:"message"`
	RequiredPlan Plan   `json:"required_plan,omitempty"`
}

func (e *EntitlementError) Error() string {
	if e.RequiredPlan != "" {
		return fmt.Sprintf("%s (requires %s plan)", e.Message, e.RequiredPlan)
	}
	return e.Message
}

func ErrLimitReached(resource string, limit int, nextPlan Plan) *EntitlementError {
	return &EntitlementError{
		Code:         "limit_reached",
		Message:      fmt.Sprintf("%s limit reached (%d)", resource, limit),
		RequiredPlan: nextPlan,
	}
}

func ErrUpgradeRequired(feature string, requiredPlan Plan) *EntitlementError {
	return &EntitlementError{
		Code:         "upgrade_required",
		Message:      fmt.Sprintf("feature %q requires an upgrade", feature),
		RequiredPlan: requiredPlan,
	}
}

var ErrReadOnlyOverLimit = &EntitlementError{
	Code:    "read_only_over_limit",
	Message: "code is read-only due to plan downgrade limit; upgrade to edit",
}

// NormalisePlan converts any string to a recognized Plan or defaults to PlanFree.
func NormalisePlan(name string) Plan {
	switch Plan(name) {
	case PlanPro:
		return PlanPro
	case PlanBusiness:
		return PlanBusiness
	case PlanEnterprise:
		return PlanEnterprise
	default:
		return PlanFree
	}
}

// GetLimits returns the resource limits for a plan.
func GetLimits(planName string) Limits {
	p := NormalisePlan(planName)
	return PlanLimits[p]
}

// HasFeature returns whether the specified plan includes the requested feature.
func HasFeature(planName string, feature string) bool {
	p := NormalisePlan(planName)
	switch p {
	case PlanEnterprise:
		if _, ok := enterpriseFeatures[feature]; ok {
			return true
		}
		fallthrough
	case PlanBusiness:
		if _, ok := businessFeatures[feature]; ok {
			return true
		}
		fallthrough
	case PlanPro:
		if _, ok := proFeatures[feature]; ok {
			return true
		}
		return false
	default:
		return false
	}
}

// RequiredPlan returns the minimum plan required to use a feature.
func RequiredPlan(feature string) Plan {
	if _, ok := enterpriseFeatures[feature]; ok {
		return PlanEnterprise
	}
	if _, ok := businessFeatures[feature]; ok {
		return PlanBusiness
	}
	if _, ok := proFeatures[feature]; ok {
		return PlanPro
	}
	return PlanFree
}

// NextPlan returns the next upgrade tier for a plan, or PlanEnterprise if at top tier.
func NextPlan(p Plan) Plan {
	switch p {
	case PlanFree:
		return PlanPro
	case PlanPro:
		return PlanBusiness
	default:
		return PlanEnterprise
	}
}

// CheckDynamicCodeLimit verifies if another dynamic code can be created.
func CheckDynamicCodeLimit(planName string, currentCount int) error {
	p := NormalisePlan(planName)
	limits := PlanLimits[p]
	if currentCount >= limits.DynamicCodes {
		return ErrLimitReached("dynamic_codes", limits.DynamicCodes, NextPlan(p))
	}
	return nil
}

// CheckFeature verifies if a feature is allowed, returning an EntitlementError if not.
func CheckFeature(planName string, feature string) error {
	if HasFeature(planName, feature) {
		return nil
	}
	return ErrUpgradeRequired(feature, RequiredPlan(feature))
}
