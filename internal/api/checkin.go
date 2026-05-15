package api

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

type Rule struct {
	RuleID      int    `json:"ruleId"`
	RuleName    string `json:"ruleName"`
	StartTime   string `json:"startTime"`
	EndTime     string `json:"endTime"`
	Description string `json:"description"`
}

type Status struct {
	CanCheckin       bool            `json:"canCheckin"`
	HasCheckedIn     *bool           `json:"hasCheckedIn"`
	IsExempt         *bool           `json:"isExempt"`
	IsBoarding       bool            `json:"isBoarding"`
	Message          string          `json:"message"`
	ExemptReason     *string         `json:"exemptReason"`
	MinutesRemaining *int            `json:"minutesRemaining"`
	CurrentRule      *Rule           `json:"currentRule"`
	TodayRecord      json.RawMessage `json:"todayRecord"`
}

// AvailableRules lists rules currently applicable to the authenticated user.
func (c *Client) AvailableRules(ctx context.Context) ([]Rule, error) {
	var rs []Rule
	if err := c.do(ctx, "GET", "/checkin/available-rules", nil, nil, &rs); err != nil {
		return nil, err
	}
	return rs, nil
}

// CheckinStatus returns the current checkin status for the given rule.
func (c *Client) CheckinStatus(ctx context.Context, ruleID int) (*Status, error) {
	q := url.Values{"ruleId": []string{strconv.Itoa(ruleID)}}
	var s Status
	if err := c.do(ctx, "GET", "/checkin/status", q, nil, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// SignRequest mirrors the body sent by the frontend on POST /checkin.
// Address fields use omitempty: when blank they are omitted from the request body,
// matching the "minimal payload" mode an admin can choose per dorm.
type SignRequest struct {
	RuleID          int     `json:"ruleId"`
	Latitude        float64 `json:"latitude"`
	Longitude       float64 `json:"longitude"`
	DeviceModel     string  `json:"deviceModel,omitempty"`
	DeviceSystem    string  `json:"deviceSystem,omitempty"`
	LocationAddress string  `json:"locationAddress,omitempty"`
	City            string  `json:"city,omitempty"`
	Road            string  `json:"road,omitempty"`
	Poi             string  `json:"poi,omitempty"`
}

// Sign performs a checkin. Returns the raw `data` payload on success.
func (c *Client) Sign(ctx context.Context, req SignRequest) (json.RawMessage, error) {
	var data json.RawMessage
	if err := c.do(ctx, "POST", "/checkin", nil, req, &data); err != nil {
		return nil, err
	}
	return data, nil
}
