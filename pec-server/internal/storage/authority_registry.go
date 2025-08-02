package pec_storage

type PECAuthority struct {
	Name                      string
	SMTPAddr                  string
	NotificationAddress       string
	ProviderCertificateHashes []string
}

// AuthorityRegistryStore defines the interface for authority registry storage backends.
type AuthorityRegistryStore interface {
	// GetByDomain returns the authority for a given domain (or name).
	GetByDomain(domain string) (*PECAuthority, error)
	// GetByCertHash returns the authority for a given certificate hash.
	GetByCertHash(hash string) (*PECAuthority, error)
	// (Optional) List all authorities.
	ListAuthorities() ([]*PECAuthority, error)
}
