package lookup

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func HP(serial string) (*WarrantyInfo, error) {
	url := fmt.Sprintf("https://warrantyapi.hp.com/warrantyinfo/GetWarrantyInfo?sn=%s", serial)
	client := &http.Client{Timeout: 8 * time.Second}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hp warranty API: HTTP %d", resp.StatusCode)
	}

	var result struct {
		WarrantyEndDate string `json:"warrantyEndDate"`
		ProductName     string `json:"productName"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &WarrantyInfo{
		Vendor: "HP",
		Expiry: result.WarrantyEndDate,
		Model:  result.ProductName,
	}, nil
}
