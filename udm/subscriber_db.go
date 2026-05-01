package main

// SubscriberProfile represents a 5G subscriber's data (AM Data).
type SubscriberProfile struct {
	SUPI              string   `json:"supi"`
	MSISDN            string   `json:"msisdn"`
	SubscriberName    string   `json:"subscriberName"`
	PLMN              string   `json:"servingPlmn"`
	AccessType        string   `json:"accessType"`
	DefaultDNN        string   `json:"defaultDnn"`
	AllowedDNNs       []string `json:"allowedDnns"`
	AMBRUL            string   `json:"subscribedUeAmbrUl"`
	AMBRDL            string   `json:"subscribedUeAmbrDl"`
	NSSAI             []SNSSAI `json:"subscribedNssai"`
	AuthMethod        string   `json:"authMethod"`
	PermanentKey      string   `json:"permanentKey"`
	OPc               string   `json:"opc"`
	SequenceNumber    string   `json:"sqn"`
	BillingPlan       string   `json:"billingPlan"`
	MonthlyDataCapGB  int      `json:"monthlyDataCapGb"`
	LocationAreaCodes []string `json:"locationAreaCodes"`
}

// SNSSAI represents a Single Network Slice Selection Assistance Information.
type SNSSAI struct {
	SST int    `json:"sst"`
	SD  string `json:"sd"`
}

// SubscriberDB is an in-memory subscriber database.
type SubscriberDB struct {
	subscribers map[string]*SubscriberProfile
}

// NewSubscriberDB creates a pre-populated subscriber database.
func NewSubscriberDB() *SubscriberDB {
	db := &SubscriberDB{
		subscribers: make(map[string]*SubscriberProfile),
	}

	// Populate with fake subscribers
	db.subscribers["imsi-001010000000001"] = &SubscriberProfile{
		SUPI:           "imsi-001010000000001",
		MSISDN:         "+1-555-0101",
		SubscriberName: "Alice Johnson",
		PLMN:           "00101",
		AccessType:     "3GPP_ACCESS",
		DefaultDNN:     "internet",
		AllowedDNNs:    []string{"internet", "ims", "enterprise"},
		AMBRUL:         "500 Mbps",
		AMBRDL:         "1 Gbps",
		NSSAI: []SNSSAI{
			{SST: 1, SD: "000001"},
			{SST: 2, SD: "000002"},
		},
		AuthMethod:        "5G_AKA",
		PermanentKey:      "465B5CE8B199B49FAA5F0A2EE238A6BC",
		OPc:               "E8ED289DEBA952E4283B54E88E6183CA",
		SequenceNumber:    "000000000020",
		BillingPlan:       "PREMIUM_UNLIMITED",
		MonthlyDataCapGB:  -1,
		LocationAreaCodes: []string{"LAC-001", "LAC-002", "LAC-003"},
	}

	db.subscribers["imsi-001010000000002"] = &SubscriberProfile{
		SUPI:           "imsi-001010000000002",
		MSISDN:         "+1-555-0102",
		SubscriberName: "Bob Smith",
		PLMN:           "00101",
		AccessType:     "3GPP_ACCESS",
		DefaultDNN:     "internet",
		AllowedDNNs:    []string{"internet", "ims"},
		AMBRUL:         "100 Mbps",
		AMBRDL:         "200 Mbps",
		NSSAI: []SNSSAI{
			{SST: 1, SD: "000001"},
		},
		AuthMethod:        "5G_AKA",
		PermanentKey:      "0C0A34601D4F07677303652C0462535B",
		OPc:               "63BFA50EE6523365FF14C1F45F88737D",
		SequenceNumber:    "000000000041",
		BillingPlan:       "STANDARD_50GB",
		MonthlyDataCapGB:  50,
		LocationAreaCodes: []string{"LAC-004", "LAC-005"},
	}

	db.subscribers["imsi-001010000000003"] = &SubscriberProfile{
		SUPI:           "imsi-001010000000003",
		MSISDN:         "+1-555-0103",
		SubscriberName: "Charlie Davis",
		PLMN:           "00101",
		AccessType:     "3GPP_ACCESS",
		DefaultDNN:     "enterprise",
		AllowedDNNs:    []string{"internet", "ims", "enterprise", "v2x"},
		AMBRUL:         "1 Gbps",
		AMBRDL:         "2 Gbps",
		NSSAI: []SNSSAI{
			{SST: 1, SD: "000001"},
			{SST: 3, SD: "000003"},
		},
		AuthMethod:        "5G_AKA",
		PermanentKey:      "A3B2C1D4E5F60718293A4B5C6D7E8F90",
		OPc:               "1A2B3C4D5E6F708192A3B4C5D6E7F801",
		SequenceNumber:    "000000000062",
		BillingPlan:       "ENTERPRISE_VIP",
		MonthlyDataCapGB:  -1,
		LocationAreaCodes: []string{"LAC-001", "LAC-006", "LAC-007"},
	}

	db.subscribers["imsi-001010000000004"] = &SubscriberProfile{
		SUPI:           "imsi-001010000000004",
		MSISDN:         "+1-555-0104",
		SubscriberName: "Diana Martinez",
		PLMN:           "00101",
		AccessType:     "3GPP_ACCESS",
		DefaultDNN:     "internet",
		AllowedDNNs:    []string{"internet"},
		AMBRUL:         "50 Mbps",
		AMBRDL:         "100 Mbps",
		NSSAI: []SNSSAI{
			{SST: 1, SD: "000001"},
		},
		AuthMethod:        "5G_AKA",
		PermanentKey:      "F1E2D3C4B5A69078162534A7B8C9D0E1",
		OPc:               "98765432ABCDEF0011223344AABBCCDD",
		SequenceNumber:    "000000000010",
		BillingPlan:       "PREPAID_BASIC",
		MonthlyDataCapGB:  10,
		LocationAreaCodes: []string{"LAC-008"},
	}

	db.subscribers["imsi-001010000000005"] = &SubscriberProfile{
		SUPI:           "imsi-001010000000005",
		MSISDN:         "+1-555-0105",
		SubscriberName: "Edward Wilson",
		PLMN:           "00101",
		AccessType:     "3GPP_ACCESS",
		DefaultDNN:     "internet",
		AllowedDNNs:    []string{"internet", "ims", "enterprise"},
		AMBRUL:         "250 Mbps",
		AMBRDL:         "500 Mbps",
		NSSAI: []SNSSAI{
			{SST: 1, SD: "000001"},
			{SST: 2, SD: "000002"},
		},
		AuthMethod:        "5G_AKA",
		PermanentKey:      "AABBCCDD11223344EEFF5566778899AA",
		OPc:               "DEADBEEFCAFE1234567890ABCDEF0123",
		SequenceNumber:    "000000000033",
		BillingPlan:       "FAMILY_SHARED",
		MonthlyDataCapGB:  100,
		LocationAreaCodes: []string{"LAC-002", "LAC-009"},
	}

	return db
}

// GetSubscriber retrieves a subscriber profile by SUPI.
func (db *SubscriberDB) GetSubscriber(supi string) (*SubscriberProfile, bool) {
	sub, ok := db.subscribers[supi]
	return sub, ok
}

// ListSUPIs returns all known SUPIs.
func (db *SubscriberDB) ListSUPIs() []string {
	supis := make([]string, 0, len(db.subscribers))
	for k := range db.subscribers {
		supis = append(supis, k)
	}
	return supis
}
