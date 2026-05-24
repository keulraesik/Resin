package platform

import (
	"fmt"
	"net/netip"
	"regexp"
	"strings"

	"github.com/Resinat/Resin/internal/model"
)

func isLowerAlpha2(s string) bool {
	if len(s) != 2 {
		return false
	}
	return s[0] >= 'a' && s[0] <= 'z' && s[1] >= 'a' && s[1] <= 'z'
}

// ValidateRegionFilters validates region filters against lowercase ISO alpha-2 format.
// Entries may optionally be prefixed with "!" to indicate negation (e.g. !hk).
func ValidateRegionFilters(regionFilters []string) error {
	for i, r := range regionFilters {
		code := r
		if len(r) > 0 && r[0] == '!' {
			code = r[1:]
		}
		if !isLowerAlpha2(code) {
			return fmt.Errorf("region_filters[%d]: must be a 2-letter lowercase ISO 3166-1 alpha-2 code (e.g. us, jp) or negation (e.g. !hk)", i)
		}
	}
	return nil
}

// ValidateRegionFailoverOrder validates strict region priority entries.
func ValidateRegionFailoverOrder(regions []string) error {
	seen := map[string]struct{}{}
	for i, r := range regions {
		if !isLowerAlpha2(r) {
			return fmt.Errorf("region_failover_order[%d]: must be a 2-letter lowercase ISO 3166-1 alpha-2 code (e.g. us, jp)", i)
		}
		if _, exists := seen[r]; exists {
			return fmt.Errorf("region_failover_order[%d]: duplicate region %q", i, r)
		}
		seen[r] = struct{}{}
	}
	return nil
}

func normalizeIPAddr(raw string) (string, error) {
	ip, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	return ip.String(), nil
}

// NormalizeBlockedEgressIPs validates, canonicalizes, and deduplicates blocked egress IPs.
func NormalizeBlockedEgressIPs(values []string) ([]string, error) {
	normalized := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for i, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		ip, err := normalizeIPAddr(trimmed)
		if err != nil {
			return nil, fmt.Errorf("blocked_egress_ips[%d]: must be a valid IPv4 or IPv6 address", i)
		}
		if _, exists := seen[ip]; exists {
			continue
		}
		seen[ip] = struct{}{}
		normalized = append(normalized, ip)
	}
	return normalized, nil
}

// NormalizeBlockedEgressIP validates and canonicalizes one blocked egress IP.
func NormalizeBlockedEgressIP(raw string) (string, error) {
	ip, err := normalizeIPAddr(raw)
	if err != nil {
		return "", fmt.Errorf("egress_ip: must be a valid IPv4 or IPv6 address")
	}
	return ip, nil
}

func blockedEgressIPSet(values []string) map[netip.Addr]struct{} {
	if len(values) == 0 {
		return nil
	}
	set := make(map[netip.Addr]struct{}, len(values))
	for _, raw := range values {
		ip, err := netip.ParseAddr(strings.TrimSpace(raw))
		if err != nil {
			continue
		}
		set[ip] = struct{}{}
	}
	return set
}

// CompileRegexFilters compiles regex filters in order.
func CompileRegexFilters(regexFilters []string) ([]*regexp.Regexp, error) {
	compiled := make([]*regexp.Regexp, 0, len(regexFilters))
	for i, re := range regexFilters {
		c, err := regexp.Compile(re)
		if err != nil {
			return nil, fmt.Errorf("regex_filters[%d]: invalid regex: %v", i, err)
		}
		compiled = append(compiled, c)
	}
	return compiled, nil
}

// NewConfiguredPlatform builds a runtime platform with non-filter settings applied.
func NewConfiguredPlatform(
	id, name string,
	regexFilters []*regexp.Regexp,
	regionFilters []string,
	regionFailoverOrder []string,
	blockedEgressIPs []string,
	stickyTTLNs int64,
	stickyTTLSliding bool,
	missAction string,
	emptyAccountBehavior string,
	fixedAccountHeader string,
	allocationPolicy string,
	passiveCircuitBreakerDisabled bool,
) *Platform {
	normalizedFixedHeaders, fixedHeaders, err := NormalizeFixedAccountHeaders(fixedAccountHeader)
	if err != nil {
		normalizedFixedHeaders = strings.TrimSpace(fixedAccountHeader)
		fixedHeaders = nil
	}
	plat := NewPlatform(id, name, regexFilters, regionFilters)
	plat.RegionFailoverOrder = append([]string(nil), regionFailoverOrder...)
	plat.BlockedEgressIPs = blockedEgressIPSet(blockedEgressIPs)
	plat.StickyTTLNs = stickyTTLNs
	plat.StickyTTLSliding = stickyTTLSliding
	plat.ReverseProxyMissAction = missAction
	plat.ReverseProxyEmptyAccountBehavior = emptyAccountBehavior
	plat.ReverseProxyFixedAccountHeader = normalizedFixedHeaders
	plat.ReverseProxyFixedAccountHeaders = append([]string(nil), fixedHeaders...)
	plat.AllocationPolicy = ParseAllocationPolicy(allocationPolicy)
	plat.PassiveCircuitBreakerDisabled = passiveCircuitBreakerDisabled
	return plat
}

// CompileModelRegexFilters compiles regex filters from persisted model values.
func CompileModelRegexFilters(platformID string, regexFilters []string) ([]*regexp.Regexp, error) {
	compiled, err := CompileRegexFilters(regexFilters)
	if err != nil {
		return nil, fmt.Errorf("decode platform %s regex_filters: %w", platformID, err)
	}
	return compiled, nil
}

// BuildFromModel builds a runtime platform from a persisted model.Platform.
func BuildFromModel(mp model.Platform) (*Platform, error) {
	regexFilters, err := CompileModelRegexFilters(mp.ID, mp.RegexFilters)
	if err != nil {
		return nil, err
	}
	if err := ValidateRegionFilters(mp.RegionFilters); err != nil {
		return nil, err
	}
	if err := ValidateRegionFailoverOrder(mp.RegionFailoverOrder); err != nil {
		return nil, err
	}
	blockedEgressIPs, err := NormalizeBlockedEgressIPs(mp.BlockedEgressIPs)
	if err != nil {
		return nil, err
	}
	emptyAccountBehavior := mp.ReverseProxyEmptyAccountBehavior
	if !ReverseProxyEmptyAccountBehavior(emptyAccountBehavior).IsValid() {
		emptyAccountBehavior = string(ReverseProxyEmptyAccountBehaviorRandom)
	}
	missAction := NormalizeReverseProxyMissAction(mp.ReverseProxyMissAction)
	if missAction == "" {
		return nil, fmt.Errorf(
			"decode platform %s reverse_proxy_miss_action: invalid value %q",
			mp.ID,
			mp.ReverseProxyMissAction,
		)
	}
	fixedHeader, _, err := NormalizeFixedAccountHeaders(mp.ReverseProxyFixedAccountHeader)
	if err != nil {
		return nil, fmt.Errorf("decode platform %s reverse_proxy_fixed_account_header: %w", mp.ID, err)
	}
	if emptyAccountBehavior == string(ReverseProxyEmptyAccountBehaviorFixedHeader) && fixedHeader == "" {
		return nil, fmt.Errorf(
			"decode platform %s reverse_proxy_fixed_account_header: required when reverse_proxy_empty_account_behavior is %s",
			mp.ID,
			ReverseProxyEmptyAccountBehaviorFixedHeader,
		)
	}

	return NewConfiguredPlatform(
		mp.ID,
		mp.Name,
		regexFilters,
		append([]string(nil), mp.RegionFilters...),
		append([]string(nil), mp.RegionFailoverOrder...),
		blockedEgressIPs,
		mp.StickyTTLNs,
		mp.StickyTTLSliding,
		string(missAction),
		emptyAccountBehavior,
		fixedHeader,
		mp.AllocationPolicy,
		mp.PassiveCircuitBreakerDisabled,
	), nil
}
