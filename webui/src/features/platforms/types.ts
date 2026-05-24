export type PlatformMissAction = "TREAT_AS_EMPTY" | "REJECT";
export type PlatformEmptyAccountBehavior = "RANDOM" | "FIXED_HEADER" | "ACCOUNT_HEADER_RULE";
export type PlatformAllocationPolicy = "BALANCED" | "PREFER_LOW_LATENCY" | "PREFER_IDLE_IP";

export type Platform = {
  id: string;
  name: string;
  sticky_ttl: string;
  sticky_ttl_sliding: boolean;
  regex_filters: string[];
  region_filters: string[];
  region_failover_order: string[];
  blocked_egress_ips: string[];
  routable_node_count: number;
  reverse_proxy_miss_action: PlatformMissAction;
  reverse_proxy_empty_account_behavior: PlatformEmptyAccountBehavior;
  reverse_proxy_fixed_account_header: string;
  allocation_policy: PlatformAllocationPolicy;
  passive_circuit_breaker_disabled: boolean;
  updated_at: string;
};

export type PageResponse<T> = {
  items: T[];
  total: number;
  limit: number;
  offset: number;
};

export type PlatformCreateInput = {
  name: string;
  sticky_ttl?: string;
  sticky_ttl_sliding?: boolean;
  regex_filters?: string[];
  region_filters?: string[];
  region_failover_order?: string[];
  blocked_egress_ips?: string[];
  reverse_proxy_miss_action?: PlatformMissAction;
  reverse_proxy_empty_account_behavior?: PlatformEmptyAccountBehavior;
  reverse_proxy_fixed_account_header?: string;
  allocation_policy?: PlatformAllocationPolicy;
  passive_circuit_breaker_disabled?: boolean;
};

export type PlatformUpdateInput = {
  name?: string;
  sticky_ttl?: string;
  sticky_ttl_sliding?: boolean;
  regex_filters?: string[];
  region_filters?: string[];
  region_failover_order?: string[];
  blocked_egress_ips?: string[];
  reverse_proxy_miss_action?: PlatformMissAction;
  reverse_proxy_empty_account_behavior?: PlatformEmptyAccountBehavior;
  reverse_proxy_fixed_account_header?: string;
  allocation_policy?: PlatformAllocationPolicy;
  passive_circuit_breaker_disabled?: boolean;
};

export type AccountRegion = {
  platform_id: string;
  account: string;
  primary_region: string;
  created_at: string;
  updated_at: string;
};

export type PlatformLease = {
  platform_id: string;
  account: string;
  node_hash: string;
  node_tag: string;
  egress_ip: string;
  expiry: string;
  last_accessed: string;
};
