package pph

import (
	"errors"
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

func fromRecord(zone string, record libdns.Record) (dnsRecord client.Record, err error) {
	relativeName := libdns.RelativeName(record.RR().Name, zone)
	switch record.RR().Type {
	case "A":
		// lol break is default
		fallthrough
	case "AAAA":
		// lol break is default
		fallthrough
	case "TXT":
		// lol break is default
		fallthrough
	case "CNAME":
		dnsRecord = client.Record{
			Name:    relativeName,
			Type:    record.RR().Type,
			TTL:     int(record.RR().TTL.Abs().Seconds()),
			Content: record.RR().Data,
		}
	case "MX":
		parsed, err := record.RR().Parse()
		if err != nil {
			return dnsRecord, fmt.Errorf("libdns.Parse(): %w", err)
		}
		if parsedMX, ok := parsed.(libdns.MX); ok {
			dnsRecord = client.Record{
				Name:    relativeName,
				Type:    parsedMX.RR().Type,
				TTL:     int(parsedMX.RR().TTL.Abs().Seconds()),
				Content: parsedMX.Target,
				Prio:    parsedMX.Preference,
			}
		}
	}
	return dnsRecord, nil
}

var NotFoundMatchError = errors.New("no matching Record found")

func findClosestMatches(zone string, record libdns.Record, currentRecords []client.RecordGet, delete bool) (matchingRecord *client.Record, err error) {
	if record == nil {
		return matchingRecord, fmt.Errorf("missing record")
	}
	var matchingRecords []client.Record
	for _, currentRecord := range currentRecords {
		libDnsRecord, err := toRecord(zone, currentRecord.Record)
		if err != nil {
			return matchingRecord, err
		}
		if libDnsRecord == nil {
			continue
		}
		// most specific match to find Record
		if (record.RR().Name != "" && record.RR().Name == libDnsRecord.RR().Name) &&
			(record.RR().Data != "" && record.RR().Data == libDnsRecord.RR().Data) &&
			(record.RR().Type != "" && record.RR().Type == libDnsRecord.RR().Type) &&
			(record.RR().TTL == libDnsRecord.RR().TTL) {
			matchingRecords = append(matchingRecords, currentRecord.Record)
			continue
		}
		// try to use Data as a unique identifier
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
			// this is needed, to decide if we are called from SetRecords or from DeleteRecords
			// When called from SetRecords we want a match even if the content differs as we will
			// update it anyways.
			((record.RR().Data == "") && delete || (record.RR().Data != "") && !delete) &&
			(record.RR().TTL == time.Duration(0)) {
			matchingRecords = append(matchingRecords, currentRecord.Record)
			continue
		}
		if record.RR().Name == "" || record.RR().Type == "" {
			return matchingRecord, fmt.Errorf("missing enough information name=%q or type=%q are empty", record.RR().Name, record.RR().Type)
		}
	}
	if len(matchingRecords) < 1 {
		return matchingRecord, NotFoundMatchError
	}
	if len(matchingRecords) > 1 {
		return matchingRecord, fmt.Errorf("finding multiple matching record for name=%q type=%q content=%q found: %v [%d]", record.RR().Name, record.RR().Type, record.RR().Data, matchingRecords, len(matchingRecords))
	}
	// now it is unique
	matchingRecord = &matchingRecords[0]
	return matchingRecord, err

}
