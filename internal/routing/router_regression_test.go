package routing

import (
	"encoding/json"
	"errors"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/Resinat/Resin/internal/model"
	"github.com/Resinat/Resin/internal/node"
	"github.com/Resinat/Resin/internal/platform"
	"github.com/Resinat/Resin/internal/testutil"
)

type routerTestPool struct {
	mu          sync.RWMutex
	entries     map[node.Hash]*node.NodeEntry
	platsByID   map[string]*platform.Platform
	platsByName map[string]*platform.Platform
}

type memoryAccountRegionStore struct {
	mu      sync.Mutex
	regions map[model.AccountRegionKey]string
}

func newMemoryAccountRegionStore() *memoryAccountRegionStore {
	return &memoryAccountRegionStore{regions: map[model.AccountRegionKey]string{}}
}

func (s *memoryAccountRegionStore) GetAccountRegion(platformID, account string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	region, ok := s.regions[model.AccountRegionKey{PlatformID: platformID, Account: account}]
	return region, ok
}

func (s *memoryAccountRegionStore) EnsureAccountRegion(platformID, account, region string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := model.AccountRegionKey{PlatformID: platformID, Account: account}
	if _, exists := s.regions[key]; exists {
		return false
	}
	s.regions[key] = region
	return true
}

func newRouterTestPool() *routerTestPool {
	return &routerTestPool{
		entries:     make(map[node.Hash]*node.NodeEntry),
		platsByID:   make(map[string]*platform.Platform),
		platsByName: make(map[string]*platform.Platform),
	}
}

func (p *routerTestPool) addPlatform(plat *platform.Platform) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.platsByID[plat.ID] = plat
	p.platsByName[plat.Name] = plat
}

func (p *routerTestPool) addEntry(h node.Hash, entry *node.NodeEntry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.entries[h] = entry
}

func (p *routerTestPool) removeEntry(h node.Hash) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.entries, h)
}

func (p *routerTestPool) GetEntry(hash node.Hash) (*node.NodeEntry, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	e, ok := p.entries[hash]
	return e, ok
}

func (p *routerTestPool) GetPlatform(id string) (*platform.Platform, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	plat, ok := p.platsByID[id]
	return plat, ok
}

func (p *routerTestPool) GetPlatformByName(name string) (*platform.Platform, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	plat, ok := p.platsByName[name]
	return plat, ok
}

func (p *routerTestPool) RangePlatforms(fn func(*platform.Platform) bool) {
	p.mu.RLock()
	plats := make([]*platform.Platform, 0, len(p.platsByID))
	for _, plat := range p.platsByID {
		plats = append(plats, plat)
	}
	p.mu.RUnlock()

	for _, plat := range plats {
		if !fn(plat) {
			return
		}
	}
}

func (p *routerTestPool) rebuildPlatformView(plat *platform.Platform) {
	p.mu.RLock()
	snapshot := make(map[node.Hash]*node.NodeEntry, len(p.entries))
	for h, e := range p.entries {
		snapshot[h] = e
	}
	p.mu.RUnlock()

	plat.FullRebuild(
		func(fn func(node.Hash, *node.NodeEntry) bool) {
			for h, e := range snapshot {
				if !fn(h, e) {
					return
				}
			}
		},
		func(_ string, _ node.Hash) (string, bool, []string, bool) { return "", true, nil, true },
		func(_ netip.Addr) string { return "" },
	)
}

func newRoutableEntry(t *testing.T, raw, ip string) (node.Hash, *node.NodeEntry) {
	t.Helper()
	rawOpts := json.RawMessage(raw)
	h := node.HashFromRawOptions(rawOpts)
	e := node.NewNodeEntry(h, rawOpts, time.Now(), 16)
	// Empty platform regex still requires at least one enabled subscription.
	e.AddSubscriptionID("sub-test")

	parsedIP, err := netip.ParseAddr(ip)
	if err != nil {
		t.Fatalf("parse ip %q: %v", ip, err)
	}
	e.SetEgressIP(parsedIP)

	// Keep at least one latency sample so the node remains routable.
	e.LatencyTable.Update("cloudflare.com", 100*time.Millisecond, 10*time.Minute)
	waitForDomainLatency(t, e, "cloudflare.com")

	// Any non-nil outbound value is enough for platform filtering.
	ob := testutil.NewNoopOutbound()
	e.Outbound.Store(&ob)

	return h, e
}

func newRoutableEntryWithRegion(t *testing.T, raw, ip, region string) (node.Hash, *node.NodeEntry) {
	t.Helper()
	h, e := newRoutableEntry(t, raw, ip)
	e.SetEgressRegion(region)
	return h, e
}

func waitForDomainLatency(t *testing.T, e *node.NodeEntry, domain string) {
	t.Helper()
	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		if e.LatencyTable != nil {
			if _, ok := e.LatencyTable.GetDomainStats(domain); ok {
				return
			}
		}
		time.Sleep(2 * time.Millisecond)
	}
	if e.LatencyTable == nil {
		t.Fatalf("latency sample for domain %q did not become visible: latency table is nil", domain)
	}
	t.Fatalf(
		"latency sample for domain %q did not become visible in time: table size=%d",
		domain,
		e.LatencyTable.Size(),
	)
}

func newTestRouter(pool PoolAccessor, onEvent LeaseEventFunc) *Router {
	return NewRouter(RouterConfig{
		Pool:         pool,
		Authorities:  func() []string { return []string{"cloudflare.com"} },
		P2CWindow:    func() time.Duration { return 10 * time.Minute },
		OnLeaseEvent: onEvent,
	})
}

func newTestRouterWithAccountRegions(pool PoolAccessor, store AccountRegionStore) *Router {
	return NewRouter(RouterConfig{
		Pool:           pool,
		Authorities:    func() []string { return []string{"cloudflare.com"} },
		P2CWindow:      func() time.Duration { return 10 * time.Minute },
		AccountRegions: store,
	})
}

func TestStickyRoute_FirstLeaseWritesPrimaryRegionAndPrefersKnownRegion(t *testing.T) {
	pool := newRouterTestPool()
	plat := platform.NewPlatform("plat-region-first", "Plat-Region-First", nil, nil)
	plat.StickyTTLNs = int64(time.Hour)
	pool.addPlatform(plat)

	unknownHash, unknownEntry := newRoutableEntry(t, `{"id":"unknown-region"}`, "198.51.100.10")
	usHash, usEntry := newRoutableEntryWithRegion(t, `{"id":"us-region"}`, "198.51.100.11", "us")
	pool.addEntry(unknownHash, unknownEntry)
	pool.addEntry(usHash, usEntry)
	pool.rebuildPlatformView(plat)

	store := newMemoryAccountRegionStore()
	router := newTestRouterWithAccountRegions(pool, store)

	res, err := router.RouteRequest(plat.Name, "acct-region-first", "https://example.com")
	if err != nil {
		t.Fatalf("RouteRequest: %v", err)
	}
	if res.NodeHash != usHash {
		t.Fatalf("first sticky route should prefer known-region node: got %s want %s", res.NodeHash.Hex(), usHash.Hex())
	}
	region, ok := store.GetAccountRegion(plat.ID, "acct-region-first")
	if !ok || region != "us" {
		t.Fatalf("stored primary region: got (%q,%v), want (us,true)", region, ok)
	}
}

func TestStickyRoute_PrimaryRegionThenFailoverOrder(t *testing.T) {
	pool := newRouterTestPool()
	plat := platform.NewPlatform("plat-region-failover", "Plat-Region-Failover", nil, nil)
	plat.StickyTTLNs = int64(time.Hour)
	plat.RegionFailoverOrder = []string{"jp", "sg"}
	pool.addPlatform(plat)

	jpHash, jpEntry := newRoutableEntryWithRegion(t, `{"id":"jp-region"}`, "198.51.100.20", "jp")
	sgHash, sgEntry := newRoutableEntryWithRegion(t, `{"id":"sg-region"}`, "198.51.100.21", "sg")
	pool.addEntry(jpHash, jpEntry)
	pool.addEntry(sgHash, sgEntry)
	pool.rebuildPlatformView(plat)

	store := newMemoryAccountRegionStore()
	store.EnsureAccountRegion(plat.ID, "acct-primary-sg", "sg")
	router := newTestRouterWithAccountRegions(pool, store)

	res, err := router.RouteRequest(plat.Name, "acct-primary-sg", "https://example.com")
	if err != nil {
		t.Fatalf("RouteRequest primary: %v", err)
	}
	if res.NodeHash != sgHash {
		t.Fatalf("primary region should win before failover order: got %s want %s", res.NodeHash.Hex(), sgHash.Hex())
	}

	router.DeleteLease(plat.ID, "acct-primary-sg")
	sgEntry.CircuitOpenSince.Store(time.Now().UnixNano())
	plat.NotifyDirty(
		sgHash,
		pool.GetEntry,
		func(_ string, _ node.Hash) (string, bool, []string, bool) { return "", true, nil, true },
		func(_ netip.Addr) string { return "" },
	)

	res, err = router.RouteRequest(plat.Name, "acct-primary-sg", "https://example.com")
	if err != nil {
		t.Fatalf("RouteRequest failover: %v", err)
	}
	if res.NodeHash != jpHash {
		t.Fatalf("primary unavailable should use first available failover region: got %s want %s", res.NodeHash.Hex(), jpHash.Hex())
	}
	region, _ := store.GetAccountRegion(plat.ID, "acct-primary-sg")
	if region != "sg" {
		t.Fatalf("fallback route must not rewrite primary region, got %q", region)
	}
}

func TestStickyRoute_SameIPRotationBeatsRegionFailover(t *testing.T) {
	pool := newRouterTestPool()
	plat := platform.NewPlatform("plat-same-ip-region", "Plat-Same-IP-Region", nil, nil)
	plat.StickyTTLNs = int64(time.Hour)
	plat.RegionFailoverOrder = []string{"jp"}
	pool.addPlatform(plat)

	currentHash, currentEntry := newRoutableEntryWithRegion(t, `{"id":"current-us"}`, "198.51.100.30", "us")
	sameIPHash, sameIPEntry := newRoutableEntryWithRegion(t, `{"id":"same-ip-us"}`, "198.51.100.30", "us")
	jpHash, jpEntry := newRoutableEntryWithRegion(t, `{"id":"jp-other-ip"}`, "198.51.100.31", "jp")
	pool.addEntry(currentHash, currentEntry)
	pool.addEntry(sameIPHash, sameIPEntry)
	pool.addEntry(jpHash, jpEntry)
	pool.rebuildPlatformView(plat)

	currentEntry.CircuitOpenSince.Store(time.Now().UnixNano())
	plat.NotifyDirty(
		currentHash,
		pool.GetEntry,
		func(_ string, _ node.Hash) (string, bool, []string, bool) { return "", true, nil, true },
		func(_ netip.Addr) string { return "" },
	)

	store := newMemoryAccountRegionStore()
	store.EnsureAccountRegion(plat.ID, "acct-same-ip", "jp")
	router := newTestRouterWithAccountRegions(pool, store)
	state := router.ensurePlatformState(plat.ID)
	state.Leases.CreateLease("acct-same-ip", Lease{
		NodeHash:       currentHash,
		EgressIP:       currentEntry.GetEgressIP(),
		ExpiryNs:       time.Now().Add(time.Hour).UnixNano(),
		LastAccessedNs: time.Now().UnixNano(),
	})

	res, err := router.RouteRequest(plat.Name, "acct-same-ip", "https://example.com")
	if err != nil {
		t.Fatalf("RouteRequest: %v", err)
	}
	if res.NodeHash != sameIPHash {
		t.Fatalf("same-IP replacement should beat region preference: got %s want %s (jp=%s)", res.NodeHash.Hex(), sameIPHash.Hex(), jpHash.Hex())
	}
	if res.LeaseCreated {
		t.Fatal("same-IP replacement should not create a new lease")
	}
}

func TestStickyRoute_ExpiredLeaseRecreatesOnSameIPBeforeRegion(t *testing.T) {
	pool := newRouterTestPool()
	plat := platform.NewPlatform("plat-expired-same-ip", "Plat-Expired-Same-IP", nil, nil)
	plat.StickyTTLNs = int64(time.Hour)
	plat.RegionFailoverOrder = []string{"jp"}
	pool.addPlatform(plat)

	currentHash, currentEntry := newRoutableEntryWithRegion(t, `{"id":"expired-current-us"}`, "198.51.100.40", "us")
	sameIPHash, sameIPEntry := newRoutableEntryWithRegion(t, `{"id":"expired-same-ip-us"}`, "198.51.100.40", "us")
	jpHash, jpEntry := newRoutableEntryWithRegion(t, `{"id":"expired-jp"}`, "198.51.100.41", "jp")
	pool.addEntry(currentHash, currentEntry)
	pool.addEntry(sameIPHash, sameIPEntry)
	pool.addEntry(jpHash, jpEntry)
	pool.rebuildPlatformView(plat)

	currentEntry.CircuitOpenSince.Store(time.Now().UnixNano())
	plat.NotifyDirty(
		currentHash,
		pool.GetEntry,
		func(_ string, _ node.Hash) (string, bool, []string, bool) { return "", true, nil, true },
		func(_ netip.Addr) string { return "" },
	)

	store := newMemoryAccountRegionStore()
	store.EnsureAccountRegion(plat.ID, "acct-expired-same-ip", "jp")
	router := newTestRouterWithAccountRegions(pool, store)
	state := router.ensurePlatformState(plat.ID)
	state.Leases.CreateLease("acct-expired-same-ip", Lease{
		NodeHash:       currentHash,
		EgressIP:       currentEntry.GetEgressIP(),
		CreatedAtNs:    time.Now().Add(-2 * time.Hour).UnixNano(),
		ExpiryNs:       time.Now().Add(-1 * time.Minute).UnixNano(),
		LastAccessedNs: time.Now().Add(-2 * time.Hour).UnixNano(),
	})

	res, err := router.RouteRequest(plat.Name, "acct-expired-same-ip", "https://example.com")
	if err != nil {
		t.Fatalf("RouteRequest: %v", err)
	}
	if res.NodeHash != sameIPHash {
		t.Fatalf("expired lease should recreate on same IP before region: got %s want %s (jp=%s)", res.NodeHash.Hex(), sameIPHash.Hex(), jpHash.Hex())
	}
	if !res.LeaseCreated {
		t.Fatal("expired same-IP reuse should create a fresh lease")
	}
	if got := state.IPLoadStats.Get(sameIPEntry.GetEgressIP()); got != 1 {
		t.Fatalf("same IP load should remain 1 after expired recreate, got %d", got)
	}
}

func TestStickyLeaseHit_RefreshesExpiryOnlyWhenSlidingEnabled(t *testing.T) {
	pool := newRouterTestPool()
	plat := platform.NewPlatform("plat-sliding", "Plat-Sliding", nil, nil)
	plat.StickyTTLNs = int64(10 * time.Minute)
	pool.addPlatform(plat)

	h, entry := newRoutableEntry(t, `{"id":"sliding-hit"}`, "203.0.113.70")
	pool.addEntry(h, entry)
	pool.rebuildPlatformView(plat)

	router := newTestRouter(pool, nil)
	state := router.ensurePlatformState(plat.ID)
	oldExpiry := time.Now().Add(30 * time.Second).UnixNano()
	state.Leases.CreateLease("acct-fixed", Lease{
		NodeHash:       h,
		EgressIP:       entry.GetEgressIP(),
		CreatedAtNs:    time.Now().Add(-time.Minute).UnixNano(),
		ExpiryNs:       oldExpiry,
		LastAccessedNs: time.Now().Add(-time.Minute).UnixNano(),
	})

	if _, err := router.RouteRequest(plat.Name, "acct-fixed", "https://example.com"); err != nil {
		t.Fatalf("fixed route: %v", err)
	}
	fixedLease, ok := state.Leases.GetLease("acct-fixed")
	if !ok {
		t.Fatal("expected fixed lease")
	}
	if fixedLease.ExpiryNs != oldExpiry {
		t.Fatalf("fixed lease expiry changed: got %d, want %d", fixedLease.ExpiryNs, oldExpiry)
	}
	if fixedLease.LastAccessedNs <= fixedLease.CreatedAtNs {
		t.Fatalf("fixed lease last_accessed should update: %+v", fixedLease)
	}

	plat.StickyTTLSliding = true
	slidingOldExpiry := time.Now().Add(30 * time.Second).UnixNano()
	state.Leases.CreateLease("acct-sliding", Lease{
		NodeHash:       h,
		EgressIP:       entry.GetEgressIP(),
		CreatedAtNs:    time.Now().Add(-time.Minute).UnixNano(),
		ExpiryNs:       slidingOldExpiry,
		LastAccessedNs: time.Now().Add(-time.Minute).UnixNano(),
	})

	before := time.Now()
	if _, err := router.RouteRequest(plat.Name, "acct-sliding", "https://example.com"); err != nil {
		t.Fatalf("sliding route: %v", err)
	}
	after := time.Now()
	slidingLease, ok := state.Leases.GetLease("acct-sliding")
	if !ok {
		t.Fatal("expected sliding lease")
	}
	if slidingLease.ExpiryNs <= slidingOldExpiry {
		t.Fatalf("sliding lease expiry did not extend: got %d, old %d", slidingLease.ExpiryNs, slidingOldExpiry)
	}
	if slidingLease.ExpiryNs < before.Add(platTTLDuration(plat)).UnixNano() ||
		slidingLease.ExpiryNs > after.Add(platTTLDuration(plat)).UnixNano() {
		t.Fatalf("sliding expiry outside expected window: got %v before=%v after=%v",
			time.Unix(0, slidingLease.ExpiryNs), before, after)
	}
}

func TestRouteRequest_SameIPRotationPrefersTargetLatencySample(t *testing.T) {
	pool := newRouterTestPool()
	plat := platform.NewPlatform("plat-1", "Plat-1", nil, nil)
	plat.StickyTTLNs = int64(time.Hour)
	pool.addPlatform(plat)

	currentHash, currentEntry := newRoutableEntry(t, `{"id":"current"}`, "198.51.100.77")
	candidateA, entryA := newRoutableEntry(t, `{"id":"candidate-a"}`, "198.51.100.77")
	candidateB, entryB := newRoutableEntry(t, `{"id":"candidate-b"}`, "198.51.100.77")
	pool.addEntry(currentHash, currentEntry)
	pool.addEntry(candidateA, entryA)
	pool.addEntry(candidateB, entryB)
	pool.rebuildPlatformView(plat)
	if !plat.View().Contains(currentHash) || !plat.View().Contains(candidateA) || !plat.View().Contains(candidateB) {
		t.Fatalf(
			"rebuild did not include expected nodes: size=%d current(lat=%v,out=%v) a(lat=%v,out=%v) b(lat=%v,out=%v)",
			plat.View().Size(),
			currentEntry.HasLatency(), currentEntry.HasOutbound(),
			entryA.HasLatency(), entryA.HasOutbound(),
			entryB.HasLatency(), entryB.HasOutbound(),
		)
	}

	// Force lease invalidation while keeping same-IP candidates in view.
	currentEntry.CircuitOpenSince.Store(time.Now().UnixNano())
	plat.NotifyDirty(
		currentHash,
		pool.GetEntry,
		func(_ string, _ node.Hash) (string, bool, []string, bool) { return "", true, nil, true },
		func(_ netip.Addr) string { return "" },
	)

	order := make([]node.Hash, 0, 2)
	plat.View().Range(func(h node.Hash) bool {
		if h == candidateA || h == candidateB {
			order = append(order, h)
		}
		return true
	})
	if len(order) != 2 {
		t.Fatalf("expected 2 same-ip candidates in view, got %d", len(order))
	}

	entries := map[node.Hash]*node.NodeEntry{
		candidateA: entryA,
		candidateB: entryB,
	}
	noSampleHash := order[0]
	preferredHash := order[1]
	entries[preferredHash].LatencyTable.Update("example.com", 20*time.Millisecond, 10*time.Minute)
	waitForDomainLatency(t, entries[preferredHash], "example.com")
	_ = noSampleHash // intentionally keep target-domain latency empty

	router := newTestRouter(pool, nil)
	state, _ := router.states.LoadOrCompute(plat.ID, func() (*PlatformRoutingState, bool) {
		return NewPlatformRoutingState(), false
	})

	originalLease := Lease{
		NodeHash:       currentHash,
		EgressIP:       currentEntry.GetEgressIP(),
		ExpiryNs:       time.Now().Add(time.Hour).UnixNano(),
		LastAccessedNs: time.Now().UnixNano(),
	}
	state.Leases.CreateLease("acct-rotation", originalLease)

	res, err := router.RouteRequest(plat.Name, "acct-rotation", "https://example.com/path")
	if err != nil {
		t.Fatalf("route failed: %v", err)
	}
	if res.NodeHash != preferredHash {
		t.Fatalf("rotation picked %s, want %s (candidate with target-domain latency)", res.NodeHash.Hex(), preferredHash.Hex())
	}
	if res.LeaseCreated {
		t.Fatal("same-ip rotation should update existing lease, not create a new one")
	}

	updatedLease, ok := state.Leases.GetLease("acct-rotation")
	if !ok {
		t.Fatal("lease should still exist after rotation")
	}
	if updatedLease.NodeHash != preferredHash {
		t.Fatalf("lease node hash = %s, want %s", updatedLease.NodeHash.Hex(), preferredHash.Hex())
	}
	if updatedLease.ExpiryNs != originalLease.ExpiryNs {
		t.Fatalf("same-ip rotation must not change expiry: got %d want %d", updatedLease.ExpiryNs, originalLease.ExpiryNs)
	}
}

func TestRouteRequest_SameIPRotationRefreshesExpiryWhenSlidingEnabled(t *testing.T) {
	pool := newRouterTestPool()
	plat := platform.NewPlatform("plat-rotation-sliding", "Plat-Rotation-Sliding", nil, nil)
	plat.StickyTTLNs = int64(time.Hour)
	plat.StickyTTLSliding = true
	pool.addPlatform(plat)

	currentHash, currentEntry := newRoutableEntry(t, `{"id":"sliding-current"}`, "198.51.100.88")
	candidateHash, candidateEntry := newRoutableEntry(t, `{"id":"sliding-candidate"}`, "198.51.100.88")
	pool.addEntry(currentHash, currentEntry)
	pool.addEntry(candidateHash, candidateEntry)
	pool.rebuildPlatformView(plat)

	currentEntry.CircuitOpenSince.Store(time.Now().UnixNano())
	plat.NotifyDirty(
		currentHash,
		pool.GetEntry,
		func(_ string, _ node.Hash) (string, bool, []string, bool) { return "", true, nil, true },
		func(_ netip.Addr) string { return "" },
	)
	if plat.View().Contains(currentHash) {
		t.Fatal("expected current node removed from platform view")
	}
	if !plat.View().Contains(candidateHash) {
		t.Fatal("expected same-IP candidate in platform view")
	}

	router := newTestRouter(pool, nil)
	state := router.ensurePlatformState(plat.ID)
	oldExpiry := time.Now().Add(30 * time.Second).UnixNano()
	state.Leases.CreateLease("acct-rotation-sliding", Lease{
		NodeHash:       currentHash,
		EgressIP:       currentEntry.GetEgressIP(),
		CreatedAtNs:    time.Now().Add(-time.Minute).UnixNano(),
		ExpiryNs:       oldExpiry,
		LastAccessedNs: time.Now().Add(-time.Minute).UnixNano(),
	})

	before := time.Now()
	res, err := router.RouteRequest(plat.Name, "acct-rotation-sliding", "https://example.com")
	if err != nil {
		t.Fatalf("route request: %v", err)
	}
	after := time.Now()
	if res.NodeHash != candidateHash {
		t.Fatalf("routed hash: got %s want %s", res.NodeHash.Hex(), candidateHash.Hex())
	}
	if res.LeaseCreated {
		t.Fatal("same-IP rotation should replace existing lease without creating a new lease")
	}

	updatedLease, ok := state.Leases.GetLease("acct-rotation-sliding")
	if !ok {
		t.Fatal("expected updated lease")
	}
	if updatedLease.ExpiryNs <= oldExpiry {
		t.Fatalf("sliding same-IP rotation did not extend expiry: got %d old %d", updatedLease.ExpiryNs, oldExpiry)
	}
	if updatedLease.ExpiryNs < before.Add(platTTLDuration(plat)).UnixNano() ||
		updatedLease.ExpiryNs > after.Add(platTTLDuration(plat)).UnixNano() {
		t.Fatalf("sliding same-IP expiry outside expected window: got %v before=%v after=%v",
			time.Unix(0, updatedLease.ExpiryNs), before, after)
	}
}

func platTTLDuration(plat *platform.Platform) time.Duration {
	return time.Duration(plat.StickyTTLNs)
}

func TestRouteRequest_SelectedNodeRemovedAfterPick_EmitsLeaseRemove(t *testing.T) {
	pool := newRouterTestPool()
	plat := platform.NewPlatform("plat-2", "Plat-2", nil, nil)
	plat.StickyTTLNs = int64(time.Hour)
	pool.addPlatform(plat)

	selectedHash, selectedEntry := newRoutableEntry(t, `{"id":"selected"}`, "203.0.113.20")
	pool.addEntry(selectedHash, selectedEntry)
	pool.rebuildPlatformView(plat)
	if !plat.View().Contains(selectedHash) {
		t.Fatalf(
			"setup expected selected hash in view: size=%d hasLatency=%v hasOutbound=%v",
			plat.View().Size(),
			selectedEntry.HasLatency(),
			selectedEntry.HasOutbound(),
		)
	}

	// Simulate stale view: node stays in platform view but has been removed from pool.
	pool.removeEntry(selectedHash)

	var events []LeaseEvent
	router := newTestRouter(pool, func(e LeaseEvent) {
		events = append(events, e)
	})
	state, _ := router.states.LoadOrCompute(plat.ID, func() (*PlatformRoutingState, bool) {
		return NewPlatformRoutingState(), false
	})

	oldIP := netip.MustParseAddr("203.0.113.9")
	oldHash := node.HashFromRawOptions(json.RawMessage(`{"id":"old-lease-node"}`))
	state.Leases.CreateLease("acct-race", Lease{
		NodeHash:       oldHash,
		EgressIP:       oldIP,
		ExpiryNs:       time.Now().Add(time.Hour).UnixNano(),
		LastAccessedNs: time.Now().UnixNano(),
	})

	if got := state.IPLoadStats.Get(oldIP); got != 1 {
		t.Fatalf("setup ip load: got %d, want 1", got)
	}

	_, err := router.RouteRequest(plat.Name, "acct-race", "https://example.com")
	if err == nil {
		t.Fatal("expected route error when selected node disappears")
	}
	if !errors.Is(err, ErrNoAvailableNodes) {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := state.Leases.GetLease("acct-race"); ok {
		t.Fatal("lease should be removed when re-route fails after invalidation")
	}
	if got := state.IPLoadStats.Get(oldIP); got != 0 {
		t.Fatalf("ip load should decrement exactly once, got %d", got)
	}

	foundRemove := false
	for _, e := range events {
		if e.Type == LeaseRemove && e.Account == "acct-race" && e.NodeHash == oldHash && e.EgressIP == oldIP {
			foundRemove = true
			break
		}
	}
	if !foundRemove {
		t.Fatal("expected LeaseRemove event when old lease is dropped")
	}
}
