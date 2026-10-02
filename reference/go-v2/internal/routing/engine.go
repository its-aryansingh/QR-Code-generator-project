package routing

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Condition struct {
	Field string      `json:"field"`
	Op    string      `json:"op"`
	Value interface{} `json:"value"`
}

type WhenGroup struct {
	All []Condition `json:"all,omitempty"`
	Any []Condition `json:"any,omitempty"`
}

type SplitVariant struct {
	Variant        string `json:"variant"`
	Weight         int    `json:"weight"`
	DestinationURL string `json:"destination_url"`
}

type Rule struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Enabled        bool           `json:"enabled"`
	When           *WhenGroup     `json:"when"`
	DestinationURL string         `json:"destination_url,omitempty"`
	Split          []SplitVariant `json:"split,omitempty"`
	// Block denies matching scans (e.g. geo restriction) with a 451 page instead of redirecting.
	Block bool `json:"block,omitempty"`
}

// Result is the outcome of evaluating a version's rules.
type Result struct {
	URL     string
	RuleID  string
	Blocked bool
}

type Version struct {
	DefaultDestination string
	Rules              []Rule
}

type RequestFacts struct {
	QRCodeID   string
	IP         string
	UserAgent  string
	Country    string
	Region     string
	DeviceType string
	OS         string
	Language   string
	ScanCount  int64
}

// Resolve evaluates routing rules against request facts and returns (destinationUrl, ruleID).
// If no rule matches, it returns the version's default destination and an empty ruleID.
func Resolve(v Version, facts RequestFacts, now time.Time, loc *time.Location) (string, string) {
	r := Evaluate(v, facts, now, loc)
	return r.URL, r.RuleID
}

// Evaluate is Resolve with block rules surfaced: the first matching enabled rule wins.
func Evaluate(v Version, facts RequestFacts, now time.Time, loc *time.Location) Result {
	if loc == nil {
		loc = time.UTC
	}
	localNow := now.In(loc)

	for _, rule := range v.Rules {
		if !rule.Enabled {
			continue
		}

		if matchRule(rule.When, facts, localNow) {
			if rule.Block {
				return Result{RuleID: rule.ID, Blocked: true}
			}
			// A/B Split test rule
			if len(rule.Split) > 0 {
				h := sha256.New()
				h.Write([]byte(facts.QRCodeID))
				h.Write([]byte(facts.IP))
				h.Write([]byte(facts.UserAgent))
				sum := h.Sum(nil)

				bucket := int(binary.BigEndian.Uint32(sum[:4]) % 100) // 0-99
				cumWeight := 0

				for _, variant := range rule.Split {
					cumWeight += variant.Weight
					if bucket < cumWeight {
						return Result{URL: variant.DestinationURL, RuleID: fmt.Sprintf("%s:%s", rule.ID, variant.Variant)}
					}
				}
				// Fallback to last variant if weights didn't reach 100
				last := rule.Split[len(rule.Split)-1]
				return Result{URL: last.DestinationURL, RuleID: fmt.Sprintf("%s:%s", rule.ID, last.Variant)}
			}

			// Single destination rule
			if rule.DestinationURL != "" {
				return Result{URL: rule.DestinationURL, RuleID: rule.ID}
			}
		}
	}

	return Result{URL: v.DefaultDestination}
}

func matchRule(when *WhenGroup, facts RequestFacts, localNow time.Time) bool {
	if when == nil {
		return true
	}

	if len(when.All) > 0 {
		for _, cond := range when.All {
			if !evalCondition(cond, facts, localNow) {
				return false
			}
		}
		return true
	}

	if len(when.Any) > 0 {
		for _, cond := range when.Any {
			if evalCondition(cond, facts, localNow) {
				return true
			}
		}
		return false
	}

	return true
}

func evalCondition(c Condition, facts RequestFacts, localNow time.Time) bool {
	switch c.Field {
	case "country":
		return evalStringIn(facts.Country, c.Op, c.Value)
	case "region":
		return evalStringIn(facts.Region, c.Op, c.Value)
	case "device_type":
		return evalStringIn(facts.DeviceType, c.Op, c.Value)
	case "os":
		return evalStringIn(facts.OS, c.Op, c.Value)
	case "language":
		return evalStringIn(facts.Language, c.Op, c.Value)
	case "local_time":
		return evalTimeBetween(localNow, c.Value)
	case "weekday":
		return evalWeekdayIn(localNow, c.Op, c.Value)
	case "date":
		return evalDateBetween(localNow, c.Value)
	case "scan_count":
		return evalScanCount(facts.ScanCount, c.Op, c.Value)
	default:
		return false
	}
}

func toStringSlice(val interface{}) []string {
	if val == nil {
		return nil
	}
	switch v := val.(type) {
	case []string:
		return v
	case []interface{}:
		res := make([]string, len(v))
		for i, item := range v {
			res[i] = fmt.Sprintf("%v", item)
		}
		return res
	default:
		return []string{fmt.Sprintf("%v", v)}
	}
}

func evalStringIn(actual, op string, val interface{}) bool {
	items := toStringSlice(val)
	lowerActual := strings.ToLower(strings.TrimSpace(actual))

	matched := false
	for _, item := range items {
		if strings.ToLower(strings.TrimSpace(item)) == lowerActual {
			matched = true
			break
		}
	}

	if op == "not_in" {
		return !matched
	}
	return matched
}

func parseHHMM(s string) (int, bool) {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) != 2 {
		return 0, false
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

func evalTimeBetween(localNow time.Time, val interface{}) bool {
	items := toStringSlice(val)
	if len(items) != 2 {
		return false
	}

	startMin, ok1 := parseHHMM(items[0])
	endMin, ok2 := parseHHMM(items[1])
	if !ok1 || !ok2 {
		return false
	}

	currMin := localNow.Hour()*60 + localNow.Minute()

	if startMin <= endMin {
		return currMin >= startMin && currMin <= endMin
	}
	// Wrap around midnight (e.g. 22:00 - 04:00)
	return currMin >= startMin || currMin <= endMin
}

func evalWeekdayIn(localNow time.Time, op string, val interface{}) bool {
	// ISO Weekday: 1 = Mon ... 7 = Sun
	iso := int(localNow.Weekday())
	if iso == 0 {
		iso = 7
	}

	var items []int
	switch v := val.(type) {
	case []int:
		items = v
	case []interface{}:
		for _, item := range v {
			if num, err := strconv.Atoi(fmt.Sprintf("%v", item)); err == nil {
				items = append(items, num)
			}
		}
	}

	matched := false
	for _, item := range items {
		if item == iso {
			matched = true
			break
		}
	}

	if op == "not_in" {
		return !matched
	}
	return matched
}

func evalDateBetween(localNow time.Time, val interface{}) bool {
	items := toStringSlice(val)
	if len(items) != 2 {
		return false
	}

	currDate := localNow.Format("2006-01-02")
	startDate := items[0]
	endDate := items[1]

	if startDate != "" && startDate != "null" && currDate < startDate {
		return false
	}
	if endDate != "" && endDate != "null" && currDate > endDate {
		return false
	}
	return true
}

func evalScanCount(count int64, op string, val interface{}) bool {
	threshold, err := strconv.ParseInt(fmt.Sprintf("%v", val), 10, 64)
	if err != nil {
		return false
	}

	switch op {
	case "gte":
		return count >= threshold
	case "lt":
		return count < threshold
	case "gt":
		return count > threshold
	case "lte":
		return count <= threshold
	default:
		return false
	}
}
