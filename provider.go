// Package libdnsregery implements a DNS record management client compatible
// with the libdns interfaces for Regery.
package libdnsregery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/libdns/libdns"
)

type DNSRecord struct {
	Address string `json:"address"`
	Type    string `json:"type"`
	TTL     int    `json:"ttl"`
	Name    string `json:"name"`
}

type APIResponse struct {
	Records []DNSRecord `json:"records"`
}

// Provider facilitates DNS record manipulation with Regery.
type Provider struct {
	APIToken string `json:"api_token,omitempty"`
	Secret   string `json:"secret"`
}

const baseUrl = "https://api.regery.com/v1/domains"

// GetRecords lists all the records in the zone.
func (p *Provider) GetRecords(ctx context.Context, zone string) ([]libdns.Record, error) {
	url := fmt.Sprintf("%s/%s/records", baseUrl, zone)

	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", fmt.Sprintf("%s:%s", p.APIToken, p.Secret))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		contents, _ := io.ReadAll(resp.Body)
		log.Fatalf("Received non-200 response: %d %s", resp.StatusCode, contents)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response body: %v", err)
		return nil, err
	}

	var result APIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		log.Fatalf("Failed to parse JSON: %v", err)
		return nil, err
	}

	var records []libdns.Record
	for _, record := range result.Records {
		records = append(records, libdns.Record{
			ID:    record.Name,
			TTL:   time.Duration(record.TTL) * time.Second,
			Type:  record.Type,
			Name:  record.Name,
			Value: record.Address,
		})
	}
	return records, nil
}

func (p *Provider) AppendRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	url := fmt.Sprintf("%s/%s/records", baseUrl, zone)

	var regeryRecords []DNSRecord
	for _, r := range records {
		regeryRecords = append(regeryRecords, DNSRecord{
			Address: r.Value,
			Type:    r.Type,
			TTL:     int(r.TTL.Seconds()),
			Name:    r.Name,
		})
	}

	request, err := json.Marshal(APIResponse{regeryRecords})
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(request))
	req.Header.Add("Authorization", fmt.Sprintf("%s:%s", p.APIToken, p.Secret))
	req.Header.Add("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		contents, _ := io.ReadAll(resp.Body)
		log.Fatalf("Received non-200 response: %d\n%s", resp.StatusCode, contents)
	}

	return records, nil
}

// SetRecords sets the records in the zone, either by updating existing records or creating new ones.
// It returns the updated records.
func (p *Provider) SetRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	var err error

	existingRecords, err := p.GetRecords(ctx, zone)
	if err != nil {
		return nil, err
	}

	var toDelete []libdns.Record
	for _, r := range existingRecords {
		for _, newRecord := range records {
			if newRecord.Name == r.Name {
				toDelete = append(toDelete, r)
			}
		}
	}

	appendedRecords, err := p.AppendRecords(ctx, zone, records)
	if err != nil {
		return nil, err
	}

	_, err = p.DeleteRecords(ctx, zone, toDelete)
	if err != nil {
		log.Printf("Failed to delete records that were overwritten, %s", err)
	}

	return appendedRecords, nil
}

// DeleteRecords deletes the records from the zone. It returns the records that were deleted.
func (p *Provider) DeleteRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	url := fmt.Sprintf("%s/%s/records", baseUrl, zone)

	var regeryRecords []DNSRecord
	for _, r := range records {
		regeryRecords = append(regeryRecords, DNSRecord{
			Address: r.Value,
			Type:    r.Type,
			TTL:     int(r.TTL.Seconds()),
			Name:    r.Name,
		})
	}

	request, err := json.Marshal(APIResponse{regeryRecords})
	req, err := http.NewRequest("DELETE", url, bytes.NewBuffer(request))
	req.Header.Add("Authorization", fmt.Sprintf("%s:%s", p.APIToken, p.Secret))
	req.Header.Add("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		contents, _ := io.ReadAll(resp.Body)
		log.Fatalf("Received non-200 response: %d\n%s", resp.StatusCode, contents)
	}

	return records, nil
}

// Interface guards
var (
	_ libdns.RecordGetter   = (*Provider)(nil)
	_ libdns.RecordAppender = (*Provider)(nil)
	_ libdns.RecordSetter   = (*Provider)(nil)
	_ libdns.RecordDeleter  = (*Provider)(nil)
)
