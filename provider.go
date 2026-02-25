// Package pph implements a DNS record management client compatible
// with the libdns interfaces for PrepaidHoster.
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

	endpointURL = "https://fsn-01.api.pph.sh"

	envVarName = "API_TOKEN"
)

// TODO: Providers must not require additional provisioning steps by the callers; it
// should work simply by populating a struct and calling methods on it. If your DNS
// service requires long-lived state or some extra provisioning step, do it implicitly
// when methods are called; sync.Once can help with this, and/or you can use a
// sync.(RW)Mutex in your Provider struct to synchronize implicit provisioning.

// Provider facilitates DNS record manipulation with pph
type Provider struct {
	APIToken string            `json:"api_token,omitempty"`
	client   *client.PPHClient `json:"-"`
}

func New(token string) *Provider {
	if token == "" {
		token = os.Getenv(envVarName)
	}
	if token == "" {
		panic(fmt.Sprintf("%s: API token missing", applicationName))
	}
	p := Provider{
		APIToken: token,
	}
	p.client = p.getClient()
	return &p
}

func (p *Provider) getClient() *client.PPHClient {
	p.client = client.New(p.APIToken, endpointURL)
	return p.client
}

// GetRecords lists all the records in the zone.
func (p *Provider) GetRecords(ctx context.Context, zone string) (records []libdns.Record, err error) {
	records, _, err = p.getRecordsWithDomain(ctx, zone)
	return records, err
}

func (p *Provider) getRecordsWithDomain(ctx context.Context, zone string) (records []libdns.Record, domain client.DomainGet, err error) {
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

// AppendRecords adds records to the zone. It returns the records that were added.
func (p *Provider) AppendRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	prevs, domain, err := p.getRecordsWithDomain(ctx, zone)
	if err != nil {
		return nil, err
	}
	created := []libdns.Record{}
	for _, new := range records {
		toCreate := true
		for _, prev := range prevs {
			if ok, _ := Equal(new, prev, false); ok {
				toCreate = false
				break
			}
		}
		if toCreate {
			clientRecord, err := fromRecord(zone, new)
			if err != nil {
				return created, fmt.Errorf("creating clientRecord: %w", err)
			}
			createdRecord, err := p.client.CreateRecord(domain, clientRecord, true)
			if err != nil {
				return created, fmt.Errorf("invoking CreateRecord on client: %w", err)
			}
			if createdRecord == nil {
				return created, fmt.Errorf("no record created: %w", err)
			}
			record, err := toRecord(zone, *createdRecord)
			if err != nil {
				return created, fmt.Errorf("creating libdnsRecord: %w", err)
			}
			created = append(created, record)
		}
	}
	return created, nil
}

// SetRecords sets the records in the zone, either by updating existing records or creating new ones.
// It returns the updated records.
func (p *Provider) SetRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	prevs, err := p.GetRecords(ctx, zone)
	if err != nil {
		return nil, err
	}
	toDelete := []libdns.Record{}
	for _, prev := range prevs {
		for _, new := range records {
			if ok, _ := Equal(new, prev, false); !ok {
				toDelete = append(toDelete, prev)
			}
		}
	}
	_, err = p.DeleteRecords(ctx, zone, toDelete)
	return p.AppendRecords(ctx, zone, records)
}

// func (p *Provider) SetRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
// 	setRecords := []libdns.Record{}
// 	currentRecords, err := p.GetRecords(ctx, zone)
// 	if err != nil {
// 		return setRecords, fmt.Errorf("invoking GetRecords on client: %w", err)
// 	}
// 	for _, record := range records {
// 		matchingRecord, err := findClosestMatches(zone, record, currentRecords, false)
// 		if err != nil && !errors.Is(err, NotFoundMatchError) {
// 			return setRecords, fmt.Errorf("finding match: %w", err)
// 		}
// 		clientRecord, err := fromRecord(zone, record)
// 		if matchingRecord != nil {
// 			clientRecord.ID = matchingRecord.ID
// 		}
// 		createdRecord, err := p.client.CreateRecord(domain, clientRecord, true)
// 		if err != nil {
// 			return nil, fmt.Errorf("invoking CreateRecord on client: %w", err)
// 		}
// 		if createdRecord == nil {
// 			return setRecords, nil
// 		}
// 		setRecord, err := toRecord(zone, *createdRecord)
// 		if err != nil {

// 		}
// 		setRecords = append(setRecords, setRecord)
// 	}
// 	return setRecords, nil
// }

// DeleteRecords deletes the specified records from the zone. It returns the records that were deleted.
func (p *Provider) DeleteRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	prevs, domain, err := p.getRecordsWithDomain(ctx, zone)
	if err != nil {
		return nil, err
	}
	deleted := []libdns.Record{}
	for _, prev := range prevs {
		for _, record := range records {
			if ok, _ := Equal(record, prev, true); ok {
				clientRecord, err := fromRecord(zone, prev)
				if err != nil {
					return deleted, err
				}
				err = p.client.DeleteRecord(domain, clientRecord)
				if err != nil {
					return deleted, fmt.Errorf("invoking DeleteRecord on client with type=%q record=%q zone=%q: %w", record.RR().Type, record.RR().Name, zone, err)
				}
				deleted = append(deleted, prev)
			}
		}
	}
	return deleted, nil
}

// ListZones lists all the zones in the account.
func (p *Provider) ListZones(ctx context.Context) ([]libdns.Zone, error) {
	domains, err := p.client.GetDomains()
	if err != nil {
		return nil, fmt.Errorf("getting domains: %w", err)
	}
	var zones []libdns.Zone
	for _, domain := range domains {
		zones = append(zones, libdns.Zone{
			Name: domain.Domain,
		})

	}
	return zones, nil
}

// Interface guards
var (
	_ libdns.RecordGetter   = (*Provider)(nil)
	_ libdns.RecordAppender = (*Provider)(nil)
	_ libdns.RecordSetter   = (*Provider)(nil)
	_ libdns.RecordDeleter  = (*Provider)(nil)
	_ libdns.ZoneLister     = (*Provider)(nil)
)
