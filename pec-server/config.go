package main

import (
	"encoding/json"
	"os"
)

type Config struct {
	Domain            string `json:"domain"`
	SMTPServer        string `json:"smtp_server"`
	SMTPNetworkServer string `json:"smtp_network_server"`
	IMAPServer        string `json:"imap_server"`
	CertFile          string `json:"cert_file"`
	KeyFile           string `json:"key_file"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
