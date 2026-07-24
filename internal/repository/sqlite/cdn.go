package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/creamcroissant/xboard/internal/repository"
)

// Cloudflare Zone
// ----------------------------------------------------------------

type cloudflareZoneRepo struct {
	db *sql.DB
}

func newCloudflareZoneRepo(db *sql.DB) *cloudflareZoneRepo {
	return &cloudflareZoneRepo{db: db}
}

func (r *cloudflareZoneRepo) Create(ctx context.Context, zone *repository.CloudflareZone) error {
	now := time.Now().Unix()
	zone.CreatedAt = now
	zone.UpdatedAt = now

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO cdn_cloudflare_zones (name, zone_id, api_token_encrypted, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`,
		zone.ZoneName, zone.ZoneID, zone.AccountID, boolToInt(zone.Enabled),
		zone.CreatedAt, zone.UpdatedAt,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	zone.ID = id
	return nil
}

func (r *cloudflareZoneRepo) FindByID(ctx context.Context, id int64) (*repository.CloudflareZone, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, zone_id, name, api_token_encrypted, enabled, created_at, updated_at
		FROM cdn_cloudflare_zones WHERE id = ?
	`, id)

	return r.scanCFZone(row)
}

func (r *cloudflareZoneRepo) List(ctx context.Context) ([]*repository.CloudflareZone, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, zone_id, name, api_token_encrypted, enabled, created_at, updated_at
		FROM cdn_cloudflare_zones
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var zones []*repository.CloudflareZone
	for rows.Next() {
		z, err := r.scanCFZones(rows)
		if err != nil {
			return nil, err
		}
		zones = append(zones, z)
	}
	return zones, rows.Err()
}

func (r *cloudflareZoneRepo) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM cdn_cloudflare_zones WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (r *cloudflareZoneRepo) scanCFZone(row *sql.Row) (*repository.CloudflareZone, error) {
	var z repository.CloudflareZone
	var enabled int

	err := row.Scan(
		&z.ID, &z.ZoneID, &z.ZoneName, &z.AccountID,
		&enabled, &z.CreatedAt, &z.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	z.Enabled = enabled == 1
	// Map DB fields to the interface model
	z.Status = "active"
	return &z, nil
}

func (r *cloudflareZoneRepo) scanCFZones(rows *sql.Rows) (*repository.CloudflareZone, error) {
	var z repository.CloudflareZone
	var enabled int

	err := rows.Scan(
		&z.ID, &z.ZoneID, &z.ZoneName, &z.AccountID,
		&enabled, &z.CreatedAt, &z.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	z.Enabled = enabled == 1
	z.Status = "active"
	return &z, nil
}

// ----------------------------------------------------------------
// Cloudflare DNS Record
// ----------------------------------------------------------------

type cloudflareDNSRecordRepo struct {
	db *sql.DB
}

func newCloudflareDNSRecordRepo(db *sql.DB) *cloudflareDNSRecordRepo {
	return &cloudflareDNSRecordRepo{db: db}
}

func (r *cloudflareDNSRecordRepo) Create(ctx context.Context, record *repository.CloudflareDNSRecord) error {
	now := time.Now().Unix()
	record.CreatedAt = now
	record.UpdatedAt = now

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO cdn_cloudflare_dns_records (zone_id, name, type, content, proxied, ttl, cf_record_id, synced_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		record.ZoneID, record.Name, record.Type, record.Content,
		boolToInt(record.Proxied), record.TTL, record.RecordID,
		record.SyncedAt, record.CreatedAt, record.UpdatedAt,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	record.ID = id
	return nil
}

func (r *cloudflareDNSRecordRepo) FindByID(ctx context.Context, id int64) (*repository.CloudflareDNSRecord, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, zone_id, cf_record_id, name, type, content, ttl, proxied, synced_at, created_at, updated_at
		FROM cdn_cloudflare_dns_records WHERE id = ?
	`, id)

	return r.scanCFDNSRecord(row)
}

func (r *cloudflareDNSRecordRepo) ListByZoneID(ctx context.Context, zoneID int64) ([]*repository.CloudflareDNSRecord, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, zone_id, cf_record_id, name, type, content, ttl, proxied, synced_at, created_at, updated_at
		FROM cdn_cloudflare_dns_records
		WHERE zone_id = ?
		ORDER BY name ASC
	`, zoneID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*repository.CloudflareDNSRecord
	for rows.Next() {
		rec, err := r.scanCFDNSRecords(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

func (r *cloudflareDNSRecordRepo) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM cdn_cloudflare_dns_records WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (r *cloudflareDNSRecordRepo) scanCFDNSRecord(row *sql.Row) (*repository.CloudflareDNSRecord, error) {
	var rec repository.CloudflareDNSRecord
	var proxied int
	var syncedAt sql.NullInt64

	err := row.Scan(
		&rec.ID, &rec.ZoneID, &rec.RecordID, &rec.Name, &rec.Type,
		&rec.Content, &rec.TTL, &proxied, &syncedAt,
		&rec.CreatedAt, &rec.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	rec.Proxied = proxied == 1
	if syncedAt.Valid {
		rec.SyncedAt = syncedAt.Int64
	}
	return &rec, nil
}

func (r *cloudflareDNSRecordRepo) scanCFDNSRecords(rows *sql.Rows) (*repository.CloudflareDNSRecord, error) {
	var rec repository.CloudflareDNSRecord
	var proxied int
	var syncedAt sql.NullInt64

	err := rows.Scan(
		&rec.ID, &rec.ZoneID, &rec.RecordID, &rec.Name, &rec.Type,
		&rec.Content, &rec.TTL, &proxied, &syncedAt,
		&rec.CreatedAt, &rec.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	rec.Proxied = proxied == 1
	if syncedAt.Valid {
		rec.SyncedAt = syncedAt.Int64
	}
	return &rec, nil
}

// ----------------------------------------------------------------
// CloudFront Distribution
// ----------------------------------------------------------------

type cloudfrontDistRepo struct {
	db *sql.DB
}

func newCloudfrontDistRepo(db *sql.DB) *cloudfrontDistRepo {
	return &cloudfrontDistRepo{db: db}
}

func (r *cloudfrontDistRepo) Create(ctx context.Context, dist *repository.CloudFrontDistribution) error {
	now := time.Now().Unix()
	dist.CreatedAt = now
	dist.UpdatedAt = now

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO cdn_cloudfront_distributions (
			site_id, distribution_id, distribution_arn, domain_name,
			cert_arn, price_class, enabled, status, last_synced_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
			dist.SiteID, dist.DistributionID, dist.DistributionARN, dist.DomainName,
			dist.CertARN, dist.PriceClass, boolToInt(dist.Enabled),
			dist.Status, dist.LastSyncedAt, dist.CreatedAt, dist.UpdatedAt,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	dist.ID = id
	return nil
}

func (r *cloudfrontDistRepo) FindByID(ctx context.Context, id int64) (*repository.CloudFrontDistribution, error) {
	row := r.db.QueryRowContext(ctx, `
			SELECT id, site_id, distribution_id, distribution_arn, domain_name, cert_arn, price_class, enabled, status, last_synced_at, created_at, updated_at
		FROM cdn_cloudfront_distributions WHERE id = ?
	`, id)

	var d repository.CloudFrontDistribution
	var enabled int
	err := row.Scan(
		&d.ID, &d.SiteID, &d.DistributionID, &d.DistributionARN, &d.DomainName, &d.CertARN, &d.PriceClass, &enabled, &d.Status, &d.LastSyncedAt, &d.CreatedAt, &d.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	d.Enabled = enabled == 1
	return &d, nil
}

func (r *cloudfrontDistRepo) List(ctx context.Context) ([]*repository.CloudFrontDistribution, error) {
	rows, err := r.db.QueryContext(ctx, `
			SELECT id, site_id, distribution_id, distribution_arn, domain_name, cert_arn, price_class, enabled, status, last_synced_at, created_at, updated_at
		FROM cdn_cloudfront_distributions
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dists []*repository.CloudFrontDistribution
	for rows.Next() {
		var d repository.CloudFrontDistribution
		var enabled int
		if err := rows.Scan(
			&d.ID, &d.SiteID, &d.DistributionID, &d.DistributionARN, &d.DomainName, &d.CertARN, &d.PriceClass, &enabled, &d.Status, &d.LastSyncedAt, &d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, err
		}
		d.Enabled = enabled == 1
		dists = append(dists, &d)
	}
	return dists, rows.Err()
}

func (r *cloudfrontDistRepo) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM cdn_cloudfront_distributions WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

// ----------------------------------------------------------------
// CDN Site
// ----------------------------------------------------------------

type cdnSiteRepo struct {
	db *sql.DB
}

func newCDNSiteRepo(db *sql.DB) *cdnSiteRepo {
	return &cdnSiteRepo{db: db}
}

func (r *cdnSiteRepo) Create(ctx context.Context, site *repository.CDNSite) error {
	now := time.Now().Unix()
	site.CreatedAt = now
	site.UpdatedAt = now

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO cdn_sites (
			name, description, domain, origin_type, origin_url,
			cache_ttl, ssl_mode, custom_cert_pem, custom_key_pem,
			acceleration_mode, inbound_spec_id, provider, origin_path, origin_protocol,
			enabled, asn_tags, status, last_deployed_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		site.Name, site.Description, site.Domain, site.OriginType, site.OriginURL,
		site.CacheTTL, site.SSLMode, site.CustomCertPEM, site.CustomKeyPEM,
		site.AccelerationMode, nullableInt(site.InboundSpecID),
		site.Provider, site.OriginPath, site.OriginProtocol,
		boolToInt(site.Enabled), site.AsnTags, site.Status, nullableInt(site.LastDeployedAt),
		site.CreatedAt, site.UpdatedAt,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	site.ID = id
	return nil
}

func (r *cdnSiteRepo) Update(ctx context.Context, site *repository.CDNSite) error {
	site.UpdatedAt = time.Now().Unix()

	result, err := r.db.ExecContext(ctx, `
		UPDATE cdn_sites SET
			name = ?, description = ?, domain = ?, origin_type = ?, origin_url = ?,
			cache_ttl = ?, ssl_mode = ?, custom_cert_pem = ?, custom_key_pem = ?,
			acceleration_mode = ?, inbound_spec_id = ?, provider = ?, origin_path = ?,
			origin_protocol = ?, enabled = ?, asn_tags = ?, status = ?, last_deployed_at = ?
		WHERE id = ?
	`,
		site.Name, site.Description, site.Domain, site.OriginType, site.OriginURL,
		site.CacheTTL, site.SSLMode, site.CustomCertPEM, site.CustomKeyPEM,
		site.AccelerationMode, nullableInt(site.InboundSpecID),
		site.Provider, site.OriginPath, site.OriginProtocol,
		boolToInt(site.Enabled), site.AsnTags, site.Status, nullableInt(site.LastDeployedAt),
		site.ID,
	)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (r *cdnSiteRepo) FindByID(ctx context.Context, id int64) (*repository.CDNSite, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, description, domain, origin_type, origin_url,
			cache_ttl, ssl_mode, custom_cert_pem, custom_key_pem,
			acceleration_mode, inbound_spec_id, provider, origin_path, origin_protocol,
			enabled, asn_tags, status, last_deployed_at, created_at, updated_at
		FROM cdn_sites WHERE id = ?
	`, id)

	return r.scanSite(row)
}

func (r *cdnSiteRepo) FindByDomain(ctx context.Context, domain string) (*repository.CDNSite, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, description, domain, origin_type, origin_url,
			cache_ttl, ssl_mode, custom_cert_pem, custom_key_pem,
			acceleration_mode, inbound_spec_id, provider, origin_path, origin_protocol,
			enabled, asn_tags, status, last_deployed_at, created_at, updated_at
		FROM cdn_sites WHERE domain = ?
	`, domain)

	return r.scanSite(row)
}

func (r *cdnSiteRepo) FindByInboundSpecID(ctx context.Context, specID int64) (*repository.CDNSite, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, description, domain, origin_type, origin_url,
			cache_ttl, ssl_mode, custom_cert_pem, custom_key_pem,
			acceleration_mode, inbound_spec_id, provider, origin_path, origin_protocol,
			enabled, asn_tags, status, last_deployed_at, created_at, updated_at
		FROM cdn_sites WHERE inbound_spec_id = ?
	`, specID)

	return r.scanSite(row)
}

func (r *cdnSiteRepo) list(ctx context.Context, whereClause string, args []interface{}) ([]*repository.CDNSite, error) {
	q := `SELECT id, name, description, domain, origin_type, origin_url,
		cache_ttl, ssl_mode, custom_cert_pem, custom_key_pem,
		acceleration_mode, inbound_spec_id, provider, origin_path, origin_protocol,
		enabled, asn_tags, status, last_deployed_at, created_at, updated_at
	FROM cdn_sites`
	if whereClause != "" {
		q += " WHERE " + whereClause
	}
	q += " ORDER BY id ASC"

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sites []*repository.CDNSite
	for rows.Next() {
		site, err := r.scanSites(rows)
		if err != nil {
			return nil, err
		}
		sites = append(sites, site)
	}
	return sites, rows.Err()
}

func (r *cdnSiteRepo) List(ctx context.Context, filter repository.CDNSiteFilter) ([]*repository.CDNSite, error) {
	where, args := r.buildSiteFilter(filter)
	limit := ""
	if filter.Limit > 0 {
		limit = " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		limit += " OFFSET ?"
		args = append(args, filter.Offset)
	}
	return r.list(ctx, where+limit, args)
}

func (r *cdnSiteRepo) Count(ctx context.Context, filter repository.CDNSiteFilter) (int64, error) {
	where, args := r.buildSiteFilter(filter)
	q := "SELECT COUNT(*) FROM cdn_sites"
	if where != "" {
		q += " WHERE " + where
	}
	var count int64
	err := r.db.QueryRowContext(ctx, q, args...).Scan(&count)
	return count, err
}

func (r *cdnSiteRepo) buildSiteFilter(filter repository.CDNSiteFilter) (string, []interface{}) {
	conds := []string{}
	args := []interface{}{}

	if filter.Keyword != "" {
		conds = append(conds, "(name LIKE ? OR domain LIKE ?)")
		kw := "%" + filter.Keyword + "%"
		args = append(args, kw, kw)
	}
	if filter.Status != nil {
		conds = append(conds, "status = ?")
		args = append(args, *filter.Status)
	}
	if filter.Enabled != nil {
		conds = append(conds, "enabled = ?")
		args = append(args, boolToInt(*filter.Enabled))
	}

	where := ""
	if len(conds) > 0 {
		where = conds[0]
		for i := 1; i < len(conds); i++ {
			where += " AND " + conds[i]
		}
	}
	return where, args
}

func (r *cdnSiteRepo) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM cdn_sites WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (r *cdnSiteRepo) scanSite(row *sql.Row) (*repository.CDNSite, error) {
	var site repository.CDNSite
	var enabled int
	var inboundSpecID sql.NullInt64
	var lastDeployedAt sql.NullInt64

	err := row.Scan(
		&site.ID, &site.Name, &site.Description, &site.Domain,
		&site.OriginType, &site.OriginURL, &site.CacheTTL, &site.SSLMode,
		&site.CustomCertPEM, &site.CustomKeyPEM, &site.AccelerationMode,
		&inboundSpecID, &site.Provider, &site.OriginPath, &site.OriginProtocol,
		&enabled, &site.AsnTags, &site.Status, &lastDeployedAt, &site.CreatedAt, &site.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	site.Enabled = enabled == 1
	if inboundSpecID.Valid {
		site.InboundSpecID = &inboundSpecID.Int64
	}
	if lastDeployedAt.Valid {
		site.LastDeployedAt = &lastDeployedAt.Int64
	}
	return &site, nil
}

func (r *cdnSiteRepo) scanSites(rows *sql.Rows) (*repository.CDNSite, error) {
	var site repository.CDNSite
	var enabled int
	var inboundSpecID sql.NullInt64
	var lastDeployedAt sql.NullInt64

	err := rows.Scan(
		&site.ID, &site.Name, &site.Description, &site.Domain,
		&site.OriginType, &site.OriginURL, &site.CacheTTL, &site.SSLMode,
		&site.CustomCertPEM, &site.CustomKeyPEM, &site.AccelerationMode,
		&inboundSpecID, &site.Provider, &site.OriginPath, &site.OriginProtocol,
		&enabled, &site.AsnTags, &site.Status, &lastDeployedAt, &site.CreatedAt, &site.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	site.Enabled = enabled == 1
	if inboundSpecID.Valid {
		site.InboundSpecID = &inboundSpecID.Int64
	}
	if lastDeployedAt.Valid {
		site.LastDeployedAt = &lastDeployedAt.Int64
	}
	return &site, nil
}

// ----------------------------------------------------------------
// CDN Edge
// ----------------------------------------------------------------

type cdnEdgeRepo struct {
	db *sql.DB
}

func newCDNEdgeRepo(db *sql.DB) *cdnEdgeRepo {
	return &cdnEdgeRepo{db: db}
}

func (r *cdnEdgeRepo) Create(ctx context.Context, edge *repository.CDNEdge) error {
	now := time.Now().Unix()
	edge.CreatedAt = now
	edge.UpdatedAt = now

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO cdn_edges (
			site_id, agent_host_id, weight, enabled, status,
			last_error, deployed_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		edge.SiteID, edge.AgentHostID, edge.Weight, boolToInt(edge.Enabled),
		edge.Status, edge.LastError, nullableInt(edge.DeployedAt),
		edge.CreatedAt, edge.UpdatedAt,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	edge.ID = id
	return nil
}

func (r *cdnEdgeRepo) Update(ctx context.Context, edge *repository.CDNEdge) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE cdn_edges SET
			site_id = ?, agent_host_id = ?, weight = ?, enabled = ?, status = ?,
			last_error = ?, deployed_at = ?, updated_at = ?
		WHERE id = ?
	`,
		edge.SiteID, edge.AgentHostID, edge.Weight, boolToInt(edge.Enabled),
		edge.Status, edge.LastError, nullableInt(edge.DeployedAt),
		time.Now().Unix(), edge.ID,
	)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (r *cdnEdgeRepo) FindByID(ctx context.Context, id int64) (*repository.CDNEdge, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, site_id, agent_host_id, weight, enabled, status,
			last_error, deployed_at, created_at, updated_at
		FROM cdn_edges WHERE id = ?
	`, id)

	return r.scanEdge(row)
}

func (r *cdnEdgeRepo) ListBySiteID(ctx context.Context, siteID int64) ([]*repository.CDNEdge, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, site_id, agent_host_id, weight, enabled, status,
			last_error, deployed_at, created_at, updated_at
		FROM cdn_edges
		WHERE site_id = ?
		ORDER BY weight ASC, id ASC
	`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var edges []*repository.CDNEdge
	for rows.Next() {
		edge, err := r.scanEdges(rows)
		if err != nil {
			return nil, err
		}
		edges = append(edges, edge)
	}
	return edges, rows.Err()
}

func (r *cdnEdgeRepo) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM cdn_edges WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (r *cdnEdgeRepo) scanEdge(row *sql.Row) (*repository.CDNEdge, error) {
	var edge repository.CDNEdge
	var enabled int
	var deployedAt sql.NullInt64

	err := row.Scan(
		&edge.ID, &edge.SiteID, &edge.AgentHostID, &edge.Weight,
		&enabled, &edge.Status, &edge.LastError, &deployedAt,
		&edge.CreatedAt, &edge.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	edge.Enabled = enabled == 1
	if deployedAt.Valid {
		edge.DeployedAt = &deployedAt.Int64
	}
	return &edge, nil
}

func (r *cdnEdgeRepo) scanEdges(rows *sql.Rows) (*repository.CDNEdge, error) {
	var edge repository.CDNEdge
	var enabled int
	var deployedAt sql.NullInt64

	err := rows.Scan(
		&edge.ID, &edge.SiteID, &edge.AgentHostID, &edge.Weight,
		&enabled, &edge.Status, &edge.LastError, &deployedAt,
		&edge.CreatedAt, &edge.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	edge.Enabled = enabled == 1
	if deployedAt.Valid {
		edge.DeployedAt = &deployedAt.Int64
	}
	return &edge, nil
}

// ----------------------------------------------------------------
// CDN Cache Rule
// ----------------------------------------------------------------

type cdnCacheRuleRepo struct {
	db *sql.DB
}

func newCDNCacheRuleRepo(db *sql.DB) *cdnCacheRuleRepo {
	return &cdnCacheRuleRepo{db: db}
}

func (r *cdnCacheRuleRepo) Create(ctx context.Context, rule *repository.CDNCacheRule) error {
	rule.CreatedAt = time.Now().Unix()

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO cdn_cache_rules (
			site_id, match_type, match_value, cache_ttl, bypass, priority, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		rule.SiteID, rule.MatchType, rule.MatchValue,
		rule.CacheTTL, boolToInt(rule.Bypass), rule.Priority, rule.CreatedAt,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	rule.ID = id
	return nil
}

func (r *cdnCacheRuleRepo) Update(ctx context.Context, rule *repository.CDNCacheRule) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE cdn_cache_rules SET
			site_id = ?, match_type = ?, match_value = ?, cache_ttl = ?,
			bypass = ?, priority = ?
		WHERE id = ?
	`,
		rule.SiteID, rule.MatchType, rule.MatchValue,
		rule.CacheTTL, boolToInt(rule.Bypass), rule.Priority, rule.ID,
	)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (r *cdnCacheRuleRepo) FindByID(ctx context.Context, id int64) (*repository.CDNCacheRule, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, site_id, match_type, match_value, cache_ttl, bypass, priority, created_at
		FROM cdn_cache_rules WHERE id = ?
	`, id)

	return r.scanCacheRule(row)
}

func (r *cdnCacheRuleRepo) ListBySiteID(ctx context.Context, siteID int64) ([]*repository.CDNCacheRule, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, site_id, match_type, match_value, cache_ttl, bypass, priority, created_at
		FROM cdn_cache_rules
		WHERE site_id = ?
		ORDER BY priority ASC, id ASC
	`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []*repository.CDNCacheRule
	for rows.Next() {
		rule, err := r.scanCacheRules(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func (r *cdnCacheRuleRepo) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM cdn_cache_rules WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(result)
}

func (r *cdnCacheRuleRepo) scanCacheRule(row *sql.Row) (*repository.CDNCacheRule, error) {
	var rule repository.CDNCacheRule
	var bypass int

	err := row.Scan(
		&rule.ID, &rule.SiteID, &rule.MatchType, &rule.MatchValue,
		&rule.CacheTTL, &bypass, &rule.Priority, &rule.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	rule.Bypass = bypass == 1
	return &rule, nil
}

func (r *cdnCacheRuleRepo) scanCacheRules(rows *sql.Rows) (*repository.CDNCacheRule, error) {
	var rule repository.CDNCacheRule
	var bypass int

	err := rows.Scan(
		&rule.ID, &rule.SiteID, &rule.MatchType, &rule.MatchValue,
		&rule.CacheTTL, &bypass, &rule.Priority, &rule.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	rule.Bypass = bypass == 1
	return &rule, nil
}
