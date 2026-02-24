// Package pph implements a DNS record management client compatible
// with the libdns interfaces for PrepaidHoster.
package main

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
	clientRecords, _, err := p.client.GetRecords(zone)
	if err != nil {
		return records, fmt.Errorf("invoking GetRecords on client: %w", err)
	}
	for _, clientRecord := range clientRecords {
		record, err := toRecord(zone, clientRecord.Record)
		if err != nil {
			return records, fmt.Errorf("constructing record %v: %w", clientRecord, err)
		}
		if record == nil {
			// This skips unsupported Records like PTR's and others
			continue
		}
		records = append(records, record)
	}
	return records, nil
}

// AppendRecords adds records to the zone. It returns the records that were added.
func (p *Provider) AppendRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	return nil, fmt.Errorf("TODO: not implemented")
}

// SetRecords sets the records in the zone, either by updating existing records or creating new ones.
// It returns the updated records.
func (p *Provider) SetRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	return nil, fmt.Errorf("TODO: not implemented")
}

// DeleteRecords deletes the specified records from the zone. It returns the records that were deleted.
func (p *Provider) DeleteRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	deletedRecords := []libdns.Record{}
	currentRecords, _, err := p.client.GetRecords(zone)
	if err != nil {
		return deletedRecords, fmt.Errorf("invoking GetRecords on client: %w", err)
	}
	for _, record := range records {
		matchingRecord, err := findClosestMatches(zone, record, currentRecords)
		if err != nil {
			return deletedRecords, fmt.Errorf("finding match: %w", err)
		}
		err = p.client.DeleteRecord(zone, matchingRecord)
		if err != nil {
			return deletedRecords, fmt.Errorf("deleting type=%q record=%q zone=%q: %w", record.RR().Type, record.RR().Name, zone, err)
		}
		deletedRecord, err := toRecord(zone, matchingRecord)
		if err != nil {
			return deletedRecords, fmt.Errorf("constructing libdns.Record: %w", err)
		}
		deletedRecords = append(deletedRecords, deletedRecord)
	}
	return deletedRecords, nil
}

// ListZones lists all the zones in the account.
func (p *Provider) ListZones(ctx context.Context) ([]libdns.Zone, error) {
	return nil, fmt.Errorf("TODO: not implemented")
}

// Interface guards
var (
	_ libdns.RecordGetter   = (*Provider)(nil)
	_ libdns.RecordAppender = (*Provider)(nil)
	_ libdns.RecordSetter   = (*Provider)(nil)
	_ libdns.RecordDeleter  = (*Provider)(nil)
	_ libdns.ZoneLister     = (*Provider)(nil)
)

func main() {
	zone := "cellebyte.de"
	dnsProvider := New("")
	ctx := context.Background()
	records, err := dnsProvider.GetRecords(ctx, zone)
	if err != nil {
		panic(fmt.Errorf("GetRecords: %w", err))
	}
	for _, record := range records {
		fmt.Println(record.RR().Type, ":", libdns.AbsoluteName(record.RR().Name, zone), "->", record.RR().Data)
	}
	deleteRecords := []libdns.Record{
		libdns.CNAME{
			Name:   "tester",
			Target: "test.cellebyte.de",
		},
	}
	deletedRecords, err := dnsProvider.DeleteRecords(ctx, "cellebyte.de", deleteRecords)
	if err != nil {
		panic(fmt.Errorf("DeleteRecords %v: %w", deleteRecords, err))
	}
	fmt.Println("Removed Records:")
	for _, record := range deletedRecords {
		fmt.Println(record.RR().Type, ":", libdns.AbsoluteName(record.RR().Name, zone), "->", record.RR().Data)
	}
}
