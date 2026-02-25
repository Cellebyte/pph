package pph

import (
	"fmt"
	"net/netip"
	"time"

	"github.com/libdns/libdns"
	"github.com/libdns/pph/internal/client"
)

func toRecord(zone string, record client.Record) (dnsRecord libdns.Record, err error) {
	// This provider supports also
	// * PTR
	// * SPF (weird as it should be a TXT record)
	// * TLSA
	relativeName := libdns.RelativeName(record.Name, zone)
	ttl := time.Duration(record.TTL) * time.Second
	providerData := client.PPHProviderData{
		ID: record.ID,
	}
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
			TTL:          ttl,
			IP:           ip,
			ProviderData: providerData,
		}
	case "TXT":
		dnsRecord = libdns.TXT{
			Name:         relativeName,
			TTL:          ttl,
			Text:         record.Content,
			ProviderData: providerData,
		}
	case "CNAME":
		dnsRecord = libdns.CNAME{
			Name:         relativeName,
			TTL:          ttl,
			Target:       record.Content,
			ProviderData: providerData,
		}
	case "MX":
		dnsRecord = libdns.MX{
			Name:         relativeName,
			TTL:          ttl,
			Preference:   record.Prio,
			Target:       record.Content,
			ProviderData: providerData,
		}
	default:
		dnsRecord = libdns.RR{
			Type: record.Type,
			Name: relativeName,
			TTL:  ttl,
			Data: record.Content,
		}
	}
	return dnsRecord, nil
}

func fromRecord(zone string, record libdns.Record) (dnsRecord client.Record, err error) {
	rr := record.RR()

	relativeName := libdns.RelativeName(record.RR().Name, zone)
	if relativeName == "@" {
		relativeName = ""
	}

	dnsRecord = client.Record{
		Type: rr.Type,
		Name: relativeName,
		TTL:  int(rr.TTL.Abs().Seconds()),
	}

	if rr, ok := record.(libdns.RR); ok {
		// if passed in variable is an RR, parse to get the specific type
		record, err = rr.Parse()
		if err != nil {
			return dnsRecord, err
		}
	}
	switch rr := record.(type) {
	case libdns.Address:
		dnsRecord.IDfromProviderData(rr.ProviderData)
		dnsRecord.Content = rr.IP.String()
	case libdns.CNAME:
		dnsRecord.IDfromProviderData(rr.ProviderData)
		dnsRecord.Content = rr.Target
	case libdns.TXT:
		dnsRecord.IDfromProviderData(rr.ProviderData)
		dnsRecord.Content = rr.Text
	case libdns.MX:
		dnsRecord.IDfromProviderData(rr.ProviderData)
		dnsRecord.Content = rr.Target
		dnsRecord.Prio = rr.Preference
	default:
		err = fmt.Errorf("dnsRecord %+v: record type not implemented", record)
	}
	return dnsRecord, err
}

func Equal(recordA libdns.Record, recordB libdns.Record, delete bool) (bool, error) {
	if recordA == nil || recordB == nil {
		return false, fmt.Errorf("missing recordA=%q or recordB=%q", recordA, recordB)
	}
	// most specific match to find Record
	if (recordA.RR().Name != "" && recordA.RR().Name == recordB.RR().Name) &&
		(recordA.RR().Data != "" && recordA.RR().Data == recordB.RR().Data) &&
		(recordA.RR().Type != "" && recordA.RR().Type == recordB.RR().Type) &&
		(recordA.RR().TTL == recordB.RR().TTL) {
		return true, nil
	}
	// try to use Data as a unique identifier
	if (recordA.RR().Name != "" && recordA.RR().Name == recordB.RR().Name) &&
		(recordA.RR().Data != "" && recordA.RR().Data == recordB.RR().Data) &&
		(recordA.RR().Type != "" && recordA.RR().Type == recordB.RR().Type) &&
		(recordA.RR().TTL == time.Duration(0)) {
		return true, nil
	}
	// try to compare using Name and Type
	if (recordA.RR().Name != "" && recordA.RR().Name == recordB.RR().Name) &&
		(recordA.RR().Type != "" && recordA.RR().Type == recordB.RR().Type) &&
		// this is needed, to decide if we are called from AppendRecords, SetRecords or from DeleteRecords
		// When called from SetRecords we want a match even if the content differs as we will
		// update it anyways.
		((recordA.RR().Data == "") && delete || (recordA.RR().Data != "") && !delete) &&
		(recordA.RR().TTL == time.Duration(0)) {
		return true, nil
	}
	return false, nil
}
