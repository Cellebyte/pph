package client

import "time"

type PPHProviderData struct {
	ID int `json:"id,omitempty"`
}

type DomainGet struct {
	ID              int       `json:"id"`
	Domain          string    `json:"domain"`
	DomainIdn       string    `json:"domain_idn"`
	Firstamount     float64   `json:"firstamount"`
	Recurringamount float64   `json:"recurringamount"`
	Status          string    `json:"status"`
	RegisterDate    time.Time `json:"register_date"`
	NextDueDate     time.Time `json:"next_due_date"`
	NextDueIn       int       `json:"next_due_in"`
	NextDueHuman    string    `json:"next_due_human"`
}
type Record struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Prio    uint16 `json:"priority,omitempty"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
}

type RecordGet struct {
	Record      `json:",inline"`
	FullName    string `json:"full_name"`
	LastChanged any    `json:"last_changed"`
	Ret         string `json:"ret"`
	Editable    bool   `json:"editable"`
}
type DomainRecordsGet struct {
	Domain  string      `json:"domain"`
	Records []RecordGet `json:"records"`
}

type APIError struct {
	Error   bool   `json:"error"`
	Type    string `json:"type"`
	URL     string `json:"url"`
	Message string `json:"message"`
	Input   struct {
		Record struct {
			Content string `json:"content"`
			Name    string `json:"name"`
			Replace bool   `json:"replace"`
			TTL     int64  `json:"ttl"`
			Type    string `json:"type"`
		} `json:"record"`
	} `json:"input"`
}
type RecordCreate struct {
	Data struct {
		Removed      int `json:"removed"`
		RecordCreate struct {
			Record       `json:",inline"`
			DomainID     int `json:"domain_id"`
			ChangeDate   int `json:"change_date"`
			Ordername    any `json:"ordername"`
			Auth         any `json:"auth"`
			Disabled     int `json:"disabled"`
			DomainCreate struct {
				ID             int    `json:"id"`
				Name           string `json:"name"`
				Master         string `json:"master"`
				LastCheck      any    `json:"last_check"`
				Type           string `json:"type"`
				NotifiedSerial int    `json:"notified_serial"`
				Account        string `json:"account"`
				VirtualizorUID any    `json:"virtualizor_uid"`
				Options        string `json:"options"`
				Catalog        int    `json:"catalog"`
			} `json:"domain"`
		} `json:"record"`
	} `json:"data"`
	Success bool `json:"success"`
}

type RecordDelete struct {
	Record `json:",inline"`
}
