package geoip

import (
	"fmt"
	"net"
	"sync"

	"github.com/oschwald/geoip2-golang"
)

// LookupResult holds GeoIP + ASN information for an IP address.
type LookupResult struct {
	Country     string // ISO country code (e.g. "CN", "JP", "SG")
	ASN         uint   // Autonomous System Number
	ASNOrg      string // ASN organization name
}

// Reader wraps MaxMind GeoIP2 databases for Country + ASN lookups.
type Reader struct {
	country *geoip2.Reader
	asn     *geoip2.Reader
	mu      sync.RWMutex
}

// OpenReader creates a Reader from MaxMind .mmdb file paths.
// If a path is empty, that lookup type is skipped (returns zero values).
func OpenReader(countryDBPath, asnDBPath string) (*Reader, error) {
	r := &Reader{}
	if countryDBPath != "" {
		db, err := geoip2.Open(countryDBPath)
		if err != nil {
			return nil, fmt.Errorf("open country db: %w", err)
		}
		r.country = db
	}
	if asnDBPath != "" {
		db, err := geoip2.Open(asnDBPath)
		if err != nil {
			return nil, fmt.Errorf("open asn db: %w", err)
		}
		r.asn = db
	}
	return r, nil
}

// Lookup performs GeoIP Country + ASN lookup for the given IP string.
func (r *Reader) Lookup(ipStr string) (*LookupResult, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP: %s", ipStr)
	}

	result := &LookupResult{}

	if r.country != nil {
		c, err := r.country.Country(ip)
		if err != nil {
			return nil, fmt.Errorf("country lookup: %w", err)
		}
		result.Country = c.Country.IsoCode
	}

	if r.asn != nil {
		a, err := r.asn.ASN(ip)
		if err != nil {
			return nil, fmt.Errorf("asn lookup: %w", err)
		}
		result.ASN = a.AutonomousSystemNumber
		result.ASNOrg = a.AutonomousSystemOrganization
	}

	return result, nil
}

// Close closes the underlying databases.
func (r *Reader) Close() error {
	if r.country != nil {
		r.country.Close()
	}
	if r.asn != nil {
		r.asn.Close()
	}
	return nil
}
