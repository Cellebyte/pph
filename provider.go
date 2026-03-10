// Package pph implements a DNS record management client compatible
// with the libdns interfaces for PrepaidHoster.
package pph

import (
	"context"
	"fmt"
	"sync"

	"github.com/libdns/libdns"
	"github.com/libdns/pph/internal/client"
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
	lock     sync.Mutex        `json:"-"`
	once     sync.Once         `json:"-"`
}

// GetRecords lists all the records in the zone.
func (p *Provider) GetRecords(ctx context.Context, zone string) (records []libdns.Record, err error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.init(ctx)
	records, _, err = p.getRecordsWithDomain(ctx, zone)
	return records, err
}

// AppendRecords adds records to the zone. It returns the records that were added.
func (p *Provider) AppendRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.init(ctx)
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
				return created, fmt.Errorf("creating libdns.Record: %w", err)
			}
			created = append(created, record)
		}
	}
	return created, nil
}

// SetRecords sets the records in the zone, either by updating existing records or creating new ones.
// It returns the updated records.
func (p *Provider) SetRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.init(ctx)
	prevs, domain, err := p.getRecordsWithDomain(ctx, zone)
	if err != nil {
		return nil, err
	}
	var updated []libdns.Record
	for _, new := range records {
		toUpdate := false
		newClientRecord, err := fromRecord(zone, new)
		if err != nil {
			return updated, fmt.Errorf("constructing newClientRecord from zone=%q: %w", zone, err)
		}
		for _, prev := range prevs {
			if ok, _ := Equal(new, prev, false); !ok {
				// found a record we want to update
				// adding the ID
				toUpdate = true
				oldClientRecord, err := fromRecord(zone, prev)
				if err != nil {
					return updated, fmt.Errorf("constructing oldClientRecord from zone=%q: %w", zone, err)
				}
				newClientRecord.ID = oldClientRecord.ID
				break
			}
		}
		clientRecord, err := p.client.CreateRecord(domain, newClientRecord, toUpdate)
		if err != nil {
			return updated, fmt.Errorf("calling CreateRecord in zone=%q: %w", zone, err)
		}
		newRecord, err := toRecord(zone, *clientRecord)
		if err != nil {
			return updated, fmt.Errorf("constructing libdns.Record from zone=%q: %w", zone, err)
		}
		updated = append(updated, newRecord)
	}
	return updated, nil
}

// DeleteRecords deletes the specified records from the zone. It returns the records that were deleted.
func (p *Provider) DeleteRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.init(ctx)
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
	p.lock.Lock()
	defer p.lock.Unlock()
	p.init(ctx)
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
