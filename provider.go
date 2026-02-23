// Package pph implements a DNS record management client compatible
// with the libdns interfaces for PrepaidHoster.
package main

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"time"

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

// Provider facilitates DNS record manipulation with <TODO: PROVIDER NAME>.
type Provider struct {
	APIToken string `json:"api_token,omitempty"`
	client   *client.PPHClient
	// Exported config fields should be JSON-serializable or omitted (`json:"-"`)
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

func findClosestMatches(zone string, record libdns.Record, currentRecords []client.RecordGet) (matchingRecord client.Record, err error) {
	if record == nil {
		return matchingRecord, fmt.Errorf("missing record")
	}
	var matchingRecords []client.Record
	for _, currentRecord := range currentRecords {
		libDnsRecord, err := constructLibDNSRecord(zone, currentRecord.Record)
		if err != nil {
			return matchingRecord, err
		}
		if libDnsRecord == nil {
			continue
		}
		// most specific match to find Record
		fmt.Println(record.RR().Data, currentRecord)
		if (record.RR().Name != "" && record.RR().Name == libDnsRecord.RR().Name) &&
			(record.RR().Data != "" && record.RR().Data == libDnsRecord.RR().Data) &&
			(record.RR().Type != "" && record.RR().Type == libDnsRecord.RR().Type) &&
			(record.RR().TTL == libDnsRecord.RR().TTL) {
			matchingRecords = append(matchingRecords, currentRecord.Record)
			continue
		}
		// try to use Content as a unique identifier
		if (record.RR().Name != "" && record.RR().Name == libDnsRecord.RR().Name) &&
			(record.RR().Data != "" && record.RR().Data == libDnsRecord.RR().Data) &&
			(record.RR().Type != "" && record.RR().Type == libDnsRecord.RR().Type) &&
			(record.RR().TTL == time.Duration(0)) {
			matchingRecords = append(matchingRecords, currentRecord.Record)
			continue
		}
		// try to compare using Name and Type
		if (record.RR().Name != "" && record.RR().Name == libDnsRecord.RR().Name) &&
			(record.RR().Type != "" && record.RR().Type == libDnsRecord.RR().Type) &&
			(record.RR().Data == "") &&
			(record.RR().TTL == time.Duration(0)) {
			matchingRecords = append(matchingRecords, currentRecord.Record)
			continue
		}
		if record.RR().Name == "" || record.RR().Type == "" {
			return matchingRecord, fmt.Errorf("missing enough information name=%q or type=%q are empty", record.RR().Name, record.RR().Type)
		}
	}
	if len(matchingRecords) < 1 || len(matchingRecords) > 1 {
		return matchingRecord, fmt.Errorf("finding matching record for name=%q type=%q content=%q found: %v [%d]", record.RR().Name, record.RR().Type, record.RR().Data, matchingRecords, len(matchingRecords))
	}
	// now it is unique
	matchingRecord = matchingRecords[0]
	return matchingRecord, err

}

func constructLibDNSRecord(zone string, record client.Record) (dnsRecord libdns.Record, err error) {
	// This provider supports also
	// * PTR
	// * SPF (weird as it should be a TXT record)
	// * TLSA
	relativeName := libdns.RelativeName(record.Name, zone)
	switch record.Type {
	case "A":
		// lol break is default
		fallthrough
	case "AAAA":
		ip, err := netip.ParseAddr(record.Content)
		if err != nil {
			return dnsRecord, fmt.Errorf("parsing address %q: %w", record.Content, err)
		}
		dnsRecord = libdns.Address{
			Name:         relativeName,
			TTL:          time.Duration(record.TTL) * time.Second,
			IP:           ip,
			ProviderData: record,
		}
	case "TXT":
		dnsRecord = libdns.TXT{
			Name:         relativeName,
			TTL:          time.Duration(record.TTL) * time.Second,
			Text:         record.Content,
			ProviderData: record,
		}
	case "CNAME":
		dnsRecord = libdns.CNAME{
			Name:         relativeName,
			TTL:          time.Duration(record.TTL) * time.Second,
			Target:       record.Content,
			ProviderData: record,
		}
	case "MX":
		dnsRecord = libdns.MX{
			Name:         relativeName,
			TTL:          time.Duration(record.TTL) * time.Second,
			Preference:   record.Prio,
			Target:       record.Content,
			ProviderData: record,
		}
	}
	return dnsRecord, nil
}

// GetRecords lists all the records in the zone.
func (p *Provider) GetRecords(ctx context.Context, zone string) (records []libdns.Record, err error) {
	// Make sure to return RR-type-specific structs, not libdns.RR structs.
	clientRecords, _, err := p.client.GetRecords(zone)
	if err != nil {
		return records, fmt.Errorf("invoking GetRecords on client: %w", err)
	}
	for _, clientRecord := range clientRecords {
		record, err := constructLibDNSRecord(zone, clientRecord.Record)
		if err != nil {
			return records, fmt.Errorf("constructing record %v: %w", clientRecord, err)
		}
		if record == nil {
			//This skips unsupported Records
			continue
		}
		records = append(records, record)
	}
	return records, nil
}

// AppendRecords adds records to the zone. It returns the records that were added.
func (p *Provider) AppendRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	// Make sure to return RR-type-specific structs, not libdns.RR structs.
	return nil, fmt.Errorf("TODO: not implemented")
}

// SetRecords sets the records in the zone, either by updating existing records or creating new ones.
// It returns the updated records.
func (p *Provider) SetRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	// Make sure to return RR-type-specific structs, not libdns.RR structs.
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
		deletedRecord, err := constructLibDNSRecord(zone, matchingRecord)
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
