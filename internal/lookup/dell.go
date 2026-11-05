package lookup

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type WarrantyInfo struct {
	Vendor  string
	Expiry  string
	Model   string
}

func Dell(serial string) (*WarrantyInfo, error) {
	url := fmt.Sprintf("https://apigtwb2c.us.dell.com/PROD/sbil/eapi/v5/asset-entitlements?servicetags=%s&us_customer_id=", serial)
	client := &http.Client{Timeout: 8 * time.Second}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("dell API: HTTP %d", resp.StatusCode)
	}

	var result []struct {
		ServiceTag  string `json:"serviceTag"`
		ProductLine string `json:"productLineDescription"`
		Entitlements []struct {
			EndDate string `json:"endDate"`
		} `json:"entitlements"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no results for serial %s", serial)
	}

	info := &WarrantyInfo{Vendor: "Dell", Model: result[0].ProductLine}
	latest := ""
	for _, e := range result[0].Entitlements {
		if e.EndDate > latest {
			latest = e.EndDate
		}
	}
	info.Expiry = latest
	return info, nil
}
