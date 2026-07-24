import { adminApi } from "./client";

// --- Type definitions (matching Go repository types, PascalCase from JSON) ---

export interface CDNSitePayload {
  ID: number;
  Name: string;
  Description: string;
  Domain: string;
  OriginType: string;
  OriginURL: string;
  CacheTTL: number;
  SSLMode: string;
  Status: string;
  CustomCertPEM: string;
  CustomKeyPEM: string;
  AccelerationMode: string;
  InboundSpecID: number | null;
  Provider: string;
  OriginPath: string;
  OriginProtocol: string;
  Enabled: boolean;
  LastDeployedAt: number | null;
  CreatedAt: number;
  UpdatedAt: number;
}

export interface CDNSite {
  id: number;
  name: string;
  description: string;
  domain: string;
  origin_type: string;
  origin_url: string;
  cache_ttl: number;
  ssl_mode: string;
  status: string;
  acceleration_mode: string;
  inbound_spec_id: number | undefined;
  provider: string;
  origin_path: string;
  origin_protocol: string;
  enabled: boolean;
  last_deployed_at: number | undefined;
  created_at: number;
  updated_at: number;
}

export interface CDNEdgePayload {
  ID: number;
  SiteID: number;
  AgentHostID: number;
  Weight: number;
  Enabled: boolean;
  Status: string;
  LastError: string;
  DeployedAt: number | null;
  CreatedAt: number;
  UpdatedAt: number;
}

export interface CDNEdge {
  id: number;
  site_id: number;
  agent_host_id: number;
  weight: number;
  enabled: boolean;
  status: string;
  last_error: string;
  deployed_at: number | undefined;
  created_at: number;
  updated_at: number;
}

export interface CDNCacheRulePayload {
  ID: number;
  SiteID: number;
  MatchType: string;
  MatchValue: string;
  CacheTTL: number;
  Bypass: boolean;
  Priority: number;
  CreatedAt: number;
}

export interface CDNCacheRule {
  id: number;
  site_id: number;
  match_type: string;
  match_value: string;
  cache_ttl: number;
  bypass: boolean;
  priority: number;
  created_at: number;
}

export interface CloudflareZonePayload {
  ID: number;
  AccountID: string;
  ZoneID: string;
  ZoneName: string;
  Status: string;
  Plan: string;
  Enabled: boolean;
  CreatedAt: number;
  UpdatedAt: number;
}

export interface CloudflareZone {
  id: number;
  zone_id: string;
  zone_name: string;
  status: string;
  plan: string;
  enabled: boolean;
  created_at: number;
  updated_at: number;
}

export interface CloudflareDNSRecord {
  id: string;
  zone_id: string;
  zone_name: string;
  name: string;
  type: string;
  content: string;
  proxied: boolean | null;
  ttl: number;
  priority: number | null;
  comment: string;
  created_on: string;
  modified_on: string;
}

export interface CloudFrontCredential {
  access_key_id: string;
  secret_access_key: string;
}

export interface CloudFrontDistributionPayload {
  ID: number;
  DistributionID: string;
  Domain: string;
  OriginDomain: string;
  Aliases: string;
  Status: string;
  Enabled: boolean;
  CreatedAt: number;
  UpdatedAt: number;
}

export interface CloudFrontDistribution {
  id: number;
  distribution_id: string;
  domain: string;
  origin_domain: string;
  aliases: string;
  status: string;
  enabled: boolean;
  created_at: number;
  updated_at: number;
}

export interface ProviderStatus {
  provider: string;
  status: string;
  domain_name: string;
  enabled: boolean;
}

export interface CDNSitesListParams {
  keyword?: string;
  status?: string;
  enabled?: boolean;
  limit?: number;
  offset?: number;
}

export interface CreateCDNSiteRequest {
  name: string;
  description?: string;
  domain: string;
  origin_type: string;
  origin_url: string;
  cache_ttl?: number;
  ssl_mode?: string;
  acceleration_mode?: string;
  enabled?: boolean;
}

export interface UpdateCDNSiteRequest {
  name?: string;
  description?: string;
  domain?: string;
  origin_type?: string;
  origin_url?: string;
  cache_ttl?: number;
  ssl_mode?: string;
  acceleration_mode?: string;
  enabled?: boolean;
}

export interface AssignCDNEdgeRequest {
  agent_host_id: number;
  weight: number;
}

export interface CreateCDNCacheRuleRequest {
  match_type: string;
  match_value: string;
  cache_ttl: number;
  bypass?: boolean;
  priority: number;
}

export interface InvalidateCacheRequest {
  paths?: string[];
}

export interface AddCloudflareZoneRequest {
  name: string;
  zone_id: string;
}

export interface SetCloudFrontCredentialsRequest {
  access_key_id: string;
  secret_access_key: string;
}

// --- Mappers ---

const mapCDNSite = (site: CDNSitePayload): CDNSite => ({
  id: site.ID,
  name: site.Name,
  description: site.Description,
  domain: site.Domain,
  origin_type: site.OriginType,
  origin_url: site.OriginURL,
  cache_ttl: site.CacheTTL,
  ssl_mode: site.SSLMode,
  status: site.Status,
  acceleration_mode: site.AccelerationMode,
  inbound_spec_id: site.InboundSpecID ?? undefined,
  provider: site.Provider,
  origin_path: site.OriginPath,
  origin_protocol: site.OriginProtocol,
  enabled: site.Enabled,
  last_deployed_at: site.LastDeployedAt ?? undefined,
  created_at: site.CreatedAt,
  updated_at: site.UpdatedAt,
});

const mapCDNEdge = (edge: CDNEdgePayload): CDNEdge => ({
  id: edge.ID,
  site_id: edge.SiteID,
  agent_host_id: edge.AgentHostID,
  weight: edge.Weight,
  enabled: edge.Enabled,
  status: edge.Status,
  last_error: edge.LastError,
  deployed_at: edge.DeployedAt ?? undefined,
  created_at: edge.CreatedAt,
  updated_at: edge.UpdatedAt,
});

const mapCDNCacheRule = (rule: CDNCacheRulePayload): CDNCacheRule => ({
  id: rule.ID,
  site_id: rule.SiteID,
  match_type: rule.MatchType,
  match_value: rule.MatchValue,
  cache_ttl: rule.CacheTTL,
  bypass: rule.Bypass,
  priority: rule.Priority,
  created_at: rule.CreatedAt,
});

const mapCloudflareZone = (zone: CloudflareZonePayload): CloudflareZone => ({
  id: zone.ID,
  zone_id: zone.ZoneID,
  zone_name: zone.ZoneName,
  status: zone.Status,
  plan: zone.Plan,
  enabled: zone.Enabled,
  created_at: zone.CreatedAt,
  updated_at: zone.UpdatedAt,
});

const mapCloudFrontDistribution = (
  dist: CloudFrontDistributionPayload
): CloudFrontDistribution => ({
  id: dist.ID,
  distribution_id: dist.DistributionID,
  domain: dist.Domain,
  origin_domain: dist.OriginDomain,
  aliases: dist.Aliases,
  status: dist.Status,
  enabled: dist.Enabled,
  created_at: dist.CreatedAt,
  updated_at: dist.UpdatedAt,
});

// ================================================================
// 1. Site management
// ================================================================

/**
 * List CDN sites with optional filtering.
 */
export async function fetchCDNSites(
  params?: CDNSitesListParams
): Promise<{ sites: CDNSite[]; total: number }> {
  const response = await adminApi.get<{
    data: { sites: CDNSitePayload[]; total: number };
  }>("/cdn/sites", { params });
  return {
    sites: response.data.data.sites.map(mapCDNSite),
    total: response.data.data.total,
  };
}

/**
 * Create a new CDN site.
 */
export async function createCDNSite(
  data: CreateCDNSiteRequest
): Promise<CDNSite> {
  const response = await adminApi.post<{ data: CDNSitePayload }>(
    "/cdn/sites",
    data
  );
  return mapCDNSite(response.data.data);
}

/**
 * Update a CDN site.
 */
export async function updateCDNSite(
  id: number,
  data: UpdateCDNSiteRequest
): Promise<CDNSite> {
  const response = await adminApi.put<{ data: CDNSitePayload }>(
    `/cdn/sites/${id}`,
    data
  );
  return mapCDNSite(response.data.data);
}

/**
 * Delete a CDN site.
 */
export async function deleteCDNSite(id: number): Promise<void> {
  await adminApi.delete(`/cdn/sites/${id}`);
}

// ================================================================
// 2. Edge management
// ================================================================

/**
 * List edges assigned to a CDN site.
 */
export async function fetchCDNEdges(
  siteID: number
): Promise<CDNEdge[]> {
  const response = await adminApi.get<{ data: CDNEdgePayload[] }>(
    `/cdn/sites/${siteID}/edges`
  );
  return response.data.data.map(mapCDNEdge);
}

/**
 * Assign an edge node to a CDN site.
 */
export async function assignCDNEdge(
  siteID: number,
  data: AssignCDNEdgeRequest
): Promise<CDNEdge> {
  const response = await adminApi.post<{ data: CDNEdgePayload }>(
    `/cdn/sites/${siteID}/edges`,
    data
  );
  return mapCDNEdge(response.data.data);
}

/**
 * Remove an edge node from a CDN site.
 */
export async function removeCDNEdge(
  siteID: number,
  edgeID: number
): Promise<void> {
  await adminApi.delete(`/cdn/sites/${siteID}/edges/${edgeID}`);
}

// ================================================================
// 3. Cache rules
// ================================================================

/**
 * List cache rules for a CDN site.
 */
export async function fetchCDNCacheRules(
  siteID: number
): Promise<CDNCacheRule[]> {
  const response = await adminApi.get<{ data: CDNCacheRulePayload[] }>(
    `/cdn/sites/${siteID}/rules`
  );
  return response.data.data.map(mapCDNCacheRule);
}

/**
 * Create a cache rule for a CDN site.
 */
export async function createCDNCacheRule(
  siteID: number,
  data: CreateCDNCacheRuleRequest
): Promise<CDNCacheRule> {
  const response = await adminApi.post<{ data: CDNCacheRulePayload }>(
    `/cdn/sites/${siteID}/rules`,
    data
  );
  return mapCDNCacheRule(response.data.data);
}

/**
 * Delete a cache rule from a CDN site.
 */
export async function deleteCDNCacheRule(
  siteID: number,
  ruleID: number
): Promise<void> {
  await adminApi.delete(`/cdn/sites/${siteID}/rules/${ruleID}`);
}

// ================================================================
// 4. Deploy / undeploy
// ================================================================

/**
 * Deploy a CDN site.
 */
export async function deployCDNSite(
  siteID: number
): Promise<{ message: string }> {
  const response = await adminApi.post<{ message: string }>(
    `/cdn/sites/${siteID}/deploy`
  );
  return response.data;
}

/**
 * Undeploy a CDN site.
 */
export async function undeployCDNSite(
  siteID: number
): Promise<{ message: string }> {
  const response = await adminApi.post<{ message: string }>(
    `/cdn/sites/${siteID}/undeploy`
  );
  return response.data;
}

// ================================================================
// 5. Cloudflare
// ================================================================

/**
 * Fetch Cloudflare zones.
 */
export async function fetchCloudflareZones(): Promise<CloudflareZone[]> {
  const response = await adminApi.get<{ data: CloudflareZonePayload[] }>(
    "/cdn/cloudflare/zones"
  );
  return response.data.data.map(mapCloudflareZone);
}

/**
 * Add a Cloudflare zone.
 */
export async function addCloudflareZone(
  data: AddCloudflareZoneRequest
): Promise<CloudflareZone> {
  const response = await adminApi.post<{ data: CloudflareZonePayload }>(
    "/cdn/cloudflare/zones",
    data
  );
  return mapCloudflareZone(response.data.data);
}

/**
 * Remove a Cloudflare zone.
 */
export async function removeCloudflareZone(zoneID: number): Promise<void> {
  await adminApi.delete(`/cdn/cloudflare/zones/${zoneID}`);
}

/**
 * Fetch DNS records for a Cloudflare zone.
 */
export async function fetchDNSRecords(
  zoneID: number
): Promise<CloudflareDNSRecord[]> {
  const response = await adminApi.get<{ data: CloudflareDNSRecord[] }>(
    `/cdn/cloudflare/zones/${zoneID}/dns`
  );
  return response.data.data;
}

// ================================================================
// 6. CloudFront
// ================================================================

/**
 * Set CloudFront AWS credentials.
 */
export async function setCloudFrontCredentials(
  data: SetCloudFrontCredentialsRequest
): Promise<void> {
  await adminApi.post("/cdn/cloudfront/credentials", data);
}

/**
 * Get CloudFront credentials (masked secret key).
 */
export async function getCloudFrontCredentials(): Promise<CloudFrontCredential | null> {
  const response = await adminApi.get<{ data: CloudFrontCredential | null }>(
    "/cdn/cloudfront/credentials"
  );
  return response.data.data;
}

/**
 * Fetch CloudFront distributions.
 */
export async function fetchCloudFrontDistributions(): Promise<
  CloudFrontDistribution[]
> {
  const response = await adminApi.get<{
    data: CloudFrontDistributionPayload[];
  }>("/cdn/cloudfront/distributions");
  return response.data.data.map(mapCloudFrontDistribution);
}

// ================================================================
// 7. Provider common
// ================================================================

/**
 * Sync a CDN site to its provider.
 */
export async function syncToProvider(
  siteID: number
): Promise<{ message: string }> {
  const response = await adminApi.post<{ message: string }>(
    `/cdn/sites/${siteID}/sync`
  );
  return response.data;
}

/**
 * Invalidate cache for a CDN site.
 */
export async function invalidateCache(
  siteID: number,
  data?: InvalidateCacheRequest
): Promise<{ message: string }> {
  const response = await adminApi.post<{ message: string }>(
    `/cdn/sites/${siteID}/invalidate`,
    data
  );
  return response.data;
}

/**
 * Get provider status for a CDN site.
 */
export async function getProviderStatus(
  siteID: number
): Promise<ProviderStatus> {
  const response = await adminApi.get<{ data: ProviderStatus }>(
    `/cdn/sites/${siteID}/provider-status`
  );
  return response.data.data;
}

// ================================================================
// 8. Latency probe
// ================================================================

export interface ProbeReportItem {
  label: string;
  target: string;
  province: string;
  isp: string;
  avg_latency_ms: number;
  packet_loss: number;
  total_probes: number;
}

export interface ProbeReportResponse {
  enabled: boolean;
  isp_summary: Record<string, number>;
  results: ProbeReportItem[];
}

/**
 * Fetch latency probe report.
 */
export async function fetchProbeReport(): Promise<ProbeReportResponse> {
  const response = await adminApi.get<{ data: ProbeReportResponse }>(
    "/cdn/probe-report"
  );
  return response.data.data;
}
