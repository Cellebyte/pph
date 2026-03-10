package pph

import (
	"context"
	"fmt"
	"os"

	"github.com/libdns/libdns"
	"github.com/libdns/pph/internal/client"
)

const (
	applicationName = "github.com/libdns/pph"
	endpointURL     = "https://fsn-01.api.pph.sh"
	envVarName      = "API_TOKEN"
)

func (p *Provider) getClient() *client.PPHClient {
	p.client = client.New(p.APIToken, endpointURL)
	return p.client
}

func (p *Provider) init(_ context.Context) {
	p.once.Do(
		func() {
			if p.APIToken == "" {
				p.APIToken = os.Getenv(envVarName)
			}
			if p.APIToken == "" {
				panic(fmt.Sprintf("%s: API token missing", applicationName))
			}
			p.client = p.getClient()
		},
	)
}

func (p *Provider) getRecordsWithDomain(_ context.Context, zone string) (records []libdns.Record, domain client.DomainGet, err error) {
	clientRecords, domain, err := p.client.GetRecords(zone)
	if err != nil {
		return records, domain, fmt.Errorf("invoking GetRecords on client: %w", err)
	}
	for _, clientRecord := range clientRecords {
		record, err := toRecord(zone, clientRecord.Record)
		if err != nil {
			return records, domain, fmt.Errorf("constructing record %+v: %w", clientRecord, err)
		}
		if record == nil {
			// This skips unsupported Records like PTR's and others
			continue
		}
		records = append(records, record)
	}
	return records, domain, nil
}
