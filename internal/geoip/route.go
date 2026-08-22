package geoip

// MatchResult holds the matched ASN information from a GeoIP lookup.
type MatchResult struct {
	Country string // ISO country code
	ASN     uint   // matched ASN number
	ASNOrg  string // ASN organization name
}

// ClassifyASN determines the carrier category for a Chinese ASN.
type Carrier string

const (
	CarrierMobile Carrier = "mobile"  // AS9808, 56041 etc.
	CarrierUnicom Carrier = "unicom"  // AS4837, 9929 etc.
	CarrierTelecom Carrier = "telecom" // AS4134, 4809 etc.
	CarrierOther  Carrier = "other"
)

// ClassifyCNCarrier returns the carrier category for a Chinese ASN.
func ClassifyCNCarrier(asn uint) Carrier {
	switch asn {
	case 9808, 56041, 56046, 56047, 56048:
		return CarrierMobile
	case 4837, 4808, 17816, 17623, 9929:
		return CarrierUnicom
	case 4134, 4809, 4812, 58543:
		return CarrierTelecom
	default:
		return CarrierOther
	}
}
