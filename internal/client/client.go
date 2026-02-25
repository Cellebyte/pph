package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	pphclient "github.com/cellebyte/go-pph"
)

type PPHClient struct {
	token         string
	client        *pphclient.APIClient
	clientContext context.Context
}

func New(token, endpointUrl string) *PPHClient {
	configuration := pphclient.NewConfiguration()
	configuration.Servers = pphclient.ServerConfigurations{
		pphclient.ServerConfiguration{
			URL:         endpointUrl,
			Description: "No description provided",
		},
	}
	apiClient := pphclient.NewAPIClient(configuration)
	return &PPHClient{token: token, client: apiClient, clientContext: context.Background()}
}

func (c PPHClient) GetDomains() (domains []DomainGet, err error) {
	resp, _, err := c.client.ClientDomainsAPI.ClientDomainsGet(c.clientContext).XToken(c.token).Execute()
	if err != nil {
		return nil, fmt.Errorf("getting domains: %w", err)
	}
	rawData, err := json.Marshal(resp.GetData())
	if err != nil {
		return nil, fmt.Errorf("constructing domain data: %w", err)
	}
	err = json.Unmarshal(rawData, &domains)
	if err != nil {
		return nil, fmt.Errorf("parsing domains response: %w", err)
	}
	return domains, nil
}

func (c PPHClient) GetRecordsByDomain(domain DomainGet) ([]RecordGet, DomainGet, error) {
	// we pass domain as it is easier to handle it that way.
	resp, _, err := c.client.ClientDomainsAPI.ClientDomainsDomainIdDnsRecordsGet(c.clientContext, domain.ID).XToken(c.token).Execute()
	if err != nil {
		return nil, domain, fmt.Errorf("getting records for domain=%q: %w", domain.Domain, err)
	}
	rawData, err := json.Marshal(resp.GetData())
	if err != nil {
		return nil, domain, fmt.Errorf("connstructing record data: %w", err)
	}
	dRecords := DomainRecordsGet{}
	err = json.Unmarshal(rawData, &dRecords)
	if err != nil {
		return nil, domain, fmt.Errorf("parsing records response %s: %w", rawData, err)
	}
	return dRecords.Records, domain, nil
}

func (c PPHClient) GetDomain(zone string) (domain DomainGet, err error) {
	domains, err := c.GetDomains()
	if err != nil {
		return domain, fmt.Errorf("calling getDomains: %w", err)
	}
	for _, domain := range domains {
		if domain.DomainIdn == zone || domain.Domain == zone {
			return domain, nil
		}
	}
	return domain, fmt.Errorf("finding domain by zone=%q: %w", zone, err)
}

func (c PPHClient) GetRecords(zone string) (records []RecordGet, domain DomainGet, err error) {
	domain, err = c.GetDomain(zone)
	if err != nil {
		return nil, domain, fmt.Errorf("getting domain for zone=%q: %w", zone, err)
	}
	return c.GetRecordsByDomain(domain)
}

func (c PPHClient) CreateRecord(domain DomainGet, record Record, replace bool) (createdRecord *Record, err error) {
	resp, err := c.client.ClientDomainsAPI.ClientDomainsDomainIdDnsRecordCreatePost(c.clientContext, domain.ID).XToken(
		c.token,
	).ClientDomainsDomainIdDnsRecordCreatePostRequest(
		pphclient.ClientDomainsDomainIdDnsRecordCreatePostRequest{
			Record: &pphclient.ClientDomainsDomainIdDnsRecordCreatePostRequestRecord{
				// Record.ID is ignored on creation.
				Name:     &record.Name,
				Content:  &record.Content,
				Type:     &record.Type,
				Ttl:      pphclient.PtrFloat32(float32(record.TTL)),
				Priority: pphclient.PtrFloat32(float32(record.Prio)),
				Replace:  &replace,
			},
		},
	).Execute()
	if err != nil {
		return createdRecord, fmt.Errorf("creating record=%q in zone=%q: %w", record.Name, domain.Domain, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}
	// this is ugly but the API does not know about statusCodes
	errorMessage := APIError{}
	err = json.Unmarshal(body, &errorMessage)
	if err == nil && errorMessage.Error {
		if !errorMessage.Error {
			return createdRecord, nil
		}
		return createdRecord, fmt.Errorf("creating record=%q in zone=%q: %v", record.Name, domain.Domain, errorMessage)
	}
	createRecord := RecordCreate{}
	err = json.Unmarshal(body, &createRecord)
	if err != nil {
		return createdRecord, fmt.Errorf("unmarshalling RecordCreate: %w", err)
	}
	return &createRecord.Data.RecordCreate.Record, nil
}

func (c PPHClient) DeleteRecord(domain DomainGet, record Record) error {
	resp, err := c.client.ClientDomainsAPI.ClientDomainsDomainIdDnsRecordDeletePost(c.clientContext, domain.ID).XToken(
		c.token,
	).ClientDomainsDomainIdDnsRecordDeletePostRequest(
		pphclient.ClientDomainsDomainIdDnsRecordDeletePostRequest{
			Record: pphclient.ClientDomainsDomainIdDnsRecordDeletePostRequestRecord{
				Id: pphclient.PtrInt32(int32(record.ID)),
			},
		}).Execute()
	if err != nil {
		return fmt.Errorf("deleting record=%q in zone=%q: %w", record.Name, domain.Domain, err)
	}
	defer resp.Body.Close()
	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading body: %w", err)
	}
	/*json.Unmarshal(body)
	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		return fmt.Errorf("calling delete endpoint got status %q: %s", resp.Status, resp.)
	}*/
	return nil
}
