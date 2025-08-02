package pec_storage

import "database/sql"

type AuthorityRegistry struct {
	db *sql.DB
}

func (ar *AuthorityRegistry) GetByDomain(domain string) (*PECAuthority, error) {
	const query = `
        SELECT id, name, smtp_addr, notification_address
        FROM pec_authorities
        WHERE name = $1 OR notification_address LIKE '%' || $1
        LIMIT 1`
	var id int
	var auth PECAuthority
	err := ar.db.QueryRow(query, domain).Scan(&id, &auth.Name, &auth.SMTPAddr, &auth.NotificationAddress)
	if err != nil {
		return nil, err
	}

	// Load certificate hashes
	const hashQuery = `SELECT sha1_hash FROM pec_cert_hashes WHERE authority_id = $1`
	rows, err := ar.db.Query(hashQuery, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			return nil, err
		}
		auth.ProviderCertificateHashes = append(auth.ProviderCertificateHashes, hash)
	}
	return &auth, nil
}

func (ar *AuthorityRegistry) GetByCertHash(hash string) (*PECAuthority, error) {
	const query = `
        SELECT a.id, a.name, a.smtp_addr, a.notification_address
        FROM pec_authorities a
        JOIN pec_cert_hashes c ON a.id = c.authority_id
        WHERE c.sha1_hash = $1
        LIMIT 1`
	var id int
	var auth PECAuthority
	err := ar.db.QueryRow(query, hash).Scan(&id, &auth.Name, &auth.SMTPAddr, &auth.NotificationAddress)
	if err != nil {
		return nil, err
	}

	// Load all hashes for this authority
	const hashQuery = `SELECT sha1_hash FROM pec_cert_hashes WHERE authority_id = $1`
	rows, err := ar.db.Query(hashQuery, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			return nil, err
		}
		auth.ProviderCertificateHashes = append(auth.ProviderCertificateHashes, h)
	}
	return &auth, nil
}

func (ar *AuthorityRegistry) ListAuthorities() ([]*PECAuthority, error) {
	const query = `SELECT id, name, smtp_addr, notification_address FROM pec_authorities`
	rows, err := ar.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var authorities []*PECAuthority
	for rows.Next() {
		var id int
		var auth PECAuthority
		if err := rows.Scan(&id, &auth.Name, &auth.SMTPAddr, &auth.NotificationAddress); err != nil {
			return nil, err
		}
		// Load hashes
		const hashQuery = `SELECT sha1_hash FROM pec_cert_hashes WHERE authority_id = $1`
		hashRows, err := ar.db.Query(hashQuery, id)
		if err != nil {
			return nil, err
		}
		for hashRows.Next() {
			var h string
			if err := hashRows.Scan(&h); err != nil {
				hashRows.Close()
				return nil, err
			}
			auth.ProviderCertificateHashes = append(auth.ProviderCertificateHashes, h)
		}
		hashRows.Close()
		authorities = append(authorities, &auth)
	}
	return authorities, nil
}
